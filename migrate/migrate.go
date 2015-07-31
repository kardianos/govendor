// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package migrate transforms a repository from a given vendor schema to
// the vendor folder schema.
package migrate

import (
	"errors"
	"os"
	"path/filepath"
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
	if hasDirs(root, "src", "vendor") {
		return sys, nil
	}
	return nil, nil
}
func (sysGb) Migrate(root string) error {
	// Move files from "src" to first GOPATH.
	// Move vendor files from "vendor/src" to "vendor".
	// Translate "vendor/manifest" to vendor.json file.
	return nil
}

type sysGodep struct{}

func (sys sysGodep) Check(root string) (system, error) {
	if hasDirs(root, "Godeps") {
		return sys, nil
	}
	return nil, nil
}
func (sysGodep) Migrate(root string) error {
	// Un-rewrite import paths.
	// Copy files from Godeps/_workspace/src to "vendor".
	// Translate Godeps/Godeps.json to vendor.json.
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
	return nil
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
