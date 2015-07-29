// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context_test

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	. "github.com/kardianos/vendor/context"
	"github.com/kardianos/vendor/internal/gt"
)

func ctx14(g *gt.GopathTest) *Context {
	c, err := NewContext(g.Current(), filepath.Join("internal", "vendor.json"), "internal", true)
	if err != nil {
		g.Fatal(err)
	}
	return c
}
func ctx15(g *gt.GopathTest) *Context {
	c, err := NewContext(g.Current(), "vendor.json", "vendor", false)
	if err != nil {
		g.Fatal(err)
	}
	return c
}

func list(g *gt.GopathTest, c *Context, name, expected string) {
	list, err := c.Status()
	if err != nil {
		g.Fatal(err)
	}
	output := &bytes.Buffer{}
	for _, item := range list {
		output.WriteString(item.String())
		output.WriteRune('\n')
	}
	if output.String() != expected {
		g.Fatalf("(%s) Got\n%s", name, output.String())
	}
}

func showVendorFile14(g *gt.GopathTest) {
	buf, err := ioutil.ReadFile(filepath.Join(g.Current(), "internal", "vendor.json"))
	if err != nil {
		g.Fatal(err)
	}
	g.Logf("%s", buf)
}
func vendorFile14(g *gt.GopathTest, expected string) {
	buf, err := ioutil.ReadFile(filepath.Join(g.Current(), "internal", "vendor.json"))
	if err != nil {
		g.Fatal(err)
	}
	if string(buf) != expected {
		g.Fatal("Got: ", string(buf))
	}
}

func showRewriteRule(c *Context, t *testing.T) {
	for from, to := range c.MoveRule {
		t.Logf("R: %s to %s\n", from, to)
	}
}
func showCurrentVendorFile(c *Context, t *testing.T) {
	t.Log("Vendor File (memory)")
	for _, vp := range c.VendorFile.Package {
		if vp.Remove {
			continue
		}
		t.Logf("\tPKG:: C: %s, L: %s\n", vp.Canonical, vp.Local)
	}
}

func TestSimple(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1", "co2/pk2"),
		gt.File("b.go", "co2/pk1", "bytes"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "strings"),
	)
	g.Setup("co2/pk2",
		gt.File("a.go", "strings"),
	)
	g.In("co1")
	c := ctx14(g)
	list(g, c, "initial", `e co2/pk1 < ["co1/pk1"]
e co2/pk2 < ["co1/pk1"]
l co1/pk1 < []
s bytes < ["co1/pk1"]
s strings < ["co2/pk1" "co2/pk2"]
`)
}

func TestImportSimple(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1", "co2/pk2"),
		gt.File("b.go", "co2/pk1", "bytes"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "strings"),
	)
	g.Setup("co2/pk2",
		gt.File("a.go", "strings"),
	)
	g.In("co1")
	c := ctx14(g)
	g.Check(c.ModifyImport("co2/pk1", AddUpdate))

	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	vendorFile14(g, `{
	"comment": "",
	"package": [
		{
			"canonical": "co2/pk1",
			"comment": "",
			"local": "co1/internal/co2/pk1",
			"revision": "",
			"revisionTime": ""
		}
	]
}`)

	expected := `v co1/internal/co2/pk1 [co2/pk1] < ["co1/pk1"]
e co2/pk2 < ["co1/pk1"]
l co1/pk1 < []
s bytes < ["co1/pk1"]
s strings < ["co1/internal/co2/pk1" "co2/pk2"]
`

	list(g, c, "same", expected)

	c = ctx14(g)
	list(g, c, "new", expected)
}

func TestDuplicatePackage(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk2", "co3/pk3"),
	)
	g.Setup("co2/pk2",
		gt.File("b.go", "co3/pk3"),
	)
	g.Setup("co3/pk3",
		gt.File("c.go", "strings"),
	)
	g.In("co2")
	c := ctx14(g)
	statusList, err := c.Status()
	g.Check(err)
	for _, item := range statusList {
		if item.Status != StatusExternal {
			continue
		}
		g.Check(c.ModifyImport(item.Local, AddUpdate))
	}
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co2 list", `v co2/internal/co3/pk3 [co3/pk3] < ["co2/pk2"]
l co2/pk2 < []
s strings < ["co2/internal/co3/pk3"]
`)

	g.In("co1")
	c = ctx14(g)
	list(g, c, "co1 pre list", `e co2/internal/co3/pk3 [co3/pk3] < ["co2/pk2"]
e co2/pk2 < ["co1/pk1"]
e co3/pk3 < ["co1/pk1"]
l co1/pk1 < []
s strings < ["co2/internal/co3/pk3" "co3/pk3"]
`)

	statusList, err = c.Status()
	g.Check(err)
	for _, item := range statusList {
		if item.Status != StatusExternal {
			continue
		}
		g.Check(c.ModifyImport(item.Local, AddUpdate))
	}

	c.Reslove(c.Check()) // Automaically resolve conflicts.
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	expected := `v co1/internal/co2/pk2 [co2/pk2] < ["co1/pk1"]
v co1/internal/co3/pk3 [co3/pk3] < ["co1/internal/co2/pk2" "co1/pk1"]
l co1/pk1 < []
s strings < ["co1/internal/co3/pk3"]
`

	list(g, c, "co1 list 1", expected)
	c = ctx14(g)
	list(g, c, "co1 list 2", expected)

	// Now remove one import.
	g.Check(c.ModifyImport("co3/pk3", Remove))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	list(g, c, "co1 remove", `v co1/internal/co2/pk2 [co2/pk2] < ["co1/pk1"]
e co3/pk3 < ["co1/internal/co2/pk2" "co1/pk1"]
l co1/pk1 < []
s strings < ["co3/pk3"]
`)
}

func TestImportSimple15(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1", "co2/pk2"),
		gt.File("b.go", "co2/pk1", "bytes"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "strings"),
	)
	g.Setup("co2/pk2",
		gt.File("a.go", "strings"),
	)
	g.In("co1")
	c := ctx15(g)
	g.Check(c.ModifyImport("co2/pk1", AddUpdate))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	expected := `v co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
e co2/pk2 < ["co1/pk1"]
l co1/pk1 < []
s bytes < ["co1/pk1"]
s strings < ["co1/vendor/co2/pk1" "co2/pk2"]
`
	list(g, c, "same", expected)

	c = ctx15(g)
	list(g, c, "new", expected)

	// Now remove an import.
	g.Check(c.ModifyImport("co2/pk1", Remove))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	list(g, c, "co1 remove", `e co2/pk1 < ["co1/pk1"]
e co2/pk2 < ["co1/pk1"]
l co1/pk1 < []
s bytes < ["co1/pk1"]
s strings < ["co2/pk1" "co2/pk2"]
`)
}
