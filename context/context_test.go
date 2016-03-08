// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	. "github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/internal/gt"
	"github.com/kardianos/govendor/pkgspec"
)

var relVendorFile = filepath.Join("vendor", "vendor.json")

func ctx(g *gt.GopathTest) *Context {
	c, err := NewContext(g.Current(), relVendorFile, "vendor", false)
	if err != nil {
		g.Fatal(err)
	}
	return c
}

func pkg(s string) *pkgspec.Pkg {
	ps, err := pkgspec.Parse("", s)
	if err != nil {
		panic("unable to parse import package")
	}
	return ps
}

func list(g *gt.GopathTest, c *Context, name, expected string) {
	list, err := c.Status()
	if err != nil {
		g.Fatal(err)
	}
	output := &bytes.Buffer{}
	for _, item := range list {
		output.WriteString(statusItemString(item))
		output.WriteRune('\n')
	}
	if strings.TrimSpace(output.String()) != strings.TrimSpace(expected) {
		g.Fatalf("(%s) Got\n%s", name, output.String())
	}
}

func verifyChecksum(g *gt.GopathTest, c *Context, name string) {
	list, err := c.VerifyVendor()
	g.Check(err)
	if len(list) != 0 {
		names := make([]string, len(list))
		for i := range list {
			names[i] = list[i].Path
		}
		g.Errorf("(%s) Failed to verify checksum %q", name, names)
	}
}

func tree(g *gt.GopathTest, c *Context, name, expected string) {
	tree := make([]string, 0, 6)
	filepath.Walk(g.Current(), func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		tree = append(tree, strings.TrimPrefix(path, g.Current()))
		return nil
	})
	sort.Strings(tree)

	output := &bytes.Buffer{}
	for _, item := range tree {
		output.WriteString(item)
		output.WriteRune('\n')
	}
	if strings.TrimSpace(output.String()) != strings.TrimSpace(expected) {
		g.Fatalf("(%s) Got\n%s", name, output.String())
	}
}
func statusItemString(li StatusItem) string {
	if li.Local == li.Canonical {
		return fmt.Sprintf("%s %s < %q", li.Status.String(), li.Canonical, li.ImportedBy)
	}
	return fmt.Sprintf("%s %s [%s] < %q", li.Status.String(), li.Local, li.Canonical, li.ImportedBy)
}

func vendorFile(g *gt.GopathTest, expected string) {
	buf, err := ioutil.ReadFile(filepath.Join(g.Current(), relVendorFile))
	if err != nil {
		g.Fatal(err)
	}
	if string(buf) != expected {
		g.Fatal("Got: ", string(buf))
	}
}

func showRewriteRule(c *Context, t *testing.T) {
	for from, to := range c.RewriteRule {
		t.Logf("R: %s to %s\n", from, to)
	}
}
func showCurrentVendorFile(c *Context, t *testing.T) {
	t.Log("Vendor File (memory)")
	for _, vp := range c.VendorFile.Package {
		if vp.Remove {
			continue
		}
		t.Logf("\tPKG:: P: %s, O: %s\n", vp.Path, vp.Origin)
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
	list(g, c, "initial", `
 e  co2/pk1 < ["co1/pk1"]
 e  co2/pk2 < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  strings < ["co2/pk1" "co2/pk2"]
`)
}

func TestTest(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("bar.go", "co2/pk1"),
	)
	g.Setup("co2/pk1",
		gt.File("foo.go", "strings"),      // norm	al file
		gt.File("test.go", "bytes"),       // normal file
		gt.File("foo_test.go", "testing"), // test file
	)
	g.In("co1")
	c := ctx(g)
	c.VendorFile.Ignore = "test" // 	ignore test files
	c.WriteVendorFile()
	c = ctx(g)

	g.Check(c.ModifyImport(pkg("co2/pk1"), AddUpdate))
	g.Check(c.Alter())

	list(g, c, "after", `
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/vendor/co2/pk1"]
 s  strings < ["co1/vendor/co2/pk1"]
`)
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
	c := ctx(g)
	statusList, err := c.Status()
	g.Check(err)
	for _, item := range statusList {
		if item.Status.Location != LocationExternal {
			continue
		}
		g.Check(c.ModifyImport(pkg(item.Local), AddUpdate))
	}
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	vendorFile(g, `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "1wArEyRQSnOYA1LDiCNvZxF4sm8=",
			"path": "co3/pk3",
			"revision": ""
		}
	]
}
`)

	list(g, c, "co2 list", `
 v  co2/vendor/co3/pk3 [co3/pk3] < ["co2/pk2"]
 l  co2/pk2 < []
 s  strings < ["co2/vendor/co3/pk3"]
`)

	g.In("co1")
	c = ctx(g)
	list(g, c, "co1 pre list", `
 e  co2/pk2 < ["co1/pk1"]
 e  co2/vendor/co3/pk3 [co3/pk3] < ["co2/pk2"]
 e  co3/pk3 < ["co1/pk1"]
 l  co1/pk1 < []
 s  strings < ["co2/vendor/co3/pk3" "co3/pk3"]
`)

	statusList, err = c.Status()
	g.Check(err)
	for _, item := range statusList {
		if item.Status.Location != LocationExternal {
			continue
		}
		g.Check(c.ModifyImport(pkg(item.Local), AddUpdate))
	}
	c.ResloveApply(ResolveAutoLongestPath(c.Check())) // Automaically resolve conflicts.
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	vendorFile(g, `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "KrGLRMVV0FyxFX0FI4NavEDVJlY=",
			"path": "co2/pk2",
			"revision": ""
		},
		{
			"checksumSHA1": "1wArEyRQSnOYA1LDiCNvZxF4sm8=",
			"origin": "co1/vendor/co3/pk3",
			"path": "co3/pk3",
			"revision": ""
		}
	]
}
`)
	verifyChecksum(g, c, "after add")

	expected := `
 v  co1/vendor/co2/pk2 [co2/pk2] < ["co1/pk1"]
 v  co1/vendor/co3/pk3 [co3/pk3] < ["co1/pk1" "co1/vendor/co2/pk2"]
 l  co1/pk1 < []
 s  strings < ["co1/vendor/co3/pk3"]
`

	list(g, c, "co1 list 1", expected)
	c = ctx(g)
	list(g, c, "co1 list 2", expected)

	// Now remove one import.
	g.Check(c.ModifyImport(pkg("co3/pk3"), Remove))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	list(g, c, "co1 remove", `
 v  co1/vendor/co2/pk2 [co2/pk2] < ["co1/pk1"]
 e  co3/pk3 < ["co1/pk1" "co1/vendor/co2/pk2"]
 l  co1/pk1 < []
 s  strings < ["co3/pk3"]
`)
}

