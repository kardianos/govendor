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
	"github.com/kardianos/govendor/help"
)

func (r *runner) List(w io.Writer, subCmdArgs []string) (help.HelpMessage, error) {
	listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
	listFlags.SetOutput(nullWriter{})
	verbose := listFlags.Bool("v", false, "verbose")
	asFilePath := listFlags.Bool("p", false, "show file path to package instead of import path")
	noStatus := listFlags.Bool("no-status", false, "do not show the status")
	err := listFlags.Parse(subCmdArgs)
	if err != nil {
		return help.MsgList, err
	}
	args := listFlags.Args()
	// fmt.Printf("Status: %q\n", f.Status)

	// Print all listed status.
	ctx, err := r.NewContextWD(context.RootVendorOrWD)
	if err != nil {
		return checkNewContextError(err)
	}
	cgp, err := currentGoPath(ctx)
	if err != nil {
		return help.MsgNone, err
	}
	f, err := parseFilter(cgp, args)
	if err != nil {
		return help.MsgList, err
	}
	if len(f.Import) == 0 {
		insertListToAllNot(&f.Status, normal)
	} else {
		insertListToAllNot(&f.Status, all)
	}

	list, err := ctx.Status()
	if err != nil {
		return help.MsgNone, err
	}

	// If not verbose, remove any entries that will just confuse people.
	// For example, one package may reference pkgA inside vendor, another
	// package may reference pkgA outside vendor, resulting in both a
	// external reference and a vendor reference.
	// In the above case, remove the external reference.
	if !*verbose {
		next := make([]context.StatusItem, 0, len(list))
		for checkIndex, check := range list {
			if check.Status.Location != context.LocationExternal {
				next = append(next, check)
				continue
			}
			found := false
			for lookIndex, look := range list {
				if checkIndex == lookIndex {
					continue
				}
				if check.Pkg.Path != look.Pkg.Path {
					continue
				}
				if look.Status.Location == context.LocationVendor {
					found = true
					break
				}
			}
			if !found {
				next = append(next, check)
			}
		}
		list = next
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

		var path string
		if *asFilePath {
			path = item.Pkg.FilePath
		} else {
			path = item.Pkg.Path
		}

		if item.Local == item.Pkg.Path {
			fmt.Fprintf(tw, formatSame, item.Status, path, item.Pkg.Version, item.VersionExact)
		} else {
			fmt.Fprintf(tw, formatDifferent, item.Status, path, strings.TrimPrefix(item.Local, ctx.RootImportPath), item.Pkg.Version, item.VersionExact)
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
	return help.MsgNone, nil
}
