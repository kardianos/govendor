// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package rewrite contains commands for writing the altered import statements.
package rewrite

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/kardianos/govendor/vendorfile"
)

// ListStatus indicates the status of the import.
type ListStatus byte

func (ls ListStatus) String() string {
	switch ls {
	case StatusUnknown:
		return "?"
	case StatusMissing:
		return "m"
	case StatusStd:
		return "s"
	case StatusLocal:
		return "l"
	case StatusExternal:
		return "e"
	case StatusInternal:
		return "i"
	case StatusUnused:
		return "u"
	}
	return ""
}

const (
	// StatusUnknown indicates the status was unable to be obtained.
	StatusUnknown ListStatus = iota
	// StatusMissing indicates import not found in GOROOT or GOPATH.
	StatusMissing
	// StatusStd indicates import found in GOROOT.
	StatusStd
	// StatusLocal indicates import is part of the local project.
	StatusLocal
	// StatusExternal indicates import is found in GOPATH and not copied.
	StatusExternal
	// StatusInternal indicates import has been copied locally under internal.
	StatusInternal
	// StatusUnused indicates import has been copied, but is no longer used.
	StatusUnused
)

// ListItem represents a package in the current project.
type ListItem struct {
	Status     ListStatus
	Path       string
	VendorPath string
}

func (li ListItem) String() string {
	return li.Status.String() + " " + li.Path
}

type listItemSort []ListItem

func (li listItemSort) Len() int      { return len(li) }
func (li listItemSort) Swap(i, j int) { li[i], li[j] = li[j], li[i] }
func (li listItemSort) Less(i, j int) bool {
	if li[i].Status == li[j].Status {
		return li[i].Path < li[j].Path
	}
	return li[i].Status > li[j].Status
}

const (
	vendorFilename = "vendor.json"
	internalFolder = "internal"

	looplimit = 10000
)

var (
	internalVendor = filepath.Join(internalFolder, vendorFilename)
)

var (
	ErrVendorFileExists  = errors.New(internalVendor + " file already exists.")
	ErrMissingVendorFile = errors.New("Unable to find internal folder with vendor file.")
	ErrMissingGOROOT     = errors.New("Unable to determine GOROOT.")
	ErrMissingGOPATH     = errors.New("Missing GOPATH.")
	ErrVendorExists      = errors.New("Package already exists as a vendor package.")
	ErrLocalPackage      = errors.New("Cannot vendor a local package.")
	ErrImportExists      = errors.New("Import exists. To update use update command.")
	ErrImportNotExists   = errors.New("Import does not exist.")
	ErrNoLocalPath       = errors.New("Import is present in vendor file, but is missing local path.")
	ErrFilesExists       = errors.New("Files exists at destination of internal vendor path.")
)

type ErrNotInGOPATH struct {
	Missing string
}

func (err ErrNotInGOPATH) Error() string {
	return fmt.Sprintf("Package %q not a go package or not in GOPATH.", err.Missing)
}

type ErrDirtyPackage struct {
	ImportPath string
}

func (err ErrDirtyPackage) Error() string {
	return fmt.Sprintf("Package %q has uncommited changes in the vcs.", err.ImportPath)
}

type ErrLoopLimit struct {
	Loop string
}

func (err ErrLoopLimit) Error() string {
	return fmt.Sprintf("BUG: Loop limit of %d was reached for loop %s. Please report this bug.", looplimit, err.Loop)
}

func CmdInit() error {
	/*
		1. Determine if CWD contains "internal/vendor.json".
		2. If exists, return error.
		3. Create directory if it doesn't exist.
		4. Create "internal/vendor.json" file.
	*/
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	_, err = os.Stat(filepath.Join(wd, internalVendor))
	if os.IsNotExist(err) == false {
		return ErrVendorFileExists
	}
	err = os.MkdirAll(filepath.Join(wd, internalFolder), 0777)
	if err != nil {
		return err
	}
	vf := &vendorfile.File{}
	return writeVendorFile(wd, vf)
}

func CmdList() ([]ListItem, error) {
	/*
		1. Find vendor root.
		2. Find vendor root import path via GOPATH.
		3. Walk directory, find all directories with go files.
		4. Parse imports for all go files.
		5. Determine the status of all imports.
		  * Std
		  * Local
		  * External Vendor
		  * Internal Vendor
		  * Unused Vendor
		6. Return Vendor import paths.
	*/
	ctx, err := NewContextWD()
	if err != nil {
		return nil, err
	}

	err = ctx.LoadPackage()
	if err != nil {
		return nil, err
	}

	list := make([]ListItem, 0, len(ctx.Package))
	for _, pkg := range ctx.Package {
		li := ListItem{
			Status:     pkg.Status,
			Path:       pkg.ImportPath,
			VendorPath: pkg.VendorPath,
		}
		if vp, found := ctx.vendorFileLocal[pkg.ImportPath]; found {
			li.VendorPath = vp.Canonical
		}
		list = append(list, li)
	}
	// Sort li by Status, then Path.
	sort.Sort(listItemSort(list))

	return list, nil
}

/*
	Add, Update, and Remove will start with the same steps as List.
	Rather then returning the results, it will find any affected files,
	alter their imports, then write the files back out. Also copy or remove
	files and folders as needed.
*/

func CmdAdd(importPath string) error {
	return addUpdateImportPath(importPath, verifyAdd)
}

