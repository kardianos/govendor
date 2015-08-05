// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// imports for this file should not contain "os".
import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	. "github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/migrate"
)

var help = `govendor: copy go packages locally and optionally re-write imports.
govendor init
govendor list [-v] [+status] [import-path-filter]
govendor {add, update, remove} [-n] [+status] [import-path-filter]
govendor migrate [auto, godep, internal]

	init
		create a vendor file if it does not exist.
	
	add
		copy one or more packages into the internal folder and re-write paths.
	
	update
		update one or more packages from GOPATH into the internal folder.
	
	remove
		remove one or more packages from the internal folder and re-write packages to vendor paths.

	migrate
		change from a one schema to use the vendor folder.

Expanding "..."
	A package import path may be expanded to other paths that
	show up in "govendor list" be ending the "import-path" with "...".
	NOTE: this uses the import tree from "vendor list" and NOT the file system.

Flags
	-n		print actions but do not run them

Status list:
	external - package does not share root path
	internal - in vendor file; copied locally
	unused - the package has been copied locally, but isn't used
	local - shares the root path and is not a vendor package
	missing - referenced but not found in GOROOT or GOPATH
	std - standard library package
	program - package is a main package
	---
	all - all of the above status
	normal - all but std status

Status can be referenced by their initial letters.
	"st" == "std"
	"e" == "external"
	
Example:
	govendor add github.com/kardianos/osext
	govendor update github.com/kardianos/...
	govendor add +external
	govendor update +ven github.com/company/project/... bitbucket.org/user/pkg
	govendor remove +vendor
	govendor list +ext +std

To opt use the standard vendor directory:
set GO15VENDOREXPERIMENT=1

When GO15VENDOREXPERIMENT=1 imports are copied to the vendor directory without
rewriting their import paths.
`

var (
	normal = []Status{StatusExternal, StatusVendor, StatusUnused, StatusMissing, StatusLocal, StatusProgram}
	all    = []Status{StatusExternal, StatusVendor, StatusUnused, StatusMissing, StatusLocal, StatusProgram, StatusStandard}
)

func parseStatus(s string) (status []Status, err error) {
	switch {
	case strings.HasPrefix("external", s):
		status = []Status{StatusExternal}
	case strings.HasPrefix("vendor", s):
		status = []Status{StatusVendor}
	case strings.HasPrefix("unused", s):
		status = []Status{StatusUnused}
	case strings.HasPrefix("missing", s):
		status = []Status{StatusMissing}
	case strings.HasPrefix("local", s):
		status = []Status{StatusLocal}
	case strings.HasPrefix("program", s):
		status = []Status{StatusProgram}
	case strings.HasPrefix("std", s):
		status = []Status{StatusStandard}
	case strings.HasPrefix("standard", s):
		status = []Status{StatusStandard}
	case strings.HasPrefix("all", s):
		status = normal
	case strings.HasPrefix("normal", s):
		status = all
	default:
		err = fmt.Errorf("unknown status %q", s)
	}
	return
}

type filter struct {
	Status []Status
	Import []string
}

func (f filter) HasStatus(item StatusItem) bool {
	for _, s := range f.Status {
		if s == item.Status {
			return true
		}
	}
	return false
}
func (f filter) HasImport(item StatusItem) bool {
	for _, imp := range f.Import {
		if imp == item.Local || imp == item.Canonical {
			return true
		}
		if strings.HasSuffix(imp, "...") {
			base := strings.TrimSuffix(imp, "...")
			if strings.HasPrefix(item.Local, base) || strings.HasPrefix(item.Canonical, base) {
				return true
			}
		}
	}
	return false
}

func parseFilter(args []string) (filter, error) {
	f := filter{
		Status: make([]Status, 0, len(args)),
		Import: make([]string, 0, len(args)),
	}
	for _, a := range args {
		if len(a) == 0 {
			continue
		}
		// Check if item is a status.
		if a[0] == '+' {
			ss, err := parseStatus(a[1:])
			if err != nil {
				return f, err
			}
			f.Status = append(f.Status, ss...)
		} else {
			f.Import = append(f.Import, a)
		}
	}
	return f, nil
}

