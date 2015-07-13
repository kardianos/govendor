// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package context gathers the status of packages and stores it in Context.
// A new Context needs to be pointed to the root of the project and any
// project owned vendor file.
package context

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kardianos/vendor/internal/pathos"
	"github.com/kardianos/vendor/vcs"
	"github.com/kardianos/vendor/vendorfile"
)

const (
	vendorFilename = "vendor.json"

	looplimit = 10000
)

// Context represents the current project context.
type Context struct {
	Rewrite bool

	GopathList []string
	Goroot     string

	RootDir        string
	RootGopath     string
	RootImportPath string

	VendorFile     *vendorfile.File
	VendorFilePath string
	VendorFolder   string // Store vendor packages in this folder.

	// Package is a map where the import path is the key.
	// Populated with LoadPackage.
	Package map[string]*Package

	loaded bool
}

// Package maintains information pertaining to a package.
type Package struct {
	Dir           string
	CanonicalPath string
	LocalPath     string
	SourcePath    string
	Gopath        string
	Files         []*File
	Status        ListStatus

	// used in resolveUnknown function. Not persisted.
	referenced map[string]*Package
}

// File holds a reference to the imports in a file and the file locaiton.
type File struct {
	Package *Package
	Path    string
	Imports []string
}

// NewContextWD creates a new context. It looks for a root folder by finding
// a vendor file.
func NewContextWD(vendorFolder string) (*Context, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := findRoot(wd, vendorFilename)
	if err != nil {
		return nil, err
	}
	return NewContext(root, vendorFilename, vendorFolder)
}

// NewContext creates new context from a given root folder and vendor file path.
// The vendorFolder is where vendor packages should be placed.
func NewContext(root, vendorFilePathRel, vendorFolder string) (*Context, error) {
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

	ctx := &Context{
		RootDir:    root,
		GopathList: gopathGoroot,
		Goroot:     goroot,

		Rewrite: true,

		VendorFile:     vf,
		VendorFilePath: vendorFilePath,
		VendorFolder:   vendorFolder,

		Package: make(map[string]*Package),
	}

	ctx.RootImportPath, ctx.RootGopath, err = ctx.findImportPath(root)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}

func (ctx *Context) vendorFilePackageLocal(local string) *vendorfile.Package {
	for _, pkg := range ctx.VendorFile.Package {
		if pkg.Remove {
			continue
		}
		if pkg.Local == local {
			return pkg
		}
	}
	return nil
}

func (ctx *Context) vendorFilePackageCanonical(canonical string) *vendorfile.Package {
	for _, pkg := range ctx.VendorFile.Package {
		if pkg.Remove {
			continue
		}
		if pkg.Canonical == canonical {
			return pkg
		}
	}
	return nil
}

// LoadPackage sets up the context with package information.
func (ctx *Context) loadPackage(alsoImportPath ...string) error {
	ctx.loaded = true
	packageUnknown := make(map[string]struct{}, 30)
	err := filepath.Walk(ctx.RootDir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return err
		}
		name := info.Name()
		if info.IsDir() && (name[0] == '.' || name[0] == '_' || name == "testdata") {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		return ctx.addFileImports(path, ctx.RootGopath, packageUnknown)
	})
	if err != nil {
		return err
	}
	for _, pkg := range ctx.Package {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				if _, found := ctx.Package[imp]; !found {
					packageUnknown[imp] = struct{}{}
				}
			}
		}
	}
	for _, path := range alsoImportPath {
		if _, found := ctx.Package[path]; !found {
			packageUnknown[path] = struct{}{}
		}
	}
	return ctx.resolveUnknown(packageUnknown)
}

// updatePackageReferences populates the referenced field in each Package.
func (ctx *Context) updatePackageReferences() {
	for _, pkg := range ctx.Package {
		pkg.referenced = make(map[string]*Package, len(pkg.referenced))
	}
	for _, pkg := range ctx.Package {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				if other, found := ctx.Package[imp]; found {
					other.referenced[pkg.CanonicalPath] = pkg
				}
			}
		}
	}
}

