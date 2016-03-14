// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
	filepath "github.com/kardianos/govendor/internal/vfilepath"
	os "github.com/kardianos/govendor/internal/vos"
)

var knownOS = make(map[string]bool)
var knownArch = make(map[string]bool)

func init() {
	for _, v := range strings.Fields(goosList) {
		knownOS[v] = true
	}
	for _, v := range strings.Fields(goarchList) {
		knownArch[v] = true
	}
}

// loadPackage sets up the context with package information and
// is called before any initial operation is performed.
func (ctx *Context) loadPackage() error {
	ctx.loaded = true
	ctx.dirty = false
	ctx.Package = make(map[string]*Package, len(ctx.Package))
	err := filepath.Walk(ctx.RootDir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return err
		}
		name := info.Name()
		// Still go into "_workspace" to aid godep migration.
		if info.IsDir() && (name[0] == '.' || name[0] == '_' || name == "testdata") && name != "_workspace" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		return ctx.addFileImports(path, ctx.RootGopath)
	})
	if err != nil {
		return err
	}
	return ctx.determinePackageStatus()
}

func (ctx *Context) getFileTags(pathname string, f *ast.File) (tags, imports []string, err error) {
	_, filenameExt := filepath.Split(pathname)

	if strings.HasSuffix(pathname, ".go") == false {
		return nil, nil, nil
	}
	if f == nil {
		f, err = parser.ParseFile(token.NewFileSet(), pathname, nil, parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			return nil, nil, err
		}
	}

	filename := filenameExt[:len(filenameExt)-3]

	l := strings.Split(filename, "_")
	tags = make([]string, 0, 6)

	if n := len(l); n > 1 && l[n-1] == "test" {
		l = l[:n-1]
		tags = append(tags, "test")
	}
	n := len(l)
	if n >= 2 && knownOS[l[n-2]] && knownArch[l[n-1]] {
		tags = append(tags, l[n-2])
		tags = append(tags, l[n-1])
	}
	if n >= 1 && knownOS[l[n-1]] {
		tags = append(tags, l[n-1])
	}
	if n >= 1 && knownArch[l[n-1]] {
		tags = append(tags, l[n-1])
	}

	const buildPrefix = "// +build "
	for _, cc := range f.Comments {
		for _, c := range cc.List {
			if strings.HasPrefix(c.Text, buildPrefix) {
				text := strings.TrimPrefix(c.Text, buildPrefix)
				ss := strings.Fields(text)
				for _, s := range ss {
					tags = append(tags, strings.Split(s, ",")...)
				}
			}
		}
	}
	imports = make([]string, 0, len(f.Imports))

	for i := range f.Imports {
		imp := f.Imports[i].Path.Value
		imp, err = strconv.Unquote(imp)
		if err != nil {
			return tags, imports, err
		}
		imports = append(imports, imp)
	}

	return tags, imports, nil
}

// addFileImports is called from loadPackage and resolveUnknown.
func (ctx *Context) addFileImports(pathname, gopath string) error {
	dir, filenameExt := filepath.Split(pathname)
	importPath := pathos.FileTrimPrefix(dir, gopath)
	importPath = pathos.SlashToImportPath(importPath)
	importPath = strings.TrimPrefix(importPath, "/")
	importPath = strings.TrimSuffix(importPath, "/")

	if strings.HasSuffix(pathname, ".go") == false {
		return nil
	}
	// No need to add the same file more then once.
	for _, pkg := range ctx.Package {
		if pathos.FileStringEquals(pkg.Dir, dir) == false {
			continue
		}
		for _, f := range pkg.Files {
			if pathos.FileStringEquals(f.Path, pathname) {
				return nil
			}
		}
		for _, f := range pkg.ignoreFile {
			if pathos.FileStringEquals(f, filenameExt) {
				return nil
			}
		}
	}
	// Ignore error here and continue on best effort.
	f, _ := parser.ParseFile(token.NewFileSet(), pathname, nil, parser.ImportsOnly|parser.ParseComments)
	if f == nil {
		f = &ast.File{}
	}

	tags, _, err := ctx.getFileTags(pathname, f)
	if err != nil {
		return err
	}
	// if file has "// +build ignore", can mix package main with normal package.
	isIgnore := false
	for _, tag := range tags {
		if tag == "ignore" {
			isIgnore = true
		}
	}
	// For now, just ignore ignored packages.
	if isIgnore {
		return nil
	}

	pkg, found := ctx.Package[importPath]
	if !found {
		status := Status{
			Type:     TypePackage,
			Location: LocationUnknown,
			Presence: PresenceFound,
		}
		if f.Name.Name == "main" {
			status.Type = TypeProgram
		}
		pkg = ctx.setPackage(dir, importPath, importPath, gopath, status)
		ctx.Package[importPath] = pkg
	}
	if pkg.Status.Location != LocationLocal {
		for _, tag := range tags {
			for _, ignore := range ctx.ignoreTag {
				if tag == ignore {
					pkg.ignoreFile = append(pkg.ignoreFile, filenameExt)
					return nil
				}
			}
		}
	}
	pf := &File{
		Package: pkg,
		Path:    pathname,
		Imports: make([]string, len(f.Imports)),
	}
	pkg.Files = append(pkg.Files, pf)
	for i := range f.Imports {
		imp := f.Imports[i].Path.Value
		imp, err = strconv.Unquote(imp)
		if err != nil {
			return err
		}
		if strings.HasPrefix(imp, "./") {
			imp = path.Join(importPath, imp)
		}
		pf.Imports[i] = imp
		err = ctx.addSingleImport(pkg.Dir, imp)
		if err != nil {
			return err
		}
	}

	// Record any import comment for file.
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
		// If it starts with the import text, assume it is the import comment.
		if index := strings.Index(ic.Text, " import "); index > 0 && index < 5 {
			q := strings.TrimSpace(ic.Text[index+len(" import "):])
			pf.ImportComment, err = strconv.Unquote(q)
			if err != nil {
				pf.ImportComment = q
			}
		}
	}

	return nil
}

