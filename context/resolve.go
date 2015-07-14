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

	"github.com/kardianos/vendor/internal/pathos"
)

// unknown represents resolving a package dependency. Due to the "/vendor/"
// folder discovery, each package must be done relative to a perticular path.
type unknown struct {
	rel string
	pkg string
}

func newUnknownSet() unknownSet {
	return make(map[unknown]struct{}, 3)
}

type unknownSet map[unknown]struct{}

func (s unknownSet) add(rel, pkg string) {
	s[unknown{rel: rel, pkg: pkg}] = struct{}{}
}

func (s unknownSet) del(rel, pkg string) {
	delete(s, unknown{rel: rel, pkg: pkg})
}

func (ctx *Context) addFileImports(pathname, gopath string, packageUnknown unknownSet) error {
	dir, _ := filepath.Split(pathname)
	importPath := pathos.FileTrimPrefix(dir, gopath)
	importPath = pathos.SlashToImportPath(importPath)
	importPath = strings.TrimPrefix(importPath, "/")
	importPath = strings.TrimSuffix(importPath, "/")

	packageUnknown.del("", importPath)

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
			packageUnknown.add("", imp)
		}
	}

	return nil
}

func (ctx *Context) resolveUnknownList(relativeTo string, packages ...string) error {
	packageUnknown := newUnknownSet()
	for _, name := range packages {
		packageUnknown.add("", name)
	}
	return ctx.resolveUnknown(packageUnknown)
}

func (ctx *Context) resolveUnknown(packageUnknown unknownSet) error {
top:
	for importPath := range packageUnknown {
		dir, gopath, err := ctx.findImportDir(importPath.rel, importPath.pkg)
		if err != nil {
			if _, ok := err.(ErrNotInGOPATH); ok {
				ctx.Package[importPath.pkg] = &Package{
					Dir:           "",
					CanonicalPath: importPath.pkg,
					LocalPath:     importPath.pkg,
					Status:        StatusMissing,
				}
				delete(packageUnknown, importPath)
				goto top
			}
			return err
		}
		if pathos.FileStringEquals(gopath, ctx.Goroot) {
			ctx.Package[importPath.pkg] = &Package{
				Dir:           dir,
				CanonicalPath: importPath.pkg,
				LocalPath:     importPath.pkg,
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
