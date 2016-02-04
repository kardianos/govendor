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
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	. "github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/migrate"
)

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
	
govendor [fmt, build, install, clean, test] ( +status or import-path-filter )
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
	$ govendor list +local,!prog

`

var helpList = `govendor list [options]  ( +status or import-path-filter )
	List all dependencies and packages in folder tree.
	Options:
		-v           verbose listing, show dependencies of each package
		-no-status   do not prefix status to list, package names only
Examples:
	$ govendor list -no-status +local
	$ govendor list +vend,prog +local,program
	$ govendor list +local,!prog
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

var (
	outside = []Status{
		{Location: LocationExternal},
		{Presence: PresenceMissing},
	}
	normal = []Status{
		{Location: LocationExternal},
		{Location: LocationVendor},
		{Location: LocationLocal},
		{Location: LocationNotFound},
	}
	all = []Status{
		{Location: LocationStandard},
		{Location: LocationExternal},
		{Location: LocationVendor},
		{Location: LocationLocal},
		{Location: LocationNotFound},
	}
)

func statusGroupFromList(list []Status, and, not bool) StatusGroup {
	sg := StatusGroup{
		Not: not,
		And: and,
	}
	for _, s := range list {
		sg.Status = append(sg.Status, s)
	}
	return sg
}

func parseStatusGroup(statusString string) (sg StatusGroup, err error) {
	ss := strings.Split(statusString, ",")
	sg.And = true
	for _, s := range ss {
		st := Status{}
		if strings.HasPrefix(s, "!") {
			st.Not = true
			s = strings.TrimPrefix(s, "!")
		}
		var list []Status
		switch {
		case strings.HasPrefix("external", s):
			st.Location = LocationExternal
		case strings.HasPrefix("vendor", s):
			st.Location = LocationVendor
		case strings.HasPrefix("unused", s):
			st.Presence = PresenceUnsued
		case strings.HasPrefix("missing", s):
			st.Presence = PresenceMissing
		case strings.HasPrefix("local", s):
			st.Location = LocationLocal
		case strings.HasPrefix("program", s):
			st.Type = TypeProgram
		case strings.HasPrefix("std", s):
			st.Location = LocationStandard
		case strings.HasPrefix("standard", s):
			st.Location = LocationStandard
		case strings.HasPrefix("all", s):
			list = all
		case strings.HasPrefix("normal", s):
			list = normal
		case strings.HasPrefix("outside", s):
			list = outside
		default:
			err = fmt.Errorf("unknown status %q", s)
			return
		}
		if len(list) == 0 {
			sg.Status = append(sg.Status, st)
		} else {
			sg.Group = append(sg.Group, statusGroupFromList(list, false, st.Not))
		}
	}
	return
}

type filterImport struct {
	Import string
	Added  bool // Used to prevent imports from begin added twice.
}

func (f *filterImport) String() string {
	return f.Import
}

type filter struct {
	Status StatusGroup
	Import []*filterImport
}

func (f filter) String() string {
	return fmt.Sprintf("status %q, import: %q", f.Status, f.Import)
}

func (f filter) HasStatus(item StatusItem) bool {
	return item.Status.MatchGroup(f.Status)
}
func (f filter) HasImport(item StatusItem) bool {
	for _, imp := range f.Import {
		if imp.Import == item.Local || imp.Import == item.Canonical {
			imp.Added = true
			return true
		}
		if strings.HasSuffix(imp.Import, "/...") {
			base := strings.TrimSuffix(imp.Import, "/...")
			if strings.HasPrefix(item.Local, base) || strings.HasPrefix(item.Canonical, base) {
				imp.Added = true
				return true
			}
		}
		if strings.HasSuffix(imp.Import, "...") {
			base := strings.TrimSuffix(imp.Import, "...")
			if strings.HasPrefix(item.Local, base) || strings.HasPrefix(item.Canonical, base) {
				imp.Added = true
				return true
			}
		}
	}
	return false
}

func parseFilter(args []string) (filter, error) {
	f := filter{
		Import: make([]*filterImport, 0, len(args)),
	}
	for _, a := range args {
		if len(a) == 0 {
			continue
		}
		// Check if item is a status.
		if a[0] == '+' {
			sg, err := parseStatusGroup(a[1:])
			if err != nil {
				return f, err
			}
			f.Status.Group = append(f.Status.Group, sg)
		} else {
			f.Import = append(f.Import, &filterImport{Import: a})
		}
	}
	return f, nil
}

