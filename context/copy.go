// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
	"github.com/pkg/errors"
)

type fileInfoSort []os.FileInfo

func (l fileInfoSort) Len() int {
	return len(l)
}
func (l fileInfoSort) Less(i, j int) bool {
	a := l[i]
	b := l[j]
	if a.IsDir() == b.IsDir() {
		return l[i].Name() < l[j].Name()
	}
	return !a.IsDir()
}
func (l fileInfoSort) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// CopyPackage copies the files from the srcPath to the destPath, destPath
// folder and parents are are created if they don't already exist.
func (ctx *Context) CopyPackage(destPath, srcPath, lookRoot, pkgPath string, ignoreFiles []string, tree bool, h hash.Hash, beforeCopy func(deps []string) error) error {
	if pathos.FileStringEquals(destPath, srcPath) {
		return fmt.Errorf("Attempting to copy package to same location %q.", destPath)
	}
	err := os.MkdirAll(destPath, 0777)
	if err != nil {
		return err
	}

	// Ensure the dest is empty of files.
	destDir, err := os.Open(destPath)
	if err != nil {
		return err
	}
	ignoreTest := false
	for _, ignore := range ctx.ignoreTag {
		if ignore == "test" {
			ignoreTest = true
			break
		}
	}

	fl, err := destDir.Readdir(-1)
	destDir.Close()
	if err != nil {
		return err
	}
	for _, fi := range fl {
		if fi.IsDir() {
			if tree {
				err = errors.Wrap(os.RemoveAll(filepath.Join(destPath, fi.Name())), "remove all existing tree entries")
				if err != nil {
					return err
				}
			}
			continue
		}
		err = errors.Wrap(os.Remove(filepath.Join(destPath, fi.Name())), "remove existing file")
		if err != nil {
			return err
		}
	}

	// Copy files into dest.
	srcDir, err := os.Open(srcPath)
	if err != nil {
		return errors.Wrap(err, "open srcPath directory")
	}

	fl, err = srcDir.Readdir(-1)
	srcDir.Close()
	if err != nil {
		return errors.Wrap(err, "src readdir")
	}
	if h != nil {
		// Write relative path to GOPATH.
		h.Write([]byte(strings.Trim(pkgPath, "/")))
		// Sort file list to present a stable hash.
		sort.Sort(fileInfoSort(fl))
	}
fileLoop:
	for _, fi := range fl {
		name := fi.Name()
		if name[0] == '.' {
			continue
		}
		if fi.IsDir() {
			isTestdata := name == "testdata"
			if !tree && !isTestdata {
				continue
			}
			if name[0] == '_' {
				continue
			}
			if ignoreTest {
				if strings.HasSuffix(name, "_test") || isTestdata {
					continue
				}
			}
			nextDestPath := filepath.Join(destPath, name)
			nextSrcPath := filepath.Join(srcPath, name)
			var nextIgnoreFiles, deps []string
			if !isTestdata && !strings.Contains(pkgPath, "/testdata/") {
				nextIgnoreFiles, deps, err = ctx.getIgnoreFiles(nextSrcPath)
				if err != nil {
					return err
				}
			}
			if beforeCopy != nil {
				err = beforeCopy(deps)
				if err != nil {
					return errors.Wrap(err, "beforeCopy")
				}
			}
			err = ctx.CopyPackage(nextDestPath, nextSrcPath, lookRoot, path.Join(pkgPath, name), nextIgnoreFiles, true, h, beforeCopy)
			if err != nil {
				return errors.Wrapf(err,
					"CopyPackage dest=%q src=%q lookRoot=%q pkgPath=%q ignoreFiles=%q tree=%t has beforeCopy=%t",
					nextDestPath, nextSrcPath, lookRoot, path.Join(pkgPath, name), nextIgnoreFiles, true, beforeCopy != nil,
				)
			}
			continue
		}
		for _, ignore := range ignoreFiles {
			if pathos.FileStringEquals(name, ignore) {
				continue fileLoop
			}
		}
		if h != nil {
			h.Write([]byte(name))
		}
		err = copyFile(
			filepath.Join(destPath, name),
			filepath.Join(srcPath, name),
			h,
		)
		if err != nil {
			return errors.Wrapf(err, "copyFile dest=%q src=%q", filepath.Join(destPath, name), filepath.Join(srcPath, name))
		}
	}

	return errors.Wrapf(licenseCopy(lookRoot, srcPath, filepath.Join(ctx.RootDir, ctx.VendorFolder), pkgPath), "licenseCopy srcPath=%q", srcPath)
}

func copyFile(destPath, srcPath string, h hash.Hash) error {
	ss, err := os.Stat(srcPath)
	if err != nil {
		return errors.Wrap(err, "copyFile Stat")
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return errors.Wrapf(err, "open src=%q", srcPath)
	}
	defer src.Close()
	// Ensure we are not trying to copy a directory. May happen with symlinks.
	if st, err := src.Stat(); err == nil {
		if st.IsDir() {
			return nil
		}
	}

	dest, err := os.Create(destPath)
	if err != nil {
		return errors.Wrapf(err, "create dest=%q", destPath)
	}

	r := io.Reader(src)

	if h != nil {
		r = io.TeeReader(src, h)
	}

	_, err = io.Copy(dest, r)
	// Close before setting mod and time.
	dest.Close()
	if err != nil {
		return errors.Wrap(err, "copy")
	}
	err = os.Chmod(destPath, ss.Mode())
	if err != nil {
		return err
	}
	return os.Chtimes(destPath, ss.ModTime(), ss.ModTime())
}
