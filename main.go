// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// vendor tool to copy external source code to the local vendor folder.
// See README.md for usage.
package main

import (
	"fmt"
	"os"

	"github.com/kardianos/govendor/run"
)

func main() {
	msg, err := run.Run(os.Stdout, os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	msgText := ""
	switch msg {
	case run.MsgFull:
		msgText = helpFull
	case run.MsgList:
		msgText = helpList
	case run.MsgAdd:
		msgText = helpAdd
	case run.MsgUpdate:
		msgText = helpUpdate
	case run.MsgRemove:
		msgText = helpRemove
	case run.MsgFetch:
		msgText = helpFetch
	case run.MsgMigrate:
		msgText = helpMigrate
	}
	if len(msgText) > 0 {
		fmt.Fprint(os.Stderr, msgText)
	}
	if err != nil {
		os.Exit(2)
	}
	if msg != run.MsgNone {
		os.Exit(1)
	}
}
