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
	"strings"

	. "github.com/kardianos/govendor/context"
)

var help = `vendor: copy go packages locally and re-write imports.
vendor init
vendor list [status]
vendor {add, update, remove} [-status] <import-path or status>

	init
		create a vendor file if it does not exist.
	
	add
		copy one or more packages into the internal folder and re-write paths.
	
	update
		update one or more packages from GOPATH into the internal folder.
	
	remove
		remove one or more packages from the internal folder and re-write packages to vendor paths.

Expanding "..."
	A package import path may be expanded to other paths that
	show up in "vendor list" be ending the "import-path" with "...".
	NOTE: this uses the import tree from "vendor list" and NOT the file system.

Status list:
	external - package does not share root path
	internal - in vendor file; copied locally
	unused - the package has been copied locally, but isn't used
	local - shares the root path and is not a vendor package
	missing - referenced but not found in GOROOT or GOPATH
	std - standard library package

Status can be referenced by their initial letters.
	"st" == "std"
	"e" == "external"
	
Example:
	vendor add github.com/kardianos/osext
	vendor update github.com/kardianos/...
	vendor add -status external
	vendor update -status ext
	vendor remove -status internal
`

func parseStatus(s string) (status []ListStatus, err error) {
	switch {
	case strings.HasPrefix("external", s):
		status = []ListStatus{StatusExternal}
	case strings.HasPrefix("vendor", s):
		status = []ListStatus{StatusVendor}
	case strings.HasPrefix("unused", s):
		status = []ListStatus{StatusUnused}
	case strings.HasPrefix("missing", s):
		status = []ListStatus{StatusMissing}
	case strings.HasPrefix("local", s):
		status = []ListStatus{StatusLocal}
	case strings.HasPrefix("std", s):
		status = []ListStatus{StatusStd}
	case strings.HasPrefix("program", s):
		status = []ListStatus{StatusProgram}
	default:
		err = fmt.Errorf("unknown status %q", s)
	}
	return
}

// run is isoloated from main and os.Args to help with testing.
// Shouldn't directly print to console, just write through w.
func run(w io.Writer, appArgs []string) (bool, error) {
	if len(appArgs) == 1 {
		return true, nil
	}

	// TODO: Allow specifying status mixed in with paths like "+local" or "!program" or "+all".
	// TODO: Add a "migrate" command {godep, gb, internal} to {vendor}.

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
	case "list":
		listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
		verbose := listFlags.Bool("v", false, "verbose")
		err := listFlags.Parse(appArgs[2:])
		if err != nil {
			return true, err
		}
		args := listFlags.Args()
		status := []ListStatus{StatusExternal, StatusVendor, StatusUnused, StatusMissing, StatusLocal}
		// Parse status.
		if len(args) > 0 {
			status, err = parseStatus(args[0])
			if err != nil {
				return true, err
			}
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
			print := false
			for _, s := range status {
				if item.Status == s {
					print = true
					break
				}
			}
			if print {
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
		}
	case "add", "update", "remove":
		listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
		useStatus := listFlags.Bool("status", false, "")
		err := listFlags.Parse(appArgs[2:])
		if err != nil {
			return true, err
		}
		args := listFlags.Args()
		if len(args) == 0 {
			return true, errors.New("missing status")
		}
		ctx, err := NewContextWD(false)
		if err != nil {
			return checkNewContextError(err)
		}
		list, err := ctx.Status()
		if err != nil {
			return true, err
		}

		if *useStatus {
			statusList, err := parseStatus(args[0])
			if err != nil {
				return true, err
			}
			status := statusList[0]
			for _, item := range list {
				if item.Status != status {
					continue
				}

				switch cmd {
				case "add":
					err = ctx.ModifyImport(item.Local, Add)
				case "update":
					err = ctx.ModifyImport(item.Local, Update)
				case "remove":
					err = ctx.ModifyImport(item.Local, Remove)
				}
				if err != nil {
					return false, err
				}
			}
		} else {
			for _, arg := range args {
				// Expand the list based on the analysis of the import tree.
				if strings.HasSuffix(arg, "...") {
					base := strings.TrimSuffix(arg, "...")
					for _, item := range list {
						if strings.HasPrefix(item.Local, base) == false && strings.HasPrefix(item.Canonical, base) == false {
							continue
						}

						switch cmd {
						case "add":
							err = ctx.ModifyImport(item.Local, Add)
						case "update":
							err = ctx.ModifyImport(item.Local, Update)
						case "remove":
							err = ctx.ModifyImport(item.Local, Remove)
						}
						if err != nil {
							return false, err
						}
					}
				} else {
					switch cmd {
					case "add":
						err = ctx.ModifyImport(arg, Add)
					case "update":
						err = ctx.ModifyImport(arg, Update)
					case "remove":
						err = ctx.ModifyImport(arg, Remove)
					}
					if err != nil {
						return false, err
					}
				}
			}
		}
		// Auto-resolve package conflicts.
		ctx.Reslove(ctx.Check())

		// Write out vendor file and do change.
		err = ctx.WriteVendorFile()
		if err != nil {
			return false, err
		}
		err = ctx.Alter()
		if err != nil {
			return false, err
		}
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
