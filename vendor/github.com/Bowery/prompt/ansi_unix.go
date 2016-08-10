// +build linux darwin freebsd openbsd netbsd dragonfly

// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"os"
)

// AnsiReader is an io.Reader that wraps an *os.File.
type AnsiReader struct {
	*os.File
}

// NewAnsiReader creates a AnsiReader from the given input file.
func NewAnsiReader(in *os.File) *AnsiReader {
	return &AnsiReader{File: in}
}

// AnsiWriter is an io.Writer that wraps an *os.File.
type AnsiWriter struct {
	*os.File
}

// NewAnsiWriter creates a AnsiWriter from the given output file.
func NewAnsiWriter(out *os.File) *AnsiWriter {
	return &AnsiWriter{File: out}
}
