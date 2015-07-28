// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"errors"
	"fmt"
)

var (
	// ErrMissingVendorFile returns if unable to find vendor file.
	ErrMissingVendorFile = errors.New("Unable to find vendor file.")
	// ErrMissingGOROOT returns if the GOROOT was not found.
	ErrMissingGOROOT = errors.New("Unable to determine GOROOT.")
	// ErrMissingGOPATH returns if no GOPATH was found.
	ErrMissingGOPATH = errors.New("Missing GOPATH. Check your environment variable GOPATH.")
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
