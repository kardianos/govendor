// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"os"
	"syscall"
	"unsafe"
)

// Flags to control the terminals mode.
const (
	echoInputFlag      = 0x0004
	insertModeFlag     = 0x0020
	lineInputFlag      = 0x0002
	mouseInputFlag     = 0x0010
	processedInputFlag = 0x0001
	windowInputFlag    = 0x0008
)

// Error number returned for an invalid handle.
const errnoInvalidHandle = 0x6

var (
	kernel                     = syscall.NewLazyDLL("kernel32.dll")
	getConsoleScreenBufferInfo = kernel.NewProc("GetConsoleScreenBufferInfo")
	setConsoleMode             = kernel.NewProc("SetConsoleMode")
)

// consoleScreenBufferInfo contains various fields for the terminal.
type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        uint16
	window            smallRect
	maximumWindowSize coord
}

// coord contains coords for positioning.
type coord struct {
	x int16
	y int16
}

// smallRect contains positions for the window edges.
type smallRect struct {
	left   int16
	top    int16
	right  int16
	bottom int16
}

// terminalSize retrieves the cols/rows for the terminal connected to out.
func terminalSize(out *os.File) (int, int, error) {
	csbi := new(consoleScreenBufferInfo)

	ret, _, err := getConsoleScreenBufferInfo.Call(out.Fd(), uintptr(unsafe.Pointer(csbi)))
	if ret == 0 {
		return 0, 0, err
	}

	// Results are always off by one.
	cols := csbi.window.right - csbi.window.left + 1
	rows := csbi.window.bottom - csbi.window.top + 1

	return int(cols), int(rows), nil
}

// isNotTerminal checks if an error is related to the input not being a terminal.
func isNotTerminal(err error) bool {
	errno, ok := err.(syscall.Errno)

	return ok && errno == errnoInvalidHandle
}

// terminal contains the private fields for a Windows terminal.
type terminal struct {
	supportsEditing bool
	fd              uintptr
	origMode        uint32
}

// newTerminal creates a terminal and sets it to raw input mode.
func newTerminal(in *os.File) (*terminal, error) {
	term := &terminal{fd: in.Fd()}

	err := syscall.GetConsoleMode(syscall.Handle(term.fd), &term.origMode)
	if err != nil {
		return term, nil
	}
	mode := term.origMode
	term.supportsEditing = true

	// Set new mode flags.
	mode &^= (echoInputFlag | insertModeFlag | lineInputFlag | mouseInputFlag |
		processedInputFlag | windowInputFlag)

	ret, _, err := setConsoleMode.Call(term.fd, uintptr(mode))
	if ret == 0 {
		return nil, err
	}

	return term, nil
}

// Close disables the terminals raw input.
func (term *terminal) Close() error {
	if term.supportsEditing {
		ret, _, err := setConsoleMode.Call(term.fd, uintptr(term.origMode))
		if ret == 0 {
			return err
		}
	}

	return nil
}
