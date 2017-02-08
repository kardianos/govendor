// +build linux darwin freebsd openbsd netbsd dragonfly solaris

// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"fmt"
)

// Refresh rewrites the prompt and buffer.
func (buf *Buffer) Refresh() error {
	// If we're not echoing just write prompt.
	if !buf.Echo {
		_, err := buf.Out.Write(mvLeftEdge)
		if err != nil {
			return err
		}

		_, err = buf.Out.Write([]byte(buf.Prompt))
		if err != nil {
			return err
		}

		_, err = buf.Out.Write(delRight)
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

	_, err := buf.Out.Write(mvLeftEdge)
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

	_, err = buf.Out.Write(delRight)
	if err != nil {
		return err
	}

	_, err = buf.Out.Write([]byte(fmt.Sprintf(mvToCol, pos+prLen)))
	return err
}

// ClsScreen clears the screen and refreshes.
func (buf *Buffer) ClsScreen() error {
	_, err := buf.Out.Write(clsScreen)
	if err != nil {
		return err
	}

	return buf.Refresh()
}
