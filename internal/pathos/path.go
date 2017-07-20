// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pathos

import (
	"path/filepath"
	"runtime"
	"strconv"
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
	if len(prefix) > len(s) {
		return false
	}
	return caseInsensitiveEq(s[:len(prefix)], prefix)
}

func FileTrimPrefix(s, prefix string) string {
	if FileHasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

func FileHasSuffix(s, suffix string) bool {
	if len(suffix) > len(s) {
		return false
	}
	return caseInsensitiveEq(s[len(s)-len(suffix):], suffix)
}

func FileTrimSuffix(s, suffix string) string {
	if FileHasSuffix(s, suffix) {
		return s[:len(s)-len(suffix)]
	}
	return s
}

var slashSep = filepath.Separator

func TrimCommonSuffix(base, suffix string) (string, string) {
	a, b := base, suffix
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	a = strings.TrimSuffix(strings.TrimSuffix(a, "\\"), "/")
	b = strings.TrimSuffix(strings.TrimSuffix(b, "\\"), "/")
	base = strings.TrimSuffix(strings.TrimSuffix(base, "\\"), "/")

	ff := func(r rune) bool {
		return r == '/' || r == '\\'
	}
	aa := strings.FieldsFunc(a, ff)
	bb := strings.FieldsFunc(b, ff)

	min := len(aa)
	if min > len(bb) {
		min = len(bb)
	}
	i := 1
	for ; i <= min; i++ {
		// fmt.Printf("(%d) end aa: %q, end bb: %q\n", i, aa[len(aa)-i], bb[len(bb)-i])
		if aa[len(aa)-i] == bb[len(bb)-i] {
			continue
		}
		break
	}
	baseParts := strings.FieldsFunc(base, ff)
	// fmt.Printf("base parts: %q\n", baseParts)
	base1 := FileTrimSuffix(base, strings.Join(baseParts[len(baseParts)-i+1:], string(slashSep)))
	base1 = strings.TrimSuffix(strings.TrimSuffix(base1, "\\"), "/")
	base2 := strings.Trim(base[len(base1):], `\/`)
	return base1, base2
}

func FileStringEquals(s1, s2 string) bool {
	if len(s1) == 0 {
		return len(s2) == 0
	}
	if len(s2) == 0 {
		return len(s1) == 0
	}
	r1End := s1[len(s1)-1]
	r2End := s2[len(s2)-1]
	if r1End == '/' || r1End == '\\' {
		s1 = s1[:len(s1)-1]
	}
	if r2End == '/' || r2End == '\\' {
		s2 = s2[:len(s2)-1]
	}
	return caseInsensitiveEq(s1, s2)
}

func caseInsensitiveEq(s1, s2 string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(s1, s2)
	}
	return s1 == s2
}

// ParseGoEnvLine parses a "go env" line into a key value pair.
func ParseGoEnvLine(line string) (key, value string, ok bool) {
	// Remove any leading "set " found on windows.
	// Match the name to the env var + "=".
	// Remove any quotes.
	// Return result.
	line = strings.TrimPrefix(line, "set ")
	parts := strings.SplitN(line, "=", 2)
	if len(parts) < 2 {
		return "", "", false
	}

	un, err := strconv.Unquote(parts[1])
	if err != nil {
		return parts[0], parts[1], true
	}
	return parts[0], un, true
}

// GoEnv parses a "go env" line and checks for a specific
// variable name.
func GoEnv(name, line string) (value string, ok bool) {
	// Remove any leading "set " found on windows.
	// Match the name to the env var + "=".
	// Remove any quotes.
	// Return result.
	line = strings.TrimPrefix(line, "set ")
	if len(line) < len(name)+1 {
		return "", false
	}
	if name != line[:len(name)] || line[len(name)] != '=' {
		return "", false
	}
	line = line[len(name)+1:]
	if un, err := strconv.Unquote(line); err == nil {
		line = un
	}
	return line, true
}
