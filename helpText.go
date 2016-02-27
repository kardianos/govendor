// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

var helpFull = `govendor: copy go packages locally. Uses vendor folder.
govendor init
	Creates a vendor file if it does not exist.

govendor list [options] ( +status or import-path-filter )
	List all dependencies and packages in folder tree.
	Options:
		-v           verbose listing, show dependencies of each package
		-no-status   do not prefix status to list, package names only

govendor {add, update, remove} [options] ( +status or import-path-filter )
	add    - Copy one or more packages into the vendor folder.
	update - Update one or more packages from GOPATH into the vendor folder.
	remove - Remove one or more packages from the vendor folder.
	Options:
		-n           dry run and print actions that would be taken
		-tree        copy package(s) and all sub-folders under each package
		
		The following may be replaced with something else in the future.
		-short       if conflict, take short path 
		-long        if conflict, take long path

govendor migrate [auto, godep, internal]
	Change from a one schema to use the vendor folder. Default to auto detect.


govendor [fmt, build, install, clean, test, vet, generate] ( +status or import-path-filter )
	Run "go" commands using status filters.
	$ govendor test +local

Expanding "..."
	A package import path may be expanded to other paths that
	show up in "govendor list" be ending the "import-path" with "...".
	NOTE: this uses the import tree from "vendor list" and NOT the file system.

Flags
	-n		print actions but do not run them
	-short	chooses the shorter path in case of conflict
	-long	chooses the longer path in case of conflict
	
"import-path-filter" arguements:
	May be a literal individual package:
		github.com/user/supercool
		github.com/user/supercool/anotherpkg
	
	Match on any exising Go package that the project uses under "supercool"
		github.com/user/supercool/...
		
	Match the package "supercool" and also copy all sub-folders.
	Will copy non-Go files and Go packages that aren't used.
		github.com/user/supercool/^
	
	Same as specifying:
	-tree github.com/user/supercool

Status list used in "+<status>" arguments:
	external - package does not share root path
	vendor - vendor folder; copied locally
	unused - the package has been copied locally, but isn't used
	local - shares the root path and is not a vendor package
	missing - referenced but not found in GOROOT or GOPATH
	std - standard library package
	program - package is a main package
	---
	outside - external + missing
	all - all of the above status

Status can be referenced by their initial letters.
	"st" == "std"
	"e" == "external"

Status can be joined together with boolean AND and OR
	govendor list +vendor,program +e --> (vendor AND program) OR external

Ignoring files with build tags:
	The "vendor.json" file contains a string field named "ignore".
	It may contain a space separated list of build tags to ignore when
	listing and copying files. By default the init command adds the
	the "test" tag to the ignore list.

If using go1.5, ensure you set GO15VENDOREXPERIMENT=1

Examples:
	$ govendor list -no-status +local
	$ govendor list +vend,prog +local,program
	$ govendor list +local,^prog

`

var helpList = `govendor list [options]  ( +status or import-path-filter )
	List all dependencies and packages in folder tree.
	Options:
		-v           verbose listing, show dependencies of each package
		-no-status   do not prefix status to list, package names only
Examples:
	$ govendor list -no-status +local
	$ govendor list +vend,prog +local,program
	$ govendor list +local,^prog
`

var helpAdd = `govendor add [options] ( +status or import-path-filter )
	Copy one or more packages into the vendor folder.
	Options:
		-n           dry run and print actions that would be taken
		-tree        copy package(s) and all sub-folders under each package
		
		The following may be replaced with something else in the future.
		-short       if conflict, take short path 
		-long        if conflict, take long path
`

var helpUpdate = `govendor update [options] ( +status or import-path-filter )
	Update one or more packages from GOPATH into the vendor folder.
	Options:
		-n           dry run and print actions that would be taken
		-tree        copy package(s) and all sub-folders under each package
		
		The following may be replaced with something else in the future.
		-short       if conflict, take short path 
		-long        if conflict, take long path
`

var helpRemove = `govendor remove [options] ( +status or import-path-filter )
	Remove one or more packages from the vendor folder.
	Options:
		-n           dry run and print actions that would be taken
`

var helpFetch = `govendor fetch <TBD>
`

var helpMigrate = `govendor migrate [auto, godep, internal]
	Change from a one schema to use the vendor folder. Default to auto detect.
`