// AddImport adds the package to the context. The vendorFolder is where the
// package should be added to relative to the project root.
func (ctx *Context) AddImport(sourcePath string) error {
	var err error
	if !ctx.loaded {
		err = ctx.loadPackage()
		if err != nil {
			return err
		}
	}
	sourcePath = pathos.SlashToImportPath(sourcePath)

	canonicalImportPath, err := findLocalImportPath(ctx, sourcePath)
	if err != nil {
		return err
	}

	err = ctx.resolveUnknownList(sourcePath, canonicalImportPath)
	if err != nil {
		return err
	}

	// Adjust relative local path to GOPATH import path.
	localImportPath := path.Join(ctx.RootImportPath, ctx.VendorFolder, canonicalImportPath)

	localCopyExists := false
	// TODO: set localCopyExists flag
	// TODO: ensure the import path is the Canonical path.

	/*importPath, err = verify(ctx, importPath, localImportPath)
	if err != nil {
		if err == ErrFilesExists {
			localCopyExists = true
		} else {
			return err
		}
	}*/

	pkg, foundPkg := ctx.Package[sourcePath]
	if !foundPkg {
		return ErrNotInGOPATH{sourcePath}
	}
	if pkg.Status != StatusExternal {
		if pkg.Status == StatusInternal {
			return ErrVendorExists
		}
		if pkg.Status == StatusLocal {
			return ErrLocalPackage
		}
		return ErrNotInGOPATH{sourcePath}
	}

	// Update vendor file with correct Local field.
	vp := ctx.vendorFilePackageCanonical(canonicalImportPath)
	if vp == nil {
		vp = &vendorfile.Package{
			Add:       true,
			Canonical: canonicalImportPath,
			Local:     localImportPath,
			Source:    sourcePath,
		}
		ctx.VendorFile.Package = append(ctx.VendorFile.Package, vp)
	}
	if !localCopyExists {
		// Find the VCS information.
		system, err := vcs.FindVcs(pkg.Gopath, pkg.Dir)
		if err != nil {
			return err
		}
		if system != nil {
			if system.Dirty {
				return ErrDirtyPackage{pkg.CanonicalPath}
			}
			vp.Revision = system.Revision
			if system.RevisionTime != nil {
				vp.RevisionTime = system.RevisionTime.Format(time.RFC3339)
			}
		}

		// Copy the package locally.
		err = CopyPackage(filepath.Join(ctx.RootGopath, pathos.SlashToFilepath(localImportPath)), pkg.Dir)
		if err != nil {
			return err
		}
	}

	if ctx.Rewrite {
		err = RewriteFiles(ctx, localImportPath)
		if err != nil {
			return err
		}
	}

	// Remove unused external packages from listing.
	ctx.updatePackageReferences()
top:
	for i := 0; i <= looplimit; i++ {
		altered := false
		for path, pkg := range ctx.Package {
			if len(pkg.referenced) == 0 && pkg.Status == StatusExternal {
				altered = true
				delete(ctx.Package, path)
				for _, other := range ctx.Package {
					delete(other.referenced, path)
				}
				continue top
			}
		}
		if !altered {
			break
		}
		if i == looplimit {
			return errLoopLimit{"resolveUnknown() Mark Unused"}
		}
	}
	return nil
}

func (ctx *Context) addFileImports(pathname, gopath string, packageUnknown map[string]struct{}) error {
	dir, _ := filepath.Split(pathname)
	importPath := pathos.FileTrimPrefix(dir, gopath)
	importPath = pathos.SlashToImportPath(importPath)
	importPath = strings.TrimPrefix(importPath, "/")
	importPath = strings.TrimSuffix(importPath, "/")

	delete(packageUnknown, importPath)

	if strings.HasSuffix(pathname, ".go") == false {
		return nil
	}
	f, err := parser.ParseFile(token.NewFileSet(), pathname, nil, parser.ImportsOnly)
	if err != nil {
		return err
	}

	pkg, found := ctx.Package[importPath]
	if !found {
		pkg = &Package{
			Dir:           dir,
			CanonicalPath: importPath,
			LocalPath:     importPath,
			Gopath:        gopath,
		}
		ctx.Package[importPath] = pkg
	}
	pf := &File{
		Package: pkg,
		Path:    pathname,
		Imports: make([]string, len(f.Imports)),
	}
	pkg.Files = append(pkg.Files, pf)
	for i := range f.Imports {
		imp := f.Imports[i].Path.Value
		imp, err = strconv.Unquote(imp)
		if err != nil {
			return err
		}
		if strings.HasPrefix(imp, "./") {
			imp = path.Join(importPath, imp)
		}
		pf.Imports[i] = imp

		if _, found := ctx.Package[imp]; !found {
			packageUnknown[imp] = struct{}{}
		}
	}

	return nil
}

