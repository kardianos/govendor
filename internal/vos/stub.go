package vos

import (
	"os"
	"time"
)

type FileInfo os.FileInfo

func Stat(name string) (FileInfo, error) {
	l("stat", name)
	fi, err := os.Stat(name)
	return FileInfo(fi), err
}
func Lstat(name string) (FileInfo, error) {
	l("lstat", name)
	fi, err := os.Lstat(name)
	return FileInfo(fi), err
}
func IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

func Getwd() (string, error) {
	return os.Getwd()
}

func Getenv(key string) string {
	return os.Getenv(key)
}

func Open(name string) (*os.File, error) {
	l("open", name)
	return os.Open(name)
}

func MkdirAll(path string, perm os.FileMode) error {
	l("mkdirall", path)
	return os.MkdirAll(path, perm)
}

func Remove(name string) error {
	l("remove", name)
	return os.Remove(name)
}
func RemoveAll(name string) error {
	l("removeall", name)
	return os.RemoveAll(name)
}
func Create(name string) (*os.File, error) {
	l("create", name)
	return os.Create(name)
}
func Chmod(name string, mode os.FileMode) error {
	l("chmod", name)
	return os.Chmod(name, mode)
}
func Chtimes(name string, atime, mtime time.Time) error {
	l("chtimes", name)
	return os.Chtimes(name, atime, mtime)
}
