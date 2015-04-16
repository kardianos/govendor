// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// rewrite contains commands for writing the altered import statements.
package rewrite

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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
	StatusUnknown ListStatus = iota
	StatusMissing
	StatusStd
	StatusLocal
	StatusExternal
	StatusInternal
	StatusUnused
)

type ListItem struct {
	Status ListStatus
	Path   string
}

func (li ListItem) String() string {
	return li.Status.String() + " " + li.Path
}

type ListItemSort []ListItem

func (li ListItemSort) Len() int      { return len(li) }
func (li ListItemSort) Swap(i, j int) { li[i], li[j] = li[j], li[i] }
func (li ListItemSort) Less(i, j int) bool {
	if li[i].Status == li[j].Status {
		return strings.Compare(li[i].Path, li[j].Path) < 0
	}
	return li[i].Status > li[j].Status
}

const (
	vendorFilename = "vendor.json"
	internalFolder = "internal"
	toolName       = "github.com/kardianos/vendor"
)

var (
	internalVendor      = filepath.Join(internalFolder, vendorFilename)
	internalFolderSlash = string(filepath.Separator) + internalFolder + string(filepath.Separator)
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
	return fmt.Sprintf("Package %q not in GOPATH.", err.Missing)
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
	vf := &VendorFile{
		Tool: toolName,
	}
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

	li := make([]ListItem, 0, len(ctx.Package))
	for _, pkg := range ctx.Package {
		li = append(li, ListItem{Status: pkg.Status, Path: pkg.ImportPath})
	}
	// Sort li by Status, then Path.
	sort.Sort(ListItemSort(li))

	return li, nil
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
		if pkg.Vendor == importPath {
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
		if pkg.Vendor == importPath {
			return importPath, nil
		}
	}
	for _, pkg := range ctx.VendorFile.Package {
		if pkg.Local == importPath {
			return pkg.Vendor, nil
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

	// Determine correct local import path (from GOPATH).
	/*
		"crypto/tls" -> "path/to/mypkg/internal/crypto/tls"
		"yours/internal/yourpkg" -> "path/to/mypkg/internal/yourpkg"
		"github.com/kardianos/osext" -> "patn/to/mypkg/internal/github.com/kardianos/osext"
	*/
	// The following method "cheats" and doesn't look at any external vendor file.
	ss := strings.Split(importPath, internalFolderSlash)
	localImportPath := path.Join(ctx.RootImportPath, internalFolder, ss[len(ss)-1])

	importPath, err = verify(ctx, importPath, localImportPath)
	if err != nil {
		return err
	}

	err = ctx.AddImports(importPath)
	if err != nil {
		return err
	}

	pkg := ctx.Package[importPath]
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
	// TODO: find the Version and VersionTime.
	var vp *VendorPackage
	for _, vpkg := range ctx.VendorFile.Package {
		if vpkg.Vendor == importPath {
			vp = vpkg
			break
		}
	}
	if vp == nil {
		vp = &VendorPackage{
			Vendor: importPath,
			Local:  localImportPath,
		}
		ctx.VendorFile.Package = append(ctx.VendorFile.Package, vp)
	}
	vcs, err := FindVcs(pkg.Gopath, pkg.Dir)
	if err != nil {
		return err
	}
	vp.Version = vcs.Version
	if vcs.VersionTime != nil {
		vp.VersionTime = vcs.VersionTime.Format(time.RFC3339)
	}
	err = writeVendorFile(ctx.RootDir, ctx.VendorFile)
	if err != nil {
		return err
	}
	err = CopyPackage(filepath.Join(ctx.RootGopath, slashToFilepath(localImportPath)), pkg.Dir)
	if err != nil {
		return err
	}

	err = ctx.AddImports(importPath, localImportPath)
	if err != nil {
		return err
	}

	// Determine which files to touch.
	files := make(map[string]struct{}, len(ctx.VendorFile.Package)*3)

	// Rules are all lines in the vendor file.
	rules := make([]Rule, 0, len(ctx.VendorFile.Package))
	for _, vp := range ctx.VendorFile.Package {
		for f := range ctx.fileImports[vp.Vendor] {
			files[f] = struct{}{}
		}
		rules = append(rules, Rule{From: vp.Vendor, To: vp.Local})
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
		if pkg.Vendor == importPath {
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
				vendorPath = pkg.Vendor
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

	err = ctx.RewriteFiles(files, []Rule{Rule{From: localPath, To: vendorPath}})
	if err != nil {
		return err
	}

	err = RemovePackage(filepath.Join(ctx.RootGopath, slashToFilepath(localPath)))
	if err != nil {
		return err
	}
	nextPkg := make([]*VendorPackage, 0, len(ctx.VendorFile.Package)-1)
	for i, pkg := range ctx.VendorFile.Package {
		if i == vendorFileIndex {
			continue
		}
		nextPkg = append(nextPkg, pkg)
	}
	ctx.VendorFile.Package = nextPkg

	return writeVendorFile(ctx.RootDir, ctx.VendorFile)
}
