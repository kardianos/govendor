// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"syscall"
)

const (
	tcgets  = syscall.TCGETS
	tcsets  = syscall.TCSETS
	tcsetsf = 0x5404
)
