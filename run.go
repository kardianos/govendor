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
	"strings"

	"github.com/kardianos/vendor/rewrite"
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

func parseStatus(s string) (status []rewrite.ListStatus, err error) {
	switch {
	case strings.HasPrefix("external", s):
		status = []rewrite.ListStatus{rewrite.StatusExternal}
	case strings.HasPrefix("internal", s):
		status = []rewrite.ListStatus{rewrite.StatusInternal}
	case strings.HasPrefix("unused", s):
		status = []rewrite.ListStatus{rewrite.StatusUnused}
	case strings.HasPrefix("missing", s):
		status = []rewrite.ListStatus{rewrite.StatusMissing}
	case strings.HasPrefix("local", s):
		status = []rewrite.ListStatus{rewrite.StatusLocal}
	case strings.HasPrefix("std", s):
		status = []rewrite.ListStatus{rewrite.StatusStd}
	default:
		err = fmt.Errorf("unknown status %q", s)
	}
	return
}

// run is isoloated from main and os.Args to help with testing.
// Shouldn't directly print to console, just write through w.
func run(w io.Writer, appArgs []string) (help bool, err error) {
	if len(appArgs) == 1 {
		return true, nil
	}
	cmd := appArgs[1]
	switch cmd {
	case "init":
		err = rewrite.CmdInit()
	case "list":
		status := []rewrite.ListStatus{rewrite.StatusExternal, rewrite.StatusInternal, rewrite.StatusUnused, rewrite.StatusMissing, rewrite.StatusLocal}
		// Parse status.
		if len(appArgs) >= 3 {
			status, err = parseStatus(appArgs[2])
			if err != nil {
				return true, err
			}
		}
		// Print all listed status.
		var list []rewrite.ListItem
		list, err = rewrite.CmdList()
		for _, item := range list {
			print := false
			for _, s := range status {
				if item.Status == s {
					print = true
					break
				}
			}
			if print {
				fmt.Fprintln(w, item)
			}
		}
	case "add", "update", "remove":
		listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
		useStatus := listFlags.Bool("status", false, "")
		err = listFlags.Parse(appArgs[2:])
		if err != nil {
			return true, err
		}
		args := listFlags.Args()
		if len(args) == 0 {
			return true, errors.New("missing status")
		}
		if *useStatus {
			statusList, err := parseStatus(args[0])
			if err != nil {
				return true, err
			}
			status := statusList[0]

			list, err := rewrite.CmdList()
			if err != nil {
				return true, err
			}
			for _, item := range list {
				if item.Status != status {
					continue
				}

				switch cmd {
				case "add":
					err = rewrite.CmdAdd(item.Path)
				case "update":
					err = rewrite.CmdUpdate(item.Path)
				case "remove":
					err = rewrite.CmdRemove(item.Path)
				}
			}
		} else {
			for _, arg := range args {
				// Expand the list based on the analysis of the import tree.
				if strings.HasSuffix(arg, "...") {
					list, err := rewrite.CmdList()
					if err != nil {
						return true, err
					}
					base := strings.TrimSuffix(arg, "...")

					for _, item := range list {
						if strings.HasPrefix(item.Path, base) == false && strings.HasPrefix(item.VendorPath, base) == false {
							continue
						}

						switch cmd {
						case "add":
							err = rewrite.CmdAdd(item.Path)
						case "update":
							err = rewrite.CmdUpdate(item.Path)
						case "remove":
							err = rewrite.CmdRemove(item.Path)
						}
					}
				} else {
					switch cmd {
					case "add":
						err = rewrite.CmdAdd(arg)
					case "update":
						err = rewrite.CmdUpdate(arg)
					case "remove":
						err = rewrite.CmdRemove(arg)
					}
				}
			}
		}
	default:
		return true, fmt.Errorf("Unknown command %q", cmd)
	}
	return false, err
}
