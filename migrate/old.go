// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package migrate

import (
	"os"
	"path/filepath"

	"github.com/kardianos/govendor/context"
)

func init() {
	register("internal", sysInternal{})
	register("old-vendor", sysOldVendor{})
}

type sysInternal struct{}

func (sys sysInternal) Check(root string) (system, error) {
	vendorFolder := "internal"
	override := os.Getenv("GOVENDORFOLDER")
	if len(override) != 0 {
		vendorFolder = override
	}
	if hasDirs(root, vendorFolder) && hasFiles(root, filepath.Join(vendorFolder, "vendor.json")) {
		return sys, nil
	}
	return nil, nil
}
func (sysInternal) Migrate(root string) error {
	// Un-rewrite import paths.
	// Copy files from internal to vendor.
	// Update and move vendor file from "internal/vendor.json" to "vendor.json".
	ctx, err := context.NewContext(root, filepath.Join("internal", "vendor.json"), "internal", true)
	if err != nil {
		return err
	}
	list, err := ctx.Status()
	if err != nil {
		return err
	}
	remove := make([]string, 0, len(list))
	for _, item := range list {
		if item.Status.Location != context.LocationVendor {
			continue
		}
		pkg := ctx.Package[item.Local]
		ctx.Operation = append(ctx.Operation, &context.Operation{
			Pkg:  pkg,
			Src:  pkg.Dir,
			Dest: filepath.Join(ctx.RootDir, "vendor", filepath.ToSlash(item.Pkg.Path)),
		})
		remove = append(remove, filepath.Join(ctx.RootGopath, filepath.ToSlash(item.Local)))
		ctx.RewriteRule[item.Local] = item.Pkg.Path
	}
	ctx.VendorFilePath = filepath.Join(ctx.RootDir, "vendor", "vendor.json")
	err = ctx.WriteVendorFile()
	if err != nil {
		return err
	}
	err = ctx.Alter()
	if err != nil {
		return err
	}

	// Remove existing.
	for _, r := range remove {
		err = context.RemovePackage(r, "", false)
		if err != nil {
			return err
		}
	}
	return os.Remove(filepath.Join(ctx.RootDir, "internal", "vendor.json"))
}

type sysOldVendor struct{}

func (sys sysOldVendor) Check(root string) (system, error) {
	if hasDirs(root, "vendor") && hasFiles(root, "vendor.json") {
		return sys, nil
	}
	return nil, nil
}
func (sysOldVendor) Migrate(root string) error {
	ctx, err := context.NewContext(root, "vendor.json", "vendor", false)
	if err != nil {
		return err
	}
	ctx.VendorFilePath = filepath.Join(ctx.RootDir, "vendor", "vendor.json")
	err = ctx.WriteVendorFile()
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(ctx.RootDir, "vendor.json"))
}
