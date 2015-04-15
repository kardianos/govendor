// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rewrite

import (
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kardianos/vendor/internal/github.com/dchest/safefile"
)

/*
	function to copy package into internal.
	function to rewrite N files with M import changes. If M == 0 then just rewrite.
	function to remove package from internal.
*/

// CopyPackage copies the files from the srcPath to the destPath, destPath
// folder and parents are are created if they don't already exist.
func CopyPackage(destPath, srcPath string) error {
	err := os.MkdirAll(destPath, 0777)
	if err != nil {
		return err
	}

	// Ensure the dest is empty of files.
	destDir, err := os.Open(destPath)
	if err != nil {
		return err
	}

	fl, err := destDir.Readdir(-1)
	destDir.Close()
	if err != nil {
		return err
	}
	for _, fi := range fl {
		if fi.IsDir() {
			continue
		}
		err = os.Remove(filepath.Join(destPath, fi.Name()))
		if err != nil {
			return err
		}
	}

	// Copy files into dest.
	srcDir, err := os.Open(srcPath)
	if err != nil {
		return err
	}

	fl, err = srcDir.Readdir(-1)
	srcDir.Close()
	if err != nil {
		return err
	}
	for _, fi := range fl {
		if fi.IsDir() {
			continue
		}
		if fi.Name()[0] == '.' {
			continue
		}
		err = copyFile(
			filepath.Join(destPath, fi.Name()),
			filepath.Join(srcPath, fi.Name()),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func copyFile(destPath, srcPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(dest, src)
	// Close before setting mod and time.
	dest.Close()
	if err != nil {
		return err
	}
	ss, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	err = os.Chmod(destPath, ss.Mode())
	if err != nil {
		return err
	}
	return os.Chtimes(destPath, ss.ModTime(), ss.ModTime())
}

// RemovePackage removes the specified folder files. If folder is empty when
// done (no nested folders, remove the folder and any empty parent folders.
func RemovePackage(path string) error {
	// TODO: RemoveFolder
	return nil
}

// Rule provides the translation from origional import path to new import path.
type Rule struct {
	From	string
	To		string
}

// RewriteFiles modified the imports according to rules and works on the
// file paths provided by filePaths.
func RewriteFiles(filePaths []string, rules []Rule) error {
	goprint := &printer.Config{
		Mode:		printer.TabIndent,
		Tabwidth:	4,
	}
	for _, path := range filePaths {
		// Read the file into AST, modify the AST.
		fileset := token.NewFileSet()
		f, err := parser.ParseFile(fileset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		for _, impNode := range f.Imports {
			imp, err := strconv.Unquote(impNode.Path.Value)
			if err != nil {
				return err
			}
			for _, rule := range rules {
				if imp != rule.From {
					continue
				}
				impNode.Path.Value = strconv.Quote(rule.To)
				break
			}
		}

		// Don't sort or modify the imports to minimize diffs.

		// Write the AST back to disk.
		fi, err := os.Stat(path)
		if err != nil {
			return err
		}
		w, err := safefile.Create(path, fi.Mode())
		if err != nil {
			return err
		}
		err = goprint.Fprint(w, fileset, f)
		if err != nil {
			w.Close()
			return err
		}
		err = w.Commit()
		if err != nil {
			return err
		}
	}
	return nil
}
