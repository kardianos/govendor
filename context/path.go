// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"io"
	"path/filepath"

	"github.com/kardianos/govendor/internal/pathos"
	os "github.com/kardianos/govendor/internal/vos"
)

// Import path is in GOROOT or is a special package.
func (ctx *Context) isStdLib(importPath string) (yes bool, err error) {
	if importPath == "builtin" || importPath == "unsafe" || importPath == "C" {
		yes = true
		return
	}

	dir := filepath.Join(ctx.Goroot, importPath)
	fi, _ := os.Stat(dir)
	if fi == nil {
		return
	}
	if fi.IsDir() == false {
		return
	}

	yes, err = hasGoFileInFolder(dir)
	return
}

// findImportDir finds the absolute directory. If rel is empty vendor folders
// are not looked in.
func (ctx *Context) findImportDir(relative, importPath string) (dir, gopath string, err error) {
	if importPath == "builtin" || importPath == "unsafe" || importPath == "C" {
		return filepath.Join(ctx.Goroot, importPath), ctx.Goroot, nil
	}
	if len(relative) != 0 {
		rel := relative
		for {
			look := filepath.Join(rel, ctx.VendorDiscoverFolder, importPath)
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
		if fi == nil {
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
	dirResolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", "", err
	}
	dirs := make([]string, 1)
	dirs = append(dirs, dir)
	if dir != dirResolved {
		dirs = append(dirs, dirResolved)
	}

	for _, gopath := range ctx.GopathList {
		for _, dir := range dirs {
			if pathos.FileHasPrefix(dir, gopath) || pathos.FileStringEquals(dir, gopath) {
				importPath = pathos.FileTrimPrefix(dir, gopath)
				importPath = pathos.SlashToImportPath(importPath)
				return importPath, gopath, nil
			}
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
func RemovePackage(path, root string, tree bool) error {
	// Ensure the path is empty of files.
	dir, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Remove package files.
	fl, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		return err
	}
	for _, fi := range fl {
		fullPath := filepath.Join(path, fi.Name())
		if fi.IsDir() {
			if tree {
				// If tree == true then remove sub-directories too.
				err = os.RemoveAll(fullPath)
				if err != nil {
					return err
				}
			}
			continue
		}
		err = os.Remove(fullPath)
		if err != nil {
			return err
		}
	}

	// Remove empty parent folders.
	// Ignore errors here.
	for i := 0; i <= looplimit; i++ {
		if pathos.FileStringEquals(path, root) {
			return nil
		}
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
			allAreLicense := true
			for _, fi := range fl {
				if isLicenseFile(fi.Name()) == false {
					allAreLicense = false
					break
				}
			}
			if !allAreLicense {
				return nil
			}
		}
		err = os.RemoveAll(path)
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
