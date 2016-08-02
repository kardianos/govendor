package vfilepath

import (
	"path/filepath"
	"strings"
)

func HasPrefixDir(path string, prefix string) bool {
	return strings.HasPrefix(makeDirPath(path), makeDirPath(prefix))
}

func makeDirPath(path string) string {
	if path = filepath.Clean(path); path != "/" {
		path += "/"
	}
	return path
}
