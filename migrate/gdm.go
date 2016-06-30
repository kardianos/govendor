// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package migrate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/vendorfile"
)

func init() {
	register("gdm", sysGdm{})
}

type sysGdm struct{}

func (sys sysGdm) Check(root string) (system, error) {
	if hasFiles(root, "Godeps") {
		return sys, nil
	}
	return nil, nil
}

func (sys sysGdm) Migrate(root string) error {
	gdmFilePath := filepath.Join(root, "Godeps")

	ctx, err := context.NewContext(root, filepath.Join("vendor", "vendor.json"), "vendor", false)
	if err != nil {
		return err
	}
	ctx.VendorFile.Ignore = "test"

	f, err := os.Open(gdmFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	pkgs, err := sys.parseGdmFile(f)
	if err != nil {
		return err
	}
	ctx.VendorFile.Package = pkgs

	if err := ctx.WriteVendorFile(); err != nil {
		return err
	}

	return os.RemoveAll(gdmFilePath)
}

func (sysGdm) parseGdmFile(r io.Reader) ([]*vendorfile.Package, error) {
	var pkgs []*vendorfile.Package
	for {
		var path, rev string
		if _, err := fmt.Fscanf(r, "%s %s\n", &path, &rev); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		pkgs = append(pkgs, &vendorfile.Package{
			Add:      true,
			Path:     path,
			Revision: rev,
			Tree:     true,
		})
	}

	return pkgs, nil
}
