// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// vendor tool to copy external source code to the local vendor folder.
/*
govendor: copy go packages locally. Uses vendor folder.
govendor init
govendor list [-v] [-no-status] [+<status>] [import-path-filter]
govendor {add, update, remove} [-n] [-short | -long] [+status] [import-path-filter]
govendor migrate [auto, godep, internal]

	init
		create a vendor file if it does not exist.

	add
		copy one or more packages into the vendor folder.

	update
		update one or more packages from GOPATH into the vendor folder.

	remove
		remove one or more packages from the vendor folder.

	migrate
		change from a one schema to use the vendor folder.

Expanding "..."
	A package import path may be expanded to other paths that
	show up in "govendor list" be ending the "import-path" with "...".
	NOTE: this uses the import tree from "vendor list" and NOT the file system.

Flags
	-n		print actions but do not run them
	-short	chooses the shorter path in case of conflict
	-long	chooses the longer path in case of conflict

Status list:
	external - package does not share root path
	vendor - vendor folder; copied locally
	unused - the package has been copied locally, but isn't used
	local - shares the root path and is not a vendor package
	missing - referenced but not found in GOROOT or GOPATH
	std - standard library package
	program - package is a main package
	---
	all - all of the above status

Status can be referenced by their initial letters.
	"st" == "std"
	"e" == "external"

Ignoring files with build tags:
	The "vendor.json" file contains a string field named "ignore".
	It may contain a space separated list of build tags to ignore when
	listing and copying files. By default the init command adds the
	the "test" tag to the ignore list.

Example:
	govendor add github.com/kardianos/osext
	govendor update github.com/kardianos/...
	govendor add +external
	govendor update +ven github.com/company/project/... bitbucket.org/user/pkg
	govendor remove +vendor
	govendor list +ext +std

If using go1.5, ensure you set GO15VENDOREXPERIMENT=1
*/
package main

import (
	"fmt"
	"os"
)

func main() {
	printHelp, err := run(os.Stdout, os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	if printHelp {
		fmt.Fprint(os.Stderr, help)
	}
	if printHelp || err != nil {
		os.Exit(1)
	}
}
