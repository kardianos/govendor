// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/migrate"
)

func Init(w io.Writer, subCmdArgs []string) (HelpMessage, error) {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})
	err := flags.Parse(subCmdArgs)
	if err != nil {
		return MsgInit, err
	}
	ctx, err := context.NewContextWD(context.RootWD)
	if err != nil {
		return MsgNone, err
	}
	ctx.VendorFile.Ignore = "test" // Add default ignore rule.
	err = ctx.WriteVendorFile()
	if err != nil {
		return MsgNone, err
	}
	err = os.MkdirAll(filepath.Join(ctx.RootDir, ctx.VendorFolder), 0777)
	return MsgNone, err
}
func Migrate(w io.Writer, subCmdArgs []string) (HelpMessage, error) {
	from := migrate.Auto
	if len(subCmdArgs) > 0 {
		switch subCmdArgs[0] {
		case "auto":
			from = migrate.Auto
		case "gb":
			from = migrate.Gb
		case "godep":
			from = migrate.Godep
		case "internal":
			from = migrate.Internal
		default:
			return MsgMigrate, fmt.Errorf("Unknown migrate command %q", subCmdArgs[0])
		}
	}
	return MsgNone, migrate.MigrateWD(from)
}

func Get(w io.Writer, subCmdArgs []string) (HelpMessage, error) {
	flags := flag.NewFlagSet("get", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})

	insecure := flags.Bool("insecure", false, "allows insecure connection")
	verbose := flags.Bool("v", false, "verbose")

	flags.Bool("u", false, "update") // For compatibility with "go get".

	err := flags.Parse(subCmdArgs)
	if err != nil {
		return MsgGet, err
	}
	logger := w
	if !*verbose {
		logger = nil
	}
	for _, a := range flags.Args() {
		err = context.Get(logger, a, *insecure)
		if err != nil {
			return MsgNone, err
		}
		GoCmd("install", []string{a})
	}
	return MsgNone, nil
}

func GoCmd(subcmd string, args []string) (HelpMessage, error) {
	ctx, err := context.NewContextWD(context.RootVendorOrWD)
	if err != nil {
		return MsgNone, err
	}
	statusArgs := make([]string, 0, len(args))
	otherArgs := make([]string, 1, len(args)+1)
	otherArgs[0] = subcmd

	for _, a := range args {
		if a[0] == '+' {
			statusArgs = append(statusArgs, a)
		} else {
			otherArgs = append(otherArgs, a)
		}
	}
	cgp, err := currentGoPath(ctx)
	if err != nil {
		return MsgNone, err
	}
	f, err := parseFilter(cgp, statusArgs)
	if err != nil {
		return MsgNone, err
	}
	list, err := ctx.Status()
	if err != nil {
		return MsgNone, err
	}

	for _, item := range list {
		if f.HasStatus(item) {
			otherArgs = append(otherArgs, item.Local)
		}
	}
	cmd := exec.Command("go", otherArgs...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return MsgNone, cmd.Run()
}

func Status(w io.Writer, subCmdArgs []string) (HelpMessage, error) {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})
	err := flags.Parse(subCmdArgs)
	if err != nil {
		return MsgStatus, err
	}
	ctx, err := context.NewContextWD(context.RootVendor)
	if err != nil {
		return MsgStatus, err
	}
	outOfDate, err := ctx.VerifyVendor()
	if err != nil {
		return MsgStatus, err
	}
	if len(outOfDate) == 0 {
		return MsgNone, nil
	}
	fmt.Fprintf(w, "The following packages are missing or modified locally:\n")
	for _, pkg := range outOfDate {
		fmt.Fprintf(w, "\t%s\n", pkg.Path)
	}
	return MsgNone, nil
}
