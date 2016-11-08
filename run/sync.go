// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"flag"
	"io"

	"github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/help"
)

func (r *runner) Sync(w io.Writer, subCmdArgs []string) (help.HelpMessage, error) {
	flags := flag.NewFlagSet("sync", flag.ContinueOnError)
	insecure := flags.Bool("insecure", false, "allow insecure network updates")
	dryrun := flags.Bool("n", false, "dry run, print what would be done")
	verbose := flags.Bool("v", false, "verbose output")
	flags.SetOutput(nullWriter{})
	err := flags.Parse(subCmdArgs)
	if err != nil {
		return help.MsgSync, err
	}
	ctx, err := r.NewContextWD(context.RootVendor)
	if err != nil {
		return help.MsgSync, err
	}
	ctx.Insecure = *insecure
	if *dryrun || *verbose {
		ctx.Logger = w
	}
	return help.MsgNone, ctx.Sync(*dryrun)
}
