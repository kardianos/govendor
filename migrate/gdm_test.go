// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package migrate

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kardianos/govendor/vendorfile"
)

var gdmFile = `co1/pk1 9fc824c70f713ea0f058a07b49a4c563ef2a3b98
co1/pk2 a4eecd407cf4129fc902ece859a0114e4cf1a7f4
co1/pk3 345426c77237ece5dab0e1605c3e4b35c3f54757`

var gdmPackages = []*vendorfile.Package{
	&vendorfile.Package{
		Add:      true,
		Path:     "co1/pk1",
		Revision: "9fc824c70f713ea0f058a07b49a4c563ef2a3b98",
		Tree:     true,
	},
	&vendorfile.Package{
		Add:      true,
		Path:     "co1/pk2",
		Revision: "a4eecd407cf4129fc902ece859a0114e4cf1a7f4",
		Tree:     true,
	},
	&vendorfile.Package{
		Add:      true,
		Path:     "co1/pk3",
		Revision: "345426c77237ece5dab0e1605c3e4b35c3f54757",
		Tree:     true,
	},
}

func TestParseGDM(t *testing.T) {
	gdm := sysGdm{}
	pkgs, err := gdm.parseGdmFile(strings.NewReader(gdmFile))
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(gdmPackages, pkgs) {
		t.Fatalf("expected parsed gdmFile to match gdmPackages")
	}
}
