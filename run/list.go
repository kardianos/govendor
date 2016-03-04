// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/kardianos/govendor/context"
)

func List(w io.Writer, subCmdArgs []string) (HelpMessage, error) {
	listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
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
	insertListToAllNot(&f.Status, normal)

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
	return MsgNone, nil
}
