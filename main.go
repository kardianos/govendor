// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// vendor tool to copy external source code to the local repository.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/kardianos/vendor/rewrite"

	kp "gopkg.in/alecthomas/kingpin.v1"
)

var (
	initCmd = kp.Command("init", "create a new internal folder and vendor file")

	add       = kp.Command("add", "vendor a new package")
	addImport = add.Arg("import", "import path").Required().String()

	update       = kp.Command("update", "update an existing package")
	updateImport = update.Arg("import", "import path").Required().String()

	remove       = kp.Command("remove", "un-vendor existing package")
	removeImport = remove.Arg("import", "import path").Required().String()

	list       = kp.Command("list", "list imports for packages")
	listStatus = list.Arg("status", "[e]xternal, [i]nternal, [u]nused").String()
)

func main() {
	var err error
	switch kp.Parse() {
	case "init":
		err = rewrite.CmdInit()
	case "add":
		err = rewrite.CmdAdd(*addImport)
	case "update":
		err = rewrite.CmdUpdate(*updateImport)
	case "remove":
		err = rewrite.CmdRemove(*removeImport)
	case "list":
		var list []rewrite.ListItem
		var status []rewrite.ListStatus
		list, err = rewrite.CmdList()
		switch {
		case len(*listStatus) == 0:
			status = []rewrite.ListStatus{rewrite.StatusExternal, rewrite.StatusInternal, rewrite.StatusUnused, rewrite.StatusMissing, rewrite.StatusLocal}
		case strings.HasPrefix("external", *listStatus):
			status = []rewrite.ListStatus{rewrite.StatusExternal}
		case strings.HasPrefix("internal", *listStatus):
			status = []rewrite.ListStatus{rewrite.StatusInternal}
		case strings.HasPrefix("unused", *listStatus):
			status = []rewrite.ListStatus{rewrite.StatusUnused}
		case strings.HasPrefix("missing", *listStatus):
			status = []rewrite.ListStatus{rewrite.StatusMissing}
		case strings.HasPrefix("local", *listStatus):
			status = []rewrite.ListStatus{rewrite.StatusLocal}
		case strings.HasPrefix("std", *listStatus):
			status = []rewrite.ListStatus{rewrite.StatusStd}
		default:
			kp.UsageErrorf("Unknown status to print: %s", *listStatus)
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
				fmt.Println(item)
			}
		}
	default:
		kp.Usage()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
