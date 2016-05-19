// Copyright 2013-2015 Bowery, Inc.

package prompt

// Line ending in raw mode.
var crlf = []byte("\r\n")

const (
	backKey  = '\u007f'
	escKey   = '\u001B'
	spaceKey = '\u0020'
)

const (
	ctrlA = iota + 1
	ctrlB
	ctrlC
	ctrlD
	ctrlE
	ctrlF
	ctrlG
	ctrlH
	tabKey
	ctrlJ
	ctrlK
	ctrlL
	returnKey
	ctrlN
	ctrlO
	ctrlP
	ctrlQ
	ctrlR
	ctrlS
	ctrlT
	ctrlU
	ctrlV
	ctrlW
	ctrlX
	ctrlY
	ctrlZ
)
