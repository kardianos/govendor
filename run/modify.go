// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/kardianos/govendor/context"
)

func Modify(w io.Writer, subCmdArgs []string, mod context.Modify) (HelpMessage, error) {
	msg := MsgFull
	switch mod {
	case context.Add:
		msg = MsgAdd
	case context.Update, context.AddUpdate:
		msg = MsgUpdate
	case context.Remove:
		msg = MsgRemove
	case context.Fetch:
		msg = MsgFetch
	}
	listFlags := flag.NewFlagSet("mod", flag.ContinueOnError)
	dryrun := listFlags.Bool("n", false, "dry-run")
	short := listFlags.Bool("short", false, "choose the short path")
	long := listFlags.Bool("long", false, "choose the long path")
	tree := listFlags.Bool("tree", false, "copy all folders including and under selected folder")
	err := listFlags.Parse(subCmdArgs)
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
	ctx, err := context.NewContextWD(context.RootVendor)
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
		if strings.HasSuffix(s, context.TreeSuffix) {
			return s
		}
		return path.Join(s, context.TreeSuffix)
	}

	for _, item := range list {
		if f.HasStatus(item) {
			if mod == context.Add && ctx.VendorFilePackagePath(item.Canonical) != nil {
				continue
			}
			err = ctx.ModifyImport(addTree(item.Local), mod)
			if err != nil {
				// Skip these errors if from status.
				if _, is := err.(context.ErrTreeChildren); is {
					continue
				}
				if _, is := err.(context.ErrTreeParents); is {
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
		conflicts = context.ResolveAutoLongestPath(conflicts)
	}
	if *short {
		conflicts = context.ResolveAutoShortestPath(conflicts)
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
	err = ctx.Alter()
	if err != nil {
		return MsgNone, err
	}
	err = ctx.WriteVendorFile()
	if err != nil {
		return MsgNone, err
	}
	return MsgNone, nil
}
