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
	"github.com/kardianos/govendor/help"
	"github.com/kardianos/govendor/migrate"
)

func (r *runner) Init(w io.Writer, subCmdArgs []string) (help.HelpMessage, error) {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})
	err := flags.Parse(subCmdArgs)
	if err != nil {
		return help.MsgInit, err
	}
	ctx, err := r.NewContextWD(context.RootWD)
	if err != nil {
		return help.MsgNone, err
	}
	ctx.VendorFile.Ignore = "test" // Add default ignore rule.
	err = ctx.WriteVendorFile()
	if err != nil {
		return help.MsgNone, err
	}
	err = os.MkdirAll(filepath.Join(ctx.RootDir, ctx.VendorFolder), 0777)
	return help.MsgNone, err
}
func (r *runner) Migrate(w io.Writer, subCmdArgs []string) (help.HelpMessage, error) {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})
	err := flags.Parse(subCmdArgs)
	if err != nil {
		return help.MsgMigrate, err
	}

	from := migrate.From("auto")
	if len(flags.Args()) > 0 {
		from = migrate.From(flags.Arg(0))
	}
	err = migrate.MigrateWD(from)
	if err != nil {
		return help.MsgNone, err
	}
	fmt.Fprintf(w, `You may wish to run "govendor sync" now.%s`, "\n")
	return help.MsgNone, nil
}

func (r *runner) Get(w io.Writer, subCmdArgs []string) (help.HelpMessage, error) {
	flags := flag.NewFlagSet("get", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})

	insecure := flags.Bool("insecure", false, "allows insecure connection")
	verbose := flags.Bool("v", false, "verbose")

	flags.Bool("u", false, "update") // For compatibility with "go get".

	err := flags.Parse(subCmdArgs)
	if err != nil {
		return help.MsgGet, err
	}
	logger := w
	if !*verbose {
		logger = nil
	}
	for _, a := range flags.Args() {
		pkg, err := context.Get(logger, a, *insecure)
		if err != nil {
			return help.MsgNone, err
		}

		r.GoCmd("install", []string{pkg.Path})
	}
	return help.MsgNone, nil
}

func (r *runner) GoCmd(subcmd string, args []string) (help.HelpMessage, error) {
	ctx, err := r.NewContextWD(context.RootVendorOrWDOrFirstGOPATH)
	if err != nil {
		return help.MsgNone, err
	}
	list, err := ctx.Status()
	if err != nil {
		return help.MsgNone, err
	}
	cgp, err := currentGoPath(ctx)
	if err != nil {
		return help.MsgNone, err
	}

	otherArgs := make([]string, 1, len(args)+1)
	otherArgs[0] = subcmd

	// Expand any status flags in-place. Some wrapped commands the order is
	// important to the operation of the command.
	for _, a := range args {
		if a[0] == '+' {
			f, err := parseFilter(cgp, []string{a})
			if err != nil {
				return help.MsgNone, err
			}
			for _, item := range list {
				if f.HasStatus(item) {
					add := item.Local
					// "go tool vet" takes dirs, not pkgs, so special case it.
					if subcmd == "tool" && len(args) > 0 && args[0] == "vet" {
						add = filepath.Join(ctx.RootGopath, add)
					}
					otherArgs = append(otherArgs, add)
				}
			}
		} else {
			otherArgs = append(otherArgs, a)
		}
	}

	cmd := exec.Command("go", otherArgs...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return help.MsgNone, cmd.Run()
}

func (r *runner) Status(w io.Writer, subCmdArgs []string) (help.HelpMessage, error) {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})
	err := flags.Parse(subCmdArgs)
	if err != nil {
		return help.MsgStatus, err
	}
	ctx, err := r.NewContextWD(context.RootVendor)
	if err != nil {
		return help.MsgStatus, err
	}
	outOfDate, err := ctx.VerifyVendor()
	if err != nil {
		return help.MsgStatus, err
	}
	if len(outOfDate) == 0 {
		return help.MsgNone, nil
	}
	fmt.Fprintf(w, "The following packages are missing or modified locally:\n")
	for _, pkg := range outOfDate {
		fmt.Fprintf(w, "\t%s\n", pkg.Path)
	}
	return help.MsgNone, fmt.Errorf("status failed for %d package(s)", len(outOfDate))
}
