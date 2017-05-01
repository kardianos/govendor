// +build linux darwin freebsd openbsd netbsd dragonfly solaris

// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"os"
)

// AnsiReader is an io.Reader that wraps an *os.File.
type AnsiReader struct {
	file *os.File
}

// NewAnsiReader creates a AnsiReader from the given input file.
func NewAnsiReader(in *os.File) *AnsiReader {
	return &AnsiReader{file: in}
}

// Read reads data from the input file into b.
func (ar *AnsiReader) Read(b []byte) (int, error) {
	return ar.file.Read(b)
}

// AnsiWriter is an io.Writer that wraps an *os.File.
type AnsiWriter struct {
	file *os.File
}

// NewAnsiWriter creates a AnsiWriter from the given output file.
func NewAnsiWriter(out *os.File) *AnsiWriter {
	return &AnsiWriter{file: out}
}

// Write writes data from b into the input file.
func (aw *AnsiWriter) Write(b []byte) (int, error) {
	return aw.file.Write(b)
}
