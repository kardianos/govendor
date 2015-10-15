// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package context gathers the status of packages and stores it in Context.
// A new Context needs to be pointed to the root of the project and any
// project owned vendor file.
package context

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kardianos/govendor/internal/pathos"
	"github.com/kardianos/govendor/vcs"
	"github.com/kardianos/govendor/vendorfile"
)

const (
	debug     = false
	looplimit = 10000

	vendorFilename = "vendor.json"
)

func dprintf(f string, v ...interface{}) {
	if debug {
		fmt.Printf(f, v...)
	}
}

// OperationState is the state of the given package move operation.
type OperationState byte

const (
	OpReady  OperationState = iota // Operation is ready to go.
	OpIgnore                       // Operation should be ignored.
	OpDone                         // Operation has been completed.
)

// Operation defines how packages should be moved.
type Operation struct {
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
}

// Conflict reports packages that are scheduled to conflict.
type Conflict struct {
	Canonical string
	Local     string
	Operation []*Operation
	OpIndex   int
	Resolved  bool
}

// Context represents the current project context.
type Context struct {
	GopathList []string // List of GOPATHs in environment.
	Goroot     string   // The path to the standard library.

	RootDir        string // Full path to the project root.
	RootGopath     string // The GOPATH the project is in.
	RootImportPath string // The import path to the project.

	VendorFile         *vendorfile.File
	VendorFilePath     string // File path to vendor file.
	VendorFolder       string // Store vendor packages in this folder.
	VendorFileToFolder string // The relative path from the vendor file to the vendor folder.
	RootToVendorFile   string // The relative path from the project root to the vendor file directory.

	VendorDiscoverFolder string // Normally auto-set to "vendor"

	// Package is a map where the import path is the key.
	// Populated with LoadPackage.
	Package map[string]*Package
	// Change to unkown structure (rename). Maybe...

	// MoveRule provides the translation from origional import path to new import path.
	RewriteRule map[string]string // map[from]to

	Operation []*Operation

	loaded, dirty  bool
	rewriteImports bool

	ignoreTag []string // list of tags to ignore
}

// Package maintains information pertaining to a package.
type Package struct {
	Dir        string
	Canonical  string
	Local      string
	SourcePath string
	Gopath     string
	Files      []*File
	Status     Status

	ignoreFile []string

	// used in resolveUnknown function. Not persisted.
	referenced map[string]*Package
}

// File holds a reference to the imports in a file and the file locaiton.
type File struct {
	Package *Package
	Path    string
	Imports []string

	ImportComment string
}

// NewContextWD creates a new context. It looks for a root folder by finding
// a vendor file.
func NewContextWD(wdIsRoot bool) (*Context, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	pathToVendorFile := filepath.Join("vendor", vendorFilename)
	rootIndicator := "vendor"
	vendorFolder := "vendor"
	go15VendorExperiment := os.Getenv("GO15VENDOREXPERIMENT") == "1"
	if !go15VendorExperiment {
		vendorFolder = "internal"
		override := os.Getenv("GOVENDORFOLDER")
		if len(override) != 0 {
			vendorFolder = override
		}
		pathToVendorFile = filepath.Join(vendorFolder, vendorFilename)
		rootIndicator = pathToVendorFile
	}
	root := wd
	if !wdIsRoot {
		root, err = findRoot(wd, rootIndicator)
		if err != nil {
			return nil, err
		}
	}

	// Check for old vendor file location.
	oldLocation := filepath.Join(root, vendorFilename)
	if _, err := os.Stat(oldLocation); err == nil {
		return nil, ErrOldVersion{`Use the "migrate" command to update.`}
	}

	return NewContext(root, pathToVendorFile, vendorFolder, !go15VendorExperiment)
}

