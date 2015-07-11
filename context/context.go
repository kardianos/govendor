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
	// TODO: Rethink which of these should be public. Most actions should be methods.

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

	parserFileSet  *token.FileSet
	packageUnknown map[string]struct{}
	fileImports    map[string]map[string]*File // ImportPath -> []file paths.

	vendorFileLocal map[string]*vendorfile.Package // Vendor file "Local" field lookup for packages.
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
		goEnv, err := cmd.CombinedOutput()
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

		VendorFile:     vf,
		VendorFilePath: vendorFilePath,
		VendorFolder:   vendorFolder,

		Package: make(map[string]*Package),

		parserFileSet:   token.NewFileSet(),
		packageUnknown:  make(map[string]struct{}),
		vendorFileLocal: make(map[string]*vendorfile.Package, len(vf.Package)),
		fileImports:     make(map[string]map[string]*File),
	}

	ctx.RootImportPath, ctx.RootGopath, err = ctx.findImportPath(root)
	if err != nil {
		return nil, err
	}

	for _, pkg := range ctx.VendorFile.Package {
		ctx.vendorFileLocal[pkg.Local] = pkg
	}

	return ctx, nil
}

// findImportDir finds the absolute directory. If gopath is not empty, it is used.
func (ctx *Context) findImportDir(importPath, useGopath string) (dir, gopath string, err error) {
	paths := ctx.GopathList
	if len(useGopath) != 0 {
		paths = []string{useGopath}
	}
	if importPath == "builtin" || importPath == "unsafe" || importPath == "C" {
		return filepath.Join(ctx.Goroot, importPath), ctx.Goroot, nil
	}
	for _, gopath = range paths {
		dir := filepath.Join(gopath, importPath)
		fi, err := os.Stat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if fi.IsDir() == false {
			continue
		}
		return dir, gopath, nil
	}
	return "", "", ErrNotInGOPATH{importPath}
}

// findImportPath takes a absolute directory and returns the import path and go path.
func (ctx *Context) findImportPath(dir string) (importPath, gopath string, err error) {
	for _, gopath := range ctx.GopathList {
		if pathos.FileHasPrefix(dir, gopath) {
			importPath = pathos.FileTrimPrefix(dir, gopath)
			importPath = pathos.SlashToImportPath(importPath)
			return importPath, gopath, nil
		}
	}
	return "", "", ErrNotInGOPATH{dir}
}

type Package struct {
	Dir        string
	ImportPath string
	VendorPath string
	Gopath     string
	Files      []*File
	Status     ListStatus

	referenced map[string]*Package
}
type File struct {
	Package *Package
	Path    string
	Imports []string
}

func (ctx *Context) LoadPackage(alsoImportPath ...string) error {
	err := filepath.Walk(ctx.RootDir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return err
		}
		if info.IsDir() && info.Name()[0] == '.' {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		return ctx.addFileImports(path, ctx.RootGopath)
	})
	if err != nil {
		return err
	}
	for _, pkg := range ctx.Package {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				if _, found := ctx.Package[imp]; !found {
					ctx.packageUnknown[imp] = struct{}{}
				}
			}
		}
	}
	for _, path := range alsoImportPath {
		if _, found := ctx.Package[path]; !found {
			ctx.packageUnknown[path] = struct{}{}
		}
	}
	return ctx.resolveUnknown()
}

// AddImport adds the package to the context. The vendorFolder is where the
// package should be added to relative to the project root.
// TODO: support adding multiple imports in same call.
func (ctx *Context) AddImport(importPath string) error {
	importPath = pathos.SlashToImportPath(importPath)

	err := ctx.LoadPackage(importPath)
	if err != nil {
		return err
	}

	localImportPath, err := findLocalImportPath(ctx, importPath)
	if err != nil {
		return err
	}
	// Adjust relative local path to GOPATH import path.
	localImportPath = path.Join(ctx.RootImportPath, ctx.VendorFolder, localImportPath)

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

	err = ctx.addImports(importPath)
	if err != nil {
		return err
	}

	pkg, foundPkg := ctx.Package[importPath]
	if !foundPkg {
		return ErrNotInGOPATH{importPath}
	}
	if pkg.Status != StatusExternal {
		if pkg.Status == StatusInternal {
			return ErrVendorExists
		}
		if pkg.Status == StatusLocal {
			return ErrLocalPackage
		}
		return ErrNotInGOPATH{importPath}
	}

	// Update vendor file with correct Local field.
	var vp *vendorfile.Package
	for _, vpkg := range ctx.VendorFile.Package {
		if vpkg.Canonical == importPath {
			vp = vpkg
			break
		}
	}
	if vp == nil {
		vp = &vendorfile.Package{
			Add:       true,
			Canonical: importPath,
			Local:     localImportPath,
		}
		ctx.VendorFile.Package = append(ctx.VendorFile.Package, vp)
		ctx.vendorFileLocal[vp.Local] = vp
	}
	if !localCopyExists {
		// Find the VCS information.
		vcs, err := vcs.FindVcs(pkg.Gopath, pkg.Dir)
		if err != nil {
			return err
		}
		if vcs != nil {
			if vcs.Dirty {
				return ErrDirtyPackage{pkg.ImportPath}
			}
			vp.Revision = vcs.Revision
			if vcs.RevisionTime != nil {
				vp.RevisionTime = vcs.RevisionTime.Format(time.RFC3339)
			}
		}

		// Copy the package locally.
		// TODO: do not copy locally, just mark as needing a copy.
		/*err = CopyPackage(filepath.Join(ctx.RootGopath, pathos.SlashToFilepath(localImportPath)), pkg.Dir)
		if err != nil {
			return err
		}*/
	}
	return nil
}

