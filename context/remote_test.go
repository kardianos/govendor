// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	"testing"

	"github.com/kardianos/govendor/internal/gt"
)

func TestFetchSimple(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.Setup("remote/co3/vendor/co2/pk1",
		gt.File("a.go", "strings"),
	)
	g.In("remote")
	remote := gt.NewHttpHandler(g, "git")

	g.In("remote/co3")
	vcs := remote.Setup()
	commitRev, commitTime := vcs.Commit()

	g.In("co1")
	c := ctx(g)

	remoteOrigin := remote.HttpAddr() + "/remote/co3/vendor/co2/pk1"

	g.Check(c.ModifyImport(pkg("co2/pk1::"+remoteOrigin), Fetch))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	tree(g, "post", `
/pk1/a.go
/vendor/co2/pk1/a.go
/vendor/vendor.json
`)

	vendorFile(g, "", `{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "uL2Z45bjLtrTugQclzHmwbmiTb4=",
			"origin": "`+remoteOrigin+`",
			"path": "co2/pk1",
			"revision": "`+commitRev+`",
			"revisionTime": "`+commitTime+`"
		}
	],
	"rootPath": "co1"
}
`)

	list(g, c, "post list", `
 v  co1/vendor/co2/pk1 [co2/pk1] < ["co1/pk1"]
 l  co1/pk1 < []
 s  strings < ["co1/vendor/co2/pk1"]
`)

}

func TestFetchVerbose(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.Setup("remote/co3/vendor/co2/pk1",
		gt.File("a.go", "strings"),
	)
	g.In("remote")
	remote := gt.NewHttpHandler(g, "git")

	g.In("remote/co3")
	remote.Setup().Commit()

	g.In("co1")
	c := ctx(g)

	buf := &bytes.Buffer{}
	c.Logger = buf

	remoteOrigin := remote.HttpAddr() + "/remote/co3/vendor/co2/pk1"

	g.Check(c.ModifyImport(pkg("co2/pk1::"+remoteOrigin), Fetch))
	g.Check(c.Alter())

	t.Logf("Log\n%s\n", buf.Bytes())
}

func TestUpdateOrigin(t *testing.T) {
	g := gt.New(t)
	defer g.Clean()

	g.Setup("co1/pk1",
		gt.File("a.go", "co2/pk1"),
	)
	g.Setup("co2/pk1",
		gt.File("a.go", "bytes"),
	)
	g.Setup("remote/co3/vendor/co2/pk1",
		gt.File("a.go", "strings"),
	)
	g.In("remote")
	remote := gt.NewHttpHandler(g, "git")

	g.In("remote/co3")
	commitRev, commitTime := remote.Setup().Commit()

	g.In("co1")
	c := ctx(g)

	g.Check(c.ModifyImport(pkg("co2/pk1"), Add))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	remoteOrigin := remote.HttpAddr() + "/remote/co3/vendor/co2/pk1"

	g.Check(c.ModifyImport(pkg("co2/pk1::"+remoteOrigin), Fetch))
	g.Check(c.Alter())
	g.Check(c.WriteVendorFile())

	vendorFile(g, "", `
{
	"comment": "",
	"ignore": "",
	"package": [
		{
			"checksumSHA1": "uL2Z45bjLtrTugQclzHmwbmiTb4=",
			"origin": "`+remoteOrigin+`",
			"path": "co2/pk1",
			"revision": "`+commitRev+`",
			"revisionTime": "`+commitTime+`"
		}
	],
	"rootPath": "co1"
}
`)
}
