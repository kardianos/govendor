// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"net/http"
	_ "net/http/pprof" // imported for side effect of registering handler

	"github.com/kardianos/govendor/help"

	"github.com/Bowery/prompt"
	"github.com/google/shlex"
)

func (r *runner) Shell(w io.Writer, subCmdArgs []string) (help.HelpMessage, error) {
	flags := flag.NewFlagSet("shell", flag.ContinueOnError)

	pprofHandlerAddr := flags.String("pprof-handler", "", "if set, turns on an HTTP server that offers pprof handlers")

	flags.SetOutput(nullWriter{})
	err := flags.Parse(subCmdArgs)
	if err != nil {
		return help.MsgShell, err
	}

	if *pprofHandlerAddr != "" {
		tryEnableHTTPPprofHandler(*pprofHandlerAddr)
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

// tryEnableHTTPPprofHandler tries to provide an http/pprof handler on `addr`.
// if it fails, it logs an error but does not otherwise do anything.
func tryEnableHTTPPprofHandler(addr string) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "http/pprof handlers failed to create a listener: %v\n", err)
		return
	}
	// port 0 means a randomly allocated one, so we
	// need to figure out where our listener ended up
	realAddr := l.Addr()

	fmt.Fprintf(os.Stderr, "http/pprof handlers are available on %v\n", realAddr)
	go func() {
		defer l.Close()
		if err := http.Serve(l, nil); err != nil {
			fmt.Fprintf(os.Stderr, "http/pprof handlers failed to start: %v\n", err)
		}
	}()
}
