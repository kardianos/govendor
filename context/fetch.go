// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"io/ioutil"
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
	Ctx      *Context
	Bundles  []*syncBundle
	TempRoot string

	HavePkg map[string]bool
}

func newFetcher(ctx *Context) (*fetcher, error) {
	tempRoot, err := ioutil.TempDir(os.TempDir(), "govendor-cache")
	if err != nil {
		return nil, err
	}
	return &fetcher{
		Ctx:      ctx,
		Bundles:  make([]*syncBundle, 0, 9),
		TempRoot: tempRoot,
		HavePkg:  make(map[string]bool, 30),
	}, nil
}

func (f *fetcher) cleanUp() error {
	return os.RemoveAll(f.TempRoot)
}

// op fetches the repo locally if not already present.
// Transform the fetch op into a copy op.
func (f *fetcher) op(op *Operation) ([]*Operation, error) {
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

	var b *syncBundle
	for _, item := range f.Bundles {
		// Must be from the same repo (space).
		if item.RepoRoot != ps.Origin && !strings.HasPrefix(ps.Origin, item.RepoRoot+"/") {
			continue
		}
		// Must be from the same revision/version (time).
		if ps.HasVersion && ps.Version != item.Revision {
			continue
		}
		b = item
	}
	if b == nil {
		b = &syncBundle{
			Packages: []*vendorfile.Package{vpkg},
			Root:     filepath.Join(f.TempRoot, fmt.Sprintf("%d", len(f.Bundles))),
		}
		f.Bundles = append(f.Bundles, b)

		err = os.MkdirAll(b.Root, 0700)
		if err != nil {
			return nextOps, err
		}

		rr, err := vcs.RepoRootForImportPath(ps.Origin, false)
		if err != nil {
			return nextOps, err
		}
		b.RepoRoot = rr.Root

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

		if len(revision) == 0 && len(vpkg.Version) > 0 {
			// TODO (DT): resolve version to a revision.
			// revision = ...
			return nextOps, fmt.Errorf("fetching versions is not yet supported %s@%s", ps.Origin, vpkg.Version)
		}

		if len(revision) == 0 {
			// No specified revision, no version.
			err = rr.VCS.Create(b.Root, rr.Repo)
		} else {
			err = rr.VCS.CreateAtRev(b.Root, rr.Repo, revision)
		}
		if err != nil {
			return nextOps, err
		}
	}

	// set op.Src to download dir.
	// /tmp/cache/1/[[github.com/kardianos/govendor]]context
	op.Src = filepath.Join(b.Root, pathos.SlashToFilepath(strings.TrimPrefix(ps.Origin, b.RepoRoot)))
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
	system, err := gvvcs.FindVcs(f.TempRoot, op.Src)
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

	return nextOps, nil
}
