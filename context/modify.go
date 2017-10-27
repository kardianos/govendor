// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package context gathers the status of packages and stores it in Context.
// A new Context needs to be pointed to the root of the project and any
// project owned vendor file.
package context

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/kardianos/govendor/internal/pathos"
	os "github.com/kardianos/govendor/internal/vos"
	"github.com/kardianos/govendor/pkgspec"
	"github.com/kardianos/govendor/vcs"
	"github.com/kardianos/govendor/vendorfile"
	"github.com/pkg/errors"
)

// OperationState is the state of the given package move operation.
type OperationState byte

const (
	OpReady  OperationState = iota // Operation is ready to go.
	OpIgnore                       // Operation should be ignored.
	OpDone                         // Operation has been completed.
)

type OperationType byte

const (
	OpCopy OperationType = iota
	OpRemove
	OpFetch
)

func (t OperationType) String() string {
	switch t {
	default:
		panic("unknown operation type")
	case OpCopy:
		return "copy"
	case OpRemove:
		return "remove"
	case OpFetch:
		return "fetch"
	}
}

// Operation defines how packages should be moved.
//
// TODO (DT): Remove Pkg field and change Src and Dest to *pkgspec.Pkg types.
type Operation struct {
	Type OperationType

	Pkg *Package

	// Source file path to move packages from.
	// Must not be empty.
	Src string

	// Destination file path to move package to.
	// If Dest if empty the package is removed.
	Dest string

	// Files to ignore for operation.
	IgnoreFile []string

	State OperationState

	// True if the operation should treat the package as uncommitted.
	Uncommitted bool
}

// Conflict reports packages that are scheduled to conflict.
type Conflict struct {
	Canonical string
	Local     string
	Operation []*Operation
	OpIndex   int
	Resolved  bool
}

// Modify is the type of modifcation to do.
type Modify byte

const (
	AddUpdate Modify = iota // Add or update the import.
	Add                     // Only add, error if it already exists.
	Update                  // Only update, error if it doesn't currently exist.
	Remove                  // Remove from vendor path.
	Fetch                   // Get directly from remote repository.
)

type ModifyOption byte

const (
	Uncommitted ModifyOption = iota
	MatchTree
	IncludeTree
)

// ModifyStatus adds packages to the context by status.
func (ctx *Context) ModifyStatus(sg StatusGroup, mod Modify, mops ...ModifyOption) error {
	if ctx.added == nil {
		ctx.added = make(map[string]bool, 10)
	}

	list, err := ctx.Status()
	if err != nil {
		return err
	}

	// Add packages from status.
statusLoop:
	for _, item := range list {
		if !item.Status.MatchGroup(sg) {
			continue
		}
		if ctx.added[item.Pkg.PathOrigin()] {
			continue
		}
		// Do not add excluded packages
		if item.Status.Presence == PresenceExcluded {
			continue
		}
		// Do not attempt to add any existing status items that are
		// already present in vendor folder.
		if mod == Add {
			if ctx.VendorFilePackagePath(item.Pkg.Path) != nil {
				continue
			}
			for _, pkg := range ctx.Package {
				if pkg.Status.Location == LocationVendor && item.Pkg.Path == pkg.Path {
					continue statusLoop
				}
			}
		}

		err = ctx.modify(item.Pkg, mod, mops)
		if err != nil {
			// Skip these errors if from status.
			if _, is := err.(ErrTreeChildren); is {
				continue
			}
			if _, is := err.(ErrTreeParents); is {
				continue
			}
			return err
		}
	}
	return nil
}

// ModifyImport adds the package to the context.
func (ctx *Context) ModifyImport(imp *pkgspec.Pkg, mod Modify, mops ...ModifyOption) error {
	var err error
	if ctx.added == nil {
		ctx.added = make(map[string]bool, 10)
	}
	// Grap the origin of the pkg spec from the vendor file as needed.
	if len(imp.Origin) == 0 {
		for _, vpkg := range ctx.VendorFile.Package {
			if vpkg.Remove {
				continue
			}
			if vpkg.Path == imp.Path {
				imp.Origin = vpkg.Origin
			}
		}
	}
	if !imp.MatchTree {
		if !ctx.added[imp.PathOrigin()] {
			err = ctx.modify(imp, mod, mops)
			if err != nil {
				return err
			}
		}
		return nil
	}
	list, err := ctx.Status()
	if err != nil {
		return err
	}
	// If add any matched from "...".
	match := imp.Path + "/"
	for _, item := range list {
		if ctx.added[item.Pkg.PathOrigin()] {
			continue
		}
		if item.Pkg.Path != imp.Path && !strings.HasPrefix(item.Pkg.Path, match) {
			continue
		}
		if imp.HasVersion {
			item.Pkg.HasVersion = true
			item.Pkg.Version = imp.Version
		}
		item.Pkg.HasOrigin = imp.HasOrigin
		item.Pkg.Origin = path.Join(imp.PathOrigin(), strings.TrimPrefix(item.Pkg.Path, imp.Path))
		err = ctx.modify(item.Pkg, mod, mops)
		if err != nil {
			return err
		}
	}
	// cache for later use
	ctx.TreeImport = append(ctx.TreeImport, imp)
	return nil
}

