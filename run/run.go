// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package run is a front-end to govendor.
package run

import (
	"flag"
	"fmt"
	"io"

	"github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/prompt"
)

type HelpMessage byte

const (
	MsgNone HelpMessage = iota
	MsgFull
	MsgInit
	MsgList
	MsgAdd
	MsgUpdate
	MsgRemove
	MsgFetch
	MsgStatus
	MsgSync
	MsgMigrate
)

type nullWriter struct{}

func (nw nullWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// Run is isoloated from main and os.Args to help with testing.
// Shouldn't directly print to console, just write through w.
func Run(w io.Writer, appArgs []string, ask prompt.Prompt) (HelpMessage, error) {
	if len(appArgs) == 1 {
		return MsgFull, nil
	}

	flags := flag.NewFlagSet("govendor", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})
	err := flags.Parse(appArgs[1:])
	if err != nil {
		return MsgFull, err
	}

	args := flags.Args()

	cmd := args[0]
	switch cmd {
	case "init":
		return Init(w, args[1:])
	case "list":
		return List(w, args[1:])
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
			mod = context.Fetch
		}
		return Modify(w, args[1:], mod, ask)
	case "sync":
		return Sync(w, args[1:])
	case "status":
		return Status(w, args[1:])
	case "migrate":
		return Migrate(w, args[1:])
	case "fmt", "build", "install", "clean", "test", "vet", "generate":
		msg, err := Sync(w, nil)
		if err != nil {
			return msg, err
		}
		return GoCmd(cmd, args[1:])
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
