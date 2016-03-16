// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"strconv"
	"unicode"
)

// IsVersion returns true if the string is a version.
func isVersion(s string) bool {
	hasPunct := false
	onlyNumber := true
	onlyHexLetter := true
	for _, r := range s {
		isNumber := unicode.IsNumber(r)
		isLetter := unicode.IsLetter(r)

		hasPunct = hasPunct || unicode.IsPunct(r)
		onlyNumber = onlyNumber && isNumber

		if isLetter {
			low := unicode.ToLower(r)
			onlyHexLetter = onlyHexLetter && low <= 'f'
		}
	}
	if hasPunct {
		return true
	}
	if onlyHexLetter == false {
		return true
	}

	num, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		if num > 100 {
			return false // numeric revision.
		}
	}

	if len(s) > 5 && onlyHexLetter {
		return false // hex revision
	}
	return true
}
