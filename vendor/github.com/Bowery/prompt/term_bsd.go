// +build darwin freebsd

// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"syscall"
)

const (
	tcgets  = syscall.TIOCGETA
	tcsets  = syscall.TIOCSETA
	tcsetsf = syscall.TIOCSETAF
)
