// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package context gathers the status of packages and stores it in Context.
// A new Context needs to be pointed to the root of the project and any
// project owned vendor file.
package context

import (
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
	os "github.com/kardianos/govendor/internal/vos"
	"github.com/kardianos/govendor/pkgspec"
	"github.com/kardianos/govendor/vendorfile"
)

const (
	debug     = false
	looplimit = 10000

	vendorFilename = "vendor.json"
)

func dprintf(f string, v ...interface{}) {
	if debug {
		fmt.Printf(f, v...)
	}
}

// Context represents the current project context.
type Context struct {
	Logger   io.Writer // Write to the verbose log.
	Insecure bool      // Allow insecure network operations

	GopathList []string // List of GOPATHs in environment. Includes "src" dir.
	Goroot     string   // The path to the standard library.

	RootDir        string // Full path to the project root.
	RootGopath     string // The GOPATH the project is in.
	RootImportPath string // The import path to the project.

	VendorFile       *vendorfile.File
	VendorFilePath   string // File path to vendor file.
	VendorFolder     string // Store vendor packages in this folder.
	RootToVendorFile string // The relative path from the project root to the vendor file directory.

	VendorDiscoverFolder string // Normally auto-set to "vendor"

	// Package is a map where the import path is the key.
	// Populated with LoadPackage.
	Package map[string]*Package
	// Change to unknown structure (rename). Maybe...

	// MoveRule provides the translation from original import path to new import path.
	RewriteRule map[string]string // map[from]to

	TreeImport []*pkgspec.Pkg

	Operation []*Operation

	loaded, dirty  bool
	rewriteImports bool

	ignoreTag      []string // list of tags to ignore
	excludePackage []string // list of package prefixes to exclude

	statusCache []StatusItem
	added       map[string]bool
}

// Package maintains information pertaining to a package.
type Package struct {
	OriginDir string // Origin directory
	Dir       string // Physical directory path of the package.

	Status Status // Status and location of the package.
	*pkgspec.Pkg
	Local  string // Current location of a package relative to $GOPATH/src.
	Gopath string // Includes trailing "src".
	Files  []*File

	inVendor bool // Different than Status.Location, this is in *any* vendor tree.
	inTree   bool

	ignoreFile []string

	// used in resolveUnknown function. Not persisted.
	referenced map[string]*Package
}

// File holds a reference to the imports in a file and the file locaiton.
type File struct {
	Package *Package
	Path    string
	Imports []string

	ImportComment string
}

type RootType byte

const (
	RootVendor RootType = iota
	RootWD
	RootVendorOrWD
	RootVendorOrWDOrFirstGOPATH
)

func (pkg *Package) String() string {
	return pkg.Local
}

type packageList []*Package

func (li packageList) Len() int      { return len(li) }
func (li packageList) Swap(i, j int) { li[i], li[j] = li[j], li[i] }
func (li packageList) Less(i, j int) bool {
	if li[i].Path != li[j].Path {
		return li[i].Path < li[j].Path
	}
	return li[i].Local < li[j].Local
}

type Env map[string]string

func NewEnv() (Env, error) {
	env := Env{}

	// If GOROOT is not set, get from go cmd.
	cmd := exec.Command("go", "env")
	var goEnv []byte
	goEnv, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(goEnv), "\n") {
		if k, v, ok := pathos.ParseGoEnvLine(line); ok {
			env[k] = v
		}
	}

	return env, nil
}