func (ctx *Context) resolveUnknownList(packages ...string) error {
	packageUnknown := make(map[string]struct{}, len(packages))
	for _, name := range packages {
		packageUnknown[name] = struct{}{}
	}
	return ctx.resolveUnknown(packageUnknown)
}

func (ctx *Context) resolveUnknown(packageUnknown map[string]struct{}) error {
top:
	for importPath := range packageUnknown {
		dir, gopath, err := ctx.findImportDir(importPath, "")
		if err != nil {
			if _, ok := err.(ErrNotInGOPATH); ok {
				ctx.Package[importPath] = &Package{
					Dir:           "",
					CanonicalPath: importPath,
					LocalPath:     importPath,
					Status:        StatusMissing,
				}
				delete(packageUnknown, importPath)
				goto top
			}
			return err
		}
		if pathos.FileStringEquals(gopath, ctx.Goroot) {
			ctx.Package[importPath] = &Package{
				Dir:           dir,
				CanonicalPath: importPath,
				LocalPath:     importPath,
				Status:        StatusStd,
				Gopath:        ctx.Goroot,
			}
			delete(packageUnknown, importPath)
			goto top
		}
		df, err := os.Open(dir)
		if err != nil {
			return err
		}
		info, err := df.Readdir(-1)
		df.Close()
		if err != nil {
			return err
		}
		for _, fi := range info {
			if fi.IsDir() {
				continue
			}
			if fi.Name()[0] == '.' {
				continue
			}
			path := filepath.Join(dir, fi.Name())
			err = ctx.addFileImports(path, gopath, packageUnknown)
			if err != nil {
				return err
			}
		}
		goto top
	}

	// Determine the status of remaining imports.
	for _, pkg := range ctx.Package {
		if pkg.Status != StatusUnknown {
			continue
		}
		if vp := ctx.vendorFilePackageLocal(pkg.CanonicalPath); vp != nil {
			pkg.Status = StatusInternal
			pkg.LocalPath = vp.Canonical
			continue
		}
		if strings.HasPrefix(pkg.CanonicalPath, ctx.RootImportPath) {
			pkg.Status = StatusLocal
			continue
		}
		pkg.Status = StatusExternal
	}

	// Check all "external" packages for vendor.
	for _, pkg := range ctx.Package {
		if pkg.Status != StatusExternal {
			continue
		}
		root, err := findRoot(pkg.Dir, vendorFilename)
		if err != nil {
			// No vendor file found.
			if err == ErrMissingVendorFile {
				continue
			}
			return err
		}
		vf, err := readVendorFile(filepath.Join(root, vendorFilename))
		if err != nil {
			return err
		}
		for _, vp := range vf.Package {
			if vp.Local == pkg.CanonicalPath {
				// Return the vendor path the vendor package used.
				pkg.LocalPath = vp.Canonical
				break
			}
		}
	}

	// Determine any un-used internal vendor imports.
	ctx.updatePackageReferences()
	for i := 0; i <= looplimit; i++ {
		altered := false
		for _, pkg := range ctx.Package {
			if len(pkg.referenced) == 0 && pkg.Status == StatusInternal {
				altered = true
				pkg.Status = StatusUnused
				for _, other := range ctx.Package {
					delete(other.referenced, pkg.CanonicalPath)
				}
			}
		}
		if !altered {
			break
		}
		if i == looplimit {
			return errLoopLimit{"resolveUnknown() Mark Unused"}
		}
	}
	return nil
}
