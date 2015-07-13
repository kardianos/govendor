// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"os"
	"path/filepath"

	"github.com/kardianos/vendor/internal/pathos"
)

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

func findRoot(folder, vendorPath string) (root string, err error) {
	for i := 0; i <= looplimit; i++ {
		test := filepath.Join(folder, vendorPath)
		_, err := os.Stat(test)
		if os.IsNotExist(err) == false {
			return folder, nil
		}
		nextFolder := filepath.Clean(filepath.Join(folder, ".."))

		// Check for root folder.
		if nextFolder == folder {
			return "", ErrMissingVendorFile
		}
		folder = nextFolder
	}
	return "", errLoopLimit{"findRoot()"}
}

// findLocalImportPath determines the correct local import path (from GOPATH)
// and from any nested internal vendor files. It returns a string relative to
// the root "internal" folder.
func findLocalImportPath(ctx *Context, importPath string) (string, error) {
	// "crypto/tls" -> "path/to/mypkg/internal/crypto/tls"
	// "yours/internal/yourpkg" -> "path/to/mypkg/internal/yourpkg" (IIF yourpkg is a vendor package)
	// "yours/internal/myinternal" -> "path/to/mypkg/internal/yours/internal/myinternal" (IIF myinternal is not a vendor package)
	// "github.com/kardianos/osext" -> "patn/to/mypkg/internal/github.com/kardianos/osext"

	dir, _, err := ctx.findImportDir(importPath, "")
	if err != nil {
		return "", err
	}
	root, err := findRoot(dir, vendorFilename)
	if err != nil {
		// No vendor file found. Return origional.
		if err == ErrMissingVendorFile {
			return importPath, nil
		}
		return "", err
	}
	vf, err := readVendorFile(filepath.Join(root, vendorFilename))
	if err != nil {
		return "", err
	}
	for _, pkg := range vf.Package {
		if pkg.Local == importPath {
			// Return the vendor path the vendor package used.
			return pkg.Canonical, nil
		}
	}
	// Vendor file exists, but the package is not a vendor package.
	return importPath, nil
}
