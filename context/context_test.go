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

func ctx(g *gt.GopathTest) *Context {
	c, err := NewContext(g.Current(), "vendor.json", "internal")
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

func showVendorFile(g *gt.GopathTest) {
	buf, err := ioutil.ReadFile(filepath.Join(g.Current(), "vendor.json"))
	if err != nil {
		g.Fatal(err)
	}
	g.Logf("%s", buf)
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
	c := ctx(g)
	list(g, c, "initial", `e co2/pk1
e co2/pk2
l co1/pk1
s bytes
s strings
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
	c := ctx(g)
	g.Check(c.AddImport("co2/pk1"))
	c.WriteVendorFile()
	expected := `i co1/internal/co2/pk1 [co2/pk1]
e co2/pk2
l co1/pk1
s bytes
s strings
`
	list(g, c, "same", expected)

	c = ctx(g)
	list(g, c, "new", expected)
}

func TestDuplicatePackage(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1", "co3/pk1"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "co3/pk1"),
	)
	g.Setup("co3/pk1",
		gt.File("a.go", "strings"),
	)
	g.In("co2")
	c := ctx(g)
	statusList, err := c.ListStatus()
	g.Check(err)
	for _, item := range statusList {
		if item.Status != StatusExternal {
			continue
		}
		g.Check(c.AddImport(item.Path))
	}
	g.Check(c.WriteVendorFile())
	list(g, c, "co2 list", `i co2/internal/co3/pk1 [co3/pk1]
l co2/pk1
s strings
`)

	g.In("co1")
	c = ctx(g)
	list(g, c, "co1 pre list", `e co2/internal/co3/pk1 [co3/pk1]
e co2/pk1
e co3/pk1
l co1/pk1
s strings
`)

	statusList, err = c.ListStatus()
	g.Check(err)
	for _, item := range statusList {
		if item.Status != StatusExternal {
			continue
		}
		g.Check(c.AddImport(item.Path))
	}
	g.Check(c.WriteVendorFile())
	expected := `i co1/internal/co2/pk1 [co2/pk1]
i co1/internal/co3/pk1 [co3/pk1]
l co1/pk1
s strings
`
	list(g, c, "co1 list 1", expected)
	c = ctx(g)
	list(g, c, "co1 list 2", expected)
}