func TestVendorProgram(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "strings"),
	)
	g.Setup("co2/main",
		gt.File("b.go", "bytes"),
	)
	g.In("co1")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co2/main"), AddUpdate))

	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 list", `
 pv  co1/vendor/co2/main [co2/main] < []
 l  co1/pk1 < []
 s  bytes < ["co1/vendor/co2/main"]
 s  strings < ["co1/pk1"]
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
	g.Check(c.ModifyImport(pkg("co2/pk1"), AddUpdate))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	expected := `
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 e  co2/pk2 < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  strings < ["co1/vendor/co2/pk1" "co2/pk2"]
`
	list(g, c, "same", expected)

	c = ctx(g)
	list(g, c, "new", expected)

	vendorFile(g, `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "uL2Z45bjLtrTugQclzHmwbmiTb4=",
			"path": "co2/pk1",
			"revision": ""
		}
	]
}
`)
	verifyChecksum(g, c, "new")

	// Now remove an import.
	g.Check(c.ModifyImport(pkg("co2/pk1"), Remove))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	list(g, c, "co1 remove", `
 e  co2/pk1 < ["co1/pk1"]
 e  co2/pk2 < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  strings < ["co2/pk1" "co2/pk2"]
`)
}

func TestUpdate(t *testing.T) {
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
	c := ctx(g)
	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.ModifyImport(pkg("co2/pk1/pk2"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 after add", `
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 v  co1/vendor/co2/pk1/pk2 [co2/pk1/pk2] < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  strings < ["co1/vendor/co2/pk1" "co1/vendor/co2/pk1/pk2"]
`)

	vendorFile(g, `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "uL2Z45bjLtrTugQclzHmwbmiTb4=",
			"path": "co2/pk1",
			"revision": ""
		},
		{
			"checksumSHA1": "n1nb7gB6rHnnWwN+27InTig/ePo=",
			"path": "co2/pk1/pk2",
			"revision": ""
		}
	]
}
`)
	verifyChecksum(g, c, "co1 after add")

	g.Setup("co2/pk1/pk2",
		gt.File("a.go", "strings", "encoding/csv"),
	)

	// Update an import.
	g.Check(c.ModifyImport(pkg("co2/pk1/pk2"), Update))
	ct := 0
	for _, op := range c.Operation {
		if op.State == OpDone {
			continue
		}
		ct++
	}
	if ct != 1 {
		t.Fatal("Must only have a single operation. Has", ct)
	}
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 after update", `
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 v  co1/vendor/co2/pk1/pk2 [co2/pk1/pk2] < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  encoding/csv < ["co1/vendor/co2/pk1/pk2"]
 s  strings < ["co1/vendor/co2/pk1" "co1/vendor/co2/pk1/pk2"]
