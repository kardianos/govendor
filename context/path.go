// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
	"github.com/kardianos/govendor/vendorfile"
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
			if err != nil {
				continue
			}
			if fi.IsDir() == false {
				continue
			}
			for _, gopath = range ctx.GopathList {
				if pathos.FileHasPrefix(look, gopath) {
					hasGo, err := hasGoFileInFolder(look)
					if err != nil {
						return "", "", err
					}
					if hasGo {
						return look, gopath, nil
					}
				}
			}
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

		hasGo, err := hasGoFileInFolder(dir)
		if err != nil {
			return "", "", err
		}
		if hasGo {
			return dir, gopath, nil
		}
		return "", "", ErrNotInGOPATH{fmt.Sprintf("Import: %q relative: %q", importPath, relative)}
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
			return "", ErrMissingVendorFile{vendorPath}
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

	dir, gopath, err := ctx.findImportDir("", importPath)
	if err != nil {
		return "", err
	}
	root, err := findRoot(dir, vendorFilename)
	if err != nil {
		// No vendor file found. Return origional.
		if _, is := err.(ErrMissingVendorFile); is {
			return importPath, nil
		}
		return "", err
	}
	vf, err := readVendorFile(filepath.Join(root, vendorFilename))
	if err != nil {
		return "", err
	}
	vpkg := vendorFileFindLocal(vf, root, gopath, importPath)
	if vpkg != nil {
		return vpkg.Canonical, nil
	}

	// Vendor file exists, but the package is not a vendor package.
	return importPath, nil
}

func vendorFileFindLocal(vf *vendorfile.File, root, gopath, importPath string) *vendorfile.Package {
	local := pathos.SlashToImportPath(pathos.FileTrimPrefix(root, gopath)) // "/co1" = /file/src/co1, /file/src
	local = strings.TrimPrefix(strings.TrimPrefix(importPath, local), "/")
	for _, pkg := range vf.Package {
		if pkg.Remove {
			continue
		}
		if pkg.Local == local {
			return pkg
		}
	}
	return nil
}

func hasGoFileInFolder(folder string) (bool, error) {
	dir, err := os.Open(folder)
	if err != nil {
		if os.IsNotExist(err) {
			// No folder present, no need to check for files.
			return false, nil
		}
		return false, err
	}
	fl, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		return false, err
	}
	for _, fi := range fl {
		if fi.IsDir() == false && filepath.Ext(fi.Name()) == ".go" {
			return true, nil
		}
	}
	return false, nil
}

// RemovePackage removes the specified folder files. If folder is empty when
// done (no nested folders, remove the folder and any empty parent folders.
func RemovePackage(path string) error {
	// Ensure the path is empty of files.
	dir, err := os.Open(path)
	if err != nil {
		return err
	}

	fl, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		return err
	}
	for _, fi := range fl {
		if fi.IsDir() {
			continue
		}
		err = os.Remove(filepath.Join(path, fi.Name()))
		if err != nil {
			return err
		}
	}

	// Ignore errors here.
	for i := 0; i <= looplimit; i++ {
		dir, err := os.Open(path)
		if err != nil {
			// fmt.Fprintf(os.Stderr, "Failedd to open directory %q: %v\n", path, err)
			return nil
		}

		fl, err := dir.Readdir(1)
		dir.Close()
		if err != nil && err != io.EOF {
			// fmt.Fprintf(os.Stderr, "Failedd to list directory %q: %v\n", path, err)
			return nil
		}
		if len(fl) > 0 {
			return nil
		}
		err = os.Remove(path)
		if err != nil {
			// fmt.Fprintf(os.Stderr, "Failedd to remove empty directory %q: %v\n", path, err)
			return nil
		}
		nextPath := filepath.Clean(filepath.Join(path, ".."))
		// Check for root.
		if nextPath == path {
			return nil
		}
		path = nextPath
	}
	panic("removePackage() remove parent folders")
}
