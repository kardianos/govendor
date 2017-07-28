// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
	filepath "github.com/kardianos/govendor/internal/vfilepath"
	os "github.com/kardianos/govendor/internal/vos"
	"github.com/kardianos/govendor/pkgspec"
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
	ctx.statusCache = nil
	ctx.Package = make(map[string]*Package, len(ctx.Package))
	// We following the root symlink only in case the root of the repo is symlinked into the GOPATH
	// This could happen during on some CI that didn't checkout into the GOPATH
	rootdir, err := filepath.EvalSymlinks(ctx.RootDir)
	if err != nil {
		return err
	}
	err = filepath.Walk(rootdir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return err
		}
		if !info.IsDir() {
			// We replace the directory path (followed by the symlink), to the real go repo package name/path
			// ex : replace "<somewhere>/govendor.source.repo" to "github.com/kardianos/govendor"
			path = strings.Replace(path, rootdir, ctx.RootDir, 1)
			_, err = ctx.addFileImports(path, ctx.RootGopath)
			return err
		}
		name := info.Name()
		// Still go into "_workspace" to aid godep migration.
		if name == "_workspace" {
			return nil
		}
		switch name[0] {
		case '.', '_':
			return filepath.SkipDir
		}
		switch name {
		case "testdata", "node_modules":
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Finally, set any unset status.
	return ctx.determinePackageStatus()
}

func (ctx *Context) getFileTags(pathname string, f *ast.File) (tags *TagSet, imports []string, err error) {
	_, filenameExt := filepath.Split(pathname)

	if strings.HasSuffix(pathname, ".go") == false {
		return nil, nil, nil
	}
	if f == nil {
		f, err = parser.ParseFile(token.NewFileSet(), pathname, nil, parser.ImportsOnly|parser.ParseComments)
		if f == nil {
			return nil, nil, nil
		}
	}
	tags = &TagSet{}
	if strings.HasSuffix(f.Name.Name, "_test") {
		tags.AddFileTag("test")
	}
	pkgNameNormalized := strings.TrimSuffix(f.Name.Name, "_test")

	// Files with package name "documentation" should be ignored, per go build tool.
	if pkgNameNormalized == "documentation" {
		return nil, nil, nil
	}

	filename := filenameExt[:len(filenameExt)-3]

	l := strings.Split(filename, "_")

	if n := len(l); n > 1 && l[n-1] == "test" {
		l = l[:n-1]
		tags.AddFileTag("test")
	}
	n := len(l)
	if n >= 2 && knownOS[l[n-2]] && knownArch[l[n-1]] {
		tags.AddFileTag(l[n-2])
		tags.AddFileTag(l[n-1])
	}
	if n >= 1 && knownOS[l[n-1]] {
		tags.AddFileTag(l[n-1])
	}
	if n >= 1 && knownArch[l[n-1]] {
		tags.AddFileTag(l[n-1])
	}

	const buildPrefix = "// +build "
	for _, cc := range f.Comments {
		for _, c := range cc.List {
			if strings.HasPrefix(c.Text, buildPrefix) {
				text := strings.TrimPrefix(c.Text, buildPrefix)
				tags.AddBuildTags(text)
			}
		}
	}
	imports = make([]string, 0, len(f.Imports))

	for i := range f.Imports {
		imp := f.Imports[i].Path.Value
		imp, err = strconv.Unquote(imp)
		if err != nil {
			// Best errort
			continue
		}
		imports = append(imports, imp)
	}

	return tags, imports, nil
}

