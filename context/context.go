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
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kardianos/vendor/internal/pathos"
	"github.com/kardianos/vendor/vcs"
	"github.com/kardianos/vendor/vendorfile"
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

	// Package is a map where the import path is the key.
	// Populated with LoadPackage.
	Package map[string]*Package
	// Change to unkown structure (rename). Maybe...

	// MoveRule provides the translation from origional import path to new import path.
	MoveRule map[string]string // map[from]to

	loaded, dirty  bool
	rewriteImports bool
}

// Package maintains information pertaining to a package.
type Package struct {
	Dir         string
	Canonical   string
	Local       string
	SourcePath  string
	Gopath      string
	Files       []*File
	Status      ListStatus
	MovePending bool

	// used in resolveUnknown function. Not persisted.
	referenced map[string]*Package
}

// File holds a reference to the imports in a file and the file locaiton.
type File struct {
	Package *Package
	Path    string
	Imports []string
}

// NewContextWD creates a new context. It looks for a root folder by finding
// a vendor file.
func NewContextWD() (*Context, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	pathToVendorFile := vendorFilename
	rootIndicator := "vendor"
	vendorFolder := "vendor"
	go15VendorExperiment := os.Getenv("GO15VENDOREXPERIMENT") == "1"
	if !go15VendorExperiment {
		pathToVendorFile = filepath.Join("internal", vendorFilename)
		rootIndicator = pathToVendorFile
		vendorFolder = "internal"
	}
	root, err := findRoot(wd, rootIndicator)
	if err != nil {
		return nil, err
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

		Package: make(map[string]*Package),

		MoveRule: make(map[string]string, 3),

		rewriteImports: rewriteImports,
	}

	ctx.RootImportPath, ctx.RootGopath, err = ctx.findImportPath(root)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}

func (ctx *Context) vendorFilePackageLocal(local string) *vendorfile.Package {
	// // The vendor local is relative to the vendor file.
	// rel := path.Join(ctx.RootImportPath, ctx.RootToVendorFile, ctx.VendorFileToFolder)
	// local = strings.TrimPrefix(strings.TrimPrefix(local, rel), "/")
	for _, pkg := range ctx.VendorFile.Package {
		if pkg.Remove {
			continue
		}
		if pkg.Local == local {
			return pkg
		}
	}
	return nil
}

func (ctx *Context) vendorFilePackageCanonical(canonical string) *vendorfile.Package {
	for _, pkg := range ctx.VendorFile.Package {
		if pkg.Remove {
			continue
		}
		if pkg.Canonical == canonical {
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

			removeFromEnd := len(pkg.Canonical) + len("vendor/") + 1
			checkDir := pkg.Dir[:len(pkg.Dir)-removeFromEnd]
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

// AddImport adds the package to the context. The vendorFolder is where the
// package should be added to relative to the project root.
// TODO: Add parameter that specifies (Add, Update, AddUpdate, Remove, Restore).
func (ctx *Context) ModifyImport(sourcePath string) error {
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
		return err
	}
	// If the import is already vendored, ensure we have the local path and not
	// the canonical path.
	localImportPath := sourcePath
	if vendPkg := ctx.vendorFilePackageCanonical(localImportPath); vendPkg != nil {
		localImportPath = vendPkg.Local
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
		if !foundPkg {
			panic(fmt.Sprintf("Package %q should be listed internally but is not.", canonicalImportPath))
		}
	}
	pkg.MovePending = true

	// Update vendor file with correct Local field.
	vp := ctx.vendorFilePackageCanonical(pkg.Canonical)
	if vp == nil {
		vp = &vendorfile.Package{
			Add:       true,
			Canonical: pkg.Canonical,
			Local:     path.Join(ctx.RootImportPath, ctx.VendorFolder, pkg.Canonical),
		}
		ctx.VendorFile.Package = append(ctx.VendorFile.Package, vp)
	}

	// Find the VCS information.
	system, err := vcs.FindVcs(pkg.Gopath, pkg.Dir)
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

	// Add files to import map.
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

	mvSet := make(map[*Package]struct{}, 3)
	var makeSet func(pkg *Package)
	makeSet = func(pkg *Package) {
		mvSet[pkg] = struct{}{}
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				next := ctx.Package[imp]
				switch {
				default:
					if _, has := mvSet[next]; !has {
						makeSet(next)
					}
				case next == nil:
				case next.Canonical == next.Local:
				case next.Status != StatusExternal:
					continue
				}
			}
		}
	}
	makeSet(pkg)

	for r := range mvSet {
		to := path.Join(ctx.RootImportPath, ctx.VendorFolder, r.Canonical)
		dprintf("RULE: %s -> %s\n", r.Local, to)
		ctx.MoveRule[r.Local] = to
	}

	return nil
}

func (ctx *Context) copy() error {
	// Find duplicate packages that have been marked for moving.
	findDups := make(map[string][]string, 3) // map[canonical][]local
	for _, pkg := range ctx.Package {
		if !pkg.MovePending {
			continue
		}
		ll := findDups[pkg.Canonical]
		findDups[pkg.Canonical] = append(ll, pkg.Local)
	}
	// TODO: Add a new Check function that returns duplicates, possibly a Resolve function as well.
	// Make the option to "auto" resolve it explicit.
	// TODO: auto-resolve based on VCS time.
	if false {
		buf := &bytes.Buffer{}
		for canonical, ll := range findDups {
			if len(ll) == 1 {
				continue
			}
			buf.WriteString(fmt.Sprintf("Different Canonical Packages for %s\n", canonical))
			for _, l := range ll {
				buf.WriteString(fmt.Sprintf("\t%s\n", l))
			}
		}
		if buf.Len() != 0 {
			return errors.New(buf.String())
		}
	}
	for _, ll := range findDups {
		if len(ll) == 1 {
			continue
		}
		for i, l := range ll {
			if i == 0 {
				continue
			}
			ctx.Package[l].MovePending = false
		}
	}

	// Move and possibly rewrite packages.
	for _, pkg := range ctx.Package {
		if !pkg.MovePending {
			continue
		}

		// Copy the package locally.
		destDir := filepath.Join(ctx.RootDir, ctx.VendorFolder, pathos.SlashToFilepath(pkg.Canonical))
		srcDir := pkg.Dir
		if destDir == srcDir {
			panic("For package " + pkg.Local + " attempt to copy to same location")
		}
		dprintf("MV: %s (%q -> %q)\n", pkg.Local, srcDir, destDir)
		err := CopyPackage(destDir, srcDir)
		if err != nil {
			return err
		}
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
