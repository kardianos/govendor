// +build linux darwin freebsd openbsd netbsd dragonfly solaris

// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"os"

	"golang.org/x/sys/unix"
)

// List of unsupported $TERM values.
var unsupported = []string{"", "dumb", "cons25"}

// supportsEditing checks if the terminal supports ansi escapes.
func supportsEditing() bool {
	term := os.Getenv("TERM")

	for _, t := range unsupported {
		if t == term {
			return false
		}
	}

	return true
}

// isNotTerminal checks if an error is related to the input not being a terminal.
func isNotTerminal(err error) bool {
	return err == unix.ENOTTY
}

// terminal contains the private fields for a Unix terminal.
type terminal struct {
	supportsEditing bool
	fd              uintptr
	origMode        unix.Termios
}

// newTerminal creates a terminal and sets it to raw input mode.
func newTerminal(in *os.File) (*terminal, error) {
	term := &terminal{fd: in.Fd()}

	if !supportsEditing() {
		return term, nil
	}

	t, err := getTermios(term.fd)
	if err != nil {
		if IsNotTerminal(err) {
			return term, nil
		}

		return nil, err
	}
	term.origMode = *t
	mode := term.origMode
	term.supportsEditing = true

	// Set new mode flags, for reference see cfmakeraw(3).
	mode.Iflag &^= (unix.BRKINT | unix.IGNBRK | unix.ICRNL |
		unix.INLCR | unix.IGNCR | unix.ISTRIP | unix.IXON |
		unix.PARMRK)

	mode.Oflag &^= unix.OPOST

	mode.Lflag &^= (unix.ECHO | unix.ECHONL | unix.ICANON |
		unix.ISIG | unix.IEXTEN)

	mode.Cflag &^= (unix.CSIZE | unix.PARENB)
	mode.Cflag |= unix.CS8

	// Set controls; min num of bytes, and timeouts.
	mode.Cc[unix.VMIN] = 1
	mode.Cc[unix.VTIME] = 0

	err = setTermios(term.fd, true, &mode)
	if err != nil {
		return nil, err
	}

	return term, nil
}

// Close disables the terminals raw input.
func (term *terminal) Close() error {
	if term.supportsEditing {
		err := setTermios(term.fd, false, &term.origMode)
		if err != nil {
			return err
		}
	}

	return nil
}
