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
	"github.com/kardianos/govendor/help"
	"github.com/kardianos/govendor/prompt"
)

type nullWriter struct{}

func (nw nullWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

type runner struct {
	ctx *context.Context
}

func (r *runner) NewContextWD(rt context.RootType) (*context.Context, error) {
	if r.ctx != nil {
		return r.ctx, nil
	}
	var err error
	r.ctx, err = context.NewContextWD(rt)
	return r.ctx, err
}

// Run is isoloated from main and os.Args to help with testing.
// Shouldn't directly print to console, just write through w.
func Run(w io.Writer, appArgs []string, ask prompt.Prompt) (help.HelpMessage, error) {
	r := &runner{}
	return r.run(w, appArgs, ask)
}
func (r *runner) run(w io.Writer, appArgs []string, ask prompt.Prompt) (help.HelpMessage, error) {
	if len(appArgs) == 1 {
		return help.MsgFull, nil
	}

	flags := flag.NewFlagSet("govendor", flag.ContinueOnError)
	licenses := flags.Bool("govendor-licenses", false, "show govendor's licenses")
	version := flags.Bool("version", false, "show govendor version")
	flags.SetOutput(nullWriter{})
	err := flags.Parse(appArgs[1:])
	if err != nil {
		return help.MsgFull, err
	}
	if *licenses {
		return help.MsgGovendorLicense, nil
	}
	if *version {
		return help.MsgGovendorVersion, nil
	}

	args := flags.Args()

	cmd := args[0]
	switch cmd {
	case "init":
		return r.Init(w, args[1:])
	case "list":
		return r.List(w, args[1:])
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
		return r.Modify(w, args[1:], mod, ask)
	case "sync":
		return r.Sync(w, args[1:])
	case "status":
		return r.Status(w, args[1:])
	case "migrate":
		return r.Migrate(w, args[1:])
	case "get":
		return r.Get(w, args[1:])
	case "license":
		return r.License(w, args[1:])
	case "shell":
		return r.Shell(w, args[1:])
	case "fmt", "build", "install", "clean", "test", "vet", "generate", "tool":
		return r.GoCmd(cmd, args[1:])
	default:
		return help.MsgFull, fmt.Errorf("Unknown command %q", cmd)
	}
}

func checkNewContextError(err error) (help.HelpMessage, error) {
	// Diagnose error, show current value of 1.5vendor, suggest alter.
	if err == nil {
		return help.MsgNone, nil
	}
	if _, is := err.(context.ErrMissingVendorFile); is {
		err = fmt.Errorf(`%v

Ensure the current folder or a parent folder contains a folder named "vendor".
If in doubt, run "govendor init" in the project root.
`, err)
		return help.MsgNone, err
	}
	return help.MsgNone, err
}
