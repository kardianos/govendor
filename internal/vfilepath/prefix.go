package vfilepath

import "strings"

func HasPrefixDir(path string, prefix string) bool {
	return strings.HasPrefix(makeDirPath(path), makeDirPath(prefix))
}

func makeDirPath(path string) string {
	if path != "/" {
		path += "/"
	}
	return path
}
