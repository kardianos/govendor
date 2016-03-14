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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
	os "github.com/kardianos/govendor/internal/vos"
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
	Logger io.Writer // Write to the verbose log.

	GopathList []string // List of GOPATHs in environment. Includes "src" dir.
	Goroot     string   // The path to the standard library.

	RootDir        string // Full path to the project root.
	RootGopath     string // The GOPATH the project is in.
	RootImportPath string // The import path to the project.

	VendorFile         *vendorfile.File
	VendorFilePath     string // File path to vendor file.
	VendorFolder       string // Store vendor packages in this folder.
	VendorFileToFolder string // The relative path from the vendor file to the vendor folder.
	RootToVendorFile   string // The relative path from the project root to the vendor file directory.

	VendorDiscoverFolder string // Normally auto-set to "vendor"

	// Package is a map where the import path is the key.
	// Populated with LoadPackage.
	Package map[string]*Package
	// Change to unkown structure (rename). Maybe...

	// MoveRule provides the translation from origional import path to new import path.
	RewriteRule map[string]string // map[from]to

	Operation []*Operation

	loaded, dirty  bool
	rewriteImports bool

	ignoreTag []string // list of tags to ignore
}

// Package maintains information pertaining to a package.
type Package struct {
	OriginDir string // Origin directory
	Dir       string // Physical directory path of the package.
	Origin    string // Origin path for remote
	Canonical string
	Local     string
	Gopath    string // Inlcudes trailing "src".
	Files     []*File
	Status    Status
	Tree      bool // Package is a tree of folder.
	inVendor  bool // Different then Status.Location, this is in *any* vendor tree.
	inTree    bool

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
)

// NewContextWD creates a new context. It looks for a root folder by finding
// a vendor file.
func NewContextWD(rt RootType) (*Context, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	pathToVendorFile := filepath.Join("vendor", vendorFilename)
	rootIndicator := "vendor"
	vendorFolder := "vendor"

	root := wd
	if rt == RootVendor || rt == RootVendorOrWD {
		tryRoot, err := findRoot(wd, rootIndicator)
		switch rt {
		case RootVendor:
			if err != nil {
				return nil, err
			}
			root = tryRoot
		case RootVendorOrWD:
			if err == nil {
				root = tryRoot
			}
		}
	}

	// Check for old vendor file location.
	oldLocation := filepath.Join(root, vendorFilename)
	if _, err := os.Stat(oldLocation); err == nil {
		return nil, ErrOldVersion{`Use the "migrate" command to update.`}
	}

	return NewContext(root, pathToVendorFile, vendorFolder, false)
}

