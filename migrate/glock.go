// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package migrate

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/pkgspec"
)

func init() {
	register("glock", sysGlock{})
}

type sysGlock struct{}

func (sys sysGlock) Check(root string) (system, error) {
	if hasFiles(root, "GLOCKFILE") {
		return sys, nil
	}
	return nil, nil
}
func (sysGlock) Migrate(root string) error {
	err := os.MkdirAll(filepath.Join(root, "vendor"), 0777)
	if err != nil {
		return err
	}
	filebytes, err := ioutil.ReadFile(filepath.Join(root, "GLOCKFILE"))
	if err != nil {
		return err
	}
	lines := strings.Split(string(filebytes), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}

	/*
		vf := &vendorfile.File{}
		vf.Package = make([]*vendorfile.Package, 0, len(lines))
	*/
	ctx, err := context.NewContext(root, filepath.Join("vendor", "vendor.json"), "vendor", false)
	if err != nil {
		return err
	}

	const cmdPrefix = "cmd "

	for _, l := range lines {
		if len(l) == 0 {
			continue
		}
		isCmd := strings.HasPrefix(l, cmdPrefix)
		if isCmd {
			continue
		}
		field := strings.Fields(l)
		if len(field) < 2 {
			continue
		}
		ps, err := pkgspec.Parse("", field[0]+"@"+field[1])
		if err != nil {
			return err
		}
		ps.IncludeTree = true
		err = ctx.ModifyImport(ps, context.Fetch)
		if err != nil {
			return err
		}
	}
	for _, l := range lines {
		if len(l) == 0 {
			continue
		}
		isCmd := strings.HasPrefix(l, cmdPrefix)
		if !isCmd {
			continue
		}
		path := strings.TrimPrefix(l, cmdPrefix)
		ps, err := pkgspec.Parse("", path)
		if err != nil {
			return err
		}
		err = ctx.ModifyImport(ps, context.Fetch)
		if err != nil {
			return err
		}
	}
	err = ctx.WriteVendorFile()
	os.Remove(filepath.Join(root, "GLOCKFILE"))
	return err
}
