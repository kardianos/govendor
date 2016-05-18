// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"flag"
	"io"
	"os"

	"github.com/kardianos/govendor/help"

	"github.com/google/shlex"
	"golang.org/x/crypto/ssh/terminal"
)

func (r *runner) Shell(w io.Writer, subCmdArgs []string) (help.HelpMessage, error) {
	flags := flag.NewFlagSet("shell", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})
	err := flags.Parse(subCmdArgs)
	if err != nil {
		return help.MsgShell, err
	}

	fd := 0

	type rw struct {
		io.Reader
		io.Writer
	}
	termRW := rw{Reader: os.Stdin, Writer: os.Stdout}
	term := terminal.NewTerminal(termRW, "> ")

	for {
		termState, err := terminal.MakeRaw(fd)
		if err != nil {
			return help.MsgNone, err
		}
		line, err := term.ReadLine()
		terminal.Restore(fd, termState)
		if err != nil {
			break
		}
		args, err := shlex.Split(line)
		if err != nil {
			termRW.Write([]byte(err.Error()))
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
		msg, err := r.run(termRW, args, nil)
		if err != nil {
			termRW.Write([]byte(err.Error()))
		}
		msgText := msg.String()
		if len(msgText) > 0 {
			termRW.Write([]byte(msgText))
			termRW.Write([]byte("\tType \"exit\" to exit.\n"))
		}
	}

	return help.MsgNone, nil
}
