// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"os"

	"golang.org/x/sys/unix"
)

const (
	tcgets  = unix.TCGETS
	tcsetsf = unix.TCSETSF
	tcsets  = unix.TCSETS
)

// terminalSize retrieves the cols/rows for the terminal connected to out.
func terminalSize(out *os.File) (int, int, error) {
	ws, err := unix.IoctlGetWinsize(int(out.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}

	return int(ws.Col), int(ws.Row), nil
}

// getTermios retrieves the termios settings for the terminal descriptor.
func getTermios(fd uintptr) (*unix.Termios, error) {
	return unix.IoctlGetTermios(int(fd), tcgets)
}

// setTermios sets the termios settings for the terminal descriptor,
// optionally flushing the buffer before setting.
func setTermios(fd uintptr, flush bool, mode *unix.Termios) error {
	req := tcsets
	if flush {
		req = tcsetsf
	}

	return unix.IoctlSetTermios(int(fd), req, mode)
}
