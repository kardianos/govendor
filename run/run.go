// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"fmt"
	"io"

	"github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/prompt"
)

type HelpMessage byte

const (
	MsgNone HelpMessage = iota
	MsgFull
	MsgList
	MsgAdd
	MsgUpdate
	MsgRemove
	MsgFetch
	MsgSync
	MsgMigrate
)

// Run is isoloated from main and os.Args to help with testing.
// Shouldn't directly print to console, just write through w.
func Run(w io.Writer, appArgs []string, ask prompt.Prompt) (HelpMessage, error) {
	if len(appArgs) == 1 {
		return MsgFull, nil
	}

	cmd := appArgs[1]
	switch cmd {
	case "init":
		return Init()
	case "list":
		return List(w, appArgs[2:])
	case "add", "update", "remove", "fetch":
		var mod context.Modify
		switch cmd {
		case "add":
			mod = context.Add
		case "update":
			mod = context.Update
		case "remove":
			mod = context.Remove
		case "fetch":
			// TODO: enable a code path that fetches recursivly on missing status.
			mod = context.Fetch
		}
		return Modify(w, appArgs[2:], mod, ask)
	case "sync":
		return Sync(w, appArgs[2:])
	case "migrate":
		return Migrate(appArgs[2:])
	case "fmt", "build", "install", "clean", "test", "vet", "generate":
		msg, err := Sync(w, nil)
		if err != nil {
			return msg, err
		}
		return GoCmd(cmd, appArgs[2:])
	default:
		return MsgFull, fmt.Errorf("Unknown command %q", cmd)
	}
}

func checkNewContextError(err error) (HelpMessage, error) {
	// Diagnose error, show current value of 1.5vendor, suggest alter.
	if err == nil {
		return MsgNone, nil
	}
	if _, is := err.(context.ErrMissingVendorFile); is {
		err = fmt.Errorf(`%v

Ensure the current folder or a parent folder contains a folder named "vendor".
If in doubt, run "govendor init" in the project root.
`, err)
		return MsgNone, err
	}
	return MsgNone, err
}
