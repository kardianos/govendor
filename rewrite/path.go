// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rewrite

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func findRoot(folder string) (root string, err error) {
	for {
		test := filepath.Join(folder, internalVendor)
		_, err := os.Stat(test)
		if os.IsNotExist(err) == false {
			return folder, nil
		}
		nextFolder := filepath.Clean(filepath.Join(folder, ".."))

		// Check for root folder.
		if nextFolder == folder {
			return "", ErrMissingVendorFile
		}
		folder = nextFolder
	}
}

func slashToFilepath(path string) string {
	if '/' == filepath.Separator {
		return path
	}
	return strings.Replace(path, "/", string(filepath.Separator), -1)
}
func slashToImportPath(path string) string {
	return strings.Replace(path, `\`, "/", -1)
}

func fileHasPrefix(s, prefix string) bool {
	if runtime.GOOS == "windows" {
		s = strings.ToLower(s)
		prefix = strings.ToLower(prefix)
	}
	return strings.HasPrefix(s, prefix)
}

func fileTrimPrefix(s, prefix string) string {
	if fileHasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

func fileStringEquals(s1, s2 string) bool {
	if runtime.GOOS == "windows" {
		s1 = strings.ToLower(s1)
		s2 = strings.ToLower(s2)
	}
	return s1 == s2
}

// findLocalImportPath determines the correct local import path (from GOPATH)
// and from any nested internal vendor files. It returns a string relative to
// the root "internal" folder.
func findLocalImportPath(ctx *Context, importPath string) (string, error) {
	/*
		"crypto/tls" -> "path/to/mypkg/internal/crypto/tls"
		"yours/internal/yourpkg" -> "path/to/mypkg/internal/yourpkg"
		"github.com/kardianos/osext" -> "patn/to/mypkg/internal/github.com/kardianos/osext"
	*/
	// The following method "cheats" and doesn't look at any external vendor file.
	ss := strings.SplitN(importPath, "/"+internalFolder+"/", 2)

	// TODO: Look for any vendor files. If the import is contained in a vendor file
	// return the vendor field for that package.
	return ss[len(ss)-1], nil
}
