// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package run is a front-end to govendor.
package run

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"

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
	cpuProfile := flags.String("cpuprofile", "", "write a CPU profile to `file` to help debug slow operations")
	heapProfile := flags.String("heapprofile", "", "write a heap profile to `file` to help debug slow operations")

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

	if *cpuProfile != "" {
		done := collectCPUProfile(*cpuProfile)
		defer done()
	}
	if *heapProfile != "" {
		done := collectHeapProfile(*cpuProfile)
		defer done()
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

// collectHeapProfile collects a CPU profile for as long as
// `done()` is not invoked.
func collectCPUProfile(filename string) (done func()) {
	cpuProf, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create file for cpu profile: %v\n", err)
		return func() {}
	}
	if err := pprof.StartCPUProfile(cpuProf); err != nil {
		_ = cpuProf.Close()
		fmt.Fprintf(os.Stderr, "failed to write cpu profile to file: %v\n", err)
		return func() {}
	}
	return func() {
		pprof.StopCPUProfile()
		if err := cpuProf.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close file for cpu profile: %v\n", err)
		}
	}
}

// collectHeapProfile collects a heap profile _when_ `done()` is called.
func collectHeapProfile(filename string) (done func()) {
	heapProf, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create file for heap profile: %v\n", err)
		return
	}
	return func() {
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(heapProf); err != nil {
			_ = heapProf.Close()
			fmt.Fprintf(os.Stderr, "failed to write heap profile to file: %v\n", err)
			return
		}
		if err := heapProf.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close file for heap profile: %v\n", err)
			return
		}
	}
}
