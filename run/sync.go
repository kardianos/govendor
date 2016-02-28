// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import "io"

func Sync(w io.Writer, subCmdArgs []string) (HelpMessage, error) {
	return MsgSync, nil
}
