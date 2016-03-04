// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"io"

	"github.com/kardianos/govendor/context"
)

func Sync(w io.Writer, subCmdArgs []string) (HelpMessage, error) {
	ctx, err := context.NewContextWD(context.RootVendor)
	if err != nil {
		return MsgSync, err
	}
	return MsgNone, ctx.Sync()
}
