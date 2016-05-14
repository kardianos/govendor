// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package migrate

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/vendorfile"
)

func init() {
	register("godep", sysGodep{})
}

type sysGodep struct{}

func (sys sysGodep) Check(root string) (system, error) {
	if hasDirs(root, "Godeps") {
		return sys, nil
	}
	return nil, nil
}
func (sysGodep) Migrate(root string) error {
	// Determine if import paths are rewritten.
	// Un-rewrite import paths.
	// Copy files from Godeps/_workspace/src to "vendor".
	// Translate Godeps/Godeps.json to vendor.json.

	vendorFilePath := filepath.Join("Godeps", "_workspace", "src")
	vendorPath := path.Join("Godeps", "_workspace", "src")
	godepFilePath := filepath.Join(root, "Godeps", "Godeps.json")

	ctx, err := context.NewContext(root, "vendor.json", vendorFilePath, true)
	if err != nil {
		return err
	}
	ctx.VendorDiscoverFolder = vendorPath

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

	ctx.VendorDiscoverFolder = "vendor"
	ctx.VendorFile.Ignore = "test"

	// Translate then remove godeps.json file.
	type Godeps struct {
		ImportPath string
		GoVersion  string   // Abridged output of 'go version'.
		Packages   []string // Arguments to godep save, if any.
		Deps       []struct {
			ImportPath string
			Comment    string // Description of commit, if present.
			Rev        string // VCS-specific commit ID.
		}
	}

	godeps := Godeps{}
	f, err := os.Open(godepFilePath)
	if err != nil {
		return err
	}
	coder := json.NewDecoder(f)
	err = coder.Decode(&godeps)
	f.Close()
	if err != nil {
		return err
	}

	for _, d := range godeps.Deps {
		for _, pkg := range ctx.Package {
			if strings.HasPrefix(pkg.Path, d.ImportPath) == false {
				continue
			}
			vf := ctx.VendorFilePackagePath(pkg.Path)
			if vf == nil {
				ctx.VendorFile.Package = append(ctx.VendorFile.Package, &vendorfile.Package{
					Add:      true,
					Path:     pkg.Path,
					Comment:  d.Comment,
					Revision: d.Rev,
				})
			}
		}
	}

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

	return os.RemoveAll(filepath.Join(root, "Godeps"))
}