// NewContextWD creates a new context. It looks for a root folder by finding
// a vendor file.
func NewContextWD(rt RootType) (*Context, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	rootIndicator := "vendor"

	root := wd
	switch rt {
	case RootVendor:
		tryRoot, err := findRoot(wd, rootIndicator)
		if err != nil {
			return nil, err
		}
		root = tryRoot
	case RootVendorOrWD:
		tryRoot, err := findRoot(wd, rootIndicator)
		if err == nil {
			root = tryRoot
		}
	case RootVendorOrWDOrFirstGOPATH:
		root, err = findRoot(wd, rootIndicator)
		if err != nil {
			env, err := NewEnv()
			if err != nil {
				return nil, err
			}
			allgopath := env["GOPATH"]

			if len(allgopath) == 0 {
				return nil, ErrMissingGOPATH
			}
			gopathList := filepath.SplitList(allgopath)
			root = filepath.Join(gopathList[0], "src")
		}
	}

	// Check for old vendor file location.
	oldLocation := filepath.Join(root, vendorFilename)
	if _, err := os.Stat(oldLocation); err == nil {
		return nil, ErrOldVersion{`Use the "migrate" command to update.`}
	}

	return NewContextRoot(root)
}

// NewContextRoot creates a new context for the given root folder.
func NewContextRoot(root string) (*Context, error) {
	pathToVendorFile := filepath.Join("vendor", vendorFilename)
	vendorFolder := "vendor"

	return NewContext(root, pathToVendorFile, vendorFolder, false)
}

// NewContext creates new context from a given root folder and vendor file path.
// The vendorFolder is where vendor packages should be placed.
func NewContext(root, vendorFilePathRel, vendorFolder string, rewriteImports bool) (*Context, error) {
	dprintf("CTX: %s\n", root)
	var err error

	env, err := NewEnv()
	if err != nil {
		return nil, err
	}
	goroot := env["GOROOT"]
	all := env["GOPATH"]

	if goroot == "" {
		return nil, ErrMissingGOROOT
	}
	goroot = filepath.Join(goroot, "src")

	// Get the GOPATHs. Prepend the GOROOT to the list.
	if len(all) == 0 {
		return nil, ErrMissingGOPATH
	}
	gopathList := filepath.SplitList(all)
	gopathGoroot := make([]string, 0, len(gopathList)+1)
	gopathGoroot = append(gopathGoroot, goroot)
	for _, gopath := range gopathList {
		srcPath := filepath.Join(gopath, "src") + string(filepath.Separator)
		srcPathEvaled, err := filepath.EvalSymlinks(srcPath)
		if err != nil {
			return nil, err
		}
		gopathGoroot = append(gopathGoroot, srcPath, srcPathEvaled+string(filepath.Separator))
	}

	rootToVendorFile, _ := filepath.Split(vendorFilePathRel)

	vendorFilePath := filepath.Join(root, vendorFilePathRel)

	ctx := &Context{
		RootDir:    root,
		GopathList: gopathGoroot,
		Goroot:     goroot,

		VendorFilePath:   vendorFilePath,
		VendorFolder:     vendorFolder,
		RootToVendorFile: pathos.SlashToImportPath(rootToVendorFile),

		VendorDiscoverFolder: "vendor",

		Package: make(map[string]*Package),

		RewriteRule: make(map[string]string, 3),

		rewriteImports: rewriteImports,
	}

	ctx.RootImportPath, ctx.RootGopath, err = ctx.findImportPath(root)
	if err != nil {
		return nil, err
	}

	vf, err := readVendorFile(path.Join(ctx.RootImportPath, vendorFolder)+"/", vendorFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		vf = &vendorfile.File{}
	}
	ctx.VendorFile = vf

	ctx.IgnoreBuildAndPackage(vf.Ignore)

	return ctx, nil
}

// IgnoreBuildAndPackage takes a space separated list of tags or package prefixes
// to ignore.
// Tags are words, packages are folders, containing or ending with a "/".
// "a b c" will ignore tags "a" OR "b" OR "c".
// "p/x q/" will ignore packages "p/x" OR "p/x/y" OR "q" OR "q/z", etc.
func (ctx *Context) IgnoreBuildAndPackage(ignore string) {
	ctx.dirty = true
	ors := strings.Fields(ignore)
	ctx.ignoreTag = make([]string, 0, len(ors))
	ctx.excludePackage = make([]string, 0, len(ors))
	for _, or := range ors {
		if len(or) == 0 {
			continue
		}
		if strings.Index(or, "/") != -1 {
			// package
			ctx.excludePackage = append(ctx.excludePackage, strings.Trim(or, "./"))
		} else {
			// tag
			ctx.ignoreTag = append(ctx.ignoreTag, or)
		}
	}
}

