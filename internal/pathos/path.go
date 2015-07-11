// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pathos

import (
	"path/filepath"
	"runtime"
	"strings"
)

func SlashToFilepath(path string) string {
	if '/' == filepath.Separator {
		return path
	}
	return strings.Replace(path, "/", string(filepath.Separator), -1)
}
func SlashToImportPath(path string) string {
	return strings.Replace(path, `\`, "/", -1)
}

func FileHasPrefix(s, prefix string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		s = strings.ToLower(s)
		prefix = strings.ToLower(prefix)
	}
	return strings.HasPrefix(s, prefix)
}

func FileTrimPrefix(s, prefix string) string {
	if FileHasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

func FileStringEquals(s1, s2 string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		s1 = strings.ToLower(s1)
		s2 = strings.ToLower(s2)
	}
	return s1 == s2
}
