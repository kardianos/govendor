// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strconv"
	"strings"

	"github.com/dchest/safefile"
	"github.com/kardianos/govendor/internal/pathos"
	os "github.com/kardianos/govendor/internal/vos"
)

// Rewrite rewrites files to the local path.
func (ctx *Context) rewrite() error {
	if !ctx.rewriteImports {
		return nil
	}
	if ctx.dirty {
		ctx.loadPackage()
	}
	ctx.dirty = true

	fileImports := make(map[string]map[string]*File) // map[ImportPath]map[FilePath]File
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
	filePaths := make(map[string]*File, len(ctx.RewriteRule))
	for from, to := range ctx.RewriteRule {
		// Add files that contain an import path to rewrite.
		for _, f := range fileImports[from] {
			filePaths[f.Path] = f
		}

		// Add files that contain import comments to remove.
		if pkg := ctx.Package[from]; pkg != nil {
			for _, f := range pkg.Files {
				if len(f.ImportComment) != 0 {
					filePaths[f.Path] = f
				}
			}
		}
		if pkg := ctx.Package[to]; pkg != nil {
			for _, f := range pkg.Files {
				if len(f.ImportComment) != 0 {
					filePaths[f.Path] = f
				}
			}
		}
	}

	/*
		RULE: co2/internal/co3/pk3 -> co1/internal/co3/pk3

		i co1/internal/co2/pk2 [co2/pk2] < ["co1/pk1"]
		i co1/internal/co3/pk3 [co3/pk3] < ["co1/pk1"]
		e co2/internal/co3/pk3 [co3/pk3] < ["co1/internal/co2/pk2"]
		l co1/pk1 < []
		s strings < ["co1/internal/co3/pk3" "co2/internal/co3/pk3"]

		Rewrite the package "co1/internal/co2/pk2" because it references a package with a rewrite.from package.
	*/
	ctx.updatePackageReferences()
	for from := range ctx.RewriteRule {
		pkg := ctx.Package[from]
		if pkg == nil {
			continue
		}
		for _, ref := range pkg.referenced {
			for _, f := range ref.Files {
				dprintf("REF RW %s\n", f.Path)
				filePaths[f.Path] = f
			}
		}
	}

	defer func() {
		ctx.RewriteRule = make(map[string]string, 3)
	}()

	if len(ctx.RewriteRule) == 0 {
		return nil
	}
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
		if f == nil {
			return nil
		}
		pkgNameNormalized := strings.TrimSuffix(f.Name.Name, "_test")
		// Files with package name "documentation" should be ignored, per go build tool.
		if pkgNameNormalized == "documentation" {
			return nil
		}

		dprintf("RW:: File: %s\n", fileInfo.Path)

		for _, impNode := range f.Imports {
			imp, err := strconv.Unquote(impNode.Path.Value)
			if err != nil {
				return err
			}
			for from, to := range ctx.RewriteRule {
				if imp != from {
					continue
				}
				impNode.Path.Value = strconv.Quote(to)
				for i, metaImport := range fileInfo.Imports {
					if from == metaImport {
						dprintf("\tImport: %s -> %s\n", from, to)
						fileInfo.Imports[i] = to
					}
				}
				break
			}
		}

		// Remove import comment.
		st := fileInfo.Package.Status
		if st.Location == LocationVendor || st.Location == LocationExternal {
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

func (ctx *Context) makeSet(pkg *Package, mvSet map[*Package]struct{}) {
	mvSet[pkg] = struct{}{}
	for _, f := range pkg.Files {
		for _, imp := range f.Imports {
			next := ctx.Package[imp]
			switch {
			default:
				if _, has := mvSet[next]; !has {
					ctx.makeSet(next, mvSet)
				}
			case next == nil:
			case next.Path == next.Local:
			case next.Status.Location != LocationExternal:
			}
		}
	}
}
