// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"bufio"
	"os"
	"syscall"
	"unsafe"
)

const (
	echoInputFlag      = 0x0004
	insertModeFlag     = 0x0020
	lineInputFlag      = 0x0002
	mouseInputFlag     = 0x0010
	processedInputFlag = 0x0001
	windowInputFlag    = 0x0008
	errnoInvalidHandle = 0x6
)

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

// TerminalSize retrieves the cols/rows for the terminal connected to out.
func TerminalSize(out *os.File) (int, int, error) {
	csbi := new(consoleScreenBufferInfo)
	ret, _, err := getConsoleScreenBufferInfo.Call(out.Fd(),
		uintptr(unsafe.Pointer(csbi)))
	if ret == 0 {
		return 0, 0, err
	}

	// Results are always off by one.
	cols := csbi.window.right - csbi.window.left + 1
	rows := csbi.window.bottom - csbi.window.top + 1
	return int(cols), int(rows), nil
}

// IsNotTerminal checks if an error is related to io not being a terminal.
func IsNotTerminal(err error) bool {
	errno, ok := err.(syscall.Errno)

	if ok && errno == errnoInvalidHandle {
		return true
	}

	return false
}

// terminal contains the private fields for a Windows terminal.
type terminal struct {
	isTerm       bool
	simpleReader *bufio.Reader
	origMode     uint32
}

// NewTerminal creates a terminal and sets it to raw input mode.
func NewTerminal() (*Terminal, error) {
	term := &Terminal{
		In:       os.Stdin,
		Out:      os.Stdout,
		History:  make([]string, 0, 10),
		histIdx:  -1,
		terminal: new(terminal),
	}

	err := syscall.GetConsoleMode(syscall.Handle(term.In.Fd()), &term.origMode)
	if err != nil {
		return term, nil
	}
	mode := term.origMode
	term.isTerm = true

	// Set new mode flags.
	mode &^= (echoInputFlag | insertModeFlag | lineInputFlag | mouseInputFlag |
		processedInputFlag | windowInputFlag)

	ret, _, err := setConsoleMode.Call(term.In.Fd(), uintptr(mode))
	if ret == 0 {
		return nil, err
	}

	return term, nil
}

// GetPrompt gets a line with the prefix and echos input.
func (term *Terminal) GetPrompt(prefix string) (string, error) {
	if !term.isTerm {
		return term.simplePrompt(prefix)
	}

	buf := NewBuffer(prefix, term.Out, true)
	return term.prompt(buf, NewAnsiReader(term.In))
}

// GetPassword gets a line with the prefix and doesn't echo input.
func (term *Terminal) GetPassword(prefix string) (string, error) {
	if !term.isTerm {
		return term.simplePrompt(prefix)
	}

	buf := NewBuffer(prefix, term.Out, false)
	return term.password(buf, NewAnsiReader(term.In))
}

// Close disables the terminals raw input.
func (term *Terminal) Close() error {
	if term.isTerm {
		ret, _, err := setConsoleMode.Call(term.In.Fd(), uintptr(term.origMode))
		if ret == 0 {
			return err
		}
	}

	return nil
}
