// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"errors"
	"fmt"
)

var (
	ErrVendorFileExists  = errors.New(vendorFilename + " file already exists.")
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
