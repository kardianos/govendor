// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// rewrite contains commands for writing the altered import statements.
package rewrite

import (
	"errors"
	"github.com/dchest/safefile"
	"path/filepath"
)

type ListStatus byte

func (ls ListStatus) String() string {
	switch ls {
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
	StatusExternal ListStatus = iota
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

const (
	vendorFilename = "vendor.json"
	internalFolder = "internal"
)

var (
	internalVendor = filepath.Join(internalFolder, vendorFilename)
)

var (
	ErrVendorFileExists = errors.New(internalVendor + " file already exists.")
)

func CmdInit() error {
	/*
		1. Determine if CWD contains "internal/vendor.json".
		2. If exists, return error.
		3. Create directory if it doesn't exist.
		4. Create "internal/vendor.json" file.
	*/
	return nil
}
func CmdList(status ListStatus) ([]ListItem, error) {
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
	return nil, nil
}

/*
	Add, Update, and Remove will start with the same steps as List.
	Rather then returning the results, it will find any affected files,
	alter their imports, then write the files back out. Also copy or remove
	files and folders as needed.
*/

func CmdAdd(importPath string) error {
	return nil
}
func CmdUpdate(importPath string) error {
	return nil
}
func CmdRemove(importPath string) error {
	return nil
}

func write() error {
	return safefile.WriteFile("foo.go", []byte{}, 0777)
}
