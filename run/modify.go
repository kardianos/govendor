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
	"github.com/kardianos/govendor/help"
	"github.com/kardianos/govendor/prompt"
)

func (r *runner) Modify(w io.Writer, subCmdArgs []string, mod context.Modify, ask prompt.Prompt) (help.HelpMessage, error) {
	msg := help.MsgFull
	switch mod {
	case context.Add:
		msg = help.MsgAdd
	case context.Update, context.AddUpdate:
		msg = help.MsgUpdate
	case context.Remove:
		msg = help.MsgRemove
	case context.Fetch:
		msg = help.MsgFetch
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
		q.Options[2].Chosen = true
		q.Options[2] = prompt.ValidateOption(q.Options[2], "Choose again!")
		resp, err := ask.Ask(q)
		if err != nil {
			return msg, err
		}
		if resp == prompt.RespCancel {
			fmt.Printf("Cancelled\n")
			return help.MsgNone, nil
		}
		chosen := q.AnswerSingle(true)

		fmt.Printf("Chosen: %s\n", chosen.String())
	*/

	listFlags := flag.NewFlagSet("mod", flag.ContinueOnError)
	listFlags.SetOutput(nullWriter{})
	dryrun := listFlags.Bool("n", false, "dry-run")
	verbose := listFlags.Bool("v", false, "verbose")
	short := listFlags.Bool("short", false, "choose the short path")
	long := listFlags.Bool("long", false, "choose the long path")
	tree := listFlags.Bool("tree", false, "copy all folders including and under selected folder")
	insecure := listFlags.Bool("insecure", false, "allow insecure network updates")
	uncommitted := listFlags.Bool("uncommitted", false, "allows adding uncommitted changes. Doesn't update revision or checksum")
	err = listFlags.Parse(subCmdArgs)
	if err != nil {
		return msg, err
	}
	if *short && *long {
		return help.MsgNone, errors.New("cannot select both long and short path")
	}
	args := listFlags.Args()
	if len(args) == 0 {
		return msg, errors.New("missing package or status")
	}
	ctx, err := r.NewContextWD(context.RootVendor)
	if err != nil {
		return checkNewContextError(err)
	}
	if *verbose {
		ctx.Logger = w
	}
	ctx.Insecure = *insecure
	cgp, err := currentGoPath(ctx)
	if err != nil {
		return msg, err
	}
	f, err := parseFilter(cgp, args)
	if err != nil {
		return msg, err
	}

	mops := make([]context.ModifyOption, 0, 3)
	if *uncommitted {
		mops = append(mops, context.Uncommitted)
	}
	if *tree {
		mops = append(mops, context.IncludeTree)
	}

	// Add explicit imports.
	for _, imp := range f.Import {
		err = ctx.ModifyImport(imp, mod, mops...)
		if err != nil {
			return help.MsgNone, err
		}
	}
	err = ctx.ModifyStatus(f.Status, mod, mops...)
	if err != nil {
		return help.MsgNone, err
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
			switch op.Type {
			case context.OpRemove:
				fmt.Fprintf(w, "Remove %q\n", op.Src)
			case context.OpCopy:
				fmt.Fprintf(w, "Copy %q -> %q\n", op.Src, op.Dest)
				for _, ignore := range op.IgnoreFile {
					fmt.Fprintf(w, "\tIgnore %q\n", ignore)
				}
			case context.OpFetch:
				fmt.Fprintf(w, "Fetch %q\n", op.Src)
			}
		}
		return help.MsgNone, nil
	}

	// Write intent, make the changes, then record any checksums or recursive info.
	err = ctx.WriteVendorFile()
	if err != nil {
		return help.MsgNone, err
	}
	// Write out vendor file and do change.
	err = ctx.Alter()
	vferr := ctx.WriteVendorFile()
	if err != nil {
		return help.MsgNone, err
	}
	if vferr != nil {
		return help.MsgNone, vferr
	}
	return help.MsgNone, nil
}