`)

	// Now remove an import.
	g.Check(c.ModifyImport(pkg("co2/pk1"), Remove))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	list(g, c, "co1 remove", `
 v  co1/vendor/co2/pk1/pk2 [co2/pk1/pk2] < ["co1/pk1"]
 e  co2/pk1 < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  encoding/csv < ["co1/vendor/co2/pk1/pk2"]
 s  strings < ["co1/vendor/co2/pk1/pk2" "co2/pk1"]
`)
}

func TestVendor(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
		gt.File("b.go", "bytes"),
	)
	g.Setup("co2/vendor/a",
		gt.File("a.go", "strings"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "a"),
	)

	g.In("co1")
	c := ctx(g)

	list(g, c, "co1 list", `
 e  co2/pk1 < ["co1/pk1"]
 e  co2/vendor/a [a] < ["co2/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  strings < ["co2/vendor/a"]
`)

	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.ModifyImport(pkg("co2/vendor/a"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 after add", `
 v  co1/vendor/a [a] < ["co1/vendor/co2/pk1"]
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  strings < ["co1/vendor/a"]
`)

	vendorFile(g, `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "auzf5l1iVWjiCTOwR9TuaFF2Db8=",
			"origin": "co2/vendor/a",
			"path": "a",
			"revision": ""
		},
		{
			"checksumSHA1": "Ejt2NhWYzgcLKV1gpBW3Py9aF5w=",
			"path": "co2/pk1",
			"revision": ""
		}
	]
}
`)
	verifyChecksum(g, c, "co1 after add")
}

func TestUnused(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.Setup("co1/vendor/a",
		gt.File("a.go", "encoding/csv"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "bytes"),
	)
	g.Setup("co3/pk1",
		gt.File("a.go", "strings"),
	)

	g.In("co1")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.ModifyImport(pkg("co3/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 after add", `
 vu co1/vendor/a [a] < []
 vu co1/vendor/co3/pk1 [co3/pk1] < []
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/vendor/co2/pk1"]
 s  encoding/csv < ["co1/vendor/a"]
 s  strings < ["co1/vendor/co3/pk1"]
`)
}

func TestMissing(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1", "co3/pk1"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "bytes"),
	)
	g.Setup("co3/pk1",
		gt.File("a.go", "strings"),
	)

	g.In("co1")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.ModifyImport(pkg("co3/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 after add", `
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 v  co1/vendor/co3/pk1 [co3/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/vendor/co2/pk1"]
 s  strings < ["co1/vendor/co3/pk1"]
`)

	g.In("co1")
	c = ctx(g)

	g.Remove("co1/vendor/co2/pk1")

	g.Remove("co1/vendor/co3/pk1")
	g.Remove("co3/pk1")

	list(g, c, "co1 after remove", `
  m co3/pk1 < ["co1/pk1"]
 e  co2/pk1 < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co2/pk1"]
`)
}

func TestVendorFile(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
		gt.File("b.go", "bytes"),
	)
	g.Setup("a",
		gt.File("a.go", "strings"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "a"),
	)

	g.In("co2")
	c := ctx(g)
	g.Check(c.ModifyImport(pkg("a"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	// Ensure we import from vendor folder.
	// TODO: add test with "a" in GOPATH to ensure vendor folder pref.
	g.Remove("a")

	g.In("co1")
	c = ctx(g)
	list(g, c, "co1 list", `
 e  co2/pk1 < ["co1/pk1"]
 e  co2/vendor/a [a] < ["co2/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  strings < ["co2/vendor/a"]
`)

	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.ModifyImport(pkg("co2/vendor/a"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 after add", `
 v  co1/vendor/a [a] < ["co1/vendor/co2/pk1"]
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  strings < ["co1/vendor/a"]
`)

	vendorFile(g, `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "auzf5l1iVWjiCTOwR9TuaFF2Db8=",
			"origin": "co2/vendor/a",
			"path": "a",
			"revision": ""
		},
		{
			"checksumSHA1": "Ejt2NhWYzgcLKV1gpBW3Py9aF5w=",
			"path": "co2/pk1",
			"revision": ""
		}
	]
}
`)
	verifyChecksum(g, c, "co1 after add")
}

func TestTagList(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
		gt.File("a_test.go", "testing", "bytes"),
		gt.FileBuild("b.go", "appengine", "encoding/hex"),
	)
	g.Setup("co2/pk1",
		gt.File("a_not_test_foo.go", "encoding/csv"),
		gt.File("a_test.go", "testing", "encoding/json"),
		gt.FileBuild("b.go", "appengine", "encoding/binary"),
	)
	g.In("co1")
	c := ctx(g)
	c.IgnoreBuild("test appengine")

	list(g, c, "co1 list", `
 e  co2/pk1 < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  encoding/csv < ["co2/pk1"]
 s  encoding/hex < ["co1/pk1"]
 s  testing < ["co1/pk1"]