func (ctx *Context) modify(ps *pkgspec.Pkg, mod Modify, mops []ModifyOption) error {
	ctx.added[ps.PathOrigin()] = true
	for _, mop := range mops {
		switch mop {
		default:
			panic("unknown case")
		case Uncommitted:
			ps.Uncommitted = true
		case MatchTree:
			ps.MatchTree = true
		case IncludeTree:
			ps.IncludeTree = true
		}
	}
	var err error
	if !ctx.loaded || ctx.dirty {
		err = ctx.loadPackage()
		if err != nil {
			return err
		}
	}
	tree := ps.IncludeTree

	switch mod {
	// Determine if we can find the source path from an add or update.
	case Add, Update, AddUpdate:
		_, _, err = ctx.findImportDir("", ps.PathOrigin())
		if err != nil {
			return err
		}
	}

	// Does the local import exist?
	//   If so either update or just return.
	//   If not find the disk path from the canonical path, copy locally and rewrite (if needed).
	var pkg *Package
	var foundPkg bool
	if !foundPkg {
		localPath := path.Join(ctx.RootImportPath, ctx.VendorFolder, ps.Path)
		pkg, foundPkg = ctx.Package[localPath]
		foundPkg = foundPkg && pkg.Status.Presence != PresenceMissing
	}
	if !foundPkg {
		pkg, foundPkg = ctx.Package[ps.Path]
		foundPkg = foundPkg && pkg.Status.Presence != PresenceMissing
	}
	if !foundPkg {
		pkg, foundPkg = ctx.Package[ps.PathOrigin()]
		foundPkg = foundPkg && pkg.Status.Presence != PresenceMissing
	}
	if !foundPkg {
		pkg, err = ctx.addSingleImport(ctx.RootDir, ps.PathOrigin(), tree)
		if err != nil {
			return err
		}
		if pkg == nil {
			return nil
		}
		pkg.Origin = ps.PathOrigin()
		pkg.Path = ps.Path
	}

	pkg.HasOrigin = ps.HasOrigin
	if ps.HasOrigin {
		pkg.Origin = ps.Origin
	}

	// Do not support setting "tree" on Remove.
	if tree && mod != Remove {
		pkg.IncludeTree = true
	}

	// A restriction where packages cannot live inside a tree package.
	if mod != Remove {
		if pkg.IncludeTree {
			children := ctx.findPackageChild(pkg)
			if len(children) > 0 {
				return ErrTreeChildren{path: pkg.Path, children: children}
			}
		}
		treeParents := ctx.findPackageParentTree(pkg)
		if len(treeParents) > 0 {
			return ErrTreeParents{path: pkg.Path, parents: treeParents}
		}
	}

	// TODO (DT): figure out how to upgrade a non-tree package to a tree package with correct checks.
	localExists, err := hasGoFileInFolder(filepath.Join(ctx.RootDir, ctx.VendorFolder, pathos.SlashToFilepath(ps.Path)))
	if err != nil {
		return err
	}
	if mod == Add && localExists {
		return ErrPackageExists{path.Join(ctx.RootImportPath, ctx.VendorFolder, ps.Path)}
	}
	dprintf("stage 2: begin!\n")
	switch mod {
	case Add:
		return ctx.modifyAdd(pkg, ps.Uncommitted)
	case AddUpdate:
		return ctx.modifyAdd(pkg, ps.Uncommitted)
	case Update:
		return ctx.modifyAdd(pkg, ps.Uncommitted)
	case Remove:
		return ctx.modifyRemove(pkg)
	case Fetch:
		return ctx.modifyFetch(pkg, ps.Uncommitted, ps.HasVersion, ps.Version)
	default:
		panic("mod switch: case not handled")
	}
}

