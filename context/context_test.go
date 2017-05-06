// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kardianos/govendor/internal/gt"
	"github.com/kardianos/govendor/internal/pathos"
	"github.com/kardianos/govendor/pkgspec"
)

var relVendorFile = filepath.Join("vendor", "vendor.json")

func ctx(g *gt.GopathTest) *Context {
	c, err := NewContext(g.Current(), relVendorFile, "vendor", false)
	if err != nil {
		g.Fatal(err)
	}
	c.Insecure = true
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
	if ok, msg := stringSameIgnoreSpace(output.String(), expected); !ok {
		g.Fatalf("%s(%s) Got\n%s", msg, name, output.String())
	}
}

func stringSameIgnoreSpace(a, b string, ignore ...string) (bool, string) {
	// Remove any space padding on the start/end of each line.
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	aLine := strings.Split(a, "\n")
	bLine := strings.Split(b, "\n")
	ct := len(aLine)
	if len(bLine) < ct {
		ct = len(bLine)
	}
lineLoop:
	for i := 0; i < ct; i++ {
		al := strings.TrimSpace(aLine[i])
		bl := strings.TrimSpace(bLine[i])
		if al != bl {
			for _, ignorePrefix := range ignore {
				if strings.HasPrefix(al, ignorePrefix) && strings.HasPrefix(bl, ignorePrefix) {
					continue lineLoop
				}
			}
			return false, fmt.Sprintf("A: %s\nB: %s\n", al, bl)
		}
	}
	if len(aLine) != len(bLine) {
		return false, "Different Line Count"
	}
	return true, ""
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

func tree(g *gt.GopathTest, name, expected string) {
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
	if strings.TrimSpace(strings.Replace(output.String(), "\\", "/", -1)) != strings.TrimSpace(expected) {
		g.Fatalf("(%s) Got\n%s", name, output.String())
	}
}
func statusItemString(li StatusItem) string {
	if li.Local == li.Pkg.Path {
		return fmt.Sprintf("%s %s < %q", li.Status.String(), li.Pkg.Path, li.ImportedBy)
	}
	return fmt.Sprintf("%s %s [%s] < %q", li.Status.String(), li.Local, li.Pkg.Path, li.ImportedBy)
}

func vendorFile(g *gt.GopathTest, name, expected string, ignore ...string) {
	buf, err := ioutil.ReadFile(filepath.Join(g.Current(), relVendorFile))
	if err != nil {
		g.Fatal(err)
	}
	s := string(buf)
	if ok, msg := stringSameIgnoreSpace(s, expected, ignore...); !ok {
		g.Fatalf("(%s) \n%sGot:\n%s\nWant\n%s", name, msg, s, expected)
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

	g.Check(c.ModifyStatus(StatusGroup{
		Status: []Status{{Location: LocationExternal}},
	}, AddUpdate))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	vendorFile(g, "", `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "1wArEyRQSnOYA1LDiCNvZxF4sm8=",
			"path": "co3/pk3",
			"revision": ""
		}
	],
	"rootPath": "co2"
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

	g.Check(c.ModifyStatus(StatusGroup{
		Status: []Status{{Location: LocationExternal}},
	}, AddUpdate))
	c.ResloveApply(ResolveAutoLongestPath(c.Check())) // Automaically resolve conflicts.
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	vendorFile(g, "", `{
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
	],
	"rootPath": "co1"
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
		gt.File("b.go", "bytes", "co3/pk1", "notfound"),
	)
	g.Setup("co3/pk1",
		gt.File("a.go", "fmt"),
	)
	g.In("co1")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co2/main"), AddUpdate))

	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	c = ctx(g)

	list(g, c, "co1 list", `
pv  co1/vendor/co2/main [co2/main] < []
 e  co3/pk1 < ["co1/vendor/co2/main"]
 l  co1/pk1 < []
 s  bytes < ["co1/vendor/co2/main"]
 s  fmt < ["co3/pk1"]
 s  strings < ["co1/pk1"]
  m notfound < ["co1/vendor/co2/main"]
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

	vendorFile(g, "", `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "uL2Z45bjLtrTugQclzHmwbmiTb4=",
			"path": "co2/pk1",
			"revision": ""
		}
	],
	"rootPath": "co1"
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

func TestNoDep(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	// This test relies on the file list being sorted. Normally not required.
	testNeedsSortOrder = true

	g.Setup("co1/pk1",
		gt.File("a.go", "strings"),
	)
	// Include some non-go files that sort before and after the go file.
	// Triggers an edge case where the package wasn't added.
	g.Setup("co2/pk1",
		gt.File("a.txt", "fmt"),
		gt.File("b.go", "bytes"),
		gt.File("z.txt", "fmt"),
	)
	g.In("co1")
	c := ctx(g)
	list(g, c, "before", `
 l  co1/pk1 < []
 s  strings < ["co1/pk1"]
`)
	g.Check(c.ModifyImport(pkg("co2/pk1"), AddUpdate))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	expected := `
 vu co1/vendor/co2/pk1 [co2/pk1] < []
 l  co1/pk1 < []
 s  bytes < ["co1/vendor/co2/pk1"]
 s  strings < ["co1/pk1"]
	`
	list(g, c, "same", expected)

	c = ctx(g)
	list(g, c, "new", expected)

	vendorFile(g, "", `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "QkjrJA3p/33sLeZnPIazB4vv30o=",
			"path": "co2/pk1",
			"revision": ""
		}
	],
	"rootPath": "co1"
}
`)
	verifyChecksum(g, c, "new")
}

func TestMainTest(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	// This test relies on the file list being sorted. Normally not required.
	testNeedsSortOrder = true

	g.Setup("co1/pk1",
		gt.File("a.go", "strings"),
	)
	// Include some non-go files that sort before and after the go file.
	// Triggers an edge case where the package wasn't added.
	g.Setup("co2/pk1",
		gt.FilePkgBuild("a.go", "main_test", "", "testing"),
		gt.FilePkgBuild("b.go", "main", "", "fmt"),
	)
	g.In("co1")
	c := ctx(g)
	list(g, c, "before", `
 l  co1/pk1 < []
 s  strings < ["co1/pk1"]
`)
	g.Check(c.ModifyImport(pkg("co2/pk1"), AddUpdate))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "after", `
pv  co1/vendor/co2/pk1 [co2/pk1] < []
 l  co1/pk1 < []
 s  fmt < ["co1/vendor/co2/pk1"]
 s  strings < ["co1/pk1"]
 s  testing < ["co1/vendor/co2/pk1"]
`)

	c.IgnoreBuildAndPackage("test")

	list(g, c, "ignore test", `
pv  co1/vendor/co2/pk1 [co2/pk1] < []
 l  co1/pk1 < []
 s  fmt < ["co1/vendor/co2/pk1"]
 s  strings < ["co1/pk1"]
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

	vendorFile(g, "a", `{
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
	],
	"rootPath": "co1"
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

	vendorFile(g, "b", `
{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "uL2Z45bjLtrTugQclzHmwbmiTb4=",
			"path": "co2/pk1",
			"revision": ""
		},
		{
			"checksumSHA1": "0GMhcCYB/xH0CWDPYljKj3W7ylY=",
			"path": "co2/pk1/pk2",
			"revision": ""
		}
	],
	"rootPath": "co1"
}
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
 s  strings < ["co2/pk1" "co1/vendor/co2/pk1/pk2"]
`)

	vendorFile(g, "c", `
{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "0GMhcCYB/xH0CWDPYljKj3W7ylY=",
			"path": "co2/pk1/pk2",
			"revision": ""
		}
	],
	"rootPath": "co1"
}
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

	vendorFile(g, "", `{
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
	],
	"rootPath": "co1"
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
	g.Setup("co1/vendor/cmd/main",
		gt.File("a.go", "strings"),
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
pv  co1/vendor/cmd/main [cmd/main] < []
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 vu co1/vendor/co3/pk1 [co3/pk1] < []
 l  co1/pk1 < []
 s  bytes < ["co1/vendor/co2/pk1"]
 s  encoding/csv < ["co1/vendor/a"]
 s  strings < ["co1/vendor/cmd/main" "co1/vendor/co3/pk1"]
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
 e  co2/pk1 < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co2/pk1"]
  m co3/pk1 < ["co1/pk1"]
`)
}

func TestVendorFile(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1", "co3/pk1"),
		gt.File("b.go", "bytes"),
	)
	g.Setup("a",
		gt.File("a.go", "strings"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "a"),
	)
	g.Setup("co3/pk1",
		gt.File("a.go", "bytes"),
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
 e  co3/pk1 < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1" "co3/pk1"]
 s  strings < ["co2/vendor/a"]
`)

	// Write before and after, try to tickle any bugs.
	g.Check(c.WriteVendorFile())
	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.ModifyImport(pkg("co2/vendor/a"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 after co2 add", `
 v  co1/vendor/a [a] < ["co1/vendor/co2/pk1"]
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 e  co3/pk1 < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1" "co3/pk1"]
 s  strings < ["co1/vendor/a"]
`)

	vendorFile(g, "", `{
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
	],
	"rootPath": "co1"
}
`)
	verifyChecksum(g, c, "co1 after co2 add")

	// Write before and after, try to tickle any bugs.
	// Add another package. Had some issues with adding to an existing vendor file
	// with existing packages.
	g.Check(c.WriteVendorFile())
	g.Check(c.ModifyImport(pkg("co3/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 after co3 add", `
 v  co1/vendor/a [a] < ["co1/vendor/co2/pk1"]
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 v  co1/vendor/co3/pk1 [co3/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1" "co1/vendor/co3/pk1"]
 s  strings < ["co1/vendor/a"]
`)

	vendorFile(g, "", `{
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
		},
		{
			"checksumSHA1": "Y7kuSBw+31U5RgCTp4XAMsWwr5Y=",
			"path": "co3/pk1",
			"revision": ""
		}
	],
	"rootPath": "co1"
}
`)
	verifyChecksum(g, c, "co1 after co3 add")
}

func TestTestdata(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.Setup("co2/pk1",
		gt.File("b.go", "encoding/csv"),
	)
	g.Setup("co2/pk1/testdata",
		gt.File("file-a"),
	)
	g.Setup("co2/pk1/testdata/sub",
		gt.File("file-b"),
	)
	g.In("co1")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	list(g, c, "co1 list", `
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  encoding/csv < ["co1/vendor/co2/pk1"]
`)

	tree(g, "co1 after add testdata", `
/pk1/a.go
/vendor/co2/pk1/b.go
/vendor/co2/pk1/testdata/file-a
/vendor/co2/pk1/testdata/sub/file-b
/vendor/vendor.json
`)
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

	list(g, c, "co1 before ignore", `
 e  co2/pk1 < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  encoding/binary < ["co2/pk1"]
 s  encoding/csv < ["co2/pk1"]
 s  encoding/hex < ["co1/pk1"]
 s  encoding/json < ["co2/pk1"]
 s  testing < ["co1/pk1" "co2/pk1"]
`)
	c.IgnoreBuildAndPackage("test appengine")

	list(g, c, "co1 list", `
 e  co2/pk1 < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  encoding/csv < ["co2/pk1"]
 s  encoding/hex < ["co1/pk1"]
 s  testing < ["co1/pk1"]
`)

	c.IgnoreBuildAndPackage("")

	g.Check(c.ModifyStatus(StatusGroup{
		Status: []Status{{Location: LocationExternal}},
	}, AddUpdate))
	g.Check(c.Alter())

	list(g, c, "after co1 before ignore", `
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  encoding/binary < ["co1/vendor/co2/pk1"]
 s  encoding/csv < ["co1/vendor/co2/pk1"]
 s  encoding/hex < ["co1/pk1"]
 s  encoding/json < ["co1/vendor/co2/pk1"]
 s  testing < ["co1/pk1" "co1/vendor/co2/pk1"]
`)
	c.IgnoreBuildAndPackage("test appengine")

	list(g, c, "after co1 list", `
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/pk1"]
 s  encoding/csv < ["co1/vendor/co2/pk1"]
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
	g.Setup("co2/pk1/testdata",
		gt.File("file-a"),
	)
	g.In("co1")
	c := ctx(g)
	c.IgnoreBuildAndPackage("test appengine")

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

	tree(g, "co1 after add co2", `
/pk1/a.go
/pk1/a_test.go
/vendor/co2/pk1/a.go
/vendor/vendor.json
`)
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

	list(g, c, "pre", `
 l  co1/pk1 < []
  m co2/pk1 < ["co1/pk1"]
`)

	err := c.ModifyImport(pkg("co2/pk1"), Add)
	if _, is := err.(ErrNotInGOPATH); !is {
		t.Errorf("Expected not in GOPATH error. Got %v", err)
	}
	list(g, c, "post", `
 l  co1/pk1 < []
  m co2/pk1 < ["co1/pk1"]
`)
}

func TestSymlinkedGopath(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.In("co1")

	base := os.Getenv("GOPATH")

	sPath := base + "_symlink"
	os.Symlink(base, sPath)
	defer os.Remove(sPath)

	os.Setenv("GOPATH", sPath)

	c := ctx(g)

	found := false
	for _, gopath := range c.GopathList {
		if pathos.FileHasPrefix(gopath, base) {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Original base path %s is not in the GopathList", base)
	}

	resolvedPath, err := filepath.EvalSymlinks(sPath)
	if err != nil {
		t.Errorf("Error converting %s to symlink: %s", sPath, err)
	}

	if !pathos.FileHasPrefix(c.RootGopath, resolvedPath) {
		t.Errorf("c.RootGopath(%s) should include symlink(%s)", c.RootGopath, resolvedPath)
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
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 vt co1/vendor/co2/pk1/go_code [co2/pk1/go_code] < []
 l  co1/pk1 < []
 s  strings < ["co1/vendor/co2/pk1" "co1/vendor/co2/pk1/go_code"]
`)
	tree(g, "co1 after add tree", `
/pk1/a.go
/vendor/co2/pk1/a.go
/vendor/co2/pk1/c_code/stub.c
/vendor/co2/pk1/go_code/stub.go
/vendor/vendor.json
`)

	vendorFile(g, "", `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "2pIxVvLJ4iUMSmTWHwnAINQwI6A=",
			"path": "co2/pk1",
			"revision": "",
			"tree": true
		}
	],
	"rootPath": "co1"
}
`)
	verifyChecksum(g, c, "add tree")

	g.Check(c.ModifyImport(pkg("co2/pk1"), Remove))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	tree(g, "co1 after remove tree", `
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

	tree(g, "co1 after add", `
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

	tree(g, "co1 after add", `
/pk1/a.go
/vendor/co2/LICENSE
/vendor/co2/go/pk1/b.go
/vendor/vendor.json
`)

	g.Check(c.ModifyImport(pkg("co2/go/pk1"), Remove))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	tree(g, "co1 after remove", `
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

	tree(g, "co2 after add", `
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

	tree(g, "co1 after add", `
/pk1/a.go
/vendor/co2/pk1/b.go
/vendor/co3/LICENSE
/vendor/co3/pk1/c.go
/vendor/vendor.json
`)
}

func TestOriginDir(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1", "co2/pk1/sub1"),
	)
	g.Setup("co3/vendor/co2/pk1",
		gt.File("a.go", "strings"),
	)
	g.Setup("co3/vendor/co2/pk1/sub1",
		gt.File("b.go", "bytes"),
	)
	g.In("co1")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co2/pk1/...::co3/vendor/co2/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	tree(g, "pre", `
/pk1/a.go
/vendor/co2/pk1/a.go
/vendor/co2/pk1/sub1/b.go
/vendor/vendor.json
`)

	vendorFile(g, "", `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "uL2Z45bjLtrTugQclzHmwbmiTb4=",
			"origin": "co3/vendor/co2/pk1",
			"path": "co2/pk1",
			"revision": ""
		},
		{
			"checksumSHA1": "9lQcNSYn9fe09txkREelZh/RSyw=",
			"origin": "co3/vendor/co2/pk1/sub1",
			"path": "co2/pk1/sub1",
			"revision": ""
		}
	],
	"rootPath": "co1"
}
`)

	afterAlterList := `
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 v  co1/vendor/co2/pk1/sub1 [co2/pk1/sub1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co1/vendor/co2/pk1/sub1"]
 s  strings < ["co1/vendor/co2/pk1"]
`
	list(g, c, "pre list", afterAlterList)

	c = ctx(g)

	list(g, c, "post list", afterAlterList)

	found := false
	for _, pkg := range c.Package {
		if pkg.Path == "co2/pk1" {
			found = true
			if pkg.Origin != "co3/vendor/co2/pk1" {
				t.Errorf("wrong origin, got %q", pkg.Origin)
			}
			if !strings.HasSuffix(strings.Replace(pkg.OriginDir, "\\", "/", -1), "src/co3/vendor/co2/pk1") {
				t.Errorf("wrong originDir, got %q", pkg.OriginDir)
			}
		}
	}
	if !found {
		t.Errorf("Package with path \"co2/pk1\" is not found")
	}
}

func TestRelativePath(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	// Two different versions should be listed
	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1", "co3/pk1"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "strings"),
	)
	g.Setup("co3/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.Setup("co3/vendor/co2/pk1",
		gt.File("a.go", "bytes"),
	)
	g.In("co1")
	c := ctx(g)

	list(g, c, "co1 list", `
 e  co2/pk1 < ["co1/pk1"]
 e  co3/pk1 < ["co1/pk1"]
 e  co3/vendor/co2/pk1 [co2/pk1] < ["co3/pk1"]
 l  co1/pk1 < []
 s  bytes < ["co3/vendor/co2/pk1"]
 s  strings < ["co2/pk1"]
`)
}

func TestAddTreeWithSamePrefix(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go"),
	)
	g.Setup("co2/pk1-1",
		gt.File("a.go"),
	)
	g.In("co1")
	c := ctx(g)
	g.Check(c.ModifyImport(pkg("co2/pk1"), Add, IncludeTree))
	g.Check(c.Alter())
	g.Check(c.ModifyImport(pkg("co2/pk1-1"), Add, IncludeTree))
	g.Check(c.Alter())
}

func TestAddTreeWithSamePrefix2(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go"),
	)
	g.Setup("co2/pk1-1",
		gt.File("a.go"),
	)
	g.In("co1")
	c := ctx(g)
	g.Check(c.ModifyImport(pkg("co2/pk1-1"), Add, IncludeTree))
	g.Check(c.Alter())
	g.Check(c.ModifyImport(pkg("co2/pk1"), Add, IncludeTree))
	g.Check(c.Alter())
}

func TestOrginRetain(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go"),
	)
	g.Setup("co2/pk2",
		gt.File("a.go"),
	)
	g.In("co1")
	c := ctx(g)
	g.Check(c.ModifyImport(pkg("correct/name/pk2::co2/pk2"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	vendorFile(g, "after add", `
{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "KcwRyEXPUn2jwAgWGhFDjIt3deI=",
			"origin": "co2/pk2",
			"path": "correct/name/pk2",
			"revision": ""
		}
	],
	"rootPath": "co1"
}
`)

	g.Check(c.ModifyImport(pkg("correct/name/pk2"), Update))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	vendorFile(g, "after update", `
{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "KcwRyEXPUn2jwAgWGhFDjIt3deI=",
			"origin": "co2/pk2",
			"path": "correct/name/pk2",
			"revision": ""
		}
	],
	"rootPath": "co1"
}
`)
}
