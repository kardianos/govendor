// +build darwin freebsd openbsd netbsd dragonfly

// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"golang.org/x/sys/unix"
)

const (
	tcgets  = unix.TIOCGETA
	tcsets  = unix.TIOCSETA
	tcsetsf = unix.TIOCSETAF
)