// NewContext creates new context from a given root folder and vendor file path.
// The vendorFolder is where vendor packages should be placed.
func NewContext(root, vendorFilePathRel, vendorFolder string, rewriteImports bool) (*Context, error) {
	dprintf("CTX: %s\n", root)
	vendorFilePath := filepath.Join(root, vendorFilePathRel)
	vf, err := readVendorFile(vendorFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		vf = &vendorfile.File{}
	}

	// Get GOROOT. First check ENV, then run "go env" and find the GOROOT line.
	goroot := os.Getenv("GOROOT")
	if len(goroot) == 0 {
		// If GOROOT is not set, get from go cmd.
		cmd := exec.Command("go", "env")
		var goEnv []byte
		goEnv, err = cmd.CombinedOutput()
		if err != nil {
			return nil, err
		}
		const gorootLookFor = `GOROOT=`
		for _, line := range strings.Split(string(goEnv), "\n") {
			if strings.HasPrefix(line, gorootLookFor) == false {
				continue
			}
			goroot = strings.TrimPrefix(line, gorootLookFor)
			goroot, err = strconv.Unquote(goroot)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if goroot == "" {
		return nil, ErrMissingGOROOT
	}
	goroot = filepath.Join(goroot, "src")

	// Get the GOPATHs. Prepend the GOROOT to the list.
	all := os.Getenv("GOPATH")
	if len(all) == 0 {
		return nil, ErrMissingGOPATH
	}
	gopathList := filepath.SplitList(all)
	gopathGoroot := make([]string, 0, len(gopathList)+1)
	gopathGoroot = append(gopathGoroot, goroot)
	for _, gopath := range gopathList {
		gopathGoroot = append(gopathGoroot, filepath.Join(gopath, "src")+string(filepath.Separator))
	}

	rootToVendorFile, _ := filepath.Split(vendorFilePathRel)

	vendorFileDir, _ := filepath.Split(vendorFilePath)
	vendorFolderRel, err := filepath.Rel(vendorFileDir, filepath.Join(root, vendorFolder))
	if err != nil {
		return nil, err
	}
	vendorFileToFolder := pathos.SlashToImportPath(vendorFolderRel)

	ctx := &Context{
		RootDir:    root,
		GopathList: gopathGoroot,
		Goroot:     goroot,

		VendorFile:         vf,
		VendorFilePath:     vendorFilePath,
		VendorFolder:       vendorFolder,
		VendorFileToFolder: vendorFileToFolder,
		RootToVendorFile:   pathos.SlashToImportPath(rootToVendorFile),

		VendorDiscoverFolder: "vendor",

		Package: make(map[string]*Package),

		RewriteRule: make(map[string]string, 3),

		rewriteImports: rewriteImports,
	}

	ctx.RootImportPath, ctx.RootGopath, err = ctx.findImportPath(root)
	if err != nil {
		return nil, err
	}

	ctx.IgnoreBuild(vf.Ignore)

	return ctx, nil
}

// IgnoreBuild takes a space separated list of tags to ignore.
// "a b c" will ignore "a" OR "b" OR "c".
func (ctx *Context) IgnoreBuild(ignore string) {
	ors := strings.Fields(ignore)
	ctx.ignoreTag = make([]string, 0, len(ors))
	for _, or := range ors {
		if len(or) == 0 {
			continue
		}
		ctx.ignoreTag = append(ctx.ignoreTag, or)
	}
}

// VendorFilePackageLocal finds a given vendor file package give the local import path.
func (ctx *Context) VendorFilePackageLocal(local string) *vendorfile.Package {
	root, _ := filepath.Split(ctx.VendorFilePath)
	return vendorFileFindLocal(ctx.VendorFile, root, ctx.RootGopath, local)
}

// VendorFilePackageCanonical finds a given vendor file package give the canonical import path.
func (ctx *Context) VendorFilePackagePath(canonical string) *vendorfile.Package {
	for _, pkg := range ctx.VendorFile.Package {
		if pkg.Remove {
			continue
		}
		if pkg.Path == canonical {
			return pkg
		}
	}
	return nil
}

// updatePackageReferences populates the referenced field in each Package.
func (ctx *Context) updatePackageReferences() {
	findCanonicalUnderDir := func(dir, canonical string) *Package {
		for _, pkg := range ctx.Package {
			if pkg.Status != StatusVendor {
				continue
			}

			removeFromEnd := len(pkg.Canonical) + len(ctx.VendorDiscoverFolder) + 2
			nextLen := len(pkg.Dir) - removeFromEnd
			if nextLen < 0 {
				continue
			}
			checkDir := pkg.Dir[:nextLen]
			if !pathos.FileHasPrefix(dir, checkDir) {
				continue
			}
			if pkg.Canonical != canonical {
				continue
			}
			return pkg
		}
		return nil
	}
	for _, pkg := range ctx.Package {
		pkg.referenced = make(map[string]*Package, len(pkg.referenced))
	}
	for _, pkg := range ctx.Package {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				if vpkg := findCanonicalUnderDir(pkg.Dir, imp); vpkg != nil {
					vpkg.referenced[pkg.Local] = pkg
					continue
				}
				if other, found := ctx.Package[imp]; found {
					other.referenced[pkg.Local] = pkg
					continue
				}
			}
		}
	}
}

