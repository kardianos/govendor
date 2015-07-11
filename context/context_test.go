// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context_test

import (
	"bytes"
	"testing"

	. "github.com/kardianos/vendor/context"
	"github.com/kardianos/vendor/internal/gt"
)

func ctx(g *gt.GopathTest) *Context {
	c, err := NewContext(g.Current(), "vendor.json", "vendor")
	if err != nil {
		g.Fatal(err)
	}
	err = c.LoadPackage()
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
