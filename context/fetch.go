// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kardianos/govendor/internal/pathos"
	"github.com/kardianos/govendor/pkgspec"
	gvvcs "github.com/kardianos/govendor/vcs"
	"github.com/kardianos/govendor/vendorfile"

	"golang.org/x/tools/go/vcs"
)

type fetcher struct {
	Ctx       *Context
	CacheRoot string
	HavePkg   map[string]bool
}

func newFetcher(ctx *Context) (*fetcher, error) {
	// GOPATH here includes the "src" dir, go up one level.
	cacheRoot := filepath.Join(ctx.RootGopath, "..", ".cache", "govendor")
	err := os.MkdirAll(cacheRoot, 0700)
	if err != nil {
		return nil, err
	}
	return &fetcher{
		Ctx:       ctx,
		CacheRoot: cacheRoot,
		HavePkg:   make(map[string]bool, 30),
	}, nil
}

// op fetches the repo locally if not already present.
// Transform the fetch op into a copy op.
func (f *fetcher) op(op *Operation) ([]*Operation, error) {
	// vcs.ShowCmd = true
	var nextOps []*Operation
	vpkg := f.Ctx.VendorFilePackagePath(op.Pkg.Canonical)
	if vpkg == nil {
		return nextOps, fmt.Errorf("Could not find vendor file package for %q. Internal error.", op.Pkg.Canonical)
	}

	op.Type = OpCopy
	ps, err := pkgspec.Parse("", op.Src)
	if err != nil {
		return nextOps, err
	}
	if len(ps.Origin) == 0 {
		ps.Origin = ps.Path
	}

	f.HavePkg[ps.Path] = true

	// Don't check for bundle, rather check physical directory.
	// If no repo in dir, clone.
	// If there is a repo in dir, update to latest.
	// Get any tags.
	// If we have a specific revision, update to that revision.

	pkgDir := filepath.Join(f.CacheRoot, pathos.SlashToFilepath(ps.Path))
	sysVcsCmd, repoRoot, err := vcs.FromDir(pkgDir, f.CacheRoot)
	var vcsCmd *VCSCmd
	repoRootDir := filepath.Join(f.CacheRoot, repoRoot)
	if err != nil {
		rr, err := vcs.RepoRootForImportPath(ps.Origin, false)
		if err != nil {
			return nextOps, err
		}

		vcsCmd = updateVcsCmd(rr.VCS)
		repoRoot = rr.Root
		repoRootDir = filepath.Join(f.CacheRoot, repoRoot)

		err = vcsCmd.Create(repoRootDir, rr.Repo)
		if err != nil {
			return nextOps, err
		}

	} else {
		vcsCmd = updateVcsCmd(sysVcsCmd)
		err = vcsCmd.Download(repoRootDir)
		if err != nil {
			return nextOps, err
		}
	}

	revision := ""
	if ps.HasVersion {
		switch {
		case len(ps.Version) == 0:
			vpkg.Version = ""
		case isVersion(ps.Version):
			vpkg.Version = ps.Version
		default:
			revision = ps.Version
		}
	}

	switch {
	case len(revision) == 0 && len(vpkg.Version) > 0:
		// fmt.Printf("Get version %q@%s\n", vpkg.Path, vpkg.Revision)
		// Get a list of tags, match to version if possible.
		var tagNames []string
		tagNames, err = vcsCmd.Tags(repoRootDir)
		if err != nil {
			return nextOps, err
		}
		labels := make([]Label, len(tagNames))
		for i, tag := range tagNames {
			labels[i].Source = LabelTag
			labels[i].Text = tag
		}
		result := FindLabel(vpkg.Version, labels)
		if result.Source == LabelNone {
			return nextOps, fmt.Errorf("No label found for specified version %q from %s", vpkg.Version, ps.String())
		}
		err = vcsCmd.TagSync(repoRootDir, result.Text)
		if err != nil {
			return nextOps, err
		}
	case len(revision) > 0:
		// fmt.Printf("Get specific revision %q@%s\n", vpkg.Path, revision)
		// Get specific version.
		err = vcsCmd.RevisionSync(repoRootDir, revision)
		if err != nil {
			return nextOps, err
		}
	default:
		// fmt.Printf("Get latest revision %q\n", vpkg.Path)
		// Get latest version.
		err = vcsCmd.TagSync(repoRootDir, "")
		if err != nil {
			return nextOps, err
		}
	}

	// set op.Src to download dir.
	// /tmp/cache/1/[[github.com/kardianos/govendor]]context
	op.Src = pkgDir
	var deps []string
	op.IgnoreFile, deps, err = f.Ctx.getIngoreFiles(op.Src)
	if err != nil {
		return nextOps, err
	}

	// Queue up any missing package deps.
	for _, dep := range deps {
		dep = strings.TrimSpace(dep)
		if len(dep) == 0 {
			continue
		}
		if f.HavePkg[dep] {
			continue
		}
		if pkg := f.Ctx.Package[dep]; pkg != nil {
			continue
		}
		var yes bool
		yes, err = f.Ctx.isStdLib(dep)
		if err != nil {
			return nextOps, fmt.Errorf("Failed to check if in stdlib: %v", err)
		}
		if yes {
			continue
		}
		f.HavePkg[dep] = true
		dest := filepath.Join(f.Ctx.RootDir, f.Ctx.VendorFolder, dep)

		// Update vendor file with correct Local field.
		vp := f.Ctx.VendorFilePackagePath(dep)
		if vp == nil {
			vp = &vendorfile.Package{
				Add:  true,
				Path: dep,
			}
			f.Ctx.VendorFile.Package = append(f.Ctx.VendorFile.Package, vp)
		}

		nextOps = append(nextOps, &Operation{
			Type: OpFetch,
			Pkg:  &Package{Canonical: dep},
			Src:  dep,
			Dest: dest,
		})
	}

	// Once downloaded, be sure to set the revision and revisionTime
	// in the vendor file package.
	// Find the VCS information.
	system, err := gvvcs.FindVcs(f.CacheRoot, op.Src)
	if err != nil {
		return nextOps, err
	}
	if system != nil {
		if system.Dirty {
			return nextOps, ErrDirtyPackage{ps.Origin}
		}
		vpkg.Revision = system.Revision
		if system.RevisionTime != nil {
			vpkg.RevisionTime = system.RevisionTime.Format(time.RFC3339)
		}
	}

	return nextOps, f.Ctx.copyOperation(op)
}
