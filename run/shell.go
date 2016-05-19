// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"flag"
	"fmt"
	"io"

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

	term, err := prompt.NewTerminal()
	if err != nil {
		return help.MsgNone, err
	}
	defer term.Close()

	for {
		line, err := term.Basic("> ", false)
		if err != nil {
			break
		}
		args, err := shlex.Split(line)
		if err != nil {
			fmt.Fprintf(term.Out, "%v", err.Error())
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
		msg, err := r.run(term.Out, args, nil)
		if err != nil {
			fmt.Fprintf(term.Out, "%v", err.Error())
		}
		msgText := msg.String()
		if len(msgText) > 0 {
			fmt.Fprintf(term.Out, "%s\tType \"exit\" to exit.\n", msgText)
		}
	}

	return help.MsgNone, nil
}