func (ctx *Context) getIgnoreFiles(src string) (ignoreFile, imports []string, err error) {
	srcDir, err := os.Open(src)
	if err != nil {
		return nil, nil, err
	}
	fl, err := srcDir.Readdir(-1)
	srcDir.Close()
	if err != nil {
		return nil, nil, err
	}
	importMap := make(map[string]struct{}, 12)
	imports = make([]string, 0, 12)
	for _, fi := range fl {
		if fi.IsDir() {
			continue
		}
		if fi.Name()[0] == '.' {
			continue
		}
		tags, fileImports, err := ctx.getFileTags(filepath.Join(src, fi.Name()), nil)
		if err != nil {
			return nil, nil, err
		}

		if tags.IgnoreItem(ctx.ignoreTag...) {
			ignoreFile = append(ignoreFile, fi.Name())
		} else {
			// Only add imports for non-ignored files.
			for _, imp := range fileImports {
				importMap[imp] = struct{}{}
			}
		}
	}
	for imp := range importMap {
		imports = append(imports, imp)
	}
	return ignoreFile, imports, nil
}

func (ctx *Context) modifyAdd(pkg *Package, uncommitted bool) error {
	var err error
	src := pkg.OriginDir
	dprintf("found import: %q\n", src)
	// If the canonical package is also the local package, then the package
	// isn't copied locally already and has already been checked for tags.
	// If it has been vendored the source still needs to be examined.
	// Examine here and add to the operations list.
	var ignoreFile []string
	if cpkg, found := ctx.Package[pkg.Path]; found {
		ignoreFile = cpkg.ignoreFile
	} else {
		var err error
		ignoreFile, _, err = ctx.getIgnoreFiles(src)
		if err != nil {
			return err
		}
	}
	dest := filepath.Join(ctx.RootDir, ctx.VendorFolder, pathos.SlashToFilepath(pkg.Path))
	// TODO: This might cause other issues or might be hiding the underlying issues. Examine in depth later.
	if pathos.FileStringEquals(src, dest) {
		return nil
	}
	dprintf("add op: %q\n", src)

	// Update vendor file with correct Local field.
	vp := ctx.VendorFilePackagePath(pkg.Path)
	if vp == nil {
		vp = &vendorfile.Package{
			Add:  true,
			Path: pkg.Path,
		}
		ctx.VendorFile.Package = append(ctx.VendorFile.Package, vp)
	}
	if pkg.IncludeTree {
		vp.Tree = pkg.IncludeTree
	}

	if pkg.HasOrigin {
		vp.Origin = pkg.Origin
	}
	if pkg.Path != pkg.Local && pkg.inVendor && vp.Add {
		vp.Origin = pkg.Local
	}

	// Find the VCS information.
	system, err := vcs.FindVcs(pkg.Gopath, src)
	if err != nil {
		return err
	}
	dirtyAndUncommitted := false
	if system != nil {
		if system.Dirty {
			if !uncommitted {
				return ErrDirtyPackage{pkg.Path}
			}
			dirtyAndUncommitted = true
			if len(vp.ChecksumSHA1) == 0 {
				vp.ChecksumSHA1 = "uncommitted/version="
			}
		} else {
			vp.Revision = system.Revision
			if system.RevisionTime != nil {
				vp.RevisionTime = system.RevisionTime.UTC().Format(time.RFC3339)
			}
		}
	}
	ctx.Operation = append(ctx.Operation, &Operation{
		Type:       OpCopy,
		Pkg:        pkg,
		Src:        src,
		Dest:       dest,
		IgnoreFile: ignoreFile,

		Uncommitted: dirtyAndUncommitted,
	})

	if !ctx.rewriteImports {
		return nil
	}

	mvSet := make(map[*Package]struct{}, 3)
	ctx.makeSet(pkg, mvSet)

	for r := range mvSet {
		to := path.Join(ctx.RootImportPath, ctx.VendorFolder, r.Path)
		dprintf("RULE: %s -> %s\n", r.Local, to)
		ctx.RewriteRule[r.Path] = to
		ctx.RewriteRule[r.Local] = to
	}

	return nil
}

