// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
)

// loadPackage sets up the context with package information and
// is called before any initial operation is performed.
func (ctx *Context) loadPackage() error {
	ctx.loaded = true
	ctx.Package = make(map[string]*Package, len(ctx.Package))
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
		return ctx.addFileImports(path, ctx.RootGopath)
	})
	if err != nil {
		return err
	}
	return ctx.determinePackageStatus()
}

// addFileImports is called from loadPackage and resolveUnknown.
func (ctx *Context) addFileImports(pathname, gopath string) error {
	dir, _ := filepath.Split(pathname)
	importPath := pathos.FileTrimPrefix(dir, gopath)
	importPath = pathos.SlashToImportPath(importPath)
	importPath = strings.TrimPrefix(importPath, "/")
	importPath = strings.TrimSuffix(importPath, "/")

	if strings.HasSuffix(pathname, ".go") == false {
		return nil
	}
	f, err := parser.ParseFile(token.NewFileSet(), pathname, nil, parser.ImportsOnly)
	if err != nil {
		return err
	}

	pkg, found := ctx.Package[importPath]
	if !found {
		status := StatusUnknown
		if f.Name.Name == "main" {
			status = StatusProgram
		}
		pkg = ctx.setPackage(dir, importPath, importPath, gopath, status)
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
		err = ctx.addSingleImport(pkg.Dir, imp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ctx *Context) setPackage(dir, canonical, local, gopath string, status ListStatus) *Package {
	at := 0
	switch {
	case strings.Contains(canonical, "/vendor/"):
		at = strings.LastIndex(canonical, "/vendor/") + len("/vendor/")
	case strings.HasPrefix(canonical, "vendor/"):
		at = strings.LastIndex(canonical, "vendor/") + len("vendor/")
	}
	if at > 0 {
		canonical = canonical[at:]
		if status == StatusUnknown {
			status = StatusVendor
		}
	}
	pkg := &Package{
		Dir:       dir,
		Canonical: canonical,
		Local:     local,
		Gopath:    gopath,
		Status:    status,
	}
	ctx.Package[local] = pkg
	return pkg
}

func (ctx *Context) addSingleImport(pkgInDir, imp string) error {
	if _, found := ctx.Package[imp]; !found {
		dir, gopath, err := ctx.findImportDir(pkgInDir, imp)
		if err != nil {
			if _, is := err.(ErrNotInGOPATH); is {
				ctx.setPackage("", imp, imp, "", StatusMissing)
				return nil
			}
			return err
		}
		if pathos.FileStringEquals(gopath, ctx.Goroot) {
			ctx.setPackage(dir, imp, imp, ctx.Goroot, StatusStandard)
			return nil
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
			switch fi.Name()[0] {
			case '.', '_':
				continue
			}
			path := filepath.Join(dir, fi.Name())
			err = ctx.addFileImports(path, gopath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (ctx *Context) determinePackageStatus() error {
	// Determine the status of remaining imports.
	for _, pkg := range ctx.Package {
		if pkg.Status != StatusUnknown {
			continue
		}
		if vp := ctx.vendorFilePackageLocal(pkg.Local); vp != nil {
			pkg.Status = StatusVendor
			pkg.Canonical = vp.Canonical
			continue
		}
		if strings.HasPrefix(pkg.Canonical, ctx.RootImportPath) {
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
			if _, is := err.(ErrMissingVendorFile); is {
				continue
			}
			return err
		}
		vf, err := readVendorFile(filepath.Join(root, vendorFilename))
		if err != nil {
			return err
		}
		vpkg := vendorFileFindLocal(vf, root, pkg.Gopath, pkg.Local)
		if vpkg != nil {
			pkg.Canonical = vpkg.Canonical
		}
	}

	// Determine any un-used internal vendor imports.
	ctx.updatePackageReferences()
	for i := 0; i <= looplimit; i++ {
		altered := false
		for path, pkg := range ctx.Package {
			if len(pkg.referenced) == 0 && pkg.Status == StatusVendor {
				altered = true
				pkg.Status = StatusUnused
				for _, other := range ctx.Package {
					delete(other.referenced, path)
				}
			}
		}
		if !altered {
			break
		}
		if i == looplimit {
			panic("determinePackageStatus loop limit")
		}
	}
	return nil
}
