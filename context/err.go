// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"errors"
	"fmt"
)

var (
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
	return fmt.Sprintf("Package %q has uncommitted changes in the vcs.", err.ImportPath)
}

// ErrPackageExists returns if package already exists.
type ErrPackageExists struct {
	Package string
}

func (err ErrPackageExists) Error() string {
	return fmt.Sprintf("Package %q already in vendor.", err.Package)
}

// ErrMissingVendorFile returns if package already exists.
type ErrMissingVendorFile struct {
	Path string
}

func (err ErrMissingVendorFile) Error() string {
	return fmt.Sprintf("Vendor file at %q not found.", err.Path)
}

// ErrOldVersion returns if vendor file is not in the vendor folder.
type ErrOldVersion struct {
	Message string
}

func (err ErrOldVersion) Error() string {
	return fmt.Sprintf("The vendor file or is old. %s", err.Message)
}

type ErrTreeChildren struct {
	path     string
	children []*Package
}

func (err ErrTreeChildren) Error() string {
	return fmt.Sprintf("Cannot have a sub-tree %q contain sub-packages %q", err.path, err.children)
}

type ErrTreeParents struct {
	path    string
	parents []string
}

func (err ErrTreeParents) Error() string {
	return fmt.Sprintf("Cannot add package %q which is already found in sub-tree %q", err.path, err.parents)
}
