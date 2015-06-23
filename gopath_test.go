// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"path"
	"io/ioutil"
	"path/filepath"
	"os"
	"bytes"
	"strings"
	"testing"
	"text/template"
)

// Process for testing run().
// 1. Setup on-disk GOPATH and env var.
// 2. Populate GOPATH with example packages.
// 3. Set current working directory to project root.
// 4. Run vendor command(s).
// 5. Inspect project workspace for desired result.

func newGopathTest(t *testing.T) *gopathTest {
	base, err := ioutil.TempDir(os.TempDir(), "vendor_")
	if err != nil {
		t.Fatal(err)
	}
	g := &gopathTest{
		t: t,
		base: base,
	}
	g.mkdir("src")
	os.Setenv("GOPATH", g.base)
	return g
}

type gopathTest struct {
	t *testing.T
	base string
}

func (g *gopathTest) mkdir(s string) {
	p := filepath.Join(g.base, s)
	err := os.MkdirAll(p, 0700)
	if err != nil {
		g.t.Fatal(err)
	}
}
func (g *gopathTest) mksrc(s string) string {
	p := filepath.Join(g.base, "src", s)
	err := os.MkdirAll(p, 0700)
	if err != nil {
		g.t.Fatal(err)
	}
	return p
}
func (g *gopathTest) In(pkg string) {
	p := filepath.Join(g.base, "src", pkg)
	err := os.Chdir(p)
	if err != nil {
		g.t.Fatal(err)
	}
}
func (g *gopathTest) Clean() {
	err := os.RemoveAll(g.base)
	if err != nil {
		g.t.Fatal(err)
	}
}

type FileSpec struct {
	Pkg string
	Name string
	Imports []string
}

var fileSpecFile = template.Must(template.New("").Funcs(map[string]interface{}{
	"pkg": func(s string) string {
		_, pkg := path.Split(s)
		return pkg
	},
}).Parse(`
package {{.Pkg|pkg}}

import ({{range .Imports}}
	"{{.}}"
{{end}})
`))

func (fs FileSpec) Bytes() []byte {
	buf := &bytes.Buffer{}
	fileSpecFile.Execute(buf, fs)
	return buf.Bytes()
}

func File(name string, imports ...string) FileSpec {
	return FileSpec{Name: name, Imports: imports}
}

func (g *gopathTest) Setup(at string, files ...FileSpec) {
	var err error
	pkg := g.mksrc(at)
	for _, f := range files {
		f.Pkg = at
		p := filepath.Join(pkg, f.Name)
		err = ioutil.WriteFile(p, f.Bytes(), 0600)
		if err != nil {
			g.t.Fatal(err)
		}
	}
}

func (g *gopathTest) Vendor(argLine, expectedOutput string) {
	output := &bytes.Buffer{}
	args := append([]string{"testing"}, strings.Split(argLine, " ")...)
	printHelp, err := run(output, args)
	if err != nil {
		g.t.Fatal(err)
	}
	if printHelp == true {
		g.t.Fatal("Printed help")
	}
	if output.String() != expectedOutput {
		g.t.Fatalf("Got\n%s", output.String())
	}
}
