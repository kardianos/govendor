// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kardianos/govendor/help"
	"github.com/kardianos/govendor/internal/gt"
	"github.com/kardianos/govendor/prompt"
)

var relVendorFile = filepath.Join("vendor", "vendor.json")

type testPrompt struct{}

func (p *testPrompt) Ask(q *prompt.Question) (prompt.Response, error) {
	var opt *prompt.Option
	for i := range q.Options {
		if opt == nil {
			opt = &q.Options[i]
			continue
		}
		item := &q.Options[i]
		if len(item.Key().(string)) > len(opt.Key().(string)) {
			opt = item
		}
	}
	if opt != nil {
		opt.Chosen = true
	}
	return prompt.RespAnswer, nil
}

func Vendor(g *gt.GopathTest, name, argLine, expectedOutput string) {
	os.Setenv("GO15VENDOREXPERIMENT", "1")
	output := &bytes.Buffer{}
	args := append([]string{"testing"}, strings.Split(argLine, " ")...)
	msg, err := Run(output, args, &testPrompt{})
	if err != nil {
		g.Fatalf("(%s) Error: %v", name, err)
	}
	if msg != help.MsgNone {
		g.Fatalf("(%s) Printed help", name)
	}
	// Remove any space padding on the start/end of each line.
	trimLines := func(s string) string {
		lines := strings.Split(strings.TrimSpace(s), "\n")
		for i := range lines {
			lines[i] = strings.TrimSpace(lines[i])
		}
		return strings.Join(lines, "\n")
	}
	if trimLines(output.String()) != trimLines(expectedOutput) {
		g.Fatalf("(%s) Got\n%s", name, output.String())
	}
}

func vendorFile(g *gt.GopathTest, expected string) {
	buf, err := ioutil.ReadFile(filepath.Join(g.Current(), relVendorFile))
	if err != nil {
		g.Fatal(err)
	}
	if strings.TrimSpace(string(buf)) != strings.TrimSpace(expected) {
		g.Fatal("Got: ", string(buf))
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
	Vendor(g, "", "list", `
 e  co2/pk1
 e  co2/pk2
 l  co1/pk1
`)
	Vendor(g, "co1 add ext", "add +ext", "")
	Vendor(g, "co1 list", "list", `
 v  co2/pk1
 v  co2/pk2
 l  co1/pk1
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
	Vendor(g, "co1 pre list", "list", `
 e  co2/pk1
 e  co3/pk1
 e  co3/pk1
 l  co1/pk1
`)
	Vendor(g, "co1 add", "add -long +ext", "")
	Vendor(g, "co1 list", "list", `
 v  co2/pk1
 v  co3/pk1
 l  co1/pk1
`)
}

func TestEllipsisSimple(t *testing.T) {
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
	Vendor(g, "", "list", `
 e  co2/pk1
 e  co2/pk1/pk2
 l  co1/pk1
`)
	Vendor(g, "co1 add ext", "add co2/pk1/...", "")
	Vendor(g, "co1 list", "list", `
 v  co2/pk1
 v  co2/pk1/pk2
 l  co1/pk1
`)
}

func TestEllipsisRootEmpty(t *testing.T) {
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
	Vendor(g, "", "list", `
 e  co2/pk1
 e  co2/pk1/pk2
 l  co1/pk1
`)
	Vendor(g, "co1 add ext", "add co2/...", "")
	Vendor(g, "co1 list", "list", `
 v  co2/pk1
 v  co2/pk1/pk2
 l  co1/pk1
`)
}

func TestTreeRootEmpty(t *testing.T) {
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
	Vendor(g, "", "list", `
 e  co2/pk1
 e  co2/pk1/pk2
 l  co1/pk1
`)
	Vendor(g, "co1 add ext", "add co2/^", "")
	vendorFile(g, `
{
	"comment": "",
	"ignore": "test",
	"package": [
		{
			"checksumSHA1": "LEK/OLgG216wx+DABfa4rfD6j14=",
			"path": "co2",
			"revision": "",
			"tree": true
		}
	],
	"rootPath": "co1"
}
`)
	Vendor(g, "co1 list", "list", `
 vt co2/pk1
 vt co2/pk1/pk2
 v  co2
 l  co1/pk1
	`)
}
