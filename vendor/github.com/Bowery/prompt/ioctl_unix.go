// +build linux darwin freebsd openbsd netbsd dragonfly

// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// winsize contains the size for the terminal.
type winsize struct {
	rows uint16
	cols uint16
	_    uint32
}

// terminalSize retrieves the cols/rows for the terminal connected to out.
func terminalSize(out *os.File) (int, int, error) {
	ws := new(winsize)

	_, _, err := unix.Syscall(unix.SYS_IOCTL, out.Fd(),
		uintptr(unix.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if err != 0 {
		return 0, 0, err
	}

	return int(ws.cols), int(ws.rows), nil
}

// getTermios retrieves the termios settings for the terminal descriptor.
func getTermios(fd uintptr) (*unix.Termios, error) {
	termios := new(unix.Termios)

	_, _, err := unix.Syscall(unix.SYS_IOCTL, fd, tcgets,
		uintptr(unsafe.Pointer(termios)))
	if err != 0 {
		return nil, err
	}

	return termios, nil
}

// setTermios sets the termios settings for the terminal descriptor,
// optionally flushing the buffer before setting.
func setTermios(fd uintptr, flush bool, mode *unix.Termios) error {
	req := tcsets
	if flush {
		req = tcsetsf
	}

	_, _, err := unix.Syscall(unix.SYS_IOCTL, fd, uintptr(req),
		uintptr(unsafe.Pointer(mode)))
	if err != 0 {
		return err
	}

	return nil
}
