// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"golang.org/x/sys/unix"
)

const (
	tcgets  = unix.TCGETS
	tcsets  = unix.TCSETS
	tcsetsf = unix.TCSETSF
)
