// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/kardianos/govendor/context"
)

func List(w io.Writer, subCmdArgs []string) (HelpMessage, error) {
	listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
	listFlags.SetOutput(nullWriter{})
	verbose := listFlags.Bool("v", false, "verbose")
	noStatus := listFlags.Bool("no-status", false, "do not show the status")
	err := listFlags.Parse(subCmdArgs)
	if err != nil {
		return MsgList, err
	}
	args := listFlags.Args()
	// fmt.Printf("Status: %q\n", f.Status)

	// Print all listed status.
	ctx, err := context.NewContextWD(context.RootVendorOrWD)
	if err != nil {
		return checkNewContextError(err)
	}
	cgp, err := currentGoPath(ctx)
	if err != nil {
		return MsgNone, err
	}
	f, err := parseFilter(cgp, args)
	if err != nil {
		return MsgList, err
	}
	if len(f.Import) == 0 {
		insertListToAllNot(&f.Status, normal)
	} else {
		insertListToAllNot(&f.Status, all)
	}

	list, err := ctx.Status()
	if err != nil {
		return MsgNone, err
	}

	formatSame := "%[1]v %[2]s\t%[3]s\t%[4]s\n"
	formatDifferent := "%[1]v %[2]s\t%[4]s\t%[5]s\n"
	if *verbose {
		formatDifferent = "%[1]v %[2]s ::%[3]s\t%[4]s\t%[5]s\n"
	}
	if *noStatus {
		formatSame = "%[2]s\n"
		formatDifferent = "%[2]s\n"
		if *verbose {
			formatDifferent = "%[2]s ::%[3]s\n"
		}
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	defer tw.Flush()
	for _, item := range list {
		if f.HasStatus(item) == false {
			continue
		}
		if len(f.Import) != 0 && f.FindImport(item) == nil {
			continue
		}

		if item.Local == item.Pkg.Path {
			fmt.Fprintf(tw, formatSame, item.Status, item.Pkg.Path, item.Pkg.Version, item.VersionExact)
		} else {
			fmt.Fprintf(tw, formatDifferent, item.Status, item.Pkg.Path, strings.TrimPrefix(item.Local, ctx.RootImportPath), item.Pkg.Version, item.VersionExact)
		}
		if *verbose {
			for i, imp := range item.ImportedBy {
				if i != len(item.ImportedBy)-1 {
					fmt.Fprintf(tw, "    ├── %s %s\n", imp.Status, imp)
				} else {
					fmt.Fprintf(tw, "    └── %s %s\n", imp.Status, imp)
				}
			}
		}
	}
	return MsgNone, nil
}
