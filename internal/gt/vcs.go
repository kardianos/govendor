// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gt

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type vcsNewer func(h *HttpHandler) VcsHandle

type HttpHandler struct {
	runner
	httpAddr string
	vcsAddr  string
	vcsName  string
	pkg      string
	l        net.Listener
	g        *GopathTest
	newer    vcsNewer

	handles map[string]VcsHandle
}

func (h *HttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	out := w

	const templ = `<html><head><meta name="go-import" content="%s %s %s"></head></html>
`
	p := strings.TrimPrefix(r.URL.Path, "/")
	var handle VcsHandle
	for _, try := range h.handles {
		if strings.HasPrefix(p, try.pkg()) {
			handle = try
			break
		}
	}
	if handle == nil {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}

	h.g.Log("http meta: ", p)
	fmt.Fprintf(out, templ, h.httpAddr+"/"+handle.pkg(), h.vcsName, h.vcsAddr+handle.pkg()+"/.git")
}

func (h *HttpHandler) Close() error {
	return h.l.Close()
}
func (h *HttpHandler) HttpAddr() string {
	return h.httpAddr
}

// Setup returns type with Remove function that can be defer'ed.
func (h *HttpHandler) Setup() VcsHandle {
	vcs := h.newer(h)
	vcs.create()
	h.g.onClean(vcs.remove)

	h.handles[vcs.pkg()] = vcs
	return vcs
}