`)
}

func TestTagAdd(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
		gt.File("a_test.go", "fmt"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "strings"),
		gt.File("a_test.go", "bytes", "testing"),
		gt.FileBuild("b.go", "appengine", "encoding/csv"),
	)
	g.In("co1")
	c := ctx(g)
	c.IgnoreBuild("test appengine")

	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	// Test update after add. Update should behave the same.
	g.Check(c.ModifyImport(pkg("co2/pk1"), Update))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 after add", `
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  fmt < ["co1/pk1"]
 s  strings < ["co1/vendor/co2/pk1"]
`)

	checkPathBase := filepath.Join(g.Current(), "vendor", "co2", "pk1")
	if _, err := os.Stat(filepath.Join(checkPathBase, "a_test.go")); err == nil {
		t.Error("a_test.go should not be copied into vendor folder")
	}
	if _, err := os.Stat(filepath.Join(checkPathBase, "b.go")); err == nil {
		t.Error("b.go should not be copied into vendor folder")
	}
}

func TestRemove(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "strings"),
	)
	g.In("co1")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	g.In("co2/pk1")
	// Change directory to let windows delete directory.
	current := g.Current()
	g.In("co1")
	err := os.RemoveAll(current)
	if err != nil {
		g.Fatal(err)
	}

	g.Check(c.ModifyImport(pkg("co2/pk1"), Remove))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	vi, err := os.Stat(filepath.Join(c.RootDir, c.VendorFolder))
	if err != nil {
		t.Fatal("vendor folder should still be present", err)
	}
	if vi.IsDir() == false {
		t.Fatal("vendor folder is not a dir")
	}
}

func TestAddMissing(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.In("co1")
	c := ctx(g)

	err := c.ModifyImport(pkg("co2/pk1"), Add)
	if _, is := err.(ErrNotInGOPATH); !is {
		t.Fatalf("Expected not in GOPATH error. Got %v", err)
	}
}

func TestTree(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "strings"),
	)
	g.Setup("co2/pk1/c_code",
		gt.File("stub.c"),
	)
	g.Setup("co2/pk1/go_code",
		gt.File("stub.go", "strings"),
	)
	g.In("co1")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co2/pk1/^"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 after add list", `
 vt co1/vendor/co2/pk1/go_code [co2/pk1/go_code] < []
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  strings < ["co1/vendor/co2/pk1" "co1/vendor/co2/pk1/go_code"]
`)
	tree(g, c, "co1 after add tree", `
/pk1/a.go
/vendor/co2/pk1/a.go
/vendor/co2/pk1/c_code/stub.c
/vendor/co2/pk1/go_code/stub.go
/vendor/vendor.json
`)

	vendorFile(g, `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "2pIxVvLJ4iUMSmTWHwnAINQwI6A=",
			"path": "co2/pk1",
			"revision": "",
			"tree": true
		}
	]
}
`)
	verifyChecksum(g, c, "add tree")

	g.Check(c.ModifyImport(pkg("co2/pk1"), Remove))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	tree(g, c, "co1 after remove tree", `
/pk1/a.go
/vendor/vendor.json
`)
}

func TestBadImport(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.Setup("co2/pk1",
		gt.File("b.go", `
****************************************************************************
                 XXXX requires Go 1.X or later.
****************************************************************************
`),
	)
	g.In("co1")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	tree(g, c, "co1 after add", `
/pk1/a.go
/vendor/co2/pk1/b.go
/vendor/vendor.json
`)
}

func TestLicenseSimple(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.Setup("co2",
		gt.File("LICENSE"),
	)
	g.Setup("co2/go/pk1",
		gt.File("b.go", "strings"),
	)
	g.In("co1")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co2/go/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	tree(g, c, "co1 after add", `
/pk1/a.go
/vendor/co2/LICENSE
/vendor/co2/go/pk1/b.go
/vendor/vendor.json
`)

	g.Check(c.ModifyImport(pkg("co2/go/pk1"), Remove))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	tree(g, c, "co1 after remove", `
/pk1/a.go
/vendor/vendor.json
`)
}
func TestLicenseNested(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.Setup("co2/pk1",
		gt.File("b.go", "co3/pk1"),
	)
	g.Setup("co3",
		gt.File("LICENSE"),
	)
	g.Setup("co3/pk1",
		gt.File("c.go", "strings"),
	)
	g.In("co2")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co3/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	tree(g, c, "co2 after add", `
/pk1/b.go
/vendor/co3/LICENSE
/vendor/co3/pk1/c.go
/vendor/vendor.json
`)
	g.In("co1")
	c = ctx(g)

	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.ModifyImport(pkg("co2/vendor/co3/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	tree(g, c, "co1 after add", `
/pk1/a.go
/vendor/co2/pk1/b.go
/vendor/co3/LICENSE
/vendor/co3/pk1/c.go
/vendor/vendor.json
`)
}
