// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// vendor tool to copy external source code to the local repository.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kardianos/vendor/rewrite"
)

var help = `vendor: copy go packages locally and re-write imports.
vendor init
vendor list [status]
vendor {add, update, remove} [-status] <import or status>

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
	vendor add -status external
	vendor update -status ext
	vendor remove -status internal
`

func printHelpExit(errs ...error) {
	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	fmt.Fprint(os.Stderr, help)
	os.Exit(1)
}

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

func main() {
	var err error
	if len(os.Args) == 1 {
		printHelpExit()
	}
	cmd := os.Args[1]
	switch cmd {
	case "init":
		err = rewrite.CmdInit()
	case "list":
		status := []rewrite.ListStatus{rewrite.StatusExternal, rewrite.StatusInternal, rewrite.StatusUnused, rewrite.StatusMissing, rewrite.StatusLocal}
		// Parse status.
		if len(os.Args) >= 3 {
			status, err = parseStatus(os.Args[2])
			if err != nil {
				printHelpExit(err)
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
				fmt.Println(item)
			}
		}
	case "add", "update", "remove":
		listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
		useStatus := listFlags.Bool("status", false, "")
		err = listFlags.Parse(os.Args[2:])
		if err != nil {
			printHelpExit(err)
		}
		args := listFlags.Args()
		if len(args) == 0 {
			printHelpExit(fmt.Errorf("missing status"))
		}
		if *useStatus {
			statusList, err := parseStatus(args[0])
			if err != nil {
				printHelpExit(err)
			}
			status := statusList[0]

			list, err := rewrite.CmdList()
			if err != nil {
				printHelpExit(err)
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
	default:
		printHelpExit()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