// addFileImports is called from loadPackage and resolveUnknown.
func (ctx *Context) addFileImports(pathname, gopath string) (*Package, error) {
	dir, filenameExt := filepath.Split(pathname)
	importPath := pathos.FileTrimPrefix(dir, gopath)
	importPath = pathos.SlashToImportPath(importPath)
	importPath = strings.Trim(importPath, "/")

	if strings.HasSuffix(pathname, ".go") == false {
		return nil, nil
	}
	// No need to add the same file more than once.
	for _, pkg := range ctx.Package {
		if pathos.FileStringEquals(pkg.Dir, dir) == false {
			continue
		}
		for _, f := range pkg.Files {
			if pathos.FileStringEquals(f.Path, pathname) {
				return nil, nil
			}
		}
		for _, f := range pkg.ignoreFile {
			if pathos.FileStringEquals(f, filenameExt) {
				return nil, nil
			}
		}
	}
	// Ignore error here and continue on best effort.
	f, _ := parser.ParseFile(token.NewFileSet(), pathname, nil, parser.ImportsOnly|parser.ParseComments)
	if f == nil {
		return nil, nil
	}
	pkgNameNormalized := strings.TrimSuffix(f.Name.Name, "_test")

	// Files with package name "documentation" should be ignored, per go build tool.
	if pkgNameNormalized == "documentation" {
		return nil, nil
	}

	tags, _, err := ctx.getFileTags(pathname, f)
	if err != nil {
		return nil, err
	}
	// If file has "// +build ignore", can mix package main with normal package.
	// For now, just ignore ignored packages.
	if tags.IgnoreItem() {
		return nil, nil
	}

	pkg, found := ctx.Package[importPath]
	if !found {
		status := Status{
			Type:     TypePackage,
			Location: LocationUnknown,
			Presence: PresenceFound,
		}
		if pkgNameNormalized == "main" {
			status.Type = TypeProgram
		}
		pkg = ctx.setPackage(dir, importPath, importPath, gopath, status)
		ctx.Package[importPath] = pkg
	}
	if pkg.Status.Location != LocationLocal {
		if tags.IgnoreItem(ctx.ignoreTag...) {
			pkg.ignoreFile = append(pkg.ignoreFile, filenameExt)
			return pkg, nil
		}
		// package excluded if non-local && same name or sub-package of an excluded package
		for _, exclude := range ctx.excludePackage {
			if importPath == exclude || strings.HasPrefix(importPath, exclude+"/") {
				pkg.Status.Presence = PresenceExcluded
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
			// Best effort only.
			continue
		}
		if strings.HasPrefix(imp, "./") {
			imp = path.Join(importPath, imp)
		}
		pf.Imports[i] = imp
		if pkg.Status.Presence != PresenceExcluded { // do not add package imports if it was explicitly excluded
			_, err = ctx.addSingleImport(pkg.Dir, imp, pkg.IncludeTree)
			if err != nil {
				return pkg, err
			}
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

	return pkg, nil
}

func (ctx *Context) setPackage(dir, canonical, local, gopath string, status Status) *Package {
	if pkg, exists := ctx.Package[local]; exists {
		return pkg
	}
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
	// Set originDir correctly if origin is set.
	if len(origin) > 0 {
		od, _, err := ctx.findImportDir("", origin)
		if err == nil {
			originDir = od
		}
	}
	if status.Location == LocationUnknown && filepath.HasPrefixDir(canonical, ctx.RootImportPath) {
		status.Location = LocationLocal
	}
	spec, err := pkgspec.Parse("", canonical)
	if err != nil {
		panic(err)
	}
	if len(origin) > 0 && origin != canonical {
		spec.Origin = origin
	}
	spec.IncludeTree = tree
	pkg := &Package{
		OriginDir: originDir,
		Dir:       dir,
		Pkg:       spec,
		Local:     local,
		Gopath:    gopath,
		Status:    status,
		inVendor:  inVendor,
	}
	ctx.Package[local] = pkg
	return pkg
}

var testNeedsSortOrder = false

func (ctx *Context) addSingleImport(pkgInDir, imp string, tree bool) (*Package, error) {
	// Do not check for existing package right away. If a external package
	// has been added and we are looking in a vendor package, this won't work.
	// We need to search any relative vendor folders first.

	// Also need to check for vendor paths that won't use the local path in import path.
	for _, pkg := range ctx.Package {
		if pkg.Path == imp && pkg.inVendor && pathos.FileHasPrefix(pkg.Dir, pkgInDir) {
			return nil, nil
		}
	}
	dir, gopath, err := ctx.findImportDir(pkgInDir, imp)
	if err != nil {
		if _, is := err.(ErrNotInGOPATH); is {
			presence := PresenceMissing
			// excluded packages, don't need to be present
			for _, exclude := range ctx.excludePackage {
				if imp == exclude || strings.HasPrefix(imp, exclude+"/") {
					presence = PresenceExcluded
				}
			}
			return ctx.setPackage("", imp, imp, "", Status{
				Type:     TypePackage,
				Location: LocationNotFound,
				Presence: presence,
			}), nil
		}
		return nil, err
	}
	if pathos.FileStringEquals(gopath, ctx.Goroot) {
		return ctx.setPackage(dir, imp, imp, ctx.Goroot, Status{
			Type:     TypePackage,
			Location: LocationStandard,
			Presence: PresenceFound,
		}), nil
	}
	if tree {
		return ctx.setPackage(dir, imp, imp, ctx.RootGopath, Status{
			Type:     TypePackage,
			Location: LocationVendor,
			Presence: PresenceFound,
		}), nil
	}
	df, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	info, err := df.Readdir(-1)
	df.Close()
	if err != nil {
		return nil, err
	}
	if testNeedsSortOrder {
		sort.Sort(fileInfoSort(info))
	}
	var pkg *Package
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
		tryPkg, err := ctx.addFileImports(path, gopath)
		if tryPkg != nil {
			pkg = tryPkg
		}
		if err != nil {
			return pkg, err
		}
	}
	return pkg, nil
}

func (ctx *Context) determinePackageStatus() error {
	// Add any packages in the vendor file but not in GOPATH or vendor dir.
	for _, vp := range ctx.VendorFile.Package {
		if vp.Remove {
			continue
		}
		if _, found := ctx.Package[vp.Path]; found {
			continue
		}
		pkg, err := ctx.addSingleImport(ctx.RootDir, vp.Path, vp.Tree)
		if err != nil {
			return err
		}
		if pkg != nil {
			pkg.Origin = vp.Origin
			pkg.inTree = vp.Tree
			pkg.inVendor = true
		}
	}

	// Determine the status of remaining imports.
	for _, pkg := range ctx.Package {
		if pkg.Status.Location != LocationUnknown {
			continue
		}
		if filepath.HasPrefixDir(pkg.Path, ctx.RootImportPath) {
			pkg.Status.Location = LocationLocal
			continue
		}
		pkg.Status.Location = LocationExternal
	}

	ctx.updatePackageReferences()

	// Mark sub-tree packages as "tree", but leave any existing bit (unused) on the
	// parent most tree package.
	for path, pkg := range ctx.Package {
		if vp := ctx.VendorFilePackagePath(pkg.Path); vp != nil && vp.Tree {
			// Remove internal tree references.
			del := make([]string, 0, 6)
			for opath, opkg := range pkg.referenced {
				if strings.HasPrefix(opkg.Path, pkg.Path+"/") {
					del = append(del, opath)
				}
			}
			delete(pkg.referenced, pkg.Local) // remove any self reference
			for _, d := range del {
				delete(pkg.referenced, d)
			}
			continue
		}

		if parentTrees := ctx.findPackageParentTree(pkg); len(parentTrees) > 0 {
			pkg.Status.Presence = PresenceTree

			// Transfer all references from the child to the top parent.
			if parentPkg := ctx.Package[parentTrees[0]]; parentPkg != nil {
				for opath, opkg := range pkg.referenced {
					// Do not transfer internal references.
					if strings.HasPrefix(opkg.Path, parentPkg.Path+"/") {
						continue
					}
					parentPkg.referenced[opath] = opkg
				}
				pkg.referenced = make(map[string]*Package, 0)
				for _, opkg := range ctx.Package {
					if _, has := opkg.referenced[path]; has {
						opkg.referenced[parentPkg.Local] = parentPkg
						delete(opkg.referenced, path)
					}
				}
			}
		}
	}

	ctx.updatePackageReferences()

	// Determine any un-used internal vendor imports.
	for i := 0; i <= looplimit; i++ {
		altered := false
		for path, pkg := range ctx.Package {
			if pkg.Status.Presence == PresenceUnused || pkg.Status.Presence == PresenceTree || pkg.Status.Type == TypeProgram {
				continue
			}
			if len(pkg.referenced) > 0 || pkg.Status.Location != LocationVendor {
				continue
			}
			altered = true
			pkg.Status.Presence = PresenceUnused
			for _, other := range ctx.Package {
				delete(other.referenced, path)
			}
		}
		if !altered {
			break
		}
		if i == looplimit {
			panic("determinePackageStatus loop limit")
		}
	}

	ctx.updatePackageReferences()

	// Unused external references may have worked their way in through
	// vendor file. Remove any external leafs.
	for i := 0; i <= looplimit; i++ {
		altered := false
		for path, pkg := range ctx.Package {
			if len(pkg.referenced) > 0 || pkg.Status.Location != LocationExternal {
				continue
			}
			altered = true
			delete(ctx.Package, path)
			pkg.Status.Presence = PresenceUnused
			for _, other := range ctx.Package {
				delete(other.referenced, path)
			}
			continue
		}
		if !altered {
			break
		}
		if i == looplimit {
			panic("determinePackageStatus loop limit")
		}
	}

	return nil
}
