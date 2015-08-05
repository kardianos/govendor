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
	if len(s1) == 0 {
		return len(s2) == 0
	}
	if len(s2) == 0 {
		return len(s1) == 0
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		s1 = strings.ToLower(s1)
		s2 = strings.ToLower(s2)
	}
	r1End := s1[len(s1)-1]
	r2End := s2[len(s2)-1]
	if r1End == '/' || r1End == '\\' {
		s1 = s1[:len(s1)-1]
	}
	if r2End == '/' || r2End == '\\' {
		s2 = s2[:len(s2)-1]
	}
	return s1 == s2
}
