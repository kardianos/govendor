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
	"Tool": "github.com/kardianos/vendor",
	"Package": [
		{
			"Vendor": "github.com/dchest/safefile",
			"Local": "github.com/kardianos/vendor/internal/github.com/dchest/safefile",
			"Version": "74b1ec0619e722c9f674d1a21e1a703fe90c4371",
			"VersionTime": "2015-04-10T19:48:00+02:00"
		}
	]
}`
	var to = `{
	"comment": "",
	"package": [
		{
			"canonical": "github.com/dchest/safefile",
			"comment": "",
			"local": "github.com/kardianos/vendor/internal/github.com/dchest/safefile",
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
