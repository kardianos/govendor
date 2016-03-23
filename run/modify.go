// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/kardianos/govendor/context"
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
	list, err := ctx.Status()
	if err != nil {
		return msg, err
	}

	added := make(map[string]bool, 10)
	add := func(path string) {
		added[path] = true
	}

	// Add explicit imports.
	for _, imp := range f.Import {
		if *uncommitted {
			imp.Uncommitted = true
		}
		if *tree {
			imp.IncludeTree = true
		}
		add(imp.Path)
		err = ctx.ModifyImport(imp, mod)
		if err != nil {
			return MsgNone, err
		}
	}

	// If add any matched from "...".
	for _, item := range list {
		for _, imp := range f.Import {
			if added[item.Pkg.Path] {
				continue
			}
			if !imp.MatchTree {
				continue
			}
			match := imp.Path + "/"
			if !strings.HasPrefix(item.Pkg.Path, match) {
				continue
			}
			if imp.HasVersion {
				item.Pkg.HasVersion = true
				item.Pkg.Version = imp.Version
			}
			add(item.Pkg.Path)
			err = ctx.ModifyImport(item.Pkg, mod)
			if err != nil {
				return MsgNone, err
			}
		}
	}

	// Add packages from status.
statusLoop:
	for _, item := range list {
		if f.HasStatus(item) {
			if added[item.Pkg.Path] {
				continue
			}
			// Do not attempt to add any existing status items that are
			// already present in vendor folder.
			if mod == context.Add {
				if ctx.VendorFilePackagePath(item.Pkg.Path) != nil {
					continue
				}
				for _, pkg := range ctx.Package {
					if pkg.Status.Location == context.LocationVendor && item.Pkg.Path == pkg.Path {
						continue statusLoop
					}
				}
			}

			if *tree {
				item.Pkg.IncludeTree = true
			}
			if *uncommitted {
				item.Pkg.Uncommitted = true
			}
			add(item.Pkg.Path)
			err = ctx.ModifyImport(item.Pkg, mod)
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
		return MsgNone, nil
	}

	// Write intent, make the changes, then record any checksums or recursive info.
	err = ctx.WriteVendorFile()
	if err != nil {
		return MsgNone, err
	}
	// Write out vendor file and do change.
	err = ctx.Alter()
	vferr := ctx.WriteVendorFile()
	if err != nil {
		return MsgNone, err
	}
	if vferr != nil {
		return MsgNone, vferr
	}
	return MsgNone, nil
}
