// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"os"
	"unicode/utf8"
)

// Buffer contains state for line editing and writing.
type Buffer struct {
	Out    *os.File
	Prompt string
	Echo   bool
	Cols   int
	pos    int
	size   int
	data   []rune
}

// NewBuffer creates a buffer writing to out if echo is true.
func NewBuffer(prompt string, out *os.File, echo bool) *Buffer {
	return &Buffer{
		Out:    out,
		Prompt: prompt,
		Echo:   echo,
	}
}

// String returns the data as a string.
func (buf *Buffer) String() string {
	return string(buf.data[:buf.size])
}

// Insert inserts characters at the cursors position.
func (buf *Buffer) Insert(rs ...rune) error {
	rsLen := len(rs)
	total := buf.size + rsLen

	if total > len(buf.data) {
		buf.data = append(buf.data, make([]rune, rsLen)...)
	}

	// Shift characters to make room in the correct pos.
	if buf.size != buf.pos {
		copy(buf.data[buf.pos+rsLen:buf.size+rsLen], buf.data[buf.pos:buf.size])
	}

	for _, r := range rs {
		buf.data[buf.pos] = r
		buf.pos++
		buf.size++
	}

	return buf.Refresh()
}

// Set sets the content in the buffer.
func (buf *Buffer) Set(rs ...rune) error {
	rsLen := len(rs)
	buf.data = rs
	buf.pos = rsLen
	buf.size = rsLen

	return buf.Refresh()
}

// Start moves the cursor to the start.
func (buf *Buffer) Start() error {
	if buf.pos <= 0 {
		return nil
	}

	buf.pos = 0
	return buf.Refresh()
}

// End moves the cursor to the end.
func (buf *Buffer) End() error {
	if buf.pos >= buf.size {
		return nil
	}

	buf.pos = buf.size
	return buf.Refresh()
}

// Left moves the cursor one character left.
func (buf *Buffer) Left() error {
	if buf.pos <= 0 {
		return nil
	}

	buf.pos--
	return buf.Refresh()
}

// Right moves the cursor one character right.
func (buf *Buffer) Right() error {
	if buf.pos >= buf.size {
		return nil
	}

	buf.pos++
	return buf.Refresh()
}

// Del removes the character at the cursor position.
func (buf *Buffer) Del() error {
	if buf.pos >= buf.size {
		return nil
	}

	// Shift characters after position back one.
	copy(buf.data[buf.pos:], buf.data[buf.pos+1:buf.size])
	buf.size--

	return buf.Refresh()
}

// DelLeft removes the character to the left.
func (buf *Buffer) DelLeft() error {
	if buf.pos <= 0 {
		return nil
	}

	// Shift characters from position back one.
	copy(buf.data[buf.pos-1:], buf.data[buf.pos:buf.size])
	buf.pos--
	buf.size--

	return buf.Refresh()
}

// EndLine ends the line with CRLF.
func (buf *Buffer) EndLine() error {
	_, err := buf.Out.Write(crlf)
	return err
}

// toBytes converts a slice of runes to its equivalent in bytes.
func toBytes(runes []rune) []byte {
	var bytes []byte
	char := make([]byte, utf8.UTFMax)

	for _, r := range runes {
		n := utf8.EncodeRune(char, r)
		bytes = append(bytes, char[:n]...)
	}

	return bytes
}
