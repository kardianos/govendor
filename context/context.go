// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package context gathers the status of packages and stores it in Context.
// A new Context needs to be pointed to the root of the project and any
// project owned vendor file.
package context

import (
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
	vendorFilename = "vendor.json"

	looplimit = 10000
)

// Context represents the current project context.
type Context struct {
	GopathList []string
	Goroot     string

	RootDir        string
	RootGopath     string
	RootImportPath string

	VendorFile     *vendorfile.File
	VendorFilePath string
	VendorFolder   string // Store vendor packages in this folder.

	// Package is a map where the import path is the key.
	// Populated with LoadPackage.
	Package map[string]*Package

	loaded               bool
	go15VendorExperiment bool
}

// Package maintains information pertaining to a package.
type Package struct {
	Dir           string
	CanonicalPath string
	LocalPath     string
	SourcePath    string
	Gopath        string
	Files         []*File
	Status        ListStatus

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

	return NewContext(root, pathToVendorFile, vendorFolder, go15VendorExperiment)
}

// NewContext creates new context from a given root folder and vendor file path.
// The vendorFolder is where vendor packages should be placed.
func NewContext(root, vendorFilePathRel, vendorFolder string, go15VendorExperiment ...bool) (*Context, error) {
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

	ctx := &Context{
		RootDir:    root,
		GopathList: gopathGoroot,
		Goroot:     goroot,

		VendorFile:     vf,
		VendorFilePath: vendorFilePath,
		VendorFolder:   vendorFolder,

		Package: make(map[string]*Package),

		go15VendorExperiment: len(go15VendorExperiment) != 0 && go15VendorExperiment[0] == true,
	}

	ctx.RootImportPath, ctx.RootGopath, err = ctx.findImportPath(root)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}

func (ctx *Context) vendorFilePackageLocal(local string) *vendorfile.Package {
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

// LoadPackage sets up the context with package information.
func (ctx *Context) loadPackage(alsoImportPath ...string) error {
	ctx.loaded = true
	packageUnknown := newUnknownSet()
	err := filepath.Walk(ctx.RootDir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return err
		}
		name := info.Name()
		if info.IsDir() && (name[0] == '.' || name[0] == '_' || name == "testdata") {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		return ctx.addFileImports(path, ctx.RootGopath, packageUnknown)
	})
	if err != nil {
		return err
	}
	for _, pkg := range ctx.Package {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				if _, found := ctx.Package[imp]; !found {
					packageUnknown.add("", imp)
				}
			}
		}
	}
	for _, path := range alsoImportPath {
		if _, found := ctx.Package[path]; !found {
			packageUnknown.add("", path)
		}
	}
	return ctx.resolveUnknown(packageUnknown)
}

// updatePackageReferences populates the referenced field in each Package.
func (ctx *Context) updatePackageReferences() {
	for _, pkg := range ctx.Package {
		pkg.referenced = make(map[string]*Package, len(pkg.referenced))
	}
	for _, pkg := range ctx.Package {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				if other, found := ctx.Package[imp]; found {
					other.referenced[pkg.CanonicalPath] = pkg
				}
			}
		}
	}
}

// AddImport adds the package to the context. The vendorFolder is where the
// package should be added to relative to the project root.
func (ctx *Context) AddImport(sourcePath string) error {
	var err error
	if !ctx.loaded {
		err = ctx.loadPackage()
		if err != nil {
			return err
		}
	}
	sourcePath = pathos.SlashToImportPath(sourcePath)

	canonicalImportPath, err := ctx.findLocalImportPath(sourcePath)
	if err != nil {
		return err
	}

	err = ctx.resolveUnknownList("", sourcePath, canonicalImportPath)
	if err != nil {
		return err
	}

	// Adjust relative local path to GOPATH import path.
	localImportPath := path.Join(ctx.RootImportPath, ctx.VendorFolder, canonicalImportPath)

	localCopyExists := false
	// TODO: set localCopyExists flag
	// TODO: ensure the import path is the Canonical path.

	/*importPath, err = verify(ctx, importPath, localImportPath)
	if err != nil {
		if err == ErrFilesExists {
			localCopyExists = true
		} else {
			return err
		}
	}*/

	pkg, foundPkg := ctx.Package[sourcePath]
	if !foundPkg {
		return ErrNotInGOPATH{sourcePath}
	}
	if pkg.Status != StatusExternal {
		if pkg.Status == StatusInternal {
			return ErrVendorExists
		}
		if pkg.Status == StatusLocal {
			return ErrLocalPackage
		}
		return ErrNotInGOPATH{sourcePath}
	}

	// Update vendor file with correct Local field.
	vp := ctx.vendorFilePackageCanonical(canonicalImportPath)
	if vp == nil {
		vp = &vendorfile.Package{
			Add:       true,
			Canonical: canonicalImportPath,
			Local:     localImportPath,
		}
		ctx.VendorFile.Package = append(ctx.VendorFile.Package, vp)
	}
	if !localCopyExists {
		// Find the VCS information.
		system, err := vcs.FindVcs(pkg.Gopath, pkg.Dir)
		if err != nil {
			return err
		}
		if system != nil {
			if system.Dirty {
				return ErrDirtyPackage{pkg.CanonicalPath}
			}
			vp.Revision = system.Revision
			if system.RevisionTime != nil {
				vp.RevisionTime = system.RevisionTime.Format(time.RFC3339)
			}
		}

		// Copy the package locally.
		err = CopyPackage(filepath.Join(ctx.RootGopath, pathos.SlashToFilepath(localImportPath)), pkg.Dir)
		if err != nil {
			return err
		}
	}

	if !ctx.go15VendorExperiment {
		err = ctx.RewriteFiles(localImportPath)
		if err != nil {
			return err
		}
	}

	// Remove unused external packages from listing.
	ctx.updatePackageReferences()
top:
	for i := 0; i <= looplimit; i++ {
		altered := false
		for path, pkg := range ctx.Package {
			if len(pkg.referenced) == 0 && pkg.Status == StatusExternal {
				altered = true
				delete(ctx.Package, path)
				for _, other := range ctx.Package {
					delete(other.referenced, path)
				}
				continue top
			}
		}
		if !altered {
			break
		}
		if i == looplimit {
			return errLoopLimit{"resolveUnknown() Mark Unused"}
		}
	}
	return nil
}
