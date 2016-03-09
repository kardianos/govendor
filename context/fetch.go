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
	}, nil
}

func (f *fetcher) cleanUp() error {
	return os.RemoveAll(f.TempRoot)
}

// op fetches the repo locally if not already present.
// Transform the fetch op into a copy op.
func (f *fetcher) op(op *Operation) error {
	vpkg := f.Ctx.VendorFilePackagePath(op.Pkg.Canonical)
	if vpkg == nil {
		return fmt.Errorf("Could not find vendor file package for %q. Internal error.", op.Pkg.Canonical)
	}

	op.Type = OpCopy
	ps, err := pkgspec.Parse("", op.Src)
	if err != nil {
		return err
	}

	var b *syncBundle
	for _, item := range f.Bundles {
		// Must be from the same repo (space).
		if item.RepoRoot != ps.Path && !strings.HasPrefix(ps.Path, item.RepoRoot+"/") {
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
			return err
		}

		rr, err := vcs.RepoRootForImportPath(ps.Path, false)
		if err != nil {
			return err
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
			return fmt.Errorf("fetching versions is not yet supported %s@%s", ps.Path, vpkg.Version)
		}

		if len(revision) == 0 {
			// No specified revision, no version.
			err = rr.VCS.Create(b.Root, rr.Repo)
		} else {
			err = rr.VCS.CreateAtRev(b.Root, rr.Repo, revision)
		}
		if err != nil {
			return err
		}
	}

	// set op.Src to download dir.
	// /tmp/cache/1/[[github.com/kardianos/govendor]]context
	op.Src = filepath.Join(b.Root, pathos.SlashToFilepath(strings.TrimPrefix(ps.Path, b.RepoRoot)))
	op.IgnoreFile, err = f.Ctx.getIngoreFiles(op.Src)
	if err != nil {
		return err
	}

	// Once downloaded, be sure to set the revision and revisionTime
	// in the vendor file package.
	// Find the VCS information.
	system, err := gvvcs.FindVcs(f.TempRoot, op.Src)
	if err != nil {
		return err
	}
	if system != nil {
		if system.Dirty {
			return ErrDirtyPackage{ps.Path}
		}
		vpkg.Revision = system.Revision
		if system.RevisionTime != nil {
			vpkg.RevisionTime = system.RevisionTime.Format(time.RFC3339)
		}
	}

	return nil
}