// Modify is the type of modifcation to do.
type Modify byte

const (
	AddUpdate Modify = iota // Add or update the import.
	Add                     // Only add, error if it already exists.
	Update                  // Only update, error if it doesn't currently exist.
	Remove                  // Remove from vendor path.
)

// AddImport adds the package to the context. The vendorFolder is where the
// package should be added to relative to the project root.
func (ctx *Context) ModifyImport(sourcePath string, mod Modify) error {
	var err error
	if !ctx.loaded || ctx.dirty {
		err = ctx.loadPackage()
		if err != nil {
			return err
		}
	}
	// Determine canonical and local import paths.
	sourcePath = pathos.SlashToImportPath(sourcePath)
	canonicalImportPath, err := ctx.findCanonicalPath(sourcePath)
	if err != nil {
		if mod != Remove {
			return err
		}
		if _, is := err.(ErrNotInGOPATH); !is {
			return err
		}
	}
	// If the import is already vendored, ensure we have the local path and not
	// the canonical path.
	localImportPath := sourcePath
	if vendPkg := ctx.VendorFilePackagePath(localImportPath); vendPkg != nil {
		localImportPath = path.Join(ctx.RootImportPath, ctx.RootToVendorFile, vendPkg.Path)
	}

	dprintf("AI: %s, L: %s, C: %s\n", sourcePath, localImportPath, canonicalImportPath)

	// Does the local import exist?
	//   If so either update or just return.
	//   If not find the disk path from the canonical path, copy locally and rewrite (if needed).
	pkg, foundPkg := ctx.Package[localImportPath]
	if !foundPkg {
		err = ctx.addSingleImport("", canonicalImportPath)
		if err != nil {
			return err
		}
		pkg, foundPkg = ctx.Package[canonicalImportPath]
		// Find by canonical path if stored by different local path.
		if !foundPkg {
			for _, p := range ctx.Package {
				if canonicalImportPath == p.Canonical {
					foundPkg = true
					pkg = p
					break
				}
			}
		}
		if !foundPkg {
			panic(fmt.Sprintf("Package %q should be listed internally but is not.", canonicalImportPath))
		}
	}

	localExists, err := hasGoFileInFolder(filepath.Join(ctx.RootDir, ctx.VendorFolder, pathos.SlashToFilepath(canonicalImportPath)))
	if err != nil {
		return err
	}
	if mod == Add && localExists {
		return ErrPackageExists{path.Join(ctx.RootImportPath, ctx.VendorFolder, canonicalImportPath)}
	}
	switch mod {
	case Add:
		return ctx.modifyAdd(pkg)
	case AddUpdate:
		return ctx.modifyAdd(pkg)
	case Update:
		return ctx.modifyAdd(pkg)
	case Remove:
		return ctx.modifyRemove(pkg)
	default:
		panic("mod switch: case not handled")
	}
}

