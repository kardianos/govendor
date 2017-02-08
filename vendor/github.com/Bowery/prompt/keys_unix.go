// +build linux darwin freebsd openbsd netbsd dragonfly solaris

// Copyright 2013-2015 Bowery, Inc.

package prompt

const mvToCol = "\u001b[0G\u001b[%dC"

var (
	mvLeftEdge = []byte("\u001b[0G")
	clsScreen  = []byte("\u001b[H\u001b[2J")
	delRight   = []byte("\u001b[0K")
)
