// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rewrite

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kardianos/govendor/internal/github.com/dchest/safefile"
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
	// Ensure the path is empty of files.
	dir, err := os.Open(path)
	if err != nil {
		return err
	}

	fl, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		return err
	}
	for _, fi := range fl {
		if fi.IsDir() {
			continue
		}
		err = os.Remove(filepath.Join(path, fi.Name()))
		if err != nil {
			return err
		}
	}

	// Ignore errors here.
	for i := 0; i <= looplimit; i++ {
		dir, err := os.Open(path)
		if err != nil {
			// fmt.Fprintf(os.Stderr, "Failedd to open directory %q: %v\n", path, err)
			return nil
		}

		fl, err := dir.Readdir(1)
		dir.Close()
		if err != nil && err != io.EOF {
			// fmt.Fprintf(os.Stderr, "Failedd to list directory %q: %v\n", path, err)
			return nil
		}
		if len(fl) > 0 {
			return nil
		}
		err = os.Remove(path)
		if err != nil {
			// fmt.Fprintf(os.Stderr, "Failedd to remove empty directory %q: %v\n", path, err)
			return nil
		}
		nextPath := filepath.Clean(filepath.Join(path, ".."))
		// Check for root.
		if nextPath == path {
			return nil
		}
		path = nextPath
	}
	return ErrLoopLimit{"RemovePackage() remove parent folders"}
}

// Rule provides the translation from origional import path to new import path.
type Rule struct {
	From string
	To   string
}

// RewriteFiles modified the imports according to rules and works on the
// file paths provided by filePaths.
func (ctx *Context) RewriteFiles(filePaths map[string]*File, rules []Rule) error {
	goprint := &printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}
	for pathname, fileInfo := range filePaths {
		if fileHasPrefix(pathname, ctx.RootDir) == false {
			continue
		}

		// Read the file into AST, modify the AST.
		fileset := token.NewFileSet()
		f, err := parser.ParseFile(fileset, pathname, nil, parser.ParseComments)
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

		// Remove import comment.
		st := fileInfo.Package.Status
		if st == StatusInternal || st == StatusUnused {
			var ic *ast.Comment
			if f.Name != nil {
				pos := f.Name.Pos()
			big:
				// Find the next comment after the package name.
				for _, cblock := range f.Comments {
					for _, c := range cblock.List {
						if c.Pos() > pos {
							ic = c
							break big
						}
					}
				}
			}
			if ic != nil {
				// If it starts with the import text, assume it is the import comment and remove.
				if index := strings.Index(ic.Text, " import "); index > 0 && index < 5 {
					ic.Text = strings.Repeat(" ", len(ic.Text))
				}
			}
		}

		// Don't sort or modify the imports to minimize diffs.

		// Write the AST back to disk.
		fi, err := os.Stat(pathname)
		if err != nil {
			return err
		}
		w, err := safefile.Create(pathname, fi.Mode())
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
