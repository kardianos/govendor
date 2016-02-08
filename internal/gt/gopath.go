// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package gt is for GOPATH testing for vendor.
package gt

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"text/template"
)

// Process for testing run().
// 1. Setup on-disk GOPATH and env var.
// 2. Populate GOPATH with example packages.
// 3. Set current working directory to project root.
// 4. Run vendor command(s).
// 5. Inspect project workspace for desired result.

func New(t *testing.T) *GopathTest {
	base, err := ioutil.TempDir(os.TempDir(), "vendor_")
	if err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	g := &GopathTest{
		T:    t,
		base: base,

		initPath: wd,
	}
	g.mkdir("src")
	os.Setenv("GOPATH", g.base)
	return g
}

type GopathTest struct {
	*testing.T

	base    string
	current string

	initPath string
}

func (g *GopathTest) mkdir(s string) {
	p := filepath.Join(g.base, s)
	err := os.MkdirAll(p, 0700)
	if err != nil {
		g.Fatal(err)
	}
}
func (g *GopathTest) mksrc(s string) string {
	p := g.Path(s)
	err := os.MkdirAll(p, 0700)
	if err != nil {
		g.Fatal(err)
	}
	return p
}
func (g *GopathTest) In(pkg string) {
	p := g.Path(pkg)
	err := os.Chdir(p)
	if err != nil {
		g.Fatal(err)
	}
	g.current = p
}
func (g *GopathTest) Path(pkg string) string {
	return filepath.Join(g.base, "src", pkg)
}
func (g *GopathTest) Current() string {
	return g.current
}
func (g *GopathTest) Clean() {
	os.Chdir(g.initPath)
	err := os.RemoveAll(g.base)
	if err != nil {
		g.Fatal(err)
	}
}
func (g *GopathTest) Check(err error) {
	if err == nil {
		return
	}
	g.Fatal(err)
}

type FileSpec struct {
	Pkg     string
	Name    string
	Imports []string
	Build   string
}

var fileSpecFile = template.Must(template.New("").Funcs(map[string]interface{}{
	"pkg": func(s string) string {
		_, pkg := path.Split(s)
		return pkg
	},
	"imp": func(s string) string {
		return "`" + s + "`"
	},
}).Parse(` {{if .Build}}
// +build {{.Build}}
{{end}}
package {{.Pkg|pkg}}

import (
{{range .Imports}}	{{.|imp}}
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
func FileBuild(name, build string, imports ...string) FileSpec {
	return FileSpec{Name: name, Build: build, Imports: imports}
}

func (g *GopathTest) Setup(at string, files ...FileSpec) {
	var err error
	pkg := g.mksrc(at)
	for _, f := range files {
		f.Pkg = at
		p := filepath.Join(pkg, f.Name)
		err = ioutil.WriteFile(p, f.Bytes(), 0600)
		if err != nil {
			g.Fatal(err)
		}
	}
}

func (g *GopathTest) Remove(at string) {
	p := g.Path(at)
	err := os.RemoveAll(p)
	if err != nil {
		g.Fatal(err)
	}
}

func (g *GopathTest) Fatal(args ...interface{}) {
	_, file, line, _ := runtime.Caller(2)
	g.T.Logf("%s:%d", file, line)
	g.T.Fatal(args...)
}
