// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rewrite

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dchest/safefile"
)

// VendorFile is the structure of the vendor file.
type VendorFile struct {
	// The import path of the tool used to write this file.
	// Examples: "github.com/kardianos/vendor" or "golang.org/x/tools/cmd/vendor".
	Tool string

	Package []*VendorPackage
}

type VendorPackage struct {
	// Vendor import path. Example "rsc.io/pdf".
	// go get <Vendor> should fetch the remote vendor package.
	Vendor string

	// Package path as found in GOPATH.
	// Examples: "path/to/mypkg/internal/rsc.io/pdf".
	// If Local is an empty string, the tool should assume the
	// package is not currently copied locally.
	//
	// Local should always use forward slashes and must not contain the
	// path elements "." or "..".
	Local string

	// The version of the package. This field must be persisted by all
	// tools, but not all tools will interpret this field.
	// The value of Version should be a single value that can be used
	// to fetch the same or similar version.
	// Examples: "abc104...438ade0", "v1.3.5"
	Version string

	// VersionTime is the time the version was created. The time should be
	// parsed and written in the "time.RFC3339" format.
	VersionTime string
}

type vendorPackageSort []*VendorPackage

func (vp vendorPackageSort) Len() int      { return len(vp) }
func (vp vendorPackageSort) Swap(i, j int) { vp[i], vp[j] = vp[j], vp[i] }
func (vp vendorPackageSort) Less(i, j int) bool {
	if vp[i].Local == vp[j].Local {
		return strings.Compare(vp[i].Vendor, vp[j].Vendor) < 0
	}
	return strings.Compare(vp[i].Local, vp[j].Local) < 0
}

func writeVendorFile(root string, vf *VendorFile) (err error) {
	path := filepath.Join(root, internalVendor)
	perm := os.FileMode(0777)
	fi, err := os.Stat(path)
	if err == nil {
		perm = fi.Mode()
	}

	if vf.Package == nil {
		vf.Package = []*VendorPackage{}
	}

	sort.Sort(vendorPackageSort(vf.Package))

	jb, err := json.Marshal(vf)
	if err != nil {
		return
	}
	buf := &bytes.Buffer{}
	err = json.Indent(buf, jb, "", "\t")
	if err != nil {
		return
	}
	err = safefile.WriteFile(path, buf.Bytes(), perm)
	return
}

func readVendorFile(root string) (*VendorFile, error) {
	path := filepath.Join(root, internalVendor)
	bb, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var vf = &VendorFile{}
	return vf, json.Unmarshal(bb, vf)
}