func (ctx *Context) modifyAdd(pkg *Package) error {
	var err error
	src := pkg.Dir
	if pkg.Status == StatusVendor {
		src, _, err = ctx.findImportDir("", pkg.Canonical)
		if err != nil {
			return err
		}
	}
	// If the canonical package is also the local package, then the package
	// isn't copied locally already and has already been checked for tags.
	// If it has been vendored the source still needs to be examined.
	// Examine here and add to the operations list.
	var ignoreFile []string
	if cpkg, found := ctx.Package[pkg.Canonical]; found {
		ignoreFile = cpkg.ignoreFile
	} else {
		srcDir, err := os.Open(src)
		if err != nil {
			return err
		}
		fl, err := srcDir.Readdir(-1)
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
			tags, err := ctx.getFileTags(filepath.Join(src, fi.Name()), nil)
			if err != nil {
				return err
			}

			for _, tag := range tags {
				for _, ignore := range ctx.ignoreTag {
					if tag == ignore {
						ignoreFile = append(ignoreFile, fi.Name())
					}
				}
			}
		}
	}
	dest := filepath.Join(ctx.RootDir, ctx.VendorFolder, pathos.SlashToFilepath(pkg.Canonical))
	// TODO: This might cause other issues or might be hiding the underlying issues. Examine in depth later.
	if pathos.FileStringEquals(src, dest) {
		return nil
	}
	ctx.Operation = append(ctx.Operation, &Operation{
		Pkg:        pkg,
		Src:        src,
		Dest:       dest,
		IgnoreFile: ignoreFile,
	})

	// Update vendor file with correct Local field.
	vp := ctx.VendorFilePackagePath(pkg.Canonical)
	if vp == nil {
		vp = &vendorfile.Package{
			Add:  true,
			Path: pkg.Canonical,
		}
		ctx.VendorFile.Package = append(ctx.VendorFile.Package, vp)
	}

	// Find the VCS information.
	system, err := vcs.FindVcs(pkg.Gopath, src)
	if err != nil {
		return err
	}
	if system != nil {
		if system.Dirty {
			return ErrDirtyPackage{pkg.Canonical}
		}
		vp.Revision = system.Revision
		if system.RevisionTime != nil {
			vp.RevisionTime = system.RevisionTime.Format(time.RFC3339)
		}
	}

	mvSet := make(map[*Package]struct{}, 3)
	ctx.makeSet(pkg, mvSet)

	for r := range mvSet {
		to := path.Join(ctx.RootImportPath, ctx.VendorFolder, r.Canonical)
		dprintf("RULE: %s -> %s\n", r.Local, to)
		ctx.RewriteRule[r.Canonical] = to
		ctx.RewriteRule[r.Local] = to
	}

	return nil
}

func (ctx *Context) modifyRemove(pkg *Package) error {
	if len(pkg.Dir) == 0 {
		return nil
	}
	ctx.Operation = append(ctx.Operation, &Operation{
		Pkg:  pkg,
		Src:  pkg.Dir,
		Dest: "",
	})

	// Update vendor file with correct Local field.
	vp := ctx.VendorFilePackagePath(pkg.Canonical)
	if vp != nil {
		vp.Remove = true
	}
	mvSet := make(map[*Package]struct{}, 3)
	ctx.makeSet(pkg, mvSet)

	for r := range mvSet {
		dprintf("RULE: %s -> %s\n", r.Local, r.Canonical)
		ctx.RewriteRule[r.Local] = r.Canonical
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
			case next.Canonical == next.Local:
			case next.Status != StatusExternal:
			}
		}
	}
}

// Check returns any conflicts when more then one package can be moved into
// the same path.
func (ctx *Context) Check() []*Conflict {
	// Find duplicate packages that have been marked for moving.
	findDups := make(map[string][]*Operation, 3) // map[canonical][]local
	for _, op := range ctx.Operation {
		if op.State != OpReady {
			continue
		}
		findDups[op.Pkg.Canonical] = append(findDups[op.Pkg.Canonical], op)
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

func (ctx *Context) copy() error {
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

	// Move and possibly rewrite packages.
	var err error
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
		if len(op.Dest) == 0 {
			err = RemovePackage(op.Src, filepath.Join(ctx.RootDir, ctx.VendorFolder))
		} else {
			err = CopyPackage(op.Dest, op.Src, op.IgnoreFile)
		}
		if err != nil {
			return fmt.Errorf("Failed to copy package %q -> %q: %v", op.Src, op.Dest, err)
		}
		op.State = OpDone
		ctx.dirty = true
	}
	return nil
}

// Alter runs any requested package alterations.
func (ctx *Context) Alter() error {
	err := ctx.copy()
	if err != nil {
		return err
	}
	return ctx.rewrite()
}
