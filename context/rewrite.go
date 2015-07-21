// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kardianos/vendor/internal/github.com/dchest/safefile"
	"github.com/kardianos/vendor/internal/pathos"
)

// rules provides the translation from origional import path to new import path.
type ruleList map[string]string // map[from]to

// RewriteFiles rewrites files to the local path.
func (ctx *Context) rewriteFiles() error {
	ctx.dirty = true
	err := ctx.rewriteFilesByRule(ctx.MoveFile, ctx.MoveRule)
	if err != nil {
		return err
	}

	// Fixup import paths.
	for from, to := range ctx.MoveRule {
		if fromPkg := ctx.Package[from]; fromPkg != nil {
			fromPkg.Dir = filepath.Join(ctx.RootGopath, to)
			fromPkg.Status = StatusInternal
			fromPkg.Local = to
			delete(ctx.Package, from)
			ctx.Package[to] = fromPkg
		}
	}
	return ctx.determinePackageStatus()
}

// rewriteFilesByRule modified the imports according to rules and works on the
// file paths provided by filePaths.
func (ctx *Context) rewriteFilesByRule(filePaths map[string]*File, rules ruleList) error {
	goprint := &printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}
	for _, fileInfo := range filePaths {
		if pathos.FileHasPrefix(fileInfo.Path, ctx.RootDir) == false {
			continue
		}

		// Read the file into AST, modify the AST.
		fileset := token.NewFileSet()
		f, err := parser.ParseFile(fileset, fileInfo.Path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		for _, impNode := range f.Imports {
			imp, err := strconv.Unquote(impNode.Path.Value)
			if err != nil {
				return err
			}
			for from, to := range rules {
				if imp != from {
					continue
				}
				impNode.Path.Value = strconv.Quote(to)
				for i, metaImport := range fileInfo.Imports {
					if from == metaImport {
						fileInfo.Imports[i] = to
					}
				}
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
		fi, err := os.Stat(fileInfo.Path)
		if err != nil {
			return err
		}
		w, err := safefile.Create(fileInfo.Path, fi.Mode())
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
