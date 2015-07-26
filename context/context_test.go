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
	c, err := NewContext(g.Current(), filepath.Join("internal", "vendor.json"), "internal", false)
	if err != nil {
		g.Fatal(err)
	}
	return c
}
func ctx15(g *gt.GopathTest) *Context {
	c, err := NewContext(g.Current(), "vendor.json", "vendor", true)
	if err != nil {
		g.Fatal(err)
	}
	return c
}

func list(g *gt.GopathTest, c *Context, name, expected string) {
	list, err := c.ListStatus()
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
	g.Check(c.AddImport("co2/pk1"))

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

	expected := `i co1/internal/co2/pk1 [co2/pk1] < ["co1/pk1"]
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
	statusList, err := c.ListStatus()
	g.Check(err)
	for _, item := range statusList {
		if item.Status != StatusExternal {
			continue
		}
		g.Check(c.AddImport(item.Local))
	}
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co2 list", `i co2/internal/co3/pk3 [co3/pk3] < ["co2/pk2"]
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

	statusList, err = c.ListStatus()
	g.Check(err)
	for _, item := range statusList {
		if item.Status != StatusExternal {
			continue
		}
		g.Check(c.AddImport(item.Local))
	}

	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	expected := `i co1/internal/co2/pk2 [co2/pk2] < ["co1/pk1"]
i co1/internal/co3/pk3 [co3/pk3] < ["co1/internal/co2/pk2" "co1/pk1"]
l co1/pk1 < []
s strings < ["co1/internal/co3/pk3"]
`

	list(g, c, "co1 list 1", expected)
	c = ctx14(g)
	list(g, c, "co1 list 2", expected)
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
	g.Check(c.AddImport("co2/pk1"))
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
}
