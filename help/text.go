// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package help

import (
	"strings"

	"github.com/kardianos/govendor/migrate"
)

//go:generate govendor license -o licenses.go -template gen-license.template

var helpFull = `govendor (` + version + `): record dependencies and copy into vendor folder
	-govendor-licenses    Show govendor's licenses.
	-version              Show govendor version
	-cpuprofile 'file'    Writes a CPU profile to 'file' for debugging.
	-memprofile 'file'    Writes a heap profile to 'file' for debugging.

Sub-Commands

	init     Create the "vendor" folder and the "vendor.json" file.
	list     List and filter existing dependencies and packages.
	add      Add packages from $GOPATH.
	update   Update packages from $GOPATH.
	remove   Remove packages from the vendor folder.
	status   Lists any packages missing, out-of-date, or modified locally.
	fetch    Add new or update vendor folder packages from remote repository.
	sync     Pull packages into vendor folder from remote repository with revisions
  	             from vendor.json file.
	migrate  Move packages from a legacy tool to the vendor folder with metadata.
	get      Like "go get" but copies dependencies into a "vendor" folder.
	license  List discovered licenses for the given status or import paths.
	shell    Run a "shell" to make multiple sub-commands more efficient for large
	             projects.

	go tool commands that are wrapped:
	  "+status" package selection may be used with them
	fmt, build, install, clean, test, vet, generate, tool

Status Types

	+local    (l) packages in your project
	+external (e) referenced packages in GOPATH but not in current project
	+vendor   (v) packages in the vendor folder
	+std      (s) packages in the standard library

	+excluded (x) external packages explicitly excluded from vendoring
	+unused   (u) packages in the vendor folder, but unused
	+missing  (m) referenced packages but not found

	+program  (p) package is a main package

	+outside  +external +missing
	+all      +all packages

	Status can be referenced by their initial letters.

Package specifier
	<path>[::<origin>][{/...|/^}][@[<version-spec>]]

Ignoring files with build tags, or excluding packages from being vendored:
	The "vendor.json" file contains a string field named "ignore".
	It may contain a space separated list of build tags to ignore when
	listing and copying files.
	This list may also contain package prefixes (containing a "/", possibly
	as last character) to exclude when copying files in the vendor folder.
	If "foo/" appears in this field, then package "foo" and all its sub-packages
	("foo/bar", …) will be excluded (but package "bar/foo" will not).
	By default the init command adds the "test" tag to the ignore list.

If using go1.5, ensure GO15VENDOREXPERIMENT=1 is set.

`

var helpInit = `govendor init
	Create a vendor folder in the working directory and a vendor/vendor.json
	metadata file.
`

var helpList = `govendor list [options]  ( +status or import-path-filter )
	List all dependencies and packages in folder tree.
	Options:
		-v           verbose listing, show dependencies of each package
		-p           show file path to package instead of import path
		-no-status   do not prefix status to list, package names only
Examples:
	$ govendor list -no-status +local
	$ govendor list -p -no-status +local
	$ govendor list +vend,prog +local,program
	$ govendor list +local,^prog
`

var helpAdd = `govendor add [options] ( +status or import-path-filter )
	Copy one or more packages into the vendor folder from GOPATH.
	Options:
		-n           dry run and print actions that would be taken
		-tree        copy package(s) and all sub-folders under each package
		-uncommitted allows copying a package with uncommitted changes, doesn't
		             update revision or checksum so it will always be out-of-date.

		The following may be replaced with something else in the future.
		-short       if conflict, take short path
		-long        if conflict, take long path
`

var helpUpdate = `govendor update [options] ( +status or import-path-filter )
	Update one or more packages from GOPATH into the vendor folder from GOPATH.
	Options:
		-n           dry run and print actions that would be taken
		-tree        copy package(s) and all sub-folders under each package
		-uncommitted allows copying a package with uncommitted changes, doesn't
		             update revision or checksum so it will always be out-of-date.

		The following may be replaced with something else in the future.
		-short       if conflict, take short path
		-long        if conflict, take long path
`

var helpRemove = `govendor remove [options] ( +status or import-path-filter )
	Remove one or more packages from the vendor folder.
	Options:
		-n           dry run and print actions that would be taken
`

var helpFetch = `govendor fetch [options] ( +status or package-spec )
	Fetches packages directly into the vendor folder.
	package-spec = <path>[::<origin>][{/...|/^}][@[<version-spec>]]
	Options:
		-tree        copy package(s) and all sub-folders under each package
		-insecure    allow downloading over insecure connection
		-v           verbose mode
`

var helpSync = `govendor sync
	Ensures the contents of the vendor folder matches the vendor file.
	Options:
		-n           dry run, print out action only
		-insecure    allow downloading over insecure connection
		-v           verbose output
`

var helpStatus = `govendor status
	Shows any packages that are missing, out-of-date, or modified locally (according to the
	checksum) and should be sync'ed.
`

var helpMigrate = `govendor migrate [` + strings.Join(migrate.SystemList(), ", ") + `]
	Change from a one schema to use the vendor folder. Default to auto detect.
`

var helpGet = `govendor get [options] (import-path)...
	Download package into GOPATH, put all dependencies into vendor folder.
	Options:
		-insecure    allow downloading over insecure connection
		-v           verbose mode
`

var helpLicense = `govendor license [options] ( +status or package-spec )
	Attempt to find and list licenses for the specified packages.
	Options:
		-o           output to file name
		-template    template file to use, input is "[]context.License"
`
var helpShell = `govendor shell
	Open a govendor "shell". Useful for faster queries on large projects.
	Options:
		-pprof-handler    expose a pprof HTTP server on the given address
`

var msgGovendorVersion = version + `
`