// NewContext creates new context from a given root folder and vendor file path.
// The vendorFolder is where vendor packages should be placed.
func NewContext(root, vendorFilePathRel, vendorFolder string, rewriteImports bool) (*Context, error) {
	dprintf("CTX: %s\n", root)
	vendorFilePath := filepath.Join(root, vendorFilePathRel)
	vf, err := readVendorFile(vendorFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		vf = &vendorfile.File{}
	}

	// Get GOROOT. First check ENV, then run "go env" and find the GOROOT line.
	goroot := os.Getenv("GOROOT")
	if len(goroot) == 0 {
		// If GOROOT is not set, get from go cmd.
		cmd := exec.Command("go", "env")
		var goEnv []byte
		goEnv, err = cmd.CombinedOutput()
		if err != nil {
			return nil, err
		}
		const gorootLookFor = `GOROOT=`
		for _, line := range strings.Split(string(goEnv), "\n") {
			if strings.HasPrefix(line, gorootLookFor) == false {
				continue
			}
			goroot = strings.TrimPrefix(line, gorootLookFor)
			goroot, err = strconv.Unquote(goroot)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if goroot == "" {
		return nil, ErrMissingGOROOT
	}
	goroot = filepath.Join(goroot, "src")

	// Get the GOPATHs. Prepend the GOROOT to the list.
	all := os.Getenv("GOPATH")
	if len(all) == 0 {
		return nil, ErrMissingGOPATH
	}
	gopathList := filepath.SplitList(all)
	gopathGoroot := make([]string, 0, len(gopathList)+1)
	gopathGoroot = append(gopathGoroot, goroot)
	for _, gopath := range gopathList {
		gopathGoroot = append(gopathGoroot, filepath.Join(gopath, "src")+string(filepath.Separator))
	}

	rootToVendorFile, _ := filepath.Split(vendorFilePathRel)

	vendorFileDir, _ := filepath.Split(vendorFilePath)
	vendorFolderRel, err := filepath.Rel(vendorFileDir, filepath.Join(root, vendorFolder))
	if err != nil {
		return nil, err
	}
	vendorFileToFolder := pathos.SlashToImportPath(vendorFolderRel)

	ctx := &Context{
		RootDir:    root,
		GopathList: gopathGoroot,
		Goroot:     goroot,

		VendorFile:         vf,
		VendorFilePath:     vendorFilePath,
		VendorFolder:       vendorFolder,
		VendorFileToFolder: vendorFileToFolder,
		RootToVendorFile:   pathos.SlashToImportPath(rootToVendorFile),

		VendorDiscoverFolder: "vendor",

		Package: make(map[string]*Package),

		RewriteRule: make(map[string]string, 3),

		rewriteImports: rewriteImports,
	}

	ctx.RootImportPath, ctx.RootGopath, err = ctx.findImportPath(root)
	if err != nil {
		return nil, err
	}

	ctx.IgnoreBuild(vf.Ignore)

	return ctx, nil
}

// IgnoreBuild takes a space separated list of tags to ignore.
// "a b c" will ignore "a" OR "b" OR "c".
func (ctx *Context) IgnoreBuild(ignore string) {
	ors := strings.Fields(ignore)
	ctx.ignoreTag = make([]string, 0, len(ors))
	for _, or := range ors {
		if len(or) == 0 {
			continue
		}
		ctx.ignoreTag = append(ctx.ignoreTag, or)
	}
}

// Write to the set io.Writer for logging.
func (ctx *Context) Write(s []byte) (int, error) {
	if ctx.Logger != nil {
		return ctx.Logger.Write(s)
	}
	return len(s), nil
}

// VendorFilePackageLocal finds a given vendor file package give the local import path.
func (ctx *Context) VendorFilePackageLocal(local string) *vendorfile.Package {
	root, _ := filepath.Split(ctx.VendorFilePath)
	return vendorFileFindLocal(ctx.VendorFile, root, ctx.RootGopath, local)
}

// VendorFilePackageCanonical finds a given vendor file package give the import path.
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
func (ctx *Context) findPackageChild(ck *Package) []string {
	canonical := ck.Canonical
	out := make([]string, 0, 3)
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
		if strings.HasPrefix(pkg.Canonical, canonical) {
			out = append(out, pkg.Canonical)
		}
	}
	return out
}

// findPackageParentTree finds any parent tree package that would
// include the given canonical path.
func (ctx *Context) findPackageParentTree(ck *Package) []string {
	canonical := ck.Canonical
	out := make([]string, 0, 1)
	for _, pkg := range ctx.Package {
		if pkg.inVendor == false {
			continue
		}
		if pkg.Tree == false || pkg == ck {
			continue
		}
		// pkg.Canonical = github.com/usera/pkg, tree = true
		// canonical = github.com/usera/pkg/dance
		if strings.HasPrefix(canonical, pkg.Canonical) {
			out = append(out, pkg.Canonical)
		}
	}
	return out
}

// updatePackageReferences populates the referenced field in each Package.
func (ctx *Context) updatePackageReferences() {
	canonicalUnderDirLookup := make(map[string]map[string]*Package)
	findCanonicalUnderDir := func(dir, canonical string) *Package {
		if importMap, found := canonicalUnderDirLookup[dir]; found {
			if pkg, found2 := importMap[canonical]; found2 {
				return pkg
			}
		} else {
			canonicalUnderDirLookup[dir] = make(map[string]*Package)
		}
		for _, pkg := range ctx.Package {
			if !pkg.inVendor {
				continue
			}

			removeFromEnd := len(pkg.Canonical) + len(ctx.VendorDiscoverFolder) + 2
			nextLen := len(pkg.Dir) - removeFromEnd
			if nextLen < 0 {
				continue
			}
			checkDir := pkg.Dir[:nextLen]
			if !pathos.FileHasPrefix(dir, checkDir) {
				continue
			}
			if pkg.Canonical != canonical {
				continue
			}
			canonicalUnderDirLookup[dir][canonical] = pkg
			return pkg
		}
		canonicalUnderDirLookup[dir][canonical] = nil
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
}
