// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package migrate transforms a repository from a given vendor schema to
// the vendor folder schema.
package migrate

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/vendorfile"
)

// From is the current vendor schema.
type From byte

const (
	Auto     From = iota // Detect which system it uses.
	Gb                   // Dave's GB
	Godep                // tools/godep
	Internal             // kardianos/govendor
)

// Migrate from the given system using the current working directory.
func MigrateWD(from From) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	return Migrate(from, wd)
}

// Migrate from the given system using the given root.
func Migrate(from From, root string) error {
	sys, err := register[from].Check(root)
	if err != nil {
		return err
	}
	if sys == nil {
		return errors.New("Root not found.")
	}
	return sys.Migrate(root)
}

type system interface {
	Check(root string) (system, error)
	Migrate(root string) error
}

var register = map[From]system{
	Auto:     sysAuto{},
	Gb:       sysGb{},
	Godep:    sysGodep{},
	Internal: sysInternal{},
}

var errAutoSystemNotFound = errors.New("Unable to determine vendor system.")

type sysAuto struct{}

func (sysAuto) Check(root string) (system, error) {
	for from, sys := range register {
		if from == Auto {
			continue
		}
		out, err := sys.Check(root)
		if err != nil {
			return nil, err
		}
		if out != nil {
			return out, nil
		}
	}
	return nil, errAutoSystemNotFound
}
func (sysAuto) Migrate(root string) error {
	return errors.New("Auto.Migrate shouldn't be called")
}

type sysGb struct{}

func (sys sysGb) Check(root string) (system, error) {
	if hasDirs(root, "src", filepath.Join("vendor", "src")) {
		return sys, nil
	}
	return nil, nil
}
func (sysGb) Migrate(root string) error {
	// Move files from "src" to first GOPATH.
	// Move vendor files from "vendor/src" to "vendor".
	// Translate "vendor/manifest" to vendor.json file.

	_ = context.CopyPackage
	return errors.New("Migrate gb not implemented")
}

type sysGodep struct{}

func (sys sysGodep) Check(root string) (system, error) {
	if hasDirs(root, "Godeps") {
		return sys, nil
	}
	return nil, nil
}
func (sysGodep) Migrate(root string) error {
	// Determine if import paths are rewriten.
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
		if item.Status != context.StatusVendor {
			continue
		}
		ctx.Operation = append(ctx.Operation, &context.Operation{
			Pkg:  ctx.Package[item.Local],
			Dest: filepath.Join(ctx.RootDir, "vendor", filepath.ToSlash(item.Canonical)),
		})
		remove = append(remove, filepath.Join(ctx.RootGopath, filepath.ToSlash(item.Local)))
		ctx.RewriteRule[item.Local] = item.Canonical
	}
	ctx.VendorFilePath = filepath.Join(ctx.RootDir, "vendor.json")
	for _, vf := range ctx.VendorFile.Package {
		vf.Local = path.Join("vendor", vf.Canonical)
	}

	ctx.VendorDiscoverFolder = "vendor"

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
			if pkg.Status != context.StatusVendor {
				continue
			}
			if strings.HasPrefix(pkg.Canonical, d.ImportPath) == false {
				continue
			}
			vf := ctx.VendorFilePackageCanonical(pkg.Canonical)
			if vf == nil {
				ctx.VendorFile.Package = append(ctx.VendorFile.Package, &vendorfile.Package{
					Add:       true,
					Canonical: pkg.Canonical,
					Local:     path.Join(ctx.VendorDiscoverFolder, pkg.Canonical),
					Comment:   d.Comment,
					Revision:  d.Rev,
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
		err = context.RemovePackage(r, "")
		if err != nil {
			return err
		}
	}

	return nil
}

type sysInternal struct{}

func (sys sysInternal) Check(root string) (system, error) {
	if hasDirs(root, "internal") && hasFiles(root, filepath.Join("internal", "vendor.json")) {
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
		if item.Status != context.StatusVendor {
			continue
		}
		ctx.Operation = append(ctx.Operation, &context.Operation{
			Pkg:  ctx.Package[item.Local],
			Dest: filepath.Join(ctx.RootDir, "vendor", filepath.ToSlash(item.Canonical)),
		})
		remove = append(remove, filepath.Join(ctx.RootGopath, filepath.ToSlash(item.Local)))
		ctx.RewriteRule[item.Local] = item.Canonical
	}
	ctx.VendorFilePath = filepath.Join(ctx.RootDir, "vendor.json")
	for _, vf := range ctx.VendorFile.Package {
		vf.Local = path.Join("vendor", vf.Canonical)
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
		err = context.RemovePackage(r, "")
		if err != nil {
			return err
		}
	}
	return os.Remove(filepath.Join(ctx.RootDir, "internal", "vendor.json"))
}

func hasDirs(root string, dd ...string) bool {
	for _, d := range dd {
		fi, err := os.Stat(filepath.Join(root, d))
		if err != nil {
			return false
		}
		if fi.IsDir() == false {
			return false
		}
	}
	return true
}

func hasFiles(root string, dd ...string) bool {
	for _, d := range dd {
		fi, err := os.Stat(filepath.Join(root, d))
		if err != nil {
			return false
		}
		if fi.IsDir() == true {
			return false
		}
	}
	return true
}