// Write to the set io.Writer for logging.
func (ctx *Context) Write(s []byte) (int, error) {
	if ctx.Logger != nil {
		return ctx.Logger.Write(s)
	}
	return len(s), nil
}

// VendorFilePackagePath finds a given vendor file package give the import path.
func (ctx *Context) VendorFilePackagePath(path string) *vendorfile.Package {
	for _, pkg := range ctx.VendorFile.Package {
		if pkg.Remove {
			continue
		}
		if pkg.Path == path {
			return pkg
		}
	}
	return nil
}

// findPackageChild finds any package under the current package.
// Used for finding tree overlaps.
func (ctx *Context) findPackageChild(ck *Package) []*Package {
	out := make([]*Package, 0, 3)
	for _, pkg := range ctx.Package {
		if pkg == ck {
			continue
		}
		if pkg.inVendor == false {
			continue
		}
		if pkg.Status.Presence == PresenceTree {
			continue
		}
		if strings.HasPrefix(pkg.Path, ck.Path+"/") {
			out = append(out, pkg)
		}
	}
	return out
}

// findPackageParentTree finds any parent tree package that would
// include the given canonical path.
func (ctx *Context) findPackageParentTree(ck *Package) []string {
	out := make([]string, 0, 1)
	for _, pkg := range ctx.Package {
		if pkg.inVendor == false {
			continue
		}
		if pkg.IncludeTree == false || pkg == ck {
			continue
		}
		// pkg.Path = github.com/usera/pkg, tree = true
		// ck.Path = github.com/usera/pkg/dance
		if strings.HasPrefix(ck.Path, pkg.Path+"/") {
			out = append(out, pkg.Local)
		}
	}
	return out
}

// updatePackageReferences populates the referenced field in each Package.
func (ctx *Context) updatePackageReferences() {
	pathUnderDirLookup := make(map[string]map[string]*Package)
	findCanonicalUnderDir := func(dir, path string) *Package {
		if importMap, found := pathUnderDirLookup[dir]; found {
			if pkg, found2 := importMap[path]; found2 {
				return pkg
			}
		} else {
			pathUnderDirLookup[dir] = make(map[string]*Package)
		}
		for _, pkg := range ctx.Package {
			if !pkg.inVendor {
				continue
			}

			removeFromEnd := len(pkg.Path) + len(ctx.VendorDiscoverFolder) + 2
			nextLen := len(pkg.Dir) - removeFromEnd
			if nextLen < 0 {
				continue
			}
			checkDir := pkg.Dir[:nextLen]
			if !pathos.FileHasPrefix(dir, checkDir) {
				continue
			}
			if pkg.Path != path {
				continue
			}
			pathUnderDirLookup[dir][path] = pkg
			return pkg
		}
		pathUnderDirLookup[dir][path] = nil
		return nil
	}
	for _, pkg := range ctx.Package {
		pkg.referenced = make(map[string]*Package, len(pkg.referenced))
	}
	for _, pkg := range ctx.Package {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				if vpkg := findCanonicalUnderDir(pkg.Dir, imp); vpkg != nil {
					vpkg.referenced[pkg.Local] = pkg
					continue
				}
				if other, found := ctx.Package[imp]; found {
					other.referenced[pkg.Local] = pkg
					continue
				}
			}
		}
	}

	// Transfer all references from the child to the top parent.
	for _, pkg := range ctx.Package {
		if parentTrees := ctx.findPackageParentTree(pkg); len(parentTrees) > 0 {
			if parentPkg := ctx.Package[parentTrees[0]]; parentPkg != nil {
				for opath, opkg := range pkg.referenced {
					// Do not transfer internal references.
					if strings.HasPrefix(opkg.Path, parentPkg.Path+"/") {
						continue
					}
					parentPkg.referenced[opath] = opkg
				}
				pkg.referenced = make(map[string]*Package, 0)
			}
		}
	}
}