func CmdUpdate(importPath string) error {
	return addUpdateImportPath(importPath, verifyUpdate)
}

func verifyAdd(ctx *Context, importPath, local string) (string, error) {
	for _, pkg := range ctx.VendorFile.Package {
		if pkg.Canonical == importPath {
			return importPath, ErrImportExists
		}
	}
	// Check fo existing internal folders present.
	dirPath := filepath.Join(ctx.RootGopath, local)
	dir, err := os.Open(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No folder present, no need to check for files.
			return importPath, nil
		}
		return importPath, err
	}
	fl, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		return importPath, err
	}
	for _, fi := range fl {
		if fi.IsDir() == false {
			return importPath, ErrFilesExists
		}
	}
	return importPath, nil
}
func verifyUpdate(ctx *Context, importPath, local string) (string, error) {
	for _, pkg := range ctx.VendorFile.Package {
		if pkg.Canonical == importPath {
			return importPath, nil
		}
	}
	for _, pkg := range ctx.VendorFile.Package {
		if pkg.Local == importPath {
			return pkg.Canonical, nil
		}
	}
	return importPath, ErrImportNotExists
}

func addUpdateImportPath(importPath string, verify func(ctx *Context, importPath, local string) (string, error)) error {
	importPath = slashToImportPath(importPath)
	ctx, err := NewContextWD()
	if err != nil {
		return err
	}

	err = ctx.LoadPackage(importPath)
	if err != nil {
		return err
	}

	localImportPath, err := findLocalImportPath(ctx, importPath)
	if err != nil {
		return err
	}
	// Adjust relative local path to GOPATH import path.
	localImportPath = path.Join(ctx.RootImportPath, internalFolder, localImportPath)

	localCopyExists := false
	importPath, err = verify(ctx, importPath, localImportPath)
	if err != nil {
		if err == ErrFilesExists {
			localCopyExists = true
		} else {
			return err
		}
	}

	err = ctx.AddImports(importPath)
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
		vcs, err := FindVcs(pkg.Gopath, pkg.Dir)
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

		// Write the vendor file.
		err = writeVendorFile(ctx.RootDir, ctx.VendorFile)
		if err != nil {
			return err
		}

		// Copy the package locally.
		err = CopyPackage(filepath.Join(ctx.RootGopath, slashToFilepath(localImportPath)), pkg.Dir)
		if err != nil {
			return err
		}
	}

	err = ctx.AddImports(importPath, localImportPath)
	if err != nil {
		return err
	}

	// Determine which files to touch.
	files := make(map[string]*File, len(ctx.VendorFile.Package)*3)

	// Rules are all lines in the vendor file.
	rules := make([]Rule, 0, len(ctx.VendorFile.Package))
	for _, vp := range ctx.VendorFile.Package {
		for fpath, f := range ctx.fileImports[vp.Canonical] {
			files[fpath] = f
		}
		rules = append(rules, Rule{From: vp.Canonical, To: vp.Local})
	}
	// Add local package files.
	if localPkg, found := ctx.Package[localImportPath]; found {
		for _, f := range localPkg.Files {
			files[f.Path] = f
		}
	}
	// Rewrite any external package where the local path is different then the vendor path.
	for _, pkg := range ctx.Package {
		if pkg.Status != StatusExternal {
			continue
		}
		if pkg.ImportPath == pkg.VendorPath {
			continue
		}
		for _, otherPkg := range ctx.Package {
			if pkg == otherPkg {
				continue
			}
			if otherPkg.Status != StatusInternal {
				continue
			}
			if otherPkg.VendorPath != pkg.VendorPath {
				continue
			}

			for fpath, f := range ctx.fileImports[pkg.ImportPath] {
				files[fpath] = f
			}
			rules = append(rules, Rule{From: pkg.ImportPath, To: otherPkg.ImportPath})
			break
		}
	}

	return ctx.RewriteFiles(files, rules)
}
func CmdRemove(importPath string) error {
	importPath = slashToImportPath(importPath)
	ctx, err := NewContextWD()
	if err != nil {
		return err
	}

	err = ctx.LoadPackage(importPath)
	if err != nil {
		return err
	}

	localPath := ""
	vendorPath := importPath
	localFound := false
	vendorFileIndex := 0
	for i, pkg := range ctx.VendorFile.Package {
		if pkg.Canonical == importPath {
			localPath = pkg.Local
			localFound = true
			vendorFileIndex = i
			break
		}
	}
	if !localFound {
		for i, pkg := range ctx.VendorFile.Package {
			if pkg.Local == importPath {
				localPath = pkg.Local
				vendorPath = pkg.Canonical
				localFound = true
				vendorFileIndex = i
				break
			}
		}
	}
	if !localFound {
		return ErrImportNotExists
	}
	if localPath == "" {
		return ErrNoLocalPath
	}

	files := ctx.fileImports[localPath]

	err = ctx.RewriteFiles(files, []Rule{{From: localPath, To: vendorPath}})
	if err != nil {
		return err
	}

	err = RemovePackage(filepath.Join(ctx.RootGopath, slashToFilepath(localPath)))
	if err != nil {
		return err
	}
	for i, pkg := range ctx.VendorFile.Package {
		if i == vendorFileIndex {
			pkg.Remove = true
		}
	}

	return writeVendorFile(ctx.RootDir, ctx.VendorFile)
}
