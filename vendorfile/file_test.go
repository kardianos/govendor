// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vendorfile

import (
	"bytes"
	"strings"
	"testing"
)

func TestUpdate(t *testing.T) {
	var from = `{
	"Tool": "github.com/kardianos/govendor",
	"Package": [
		{
			"Vendor": "github.com/dchest/safefile",
			"Local": "github.com/kardianos/govendor/internal/github.com/dchest/safefile",
			"Version": "74b1ec0619e722c9f674d1a21e1a703fe90c4371",
			"VersionTime": "2015-04-10T19:48:00+02:00"
		}
	]
}`
	var to = `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"path": "github.com/dchest/safefile",
			"revision": "74b1ec0619e722c9f674d1a21e1a703fe90c4371",
			"revisionTime": "2015-04-10T19:48:00+02:00"
		}
	]
}`

	vf := &File{}

	err := vf.Unmarshal(strings.NewReader(from))
	if err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	err = vf.Marshal(buf)
	if err != nil {
		t.Fatal(err)
	}

	if buf.String() != to {
		t.Fatal("Got:", buf.String())
	}
}

func TestRemove(t *testing.T) {
	var from = `{
	"package": [
		{
			"canonical": "pkg1"
		},
		{
			"canonical": "pkg2"
		},
		{
			"canonical": "pkg3"
		}
	]
}`
	var to = `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"path": "pkg1",
			"revision": ""
		}
	]
}`

	vf := &File{}

	err := vf.Unmarshal(strings.NewReader(from))
	if err != nil {
		t.Fatal(err)
	}

	vf.Package[1].Remove = true
	vf.Package[2].Remove = true

	buf := &bytes.Buffer{}
	err = vf.Marshal(buf)
	if err != nil {
		t.Fatal(err)
	}

	if buf.String() != to {
		t.Fatal("Got:", buf.String())
	}
}

func TestAdd(t *testing.T) {
	var from = `{
	"package": [
		{
			"canonical": "pkg1"
		}
	]
}`
	var to = `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"path": "pkg1",
			"revision": ""
		},
		{
			"path": "pkg2",
			"revision": ""
		},
		{
			"path": "pkg3",
			"revision": "",
			"tree": true
		}
	]
}`

	vf := &File{}

	err := vf.Unmarshal(strings.NewReader(from))
	if err != nil {
		t.Fatal(err)
	}

	vf.Package = append(vf.Package, &Package{
		Add:  true,
		Path: "pkg2",
	}, &Package{
		Add:  true,
		Path: "pkg3",
		Tree: true,
	})

	buf := &bytes.Buffer{}
	err = vf.Marshal(buf)
	if err != nil {
		t.Fatal(err)
	}

	if buf.String() != to {
		t.Fatal("Got:", buf.String())
	}
}