func NewHttpHandler(g *GopathTest, vcsName string) *HttpHandler {
	listenOn := "localhost:0"
	// Windows does not allow ":" in a folder name. Thus they are not
	// allowed in the import path either. Require port 80.
	if runtime.GOOS == "windows" {
		listenOn = "localhost:80"
	}
	// Test if git is installed. If it is, enable the git test.
	// If enabled, start the http server and accept git server registrations.
	l, err := net.Listen("tcp", listenOn)
	if err != nil {
		if runtime.GOOS == "windows" {
			g.Skip("skip test on windows, unable to bind port", err)
		}
		g.Fatal(err)
	}

	h := &HttpHandler{
		runner: runner{
			cwd: g.Current(),
			t:   g,
		},
		pkg:      g.pkg,
		vcsName:  vcsName,
		httpAddr: strings.TrimSuffix(l.Addr().String(), ":80"),
		l:        l,
		g:        g,

		handles: make(map[string]VcsHandle, 6),
	}
	go func() {
		err = http.Serve(l, h)
		if err != nil {
			fmt.Printf("Error serving HTTP server %v\n", err)
			os.Exit(1)
		}
	}()

	execPath, _ := exec.LookPath(vcsName)
	if len(execPath) == 0 {
		g.Skip("unsupported vcs")
	}
	h.execPath = execPath
	switch vcsName {
	default:
		panic("unknown vcs type")
	case "git":
		port := h.freePort()
		switch port {
		default:
			h.vcsAddr = fmt.Sprintf("git://localhost:%d/", port)
		case 80:
			h.vcsAddr = "git://localhost/"
		}

		// TODO(kardianos): on windows we fail to kill the process tree. This
		// results in failing to clean up the temp dir. Find a way to
		// kill the "git daemon" processes.

		// git on windows still needs forward slashes in paths or it will fail
		// to serve, even with --export-all.
		h.runAsync(" Ready ", "daemon",
			"--listen=localhost", fmt.Sprintf("--port=%d", port),
			"--export-all", "--verbose", "--informative-errors",
			"--base-path="+strings.Replace(g.Path(""), `\`, "/", -1), strings.Replace(h.cwd, `\`, "/", -1),
		)

		h.newer = func(h *HttpHandler) VcsHandle {
			return &gitVcsHandle{
				vcsCommon: vcsCommon{
					runner: runner{
						execPath: execPath,
						cwd:      h.g.Current(),
						t:        h.g,
					},
					h:          h,
					importPath: h.g.pkg,
				},
			}
		}
	}
	return h
}

type vcsCommon struct {
	runner
	importPath string

	h *HttpHandler
}

func (vcs *vcsCommon) pkg() string {
	return vcs.importPath
}

type VcsHandle interface {
	remove()
	pkg() string
	create()
	Commit() (rev string, commitTime string)
}

type gitVcsHandle struct {
	vcsCommon
}

func (vcs *gitVcsHandle) remove() {
	delete(vcs.h.handles, vcs.pkg())
}
func (vcs *gitVcsHandle) create() {
	vcs.t.Log("create repo: ", vcs.cwd)
	vcs.run("init")
	vcs.run("config", "user.name", "tests")
	vcs.run("config", "user.email", "tests@govendor.io")
}

func (vcs *gitVcsHandle) Commit() (rev string, commitTime string) {
	vcs.run("add", "-A")
	vcs.run("commit", "-a", "-m", "msg")
	out := vcs.run("show", "--pretty=format:%H@%ai", "-s")

	line := strings.TrimSpace(string(out))
	ss := strings.Split(line, "@")
	rev = ss[0]
	tm, err := time.Parse("2006-01-02 15:04:05 -0700", ss[1])
	if err != nil {
		panic("Failed to parse time: " + ss[1] + " : " + err.Error())
	}

	return rev, tm.UTC().Format(time.RFC3339)
}

type runner struct {
	execPath string
	cwd      string
	t        *GopathTest
}

func (r *runner) run(args ...string) []byte {
	cmd := exec.Command(r.execPath, args...)
	cmd.Dir = r.cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("Failed to run %q %q: %v", r.execPath, args, err)
	}
	return out
}

func (r *runner) freePort() int {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		r.t.Fatalf("Failed to find free port %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	if runtime.GOOS == "windows" {
		time.Sleep(time.Millisecond * 300) // Wait for OS to release port.
	}
	return port
}

// Prevents a race condition in runAsync.
type safeBuf struct {
	sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuf) Write(b []byte) (int, error) {
	s.Lock()
	defer s.Unlock()
	return s.buf.Write(b)
}
func (s *safeBuf) String() string {
	s.Lock()
	defer s.Unlock()
	return s.buf.String()
}

func (r *runner) runAsync(checkFor string, args ...string) *exec.Cmd {
	r.t.Log("starting:", r.execPath, args)
	cmd := exec.Command(r.execPath, args...)
	cmd.Dir = r.t.Current()

	var buf *safeBuf
	var bufErr *safeBuf
	if checkFor != "" {
		buf = &safeBuf{}
		bufErr = &safeBuf{}
		cmd.Stdout = buf
		cmd.Stderr = bufErr
	}
	err := cmd.Start()
	if err != nil {
		r.t.Fatalf("Failed to start %q %q: %v", r.execPath, args)
	}
	r.t.onClean(func() {
		if cmd.Process == nil {
			return
		}
		cmd.Process.Signal(os.Interrupt)

		done := make(chan struct{}, 3)
		go func() {
			cmd.Process.Wait()
			done <- struct{}{}
		}()
		select {
		case <-time.After(time.Millisecond * 300):
			cmd.Process.Kill()
		case <-done:
		}

		r.t.Logf("%q StdOut: %s\n", cmd.Path, buf.String())
		r.t.Logf("%q StdErr: %s\n", cmd.Path, bufErr.String())
	})
	if checkFor != "" {
		for i := 0; i < 100; i++ {
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				r.t.Fatalf("unexpected stop %q %q\n%s\n%s\n", r.execPath, args, buf.String(), bufErr.String())
			}
			if strings.Contains(buf.String(), checkFor) {
				return cmd
			}
			if strings.Contains(bufErr.String(), checkFor) {
				return cmd
			}
			time.Sleep(time.Millisecond * 10)
		}
		r.t.Fatalf("failed to read expected output %q from %q %q\n%s\n", checkFor, r.execPath, args, bufErr.String())
	}
	return cmd
}
