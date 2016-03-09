// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/pkgspec"
	"github.com/kardianos/govendor/prompt"
)

func Modify(w io.Writer, subCmdArgs []string, mod context.Modify, ask prompt.Prompt) (HelpMessage, error) {
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
	var err error
	/*
		// Fake example prompt.
		q := &prompt.Question{
			Error:  "An error goes here",
			Prompt: "Do you want to do this?",
			Type:   prompt.TypeSelectOne,
			Options: []prompt.Option{
				prompt.NewOption("yes", "Yes, continue", false),
				prompt.NewOption("no", "No, stop", false),
				prompt.NewOption("", "What?", true),
			},
		}
		q.Options[2].Choosen = true
		q.Options[2] = prompt.ValidateOption(q.Options[2], "Choose again!")
		resp, err := ask.Ask(q)
		if err != nil {
			return msg, err
		}
		if resp == prompt.RespCancel {
			fmt.Printf("Cancelled\n")
			return MsgNone, nil
		}
		choosen := q.AnswerSingle(true)

		fmt.Printf("Choosen: %s\n", choosen.String())
	*/

	listFlags := flag.NewFlagSet("mod", flag.ContinueOnError)
	listFlags.SetOutput(nullWriter{})
	dryrun := listFlags.Bool("n", false, "dry-run")
	short := listFlags.Bool("short", false, "choose the short path")
	long := listFlags.Bool("long", false, "choose the long path")
	tree := listFlags.Bool("tree", false, "copy all folders including and under selected folder")
	uncommitted := listFlags.Bool("uncommitted", false, "allows adding uncommitted changes. Doesn't update revision or checksum")
	err = listFlags.Parse(subCmdArgs)
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
	ctx, err := context.NewContextWD(context.RootVendor)
	if err != nil {
		return checkNewContextError(err)
	}
	cgp, err := currentGoPath(ctx)
	if err != nil {
		return msg, err
	}
	f, err := parseFilter(cgp, args)
	if err != nil {
		return msg, err
	}
	list, err := ctx.Status()
	if err != nil {
		return msg, err
	}

	addTree := func(s string) *pkgspec.Pkg {
		ps, err := pkgspec.Parse("", s)
		if err != nil {
			panic("error parsing pkg path")
		}
		if *tree {
			ps.IncludeTree = true
		}
		if *uncommitted {
			ps.Uncommitted = true
		}
		return ps
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

		if *uncommitted {
			imp.Pkg.Uncommitted = true
		}
		err = ctx.ModifyImport(imp.Pkg, mod)
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
