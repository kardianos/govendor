// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kardianos/govendor/help"

	"github.com/Bowery/prompt"
	"github.com/google/shlex"
)

func (r *runner) Shell(w io.Writer, subCmdArgs []string) (help.HelpMessage, error) {
	flags := flag.NewFlagSet("shell", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})
	err := flags.Parse(subCmdArgs)
	if err != nil {
		return help.MsgShell, err
	}

	out := os.Stdout

	for {
		line, err := prompt.Basic("> ", false)
		if err != nil {
			break
		}
		args, err := shlex.Split(line)
		if err != nil {
			fmt.Fprintf(out, "%v", err.Error())
		}
		if len(args) == 0 {
			continue
		}
		cmd := args[0]
		next := make([]string, 0, len(args)+1)
		next = append(next, "govendor")
		args = append(next, args...)
		switch cmd {
		case "exit", "q", "quit", "/q":
			return help.MsgNone, nil
		case "shell":
			continue
		}
		msg, err := r.run(out, args, nil)
		if err != nil {
			fmt.Fprintf(out, "%v", err.Error())
		}
		msgText := msg.String()
		if len(msgText) > 0 {
			fmt.Fprintf(out, "%s\tType \"exit\" to exit.\n", msgText)
		}
	}

	return help.MsgNone, nil
}
