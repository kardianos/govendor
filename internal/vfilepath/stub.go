package vfilepath

import (
	"path/filepath"
)

func Split(path string) (string, string) {
	return filepath.Split(path)
}

func Join(parts ...string) string {
	return filepath.Join(parts...)
}