func (ctx *Context) modifyRemove(pkg *Package) error {
	// Update vendor file with correct Local field.
	vp := ctx.VendorFilePackagePath(pkg.Path)
	if vp != nil {
		vp.Remove = true
	}
	if len(pkg.Dir) == 0 {
		return nil
	}
	// Protect non-project paths from being removed.
	if pathos.FileHasPrefix(pkg.Dir, ctx.RootDir) == false {
		return nil
	}
	if pkg.Status.Location == LocationLocal {
		return nil
	}
	ctx.Operation = append(ctx.Operation, &Operation{
		Type: OpRemove,
		Pkg:  pkg,
		Src:  pkg.Dir,
		Dest: "",
	})

	if !ctx.rewriteImports {
		return nil
	}

	mvSet := make(map[*Package]struct{}, 3)
	ctx.makeSet(pkg, mvSet)

	for r := range mvSet {
		dprintf("RULE: %s -> %s\n", r.Local, r.Path)
		ctx.RewriteRule[r.Local] = r.Path
	}

	return nil
}

// modify function to fetch given package.
func (ctx *Context) modifyFetch(pkg *Package, uncommitted, hasVersion bool, version string) error {
	vp := ctx.VendorFilePackagePath(pkg.Path)
	if vp == nil {
		vp = &vendorfile.Package{
			Add:  true,
			Path: pkg.Path,
		}
		ctx.VendorFile.Package = append(ctx.VendorFile.Package, vp)
	}
	if hasVersion {
		vp.Version = version
		pkg.Version = version
		pkg.HasVersion = true
	}
	if pkg.IncludeTree {
		vp.Tree = pkg.IncludeTree
	}
	pkg.Origin = strings.TrimPrefix(pkg.Origin, ctx.RootImportPath+"/"+ctx.VendorFolder+"/")
	vp.Origin = pkg.Origin
	origin := vp.Origin
	if len(vp.Origin) == 0 {
		origin = vp.Path
	}
	ps := &pkgspec.Pkg{
		Path:       pkg.Path,
		Origin:     origin,
		HasVersion: hasVersion,
		Version:    version,
	}
	dest := filepath.Join(ctx.RootDir, ctx.VendorFolder, pathos.SlashToFilepath(pkg.Path))
	ctx.Operation = append(ctx.Operation, &Operation{
		Type: OpFetch,
		Pkg:  pkg,
		Src:  ps.String(),
		Dest: dest,
	})
	return nil
}

// Check returns any conflicts when more than one package can be moved into
// the same path.
func (ctx *Context) Check() []*Conflict {
	// Find duplicate packages that have been marked for moving.
	findDups := make(map[string][]*Operation, 3) // map[canonical][]local
	for _, op := range ctx.Operation {
		if op.State != OpReady {
			continue
		}
		findDups[op.Pkg.Path] = append(findDups[op.Pkg.Path], op)
	}

	var ret []*Conflict
	for canonical, lop := range findDups {
		if len(lop) == 1 {
			continue
		}
		destDir := path.Join(ctx.RootImportPath, ctx.VendorFolder, canonical)
		ret = append(ret, &Conflict{
			Canonical: canonical,
			Local:     destDir,
			Operation: lop,
		})
	}
	return ret
}

// ResolveApply applies the conflict resolution selected. It chooses the
// Operation listed in the OpIndex field.
func (ctx *Context) ResloveApply(cc []*Conflict) {
	for _, c := range cc {
		if c.Resolved == false {
			continue
		}
		for i, op := range c.Operation {
			if op.State != OpReady {
				continue
			}
			if i == c.OpIndex {
				if vp := ctx.VendorFilePackagePath(c.Canonical); vp != nil {
					vp.Origin = c.Local
				}
				continue
			}
			op.State = OpIgnore
		}
	}
}

// ResolveAutoLongestPath finds the longest local path in each conflict
// and set it to be used.
func ResolveAutoLongestPath(cc []*Conflict) []*Conflict {
	for _, c := range cc {
		if c.Resolved {
			continue
		}
		longestLen := 0
		longestIndex := 0
		for i, op := range c.Operation {
			if op.State != OpReady {
				continue
			}

			if len(op.Pkg.Local) > longestLen {
				longestLen = len(op.Pkg.Local)
				longestIndex = i
			}
		}
		c.OpIndex = longestIndex
		c.Resolved = true
	}
	return cc
}

// ResolveAutoShortestPath finds the shortest local path in each conflict
// and set it to be used.
func ResolveAutoShortestPath(cc []*Conflict) []*Conflict {
	for _, c := range cc {
		if c.Resolved {
			continue
		}
		shortestLen := math.MaxInt32
		shortestIndex := 0
		for i, op := range c.Operation {
			if op.State != OpReady {
				continue
			}

			if len(op.Pkg.Local) < shortestLen {
				shortestLen = len(op.Pkg.Local)
				shortestIndex = i
			}
		}
		c.OpIndex = shortestIndex
		c.Resolved = true
	}
	return cc
}

