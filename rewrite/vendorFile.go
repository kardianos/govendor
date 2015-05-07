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

	"github.com/kardianos/vendor/internal/github.com/dchest/safefile"
)

// VendorFile is the structure of the vendor file.
type VendorFile struct {
	// The import path of the tool used to write this file.
	// Examples: "github.com/kardianos/vendor" or "golang.org/x/tools/cmd/vendor".
	Tool string

	// Comment is free text for human use.
	Comment string `json:",omitempty"`

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

	// The revision of the package. This field must be persisted by all
	// tools, but not all tools will interpret this field.
	// The value of Revision should be a single value that can be used
	// to fetch the same or similar revision.
	// Examples: "abc104...438ade0", "v1.3.5"
	Revision string

	// RevisionTime is the time the revision was created. The time should be
	// parsed and written in the "time.RFC3339" format.
	RevisionTime string

	// Comment is free text for human use.
	Comment string `json:",omitempty"`
}

// VendorFile is the structure of the vendor file.
type readonlyVendorFile struct {
	Tool    string
	Comment string
	Package []*readonlyVendorPackage
}

type readonlyVendorPackage struct {
	Vendor       string
	Local        string
	Revision     string
	RevisionTime string
	Comment      string

	Version     string
	VersionTime string
}

type vendorPackageSort []*VendorPackage

func (vp vendorPackageSort) Len() int      { return len(vp) }
func (vp vendorPackageSort) Swap(i, j int) { vp[i], vp[j] = vp[j], vp[i] }
func (vp vendorPackageSort) Less(i, j int) bool {
	if vp[i].Local == vp[j].Local {
		return vp[i].Vendor < vp[j].Vendor
	}
	return vp[i].Local < vp[j].Local
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
	var rvf = &readonlyVendorFile{}
	err = json.Unmarshal(bb, rvf)
	if err != nil {
		return nil, err
	}
	vf := &VendorFile{
		Tool:    rvf.Tool,
		Comment: rvf.Comment,
		Package: make([]*VendorPackage, len(rvf.Package)),
	}
	for i, rpkg := range rvf.Package {
		pkg := &VendorPackage{
			Vendor:       rpkg.Vendor,
			Local:        rpkg.Local,
			Revision:     rpkg.Revision,
			RevisionTime: rpkg.RevisionTime,
			Comment:      rpkg.Comment,
		}
		vf.Package[i] = pkg

		if len(rpkg.Version) != 0 {
			pkg.Revision = rpkg.Version
		}
		if len(rpkg.VersionTime) != 0 {
			pkg.RevisionTime = rpkg.VersionTime
		}
	}
	return vf, nil
}
