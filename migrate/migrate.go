// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package migrate transforms a repository from a given vendor schema to
// the vendor folder schema.
package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type ErrNoSuchSystem struct {
	NotExist string
	Has      []string
}

func (err ErrNoSuchSystem) Error() string {
	return fmt.Sprintf("Migration system for %q doesn't exist. Current systems %q.", err.NotExist, err.Has)
}

// From is the current vendor schema.
type From string

// Migrate from the given system using the current working directory.
func MigrateWD(from From) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	return Migrate(from, wd)
}

// SystemList list available migration systems.
func SystemList() []string {
	list := make([]string, 0, len(registered))
	for key := range registered {
		list = append(list, string(key))
	}
	sort.Strings(list)
	return list
}

// Migrate from the given system using the given root.
func Migrate(from From, root string) error {
	sys, found := registered[from]
	if !found {
		return ErrNoSuchSystem{
			NotExist: string(from),
			Has:      SystemList(),
		}
	}
	sys, err := sys.Check(root)
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

func register(name From, sys system) {
	_, found := registered[name]
	if found {
		panic("system " + name + " already registered.")
	}
	registered[name] = sys
}

var registered = make(map[From]system, 10)

var errAutoSystemNotFound = errors.New("Unable to determine vendor system.")

func init() {
	register("auto", sysAuto{})
}

type sysAuto struct{}

func (auto sysAuto) Check(root string) (system, error) {
	for _, sys := range registered {
		if sys == auto {
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
