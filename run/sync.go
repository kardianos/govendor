// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"flag"
	"io"

	"github.com/kardianos/govendor/context"
)

func Sync(w io.Writer, subCmdArgs []string) (HelpMessage, error) {
	flags := flag.NewFlagSet("sync", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})
	err := flags.Parse(subCmdArgs)
	if err != nil {
		return MsgSync, err
	}
	ctx, err := context.NewContextWD(context.RootVendor)
	if err != nil {
		return MsgSync, err
	}
	return MsgNone, ctx.Sync()
}
