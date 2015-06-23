// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"testing"
)

func TestSimple(t *testing.T) {
	g := newGopathTest(t)
	defer g.Clean()
	
	g.Setup("co1/pk1",
		File("a.go", "co2/pk1", "co2/pk2"),
		File("b.go", "co2/pk1", "bytes"),
	)
	g.Setup("co2/pk1",
		File("a.go", "strings"),
	)
	g.Setup("co2/pk2",
		File("a.go", "strings"),
	)
	g.In("co1")
	g.Vendor("init", "")
	g.Vendor("list", `e co2/pk1
e co2/pk2
l co1/pk1
`)
	g.Vendor("add -status ext", "")
	g.Vendor("list", `i co1/internal/co2/pk1
i co1/internal/co2/pk2
l co1/pk1
`)
}
