// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"errors"
	"fmt"
)

var (
	// ErrVendorFileExists returns if the vendor file exists when it is not expected.
	ErrVendorFileExists = errors.New(vendorFilename + " file already exists.")
	// ErrMissingVendorFile returns if unable to find vendor file.
	ErrMissingVendorFile = errors.New("Unable to find vendor file.")
	// ErrMissingGOROOT returns if the GOROOT was not found.
	ErrMissingGOROOT = errors.New("Unable to determine GOROOT.")
	// ErrMissingGOPATH returns if no GOPATH was found.
	ErrMissingGOPATH = errors.New("Missing GOPATH. Check your environment variable GOPATH.")
	// ErrVendorExists returns if package already a local vendor package.
	ErrVendorExists = errors.New("Package already exists as a vendor package.")
	// ErrLocalPackage returns if this is a local package.
	ErrLocalPackage = errors.New("Cannot vendor a local package.")
	// ErrImportExists import already exists.
	ErrImportExists = errors.New("Import exists. To update use update command.")
	// ErrImportNotExists import does not exists.
	ErrImportNotExists = errors.New("Import does not exist.")
	// ErrFilesExists returns if file exists at destination path.
	ErrFilesExists = errors.New("Files exists at destination of internal vendor path.")
)

// ErrNotInGOPATH returns if not currently in the GOPATH.
type ErrNotInGOPATH struct {
	Missing string
}

func (err ErrNotInGOPATH) Error() string {
	return fmt.Sprintf("Package %q not a go package or not in GOPATH.", err.Missing)
}

// ErrDirtyPackage returns if package is in dirty version control.
type ErrDirtyPackage struct {
	ImportPath string
}

func (err ErrDirtyPackage) Error() string {
	return fmt.Sprintf("Package %q has uncommited changes in the vcs.", err.ImportPath)
}

type errLoopLimit struct {
	Loop string
}

func (err errLoopLimit) Error() string {
	return fmt.Sprintf("BUG: Loop limit of %d was reached for loop %s. Please report this bug.", looplimit, err.Loop)
}