func insertListToAllNot(sg *StatusGroup, list []Status) {
	if len(sg.Group) == 0 {
		allStatusNot := true
		for _, s := range sg.Status {
			if s.Not == false {
				allStatusNot = false
				break
			}
		}
		if allStatusNot {
			sg.Group = append(sg.Group, statusGroupFromList(list, false, false))
		}
	}
	for i := range sg.Group {
		insertListToAllNot(&sg.Group[i], list)
	}
}

type HelpMessage byte

const (
	MsgNone HelpMessage = iota
	MsgFull
	MsgList
	MsgAdd
	MsgUpdate
	MsgRemove
	MsgFetch
	MsgMigrate
)

// run is isoloated from main and os.Args to help with testing.
// Shouldn't directly print to console, just write through w.
// TODO (DT): replace bool with const help type.
func run(w io.Writer, appArgs []string) (HelpMessage, error) {
	if len(appArgs) == 1 {
		return MsgFull, nil
	}

	cmd := appArgs[1]
	switch cmd {
	case "init":
		ctx, err := NewContextWD(RootWD)
		if err != nil {
			return MsgNone, err
		}
		ctx.VendorFile.Ignore = "test" // Add default ignore rule.
		err = ctx.WriteVendorFile()
		if err != nil {
			return MsgNone, err
		}
		err = os.MkdirAll(filepath.Join(ctx.RootDir, ctx.VendorFolder), 0777)
		if err != nil {
			return MsgNone, err
		}
	case "list":
		listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
		verbose := listFlags.Bool("v", false, "verbose")
		noStatus := listFlags.Bool("no-status", false, "do not show the status")
		err := listFlags.Parse(appArgs[2:])
		if err != nil {
			return MsgList, err
		}
		args := listFlags.Args()
		f, err := parseFilter(args)
		if err != nil {
			return MsgList, err
		}
		insertListToAllNot(&f.Status, normal)
		// fmt.Printf("Status: %q\n", f.Status)

		// Print all listed status.
		ctx, err := NewContextWD(RootVendorOrWD)
		if err != nil {
			return checkNewContextError(err)
		}
		list, err := ctx.Status()
		if err != nil {
			return MsgNone, err
		}

		formatSame := "%[1]v %[2]s\n"
		formatDifferent := "%[1]v %[2]s\n"
		if *verbose {
			formatDifferent = "%[1]v %[2]s ::%[3]s\n"
		}
		if *noStatus {
			formatSame = "%[2]s\n"
			formatDifferent = "%[2]s\n"
			if *verbose {
				formatDifferent = "%[2]s ::%[3]s\n"
			}
		}
		for _, item := range list {
			if f.HasStatus(item) == false {
				continue
			}
			if len(f.Import) != 0 && f.HasImport(item) == false {
				continue
			}

			if item.Local == item.Canonical {
				fmt.Fprintf(w, formatSame, item.Status, item.Canonical)
			} else {
				fmt.Fprintf(w, formatDifferent, item.Status, item.Canonical, strings.TrimPrefix(item.Local, ctx.RootImportPath))
			}
			if *verbose {
				for i, imp := range item.ImportedBy {
					if i != len(item.ImportedBy)-1 {
						fmt.Fprintf(w, "    ├── %s\n", imp)
					} else {
						fmt.Fprintf(w, "    └── %s\n", imp)
					}
				}
			}
		}
	case "add", "update", "remove", "fetch":
		msg := MsgFull
		var mod Modify

		switch cmd {
		case "add":
			msg = MsgAdd
			mod = Add
		case "update":
			msg = MsgUpdate
			mod = Update
		case "remove":
			msg = MsgRemove
			mod = Remove
		case "fetch":
			msg = MsgFetch
			// TODO: enable a code path that fetches recursivly on missing status.
			mod = Fetch
		}
		listFlags := flag.NewFlagSet("mod", flag.ContinueOnError)
		dryrun := listFlags.Bool("n", false, "dry-run")
		short := listFlags.Bool("short", false, "choose the short path")
		long := listFlags.Bool("long", false, "choose the long path")
		tree := listFlags.Bool("tree", false, "copy all folders including and under selected folder")
		err := listFlags.Parse(appArgs[2:])
		if err != nil {
			return msg, err
		}
		if *short && *long {
			return MsgNone, errors.New("cannot select both long and short path")
		}
		args := listFlags.Args()
		if len(args) == 0 {
			return msg, errors.New("missing package or status")
		}
		f, err := parseFilter(args)
		if err != nil {
			return msg, err
		}
		ctx, err := NewContextWD(RootVendor)
		if err != nil {
			return checkNewContextError(err)
		}
		list, err := ctx.Status()
		if err != nil {
			return msg, err
		}

		addTree := func(s string) string {
			if !*tree {
				return s
			}
			if strings.HasSuffix(s, TreeSuffix) {
				return s
			}
			return path.Join(s, TreeSuffix)
		}

		for _, item := range list {
			if f.HasStatus(item) {
				err = ctx.ModifyImport(addTree(item.Local), mod)
				if err != nil {
					// Skip these errors if from status.
					if _, is := err.(ErrTreeChildren); is {
						continue
					}
					if _, is := err.(ErrTreeParents); is {
						continue
					}
					return MsgNone, err
				}
			}
			if f.HasImport(item) {
				err = ctx.ModifyImport(addTree(item.Local), mod)
				if err != nil {
					return MsgNone, err
				}
			}
		}
		// If import path was not added from list, then add in here.
		for _, imp := range f.Import {
			if imp.Added {
				continue
			}
			importPath := strings.TrimSuffix(imp.Import, "...")
			importPath = strings.TrimSuffix(importPath, "/")

			err = ctx.ModifyImport(addTree(importPath), mod)
			if err != nil {
				return MsgNone, err
			}
		}

		// Auto-resolve package conflicts.
		conflicts := ctx.Check()
		conflicts = ctx.ResolveAutoVendorFileOrigin(conflicts)
		if *long {
			conflicts = ResolveAutoLongestPath(conflicts)
		}
		if *short {
			conflicts = ResolveAutoShortestPath(conflicts)
		}
		ctx.ResloveApply(conflicts)

		// TODO: loop through conflicts to see if there are any remaining conflicts.
		// Print out any here.

		if *dryrun {
			for _, op := range ctx.Operation {
				if len(op.Dest) == 0 {
					fmt.Fprintf(w, "Remove %q\n", op.Src)
				} else {
					fmt.Fprintf(w, "Copy %q -> %q\n", op.Src, op.Dest)
					for _, ignore := range op.IgnoreFile {
						fmt.Fprintf(w, "\tIgnore %q\n", ignore)
					}
				}
			}
			return MsgNone, nil
		}

		// Write out vendor file and do change.
		err = ctx.WriteVendorFile()
		if err != nil {
			return MsgNone, err
		}
		err = ctx.Alter()
		if err != nil {
			return MsgNone, err
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
				return MsgMigrate, fmt.Errorf("Unknown migrate command %q", args[0])
			}
		}
		return MsgNone, migrate.MigrateWD(from)
	case "fmt", "build", "install", "clean", "test":
		return goCmd(cmd, appArgs[2:])
	default:
		return MsgFull, fmt.Errorf("Unknown command %q", cmd)
	}
	return MsgNone, nil
}