func (ctx *Context) setPackage(dir, canonical, local, gopath string, status Status) *Package {
	at := 0
	vMiddle := "/" + pathos.SlashToImportPath(ctx.VendorDiscoverFolder) + "/"
	vStart := pathos.SlashToImportPath(ctx.VendorDiscoverFolder) + "/"
	switch {
	case strings.Contains(canonical, vMiddle):
		at = strings.LastIndex(canonical, vMiddle) + len(vMiddle)
	case strings.HasPrefix(canonical, vStart):
		at = strings.LastIndex(canonical, vStart) + len(vStart)
	}

	originDir := dir
	inVendor := false
	tree := false
	origin := ""
	if at > 0 {
		canonical = canonical[at:]
		inVendor = true
		if status.Location == LocationUnknown {
			p := path.Join(ctx.RootImportPath, ctx.VendorDiscoverFolder)
			if strings.HasPrefix(local, p) {
				status.Location = LocationVendor
				od, _, err := ctx.findImportDir("", canonical)
				if err == nil {
					originDir = od
				}
			}
		}
	}
	if vp := ctx.VendorFilePackagePath(canonical); vp != nil {
		tree = vp.Tree
		origin = vp.Origin
	}
	if status.Location == LocationUnknown && strings.HasPrefix(canonical, ctx.RootImportPath) {
		status.Location = LocationLocal
	}
	pkg := &Package{
		OriginDir: originDir,
		Dir:       dir,
		Origin:    origin,
		Canonical: canonical,
		Local:     local,
		Gopath:    gopath,
		Status:    status,
		inVendor:  inVendor,
		Tree:      tree,
	}
	ctx.Package[local] = pkg
	return pkg
}

func (ctx *Context) addSingleImport(pkgInDir, imp string) error {
	if _, found := ctx.Package[imp]; found {
		return nil
	}
	// Also need to check for vendor paths that won't use the local path in import path.
	for _, pkg := range ctx.Package {
		if pkg.Canonical == imp && pkg.inVendor && pathos.FileHasPrefix(pkg.Dir, pkgInDir) {
			return nil
		}
	}
	dir, gopath, err := ctx.findImportDir(pkgInDir, imp)
	if err != nil {
		if _, is := err.(ErrNotInGOPATH); is {
			ctx.setPackage("", imp, imp, "", Status{
				Type:     TypePackage,
				Location: LocationNotFound,
				Presence: PresenceMissing,
			})
			return nil
		}
		return err
	}
	if pathos.FileStringEquals(gopath, ctx.Goroot) {
		ctx.setPackage(dir, imp, imp, ctx.Goroot, Status{
			Type:     TypePackage,
			Location: LocationStandard,
			Presence: PresenceFound,
		})
		return nil
	}
	df, err := os.Open(dir)
	if err != nil {
		return err
	}
	info, err := df.Readdir(-1)
	df.Close()
	if err != nil {
		return err
	}
	for _, fi := range info {
		if fi.IsDir() {
			continue
		}
		switch fi.Name()[0] {
		case '.', '_':
			continue
		}
		if pathos.FileStringEquals(dir, pkgInDir) {
			continue
		}
		path := filepath.Join(dir, fi.Name())
		err = ctx.addFileImports(path, gopath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ctx *Context) determinePackageStatus() error {
	// Determine the status of remaining imports.
	for _, pkg := range ctx.Package {
		if pkg.Status.Location != LocationUnknown {
			continue
		}
		if strings.HasPrefix(pkg.Canonical, ctx.RootImportPath) {
			pkg.Status.Location = LocationLocal
			continue
		}
		pkg.Status.Location = LocationExternal
	}

	// Determine any un-used internal vendor imports.
	ctx.updatePackageReferences()
	for i := 0; i <= looplimit; i++ {
		altered := false
		for path, pkg := range ctx.Package {
			if pkg.Status.Presence == PresenceUnsued || pkg.Status.Type == TypeProgram || pkg.Status.Presence == PresenceTree {
				continue
			}
			if len(pkg.referenced) == 0 && pkg.Status.Location == LocationVendor {
				altered = true
				pkg.Status.Presence = PresenceUnsued
				for _, other := range ctx.Package {
					delete(other.referenced, path)
				}
			}
		}
		if !altered {
			break
		}
		if i == looplimit {
			panic("determinePackageStatus loop limit")
		}
	}
	for _, pkg := range ctx.Package {
		if parentTrees := ctx.findPackageParentTree(pkg); len(parentTrees) > 0 {
			pkg.Status.Presence = PresenceTree
		}
	}

	// Add any packages in the vendor file but not in GOPATH or vendor dir.
	for _, vp := range ctx.VendorFile.Package {
		if vp.Remove {
			continue
		}
		if _, found := ctx.Package[vp.Path]; found {
			continue
		}
		err := ctx.addSingleImport("", vp.Path)
		if err != nil {
			return err
		}
		if pkg, found := ctx.Package[vp.Path]; found {
			pkg.Origin = vp.Origin
			pkg.inTree = vp.Tree
		}
	}
	return nil
}
