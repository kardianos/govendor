// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rewrite

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Context struct {
	GopathList []string
	Goroot     string

	RootDir        string
	RootGopath     string
	RootImportPath string

	VendorFile *VendorFile

	// Package is a map where the import path is the key.
	// Populated with LoadPackage.
	Package map[string]*Package

	parserFileSet  *token.FileSet
	packageUnknown map[string]struct{}
	fileImports    map[string]map[string]*File // ImportPath -> []file paths.

	vendorFileLocal map[string]*VendorPackage // Vendor file "Local" field lookup for packages.
}

func NewContextWD() (*Context, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := findRoot(wd)
	if err != nil {
		return nil, err
	}

	vf, err := readVendorFile(root)
	if err != nil {
		return nil, err
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

		VendorFile: vf,

		Package: make(map[string]*Package),

		parserFileSet:   token.NewFileSet(),
		packageUnknown:  make(map[string]struct{}),
		vendorFileLocal: make(map[string]*VendorPackage, len(vf.Package)),
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
	if importPath == "builtin" || importPath == "unsafe" {
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
		if fileHasPrefix(dir, gopath) {
			importPath = fileTrimPrefix(dir, gopath)
			importPath = slashToImportPath(importPath)
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

func (ctx *Context) addFileImports(path, gopath string) error {
	if strings.HasSuffix(path, ".go") == false {
		return nil
	}
	f, err := parser.ParseFile(ctx.parserFileSet, path, nil, parser.ImportsOnly)
	if err != nil {
		return err
	}

	dir, _ := filepath.Split(path)
	importPath := fileTrimPrefix(dir, gopath)
	importPath = slashToImportPath(importPath)
	importPath = strings.TrimPrefix(importPath, "/")
	importPath = strings.TrimSuffix(importPath, "/")

	pkg, found := ctx.Package[importPath]
	if !found {
		pkg = &Package{
			Dir:        dir,
			ImportPath: importPath,
			VendorPath: importPath,
			Gopath:     gopath,
		}
		ctx.Package[importPath] = pkg

		if _, found := ctx.packageUnknown[importPath]; found {
			delete(ctx.packageUnknown, importPath)
		}
	}
	pf := &File{
		Package: pkg,
		Path:    path,
		Imports: make([]string, len(f.Imports)),
	}
	pkg.Files = append(pkg.Files, pf)
	for i := range f.Imports {
		imp := f.Imports[i].Path.Value
		imp, err = strconv.Unquote(imp)
		if err != nil {
			return err
		}
		pf.Imports[i] = imp

		if _, found := ctx.Package[imp]; !found {
			ctx.packageUnknown[imp] = struct{}{}
		}
	}

	return nil
}

func (ctx *Context) AddImports(importPath ...string) error {
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
	for len(ctx.packageUnknown) > 0 {
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
					continue top
				}
				return err
			}
			if fileStringEquals(gopath, ctx.Goroot) {
				ctx.Package[importPath] = &Package{
					Dir:        dir,
					ImportPath: importPath,
					VendorPath: importPath,
					Status:     StatusStd,
					Gopath:     ctx.Goroot,
				}
				delete(ctx.packageUnknown, importPath)
				continue top
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
			continue top
		}
	}

	// Determine the status of remaining imports.
	for _, pkg := range ctx.Package {
		if pkg.Status != StatusUnknown {
			continue
		}
		if vp, found := ctx.vendorFileLocal[pkg.ImportPath]; found {
			pkg.Status = StatusInternal
			pkg.VendorPath = vp.Vendor
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
		root, err := findRoot(pkg.Dir)
		if err != nil {
			// No vendor file found.
			if err == ErrMissingVendorFile {
				continue
			}
			return err
		}
		vf, err := readVendorFile(root)
		if err != nil {
			return err
		}
		for _, vp := range vf.Package {
			if vp.Local == pkg.ImportPath {
				// Return the vendor path the vendor package used.
				pkg.VendorPath = vp.Vendor
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
