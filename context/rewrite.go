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
	"strconv"
	"strings"

	"github.com/kardianos/vendor/internal/github.com/dchest/safefile"
	"github.com/kardianos/vendor/internal/pathos"
)

// rule provides the translation from origional import path to new import path.
type rule struct {
	From string
	To   string
}

// RewriteFiles rewrites files to the local path.
func (ctx *Context) RewriteFiles(checkPaths ...string) error {
	if !ctx.loaded {
		ctx.loadPackage()
	}
	fileImports := make(map[string]map[string]*File) // map[ImportPath]map[FilePath]File
	// Add files to import map.
	for _, pkg := range ctx.Package {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				fileList := fileImports[imp]
				if fileList == nil {
					fileList = make(map[string]*File, 1)
					fileImports[imp] = fileList
				}
				fileList[f.Path] = f
			}
		}
	}

	// Determine which files to touch.
	files := make(map[string]*File, len(ctx.VendorFile.Package)*3)

	// Rules are all lines in the vendor file.
	rules := make([]rule, 0, len(ctx.VendorFile.Package))
	for _, vp := range ctx.VendorFile.Package {
		if vp.Remove == true {
			continue
		}
		for fpath, f := range fileImports[vp.Canonical] {
			files[fpath] = f
		}
		rules = append(rules, rule{From: vp.Canonical, To: vp.Local})
	}
	// Add local package files.
	for _, localImportPath := range checkPaths {
		if localPkg, found := ctx.Package[localImportPath]; found {
			for _, f := range localPkg.Files {
				files[f.Path] = f
			}
		}
	}
	// Rewrite any external package where the local path is different then the vendor path.
	for _, pkg := range ctx.Package {
		if pkg.Status != StatusExternal {
			continue
		}
		if pkg.CanonicalPath == pkg.LocalPath {
			continue
		}
		for _, otherPkg := range ctx.Package {
			if pkg == otherPkg {
				continue
			}
			if otherPkg.Status != StatusInternal {
				continue
			}
			if otherPkg.LocalPath != pkg.LocalPath {
				continue
			}

			for fpath, f := range fileImports[pkg.CanonicalPath] {
				files[fpath] = f

			}
			rules = append(rules, rule{From: pkg.CanonicalPath, To: otherPkg.CanonicalPath})
			break
		}
	}
	err := ctx.rewriteFilesByRule(files, rules)
	if err != nil {
		return err
	}

	// Fixup import paths.
	ctx.updatePackageReferences()
	st := newUnknownSet()
	for _, rule := range rules {
		if from := ctx.Package[rule.From]; from != nil {
			from.Status = StatusUnknown
			for rpath, ref := range from.referenced {
				ref.Status = StatusUnknown
				st.add("", rpath)
			}
		}
		if to := ctx.Package[rule.To]; to != nil {
			to.Status = StatusUnknown
		}
		st.add("", rule.From)
		st.add("", rule.To)
	}
	return ctx.resolveUnknown(st)
}

// rewriteFilesByRule modified the imports according to rules and works on the
// file paths provided by filePaths.
func (ctx *Context) rewriteFilesByRule(filePaths map[string]*File, rules []rule) error {
	goprint := &printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}
	for pathname, fileInfo := range filePaths {
		if pathos.FileHasPrefix(pathname, ctx.RootDir) == false {
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
				for i, metaImport := range fileInfo.Imports {
					if rule.From == metaImport {
						fileInfo.Imports[i] = rule.To
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
