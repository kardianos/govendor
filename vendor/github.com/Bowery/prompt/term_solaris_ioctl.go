// +build solaris

// Copyright 2017 Bowery, Inc.

package prompt

import (
	"os"
	"golang.org/x/sys/unix"

)

type Termios unix.Termios

// setTermios does the system dependent ioctl calls
func setTermios(fd uintptr, req uintptr, termios *Termios) error {
	return unix.IoctlSetTermios(int(fd), int(req), (*unix.Termios)(termios))
}

// getTermios does the system dependent ioctl calls
func getTermios(fd uintptr, req uintptr) (*Termios, error) {
	termios, err := unix.IoctlGetTermios(int(fd), int(req))
	if err != nil {
		return nil, err
	}
	return (*Termios)(termios), nil
}

// TerminalSize retrieves the cols/rows for the terminal connected to out.
func TerminalSize(out *os.File) (int, int, error) {
	ws, err := unix.IoctlGetWinsize(int(out.Fd()), unix.TIOCGWINSZ)

	if err != nil {
		return 0, 0, err
	}
	return int(ws.Col), int(ws.Row), nil
}
