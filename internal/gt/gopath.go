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
	current string // Current full path.
	pkg     string // Current import path package.

	initPath string

	cleaners []func()
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

// In sets the current directory as an import path.
func (g *GopathTest) In(pkg string) {
	g.pkg = pkg
	p := g.Path(pkg)
	err := os.Chdir(p)
	if err != nil {
		g.Fatal(err)
	}
	g.current = p
}

// Get path from package import path pkg.
func (g *GopathTest) Path(pkg string) string {
	return filepath.Join(g.base, "src", pkg)
}

// Current working directory.
func (g *GopathTest) Current() string {
	return g.current
}
func (g *GopathTest) onClean(f func()) {
	g.cleaners = append(g.cleaners, f)
}
func (g *GopathTest) Clean() {
	os.Chdir(g.initPath)
	g.Log("run Clean")
	for i := len(g.cleaners) - 1; i >= 0; i-- {
		func(i int) {
			defer recover()
			g.cleaners[i]()
		}(i)
	}
	g.cleaners = nil
	err := os.RemoveAll(g.base)
	if err != nil {
		g.Log("failed to remove temp dir", g.base, err)
	}
}

// Check is fatal to the test if err is not nil.
func (g *GopathTest) Check(err error) {
	if err == nil {
		return
	}
	g.Fatal(err)
}

type FileSpec struct {
	Pkg     string
	PkgName string
	Name    string
	Imports []string
	Build   string
}

var fileSpecFile = template.Must(template.New("").Funcs(map[string]interface{}{
	"imp": func(s string) string {
		return "`" + s + "`"
	},
}).Parse(` {{if .Build}}
// +build {{.Build}}
{{end}}
package {{.PkgName}}

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
func FilePkgBuild(name, pkgName, build string, imports ...string) FileSpec {
	return FileSpec{Name: name, PkgName: pkgName, Build: build, Imports: imports}
}

func (g *GopathTest) Setup(at string, files ...FileSpec) {
	var err error
	pkg := g.mksrc(at)
	for _, f := range files {
		f.Pkg = at
		if len(f.PkgName) == 0 {
			_, f.PkgName = path.Split(f.Pkg)
		}
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