// run is isoloated from main and os.Args to help with testing.
// Shouldn't directly print to console, just write through w.
func run(w io.Writer, appArgs []string) (bool, error) {
	if len(appArgs) == 1 {
		return true, nil
	}

	cmd := appArgs[1]
	switch cmd {
	case "init":
		ctx, err := NewContextWD(true)
		if err != nil {
			return false, err
		}
		err = ctx.WriteVendorFile()
		if err != nil {
			return false, err
		}
		err = os.MkdirAll(filepath.Join(ctx.RootDir, ctx.VendorFolder), 0777)
		if err != nil {
			return false, err
		}
	case "list":
		listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
		verbose := listFlags.Bool("v", false, "verbose")
		err := listFlags.Parse(appArgs[2:])
		if err != nil {
			return true, err
		}
		args := listFlags.Args()
		f, err := parseFilter(args)
		if err != nil {
			return true, err
		}
		if len(f.Status) == 0 {
			f.Status = append(f.Status, normal...)
		}
		// Print all listed status.
		ctx, err := NewContextWD(false)
		if err != nil {
			return checkNewContextError(err)
		}
		list, err := ctx.Status()
		if err != nil {
			return false, err
		}
		for _, item := range list {
			if f.HasStatus(item) == false {
				continue
			}
			if len(f.Import) != 0 && f.HasImport(item) == false {
				continue
			}
			if item.Local == item.Canonical {
				fmt.Fprintf(w, "%v %s\n", item.Status, item.Local)
			} else {
				fmt.Fprintf(w, "%v %s [%s]\n", item.Status, item.Local, item.Canonical)
			}
			if *verbose {
				for i, imp := range item.ImportedBy {
					if i != len(item.ImportedBy)-1 {
						fmt.Fprintf(w, "  ├── %s\n", imp)
					} else {
						fmt.Fprintf(w, "  └── %s\n", imp)
					}
				}
			}
		}
	case "add", "update", "remove":
		listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
		dryrun := listFlags.Bool("n", false, "dry-run")
		err := listFlags.Parse(appArgs[2:])
		if err != nil {
			return true, err
		}
		args := listFlags.Args()
		if len(args) == 0 {
			return true, errors.New("missing package or status")
		}
		f, err := parseFilter(args)
		if err != nil {
			return true, err
		}
		ctx, err := NewContextWD(false)
		if err != nil {
			return checkNewContextError(err)
		}
		list, err := ctx.Status()
		if err != nil {
			return true, err
		}

		var mod Modify
		switch cmd {
		case "add":
			mod = Add
		case "update":
			mod = Update
		case "remove":
			mod = Remove
		}

		for _, item := range list {
			if f.HasStatus(item) {
				err = ctx.ModifyImport(item.Local, mod)
				if err != nil {
					return false, err
				}
			}
			if f.HasImport(item) {
				err = ctx.ModifyImport(item.Local, mod)
				if err != nil {
					return false, err
				}
			}
		}
		for _, imp := range f.Import {
			err = ctx.ModifyImport(imp, mod)
			if err != nil {
				return false, err
			}
		}

		// Auto-resolve package conflicts.
		ctx.Reslove(ctx.Check())

		if *dryrun {
			for _, op := range ctx.Operation {
				if len(op.Dest) == 0 {
					fmt.Fprintf(w, "Remove %q\n", op.Src)
				} else {
					fmt.Fprintf(w, "Copy %q -> %q\n", op.Src, op.Dest)
				}
			}
			return false, nil
		}

		// Write out vendor file and do change.
		err = ctx.WriteVendorFile()
		if err != nil {
			return false, err
		}
		err = ctx.Alter()
		if err != nil {
			return false, err
		}
	case "migrate":
		args := appArgs[2:]
		from := migrate.Auto
		if len(args) > 0 {
			switch args[0] {
			case "auto":
				from = migrate.Auto
			case "gb":
				from = migrate.Gb
			case "godep":
				from = migrate.Godep
			case "internal":
				from = migrate.Internal
			default:
				return true, fmt.Errorf("Unknown migrate command %q", args[0])
			}
		}
		return false, migrate.MigrateWD(from)
	default:
		return true, fmt.Errorf("Unknown command %q", cmd)
	}
	return false, nil
}

func checkNewContextError(err error) (bool, error) {
	// Diagnose error, show current value of 1.5vendor, suggest alter.
	if err == nil {
		return false, nil
	}
	if _, is := err.(ErrMissingVendorFile); is {
		expValue := os.Getenv("GO15VENDOREXPERIMENT")
		err = fmt.Errorf(`%v

GO15VENDOREXPERIMENT=%q
It is possible this project requires changing the above env var
or the project is not initialized.
`, err, expValue)
		return false, err
	}
	return false, err
}