// ResolveAutoVendorFileOrigin resolves conflicts based on the vendor file
// if possible.
func (ctx *Context) ResolveAutoVendorFileOrigin(cc []*Conflict) []*Conflict {
	for _, c := range cc {
		if c.Resolved {
			continue
		}
		vp := ctx.VendorFilePackagePath(c.Canonical)
		if vp == nil {
			continue
		}
		// If this was just added, we still can't rely on it.
		// We still need to ask user.
		if vp.Add {
			continue
		}
		lookFor := vp.Path
		if len(vp.Origin) != 0 {
			lookFor = vp.Origin
		}
		for i, op := range c.Operation {
			if op.State != OpReady {
				continue
			}

			if op.Pkg.Local == lookFor {
				c.OpIndex = i
				c.Resolved = true
				break
			}
		}
	}
	return cc
}

// Alter runs any requested package alterations.
func (ctx *Context) Alter() error {
	ctx.added = nil
	// Ensure there are no conflicts at this time.
	buf := &bytes.Buffer{}
	for _, conflict := range ctx.Check() {
		buf.WriteString(fmt.Sprintf("Different Canonical Packages for %s\n", conflict.Canonical))
		for _, op := range conflict.Operation {
			buf.WriteString(fmt.Sprintf("\t%s\n", op.Pkg.Local))
		}
	}
	if buf.Len() != 0 {
		return errors.New(buf.String())
	}

	var err error
	fetch, err := newFetcher(ctx)
	if err != nil {
		return err
	}
	for {
		var nextOps []*Operation
		for _, op := range ctx.Operation {
			if op.State != OpReady {
				continue
			}

			switch op.Type {
			case OpFetch:
				var ops []*Operation
				// Download packages, transform fetch op into a copy op.
				ops, err = fetch.op(op)
				if len(ops) > 0 {
					nextOps = append(nextOps, ops...)
				}
			}
			if err != nil {
				return errors.Wrapf(err, "Failed to fetch package %q", op.Pkg.Path)
			}
		}
		if len(nextOps) == 0 {
			break
		}
		ctx.Operation = append(ctx.Operation, nextOps...)
	}
	// Move and possibly rewrite packages.
	for _, op := range ctx.Operation {
		if op.State != OpReady {
			continue
		}
		pkg := op.Pkg

		if pathos.FileStringEquals(op.Dest, op.Src) {
			panic("For package " + pkg.Local + " attempt to copy to same location: " + op.Src)
		}
		dprintf("MV: %s (%q -> %q)\n", pkg.Local, op.Src, op.Dest)
		// Copy the package or remove.
		switch op.Type {
		default:
			panic("unknown operation type")
		case OpRemove:
			ctx.dirty = true
			err = RemovePackage(op.Src, filepath.Join(ctx.RootDir, ctx.VendorFolder), pkg.IncludeTree)
			op.State = OpDone
		case OpCopy:
			err = ctx.copyOperation(op, nil)
			if os.IsNotExist(errors.Cause(err)) {
				// Ignore packages that don't exist, like appengine.
				err = nil
			}
		}
		if err != nil {
			return errors.Wrapf(err, "Failed to %v package %q -> %q", op.Type, op.Src, op.Dest)
		}
	}
	if ctx.rewriteImports {
		return ctx.rewrite()
	}
	return nil
}

func (ctx *Context) copyOperation(op *Operation, beforeCopy func(deps []string) error) error {
	var err error
	pkg := op.Pkg
	ctx.dirty = true
	h := sha1.New()
	var checksum []byte

	root, _ := pathos.TrimCommonSuffix(op.Src, pkg.Path)

	err = ctx.CopyPackage(op.Dest, op.Src, root, pkg.Path, op.IgnoreFile, pkg.IncludeTree, h, beforeCopy)
	if err == nil && !op.Uncommitted {
		checksum = h.Sum(nil)
		vpkg := ctx.VendorFilePackagePath(pkg.Path)
		if vpkg != nil {
			vpkg.ChecksumSHA1 = base64.StdEncoding.EncodeToString(checksum)
		}
	}
	op.State = OpDone
	if err != nil {
		return errors.Wrapf(err, "copy failed. dest: %q, src: %q, pkgPath %q", op.Dest, op.Src, root)
	}
	return nil
}
