// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/kardianos/govendor/internal/gt"
)

func Vendor(g *gt.GopathTest, name, argLine, expectedOutput string) {
	os.Setenv("GO15VENDOREXPERIMENT", "0")
	os.Setenv("GOVENDORFOLDER", "")
	output := &bytes.Buffer{}
	args := append([]string{"testing"}, strings.Split(argLine, " ")...)
	printHelp, err := run(output, args)
	if err != nil {
		g.Fatalf("(%s) Error: %v", name, err)
	}
	if printHelp == true {
		g.Fatalf("(%s) Printed help", name)
	}
	if output.String() != expectedOutput {
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
	Vendor(g, "co1 init", "init", "")
	Vendor(g, "", "list", `e co2/pk1
e co2/pk2
l co1/pk1
`)
	Vendor(g, "co1 add ext", "add +ext", "")
	Vendor(g, "co1 list", "list", `v co2/pk1
v co2/pk2
l co1/pk1
`)
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
	Vendor(g, "co2 init", "init", "")
	Vendor(g, "co2 add", "add +ext", "")

	g.In("co1")
	Vendor(g, "co1 init", "init", "")
	Vendor(g, "co1 pre list", "list", `e co3/pk1
e co2/pk1
e co3/pk1
l co1/pk1
`)
	Vendor(g, "co1 add", "add -long +ext", "")
	Vendor(g, "co1 list", "list", `v co2/pk1
v co3/pk1
l co1/pk1
`)
}

func TestEllipsis(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1", "co2/pk1/pk2"),
		gt.File("b.go", "co2/pk1", "bytes"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "strings"),
	)
	g.Setup("co2/pk1/pk2",
		gt.File("a.go", "strings"),
	)
	g.In("co1")
	Vendor(g, "co1 init", "init", "")
	Vendor(g, "", "list", `e co2/pk1
e co2/pk1/pk2
l co1/pk1
`)
	Vendor(g, "co1 add ext", "add co2/pk1/...", "")
	Vendor(g, "co1 list", "list", `v co2/pk1
v co2/pk1/pk2
l co1/pk1
`)
}
