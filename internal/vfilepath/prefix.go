package vfilepath

import (
	"path/filepath"
	"strings"
)

func HasPrefixDir(file string, prefix string) bool {
	return strings.HasPrefix(filepath.Clean(file), filepath.Clean(prefix)+"/")
}