func goCmd(subcmd string, args []string) (HelpMessage, error) {
	ctx, err := NewContextWD(RootVendorOrWD)
	if err != nil {
		return MsgNone, err
	}
	statusArgs := make([]string, 0, len(args))
	otherArgs := make([]string, 1, len(args)+1)
	otherArgs[0] = subcmd

	for _, a := range args {
		if a[0] == '+' {
			statusArgs = append(statusArgs, a)
		} else {
			otherArgs = append(otherArgs, a)
		}
	}
	f, err := parseFilter(statusArgs)
	if err != nil {
		return MsgNone, err
	}
	list, err := ctx.Status()
	if err != nil {
		return MsgNone, err
	}

	for _, item := range list {
		if f.HasStatus(item) {
			otherArgs = append(otherArgs, item.Local)
		}
	}
	cmd := exec.Command("go", otherArgs...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return MsgNone, cmd.Run()
}

func checkNewContextError(err error) (HelpMessage, error) {
	// Diagnose error, show current value of 1.5vendor, suggest alter.
	if err == nil {
		return MsgNone, nil
	}
	if _, is := err.(ErrMissingVendorFile); is {
		err = fmt.Errorf(`%v

Ensure the current folder or a parent folder contains a folder named "vendor".
If in doubt, run "govendor init" in the project root.
`, err)
		return MsgNone, err
	}
	return MsgNone, err
}
