// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// rewrite contains commands for writing the altered import statements.
package rewrite

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/dchest/safefile"
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
	toolName       = "github.com/kardianos/vendor"
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

func writeVendorFile(root string, vf *VendorFile) error {
	path := filepath.Join(root, internalVendor)
	perm := os.FileMode(0777)
	fi, err := os.Stat(path)
	if err == nil {
		perm = fi.Mode()
	}
	f, err := safefile.Create(path, perm)
	if err != nil {
		return err
	}
	// TODO: capture Close err.
	defer f.Close()
	coder := json.NewEncoder(f)
	return coder.Encode(vf)
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
