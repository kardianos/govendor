// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kardianos/vendor/internal/pathos"
)

// findImportDir finds the absolute directory. If rel is empty vendor folders
// are not looked in.
func (ctx *Context) findImportDir(relative, importPath string) (dir, gopath string, err error) {
	if importPath == "builtin" || importPath == "unsafe" || importPath == "C" {
		return filepath.Join(ctx.Goroot, importPath), ctx.Goroot, nil
	}
	if len(relative) != 0 {
		rel := relative
		for {
			look := filepath.Join(rel, "vendor", importPath)
			nextRel := filepath.Join(rel, "..")
			if rel == nextRel {
				break
			}
			rel = nextRel
			fi, err := os.Stat(look)
			if os.IsNotExist(err) {
				continue
			}
			if fi.IsDir() == false {
				continue
			}
			for _, gopath = range ctx.GopathList {
				if pathos.FileHasPrefix(look, gopath) {
					return look, gopath, nil
				}
			}
			return "", "", ErrNotInGOPATH{fmt.Sprintf("Import: %q relative: %q", importPath, relative)}
		}

	}
	for _, gopath = range ctx.GopathList {
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
	panic("findRoot loop limit")
}

// findCanonicalPath determines the correct local import path (from GOPATH)
// and from any nested internal vendor files. It returns a string relative to
// the root "internal" folder.
func (ctx *Context) findCanonicalPath(importPath string) (string, error) {
	// "crypto/tls" -> "path/to/mypkg/internal/crypto/tls"
	// "yours/internal/yourpkg" -> "path/to/mypkg/internal/yourpkg" (IIF yourpkg is a vendor package)
	// "yours/internal/myinternal" -> "path/to/mypkg/internal/yours/internal/myinternal" (IIF myinternal is not a vendor package)
	// "github.com/kardianos/osext" -> "patn/to/mypkg/internal/github.com/kardianos/osext"

	dir, _, err := ctx.findImportDir("", importPath)
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
