// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// vendorfile is the meta-data file for vendoring.
// Round-trips unknown fields.
// It will also allow moving the vendor file to new locations.
package vendorfile

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"sort"
)

// Name of the vendor file.
const Name = "vendor.json"

// VendorFile is the structure of the vendor file.
type File struct {
	Comment string

	Ignore string

	Package []*Package

	// all preserves unknown values.
	all map[string]interface{}
}

// VendorPackage represents each package.
type Package struct {
	// index of the package in the file as read.
	index int

	// If delete is set to true the package will not be written to the vendor file.
	Remove bool

	// If new is set to true the package will be treated as a new package to the file.
	Add bool

	// See the vendor spec for definitions.
	Canonical    string
	Local        string
	Revision     string
	RevisionTime string
	Comment      string
}

var (
	packageNames      = []string{"package", "Package"}
	ignoreNames       = []string{"ignore"}
	canonicalNames    = []string{"canonical", "Canonical", "vendor", "Vendor"}
	localNames        = []string{"local", "Local"}
	revisionNames     = []string{"revision", "Revision", "version", "Version"}
	revisionTimeNames = []string{"revisionTime", "RevisionTime", "versionTime", "VersionTime"}
	commentNames      = []string{"comment", "Comment"}
)

type vendorPackageSort []interface{}

func (vp vendorPackageSort) Len() int      { return len(vp) }
func (vp vendorPackageSort) Swap(i, j int) { vp[i], vp[j] = vp[j], vp[i] }
func (vp vendorPackageSort) Less(i, j int) bool {
	a := vp[i].(map[string]interface{})
	b := vp[j].(map[string]interface{})
	aLocal, _ := a[localNames[0]].(string)
	bLocal, _ := b[localNames[0]].(string)

	if aLocal == bLocal {
		aCanonical, _ := a[canonicalNames[0]].(string)
		bCanonical, _ := b[canonicalNames[0]].(string)
		return aCanonical < bCanonical
	}
	return aLocal < bLocal
}

func setField(field *string, object map[string]interface{}, names []string) {
	for _, name := range names {
		raw, found := object[name]
		if !found {
			continue
		}
		value, is := raw.(string)
		if !is {
			continue
		}
		*field = value
		if len(value) != 0 {
			break
		}
	}
}

func setObject(field string, object map[string]interface{}, names []string) {
	for i, name := range names {
		if i == 0 {
			object[name] = field
		} else {
			delete(object, name)
		}
	}
}

// getRawPackageList gets the array of items from all object.
func (vf *File) getRawPackageList() []interface{} {
	var rawPackageList []interface{}
	for index, name := range packageNames {
		rawPackageListObject, found := vf.all[name]
		if !found {
			continue
		}
		if index != 0 {
			vf.all[packageNames[0]] = rawPackageListObject
			delete(vf.all, name)
		}
		var is bool
		rawPackageList, is = rawPackageListObject.([]interface{})
		if is {
			break
		}
	}
	return rawPackageList
}

// toFields moves values from "all" to the field values.
func (vf *File) toFields() {
	setField(&vf.Comment, vf.all, commentNames)
	setField(&vf.Ignore, vf.all, ignoreNames)

	rawPackageList := vf.getRawPackageList()

	vf.Package = make([]*Package, len(rawPackageList))

	for index, rawPackage := range rawPackageList {
		object, is := rawPackage.(map[string]interface{})
		if !is {
			continue
		}
		pkg := &Package{}
		vf.Package[index] = pkg
		pkg.index = index
		setField(&pkg.Canonical, object, canonicalNames)
		setField(&pkg.Local, object, localNames)
		setField(&pkg.Revision, object, revisionNames)
		setField(&pkg.RevisionTime, object, revisionTimeNames)
		setField(&pkg.Comment, object, commentNames)
	}
}

// toAll moves values from field values to "all".
func (vf *File) toAll() {
	delete(vf.all, "Tool")

	setObject(vf.Comment, vf.all, commentNames)
	setObject(vf.Ignore, vf.all, ignoreNames)

	rawPackageList := vf.getRawPackageList()

	deleteCount := 0
	for _, pkg := range vf.Package {
		switch {
		case pkg.Remove:
			rawPackageList[pkg.index] = nil
			deleteCount++
		case pkg.Add:
			obj := make(map[string]interface{}, 5)
			rawPackageList = append(rawPackageList, obj)

			setObject(pkg.Canonical, obj, canonicalNames)
			setObject(pkg.Local, obj, localNames)
			setObject(pkg.Revision, obj, revisionNames)
			setObject(pkg.RevisionTime, obj, revisionTimeNames)
			setObject(pkg.Comment, obj, commentNames)
		default:
			var obj map[string]interface{}
			rawObj := rawPackageList[pkg.index]
			if rawObj == nil {
				obj = make(map[string]interface{}, 5)
			} else {
				obj = rawObj.(map[string]interface{})
			}

			setObject(pkg.Canonical, obj, canonicalNames)
			setObject(pkg.Local, obj, localNames)
			setObject(pkg.Revision, obj, revisionNames)
			setObject(pkg.RevisionTime, obj, revisionTimeNames)
			setObject(pkg.Comment, obj, commentNames)
		}
	}

	nextRawPackageList := make([]interface{}, 0, len(rawPackageList)-deleteCount)
	for _, raw := range rawPackageList {
		if raw == nil {
			continue
		}
		nextRawPackageList = append(nextRawPackageList, raw)
	}
	vf.all[packageNames[0]] = nextRawPackageList
}

// Marshal the vendor file to the specified writer.
// Retains read fields.
func (vf *File) Marshal(w io.Writer) (err error) {
	if vf.all == nil {
		vf.all = map[string]interface{}{}
	}
	vf.toAll()

	rawList := vf.getRawPackageList()

	sort.Sort(vendorPackageSort(rawList))

	jb, err := json.Marshal(vf.all)
	if err != nil {
		return
	}
	buf := &bytes.Buffer{}
	err = json.Indent(buf, jb, "", "\t")
	if err != nil {
		return
	}
	_, err = io.Copy(w, buf)
	return
}

// Unmarshal the vendor file from the specified reader.
// Stores internally all fields.
func (vf *File) Unmarshal(r io.Reader) error {
	bb, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	if vf.all == nil {
		vf.all = make(map[string]interface{}, 3)
	}
	err = json.Unmarshal(bb, &vf.all)
	if err != nil {
		return err
	}
	vf.toFields()
	return nil
}
