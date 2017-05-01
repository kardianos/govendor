// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"unsafe"
)

var (
	fillConsoleOutputCharacter = kernel.NewProc("FillConsoleOutputCharacterW")
	setConsoleCursorPosition   = kernel.NewProc("SetConsoleCursorPosition")
)

// Refresh rewrites the prompt and buffer.
func (buf *Buffer) Refresh() error {
	csbi := new(consoleScreenBufferInfo)
	ret, _, err := getConsoleScreenBufferInfo.Call(buf.Out.Fd(),
		uintptr(unsafe.Pointer(csbi)))
	if ret == 0 {
		return err
	}

	// If we're not echoing just write prompt.
	if !buf.Echo {
		err = buf.delLine(csbi)
		if err != nil {
			return err
		}

		err = buf.mvLeftEdge(csbi)
		if err != nil {
			return err
		}

		_, err = buf.Out.Write([]byte(buf.Prompt))
		return err
	}

	prLen := len(buf.Prompt)
	start := 0
	size := buf.size
	pos := buf.pos

	// Get slice range that should be visible.
	for prLen+pos >= buf.Cols {
		start++
		size--
		pos--
	}
	for prLen+size > buf.Cols {
		size--
	}

	err = buf.delLine(csbi)
	if err != nil {
		return err
	}

	err = buf.mvLeftEdge(csbi)
	if err != nil {
		return err
	}

	_, err = buf.Out.Write([]byte(buf.Prompt))
	if err != nil {
		return err
	}

	_, err = buf.Out.Write(toBytes(buf.data[start : size+start]))
	if err != nil {
		return err
	}

	return buf.mvToCol(csbi, pos+prLen)
}

// ClsScreen clears the screen and refreshes.
func (buf *Buffer) ClsScreen() error {
	var written uint32
	coords := new(coord)

	csbi := new(consoleScreenBufferInfo)
	ret, _, err := getConsoleScreenBufferInfo.Call(buf.Out.Fd(),
		uintptr(unsafe.Pointer(csbi)))
	if ret == 0 {
		return err
	}

	// Clear everything from 0,0.
	ret, _, err = fillConsoleOutputCharacter.Call(buf.Out.Fd(), uintptr(' '),
		uintptr(csbi.size.x*csbi.size.y), uintptr(*(*int32)(unsafe.Pointer(coords))),
		uintptr(unsafe.Pointer(&written)))
	if ret == 0 {
		return err
	}

	// Set cursor at 0,0.
	ret, _, err = setConsoleCursorPosition.Call(buf.Out.Fd(),
		uintptr(*(*int32)(unsafe.Pointer(coords))))
	if ret == 0 {
		return err
	}

	return buf.Refresh()
}

// delLine deletes the line the csbi cursor is positioned on.
// TODO: Possible refresh jittering reason, instead we should copy the Unix
// code and write over contents and then remove everything to the right.
func (buf *Buffer) delLine(csbi *consoleScreenBufferInfo) error {
	var written uint32
	coords := &coord{y: csbi.cursorPosition.y}

	ret, _, err := fillConsoleOutputCharacter.Call(buf.Out.Fd(), uintptr(' '),
		uintptr(csbi.size.x), uintptr(*(*int32)(unsafe.Pointer(coords))),
		uintptr(unsafe.Pointer(&written)))
	if ret == 0 {
		return err
	}

	return nil
}

// mvLeftEdge moves the cursor to the beginning of the line the csbi cursor
// is positioned on.
func (buf *Buffer) mvLeftEdge(csbi *consoleScreenBufferInfo) error {
	coords := &coord{y: csbi.cursorPosition.y}

	ret, _, err := setConsoleCursorPosition.Call(buf.Out.Fd(),
		uintptr(*(*int32)(unsafe.Pointer(coords))))
	if ret == 0 {
		return err
	}

	return nil
}

// mvTolCol moves the cursor to the col on the line the csbi cursor is
// positioned on.
func (buf *Buffer) mvToCol(csbi *consoleScreenBufferInfo, x int) error {
	coords := &coord{x: int16(x), y: csbi.cursorPosition.y}

	ret, _, err := setConsoleCursorPosition.Call(buf.Out.Fd(),
		uintptr(*(*int32)(unsafe.Pointer(coords))))
	if ret == 0 {
		return err
	}

	return nil
}