func (ctx *Context) addFileImports(pathname, gopath string) error {
	dir, _ := filepath.Split(pathname)
	importPath := pathos.FileTrimPrefix(dir, gopath)
	importPath = pathos.SlashToImportPath(importPath)
	importPath = strings.TrimPrefix(importPath, "/")
	importPath = strings.TrimSuffix(importPath, "/")

	delete(ctx.packageUnknown, importPath)

	if strings.HasSuffix(pathname, ".go") == false {
		return nil
	}
	f, err := parser.ParseFile(ctx.parserFileSet, pathname, nil, parser.ImportsOnly)
	if err != nil {
		return err
	}

	pkg, found := ctx.Package[importPath]
	if !found {
		pkg = &Package{
			Dir:        dir,
			ImportPath: importPath,
			VendorPath: importPath,
			Gopath:     gopath,
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
			ctx.packageUnknown[imp] = struct{}{}
		}
	}

	return nil
}

func (ctx *Context) addImports(importPath ...string) error {
	for _, imp := range importPath {
		if _, found := ctx.Package[imp]; found {
			continue
		}
		ctx.packageUnknown[imp] = struct{}{}
	}
	return ctx.resolveUnknown()
}

func (ctx *Context) resolveUnknown() error {
top:
	for importPath := range ctx.packageUnknown {
		dir, gopath, err := ctx.findImportDir(importPath, "")
		if err != nil {
			if _, ok := err.(ErrNotInGOPATH); ok {
				ctx.Package[importPath] = &Package{
					Dir:        "",
					ImportPath: importPath,
					VendorPath: importPath,
					Status:     StatusMissing,
				}
				delete(ctx.packageUnknown, importPath)
				goto top
			}
			return err
		}
		if pathos.FileStringEquals(gopath, ctx.Goroot) {
			ctx.Package[importPath] = &Package{
				Dir:        dir,
				ImportPath: importPath,
				VendorPath: importPath,
				Status:     StatusStd,
				Gopath:     ctx.Goroot,
			}
			delete(ctx.packageUnknown, importPath)
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
			err = ctx.addFileImports(path, gopath)
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
		if vp, found := ctx.vendorFileLocal[pkg.ImportPath]; found {
			pkg.Status = StatusInternal
			pkg.VendorPath = vp.Canonical
			continue
		}
		if strings.HasPrefix(pkg.ImportPath, ctx.RootImportPath) {
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
			if vp.Local == pkg.ImportPath {
				// Return the vendor path the vendor package used.
				pkg.VendorPath = vp.Canonical
				break
			}
		}
	}

	// Determine any un-used internal vendor imports.
	// 1. populate the references.
	for _, pkg := range ctx.Package {
		pkg.referenced = make(map[string]*Package, len(pkg.referenced))
	}
	for _, pkg := range ctx.Package {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				if other, found := ctx.Package[imp]; found {
					other.referenced[pkg.ImportPath] = pkg
				}
			}
		}
	}

	// 2. Mark as unused and remove all. Loop until it stops marking more
	// as unused.
	for i := 0; i <= looplimit; i++ {
		altered := false
		for _, pkg := range ctx.Package {
			if len(pkg.referenced) == 0 && pkg.Status == StatusInternal {
				altered = true
				pkg.Status = StatusUnused
				for _, other := range ctx.Package {
					delete(other.referenced, pkg.ImportPath)
				}
			}
		}
		if !altered {
			break
		}
		if i == looplimit {
			return ErrLoopLimit{"resolveUnknown() Mark Unused"}
		}
	}

	// Add files to import map.
	for _, pkg := range ctx.Package {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				fileList := ctx.fileImports[imp]
				if fileList == nil {
					fileList = make(map[string]*File, 1)
					ctx.fileImports[imp] = fileList
				}
				fileList[f.Path] = f
			}
		}
	}
	return nil
}
