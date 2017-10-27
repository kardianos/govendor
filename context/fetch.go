// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"os"
	"path"
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
	vpkg := f.Ctx.VendorFilePackagePath(op.Pkg.Path)
	if vpkg == nil {
		return nextOps, fmt.Errorf("Could not find vendor file package for %q. Internal error.", op.Pkg.Path)
	}

	op.Type = OpCopy
	ps, err := pkgspec.Parse("", op.Src)
	if err != nil {
		return nextOps, err
	}
	if len(ps.Version) == 0 {
		longest := ""
		for _, pkg := range f.Ctx.Package {
			if strings.HasPrefix(ps.Path, pkg.Path+"/") && len(pkg.Path) > len(longest) && pkg.HasVersion {
				longest = pkg.Path
				ps.Version = pkg.Version
				ps.HasVersion = true
			}
		}
	}

	// Don't check for bundle, rather check physical directory.
	// If no repo in dir, clone.
	// If there is a repo in dir, update to latest.
	// Get any tags.
	// If we have a specific revision, update to that revision.

	pkgDir := filepath.Join(f.CacheRoot, pathos.SlashToFilepath(ps.PathOrigin()))
	sysVcsCmd, repoRoot, err := vcs.FromDir(pkgDir, f.CacheRoot)
	var vcsCmd *VCSCmd
	repoRootDir := filepath.Join(f.CacheRoot, repoRoot)
	if err != nil {
		rr, err := vcs.RepoRootForImportPath(ps.PathOrigin(), false)
		if err != nil {
			if strings.Contains(err.Error(), "unrecognized import path") {
				return nextOps, nil
			}
			return nextOps, err
		}
		if !f.Ctx.Insecure && !vcsIsSecure(rr.Repo) {
			return nextOps, fmt.Errorf("repo remote not secure")
		}

		vcsCmd = updateVcsCmd(rr.VCS)
		repoRoot = rr.Root
		repoRootDir = filepath.Join(f.CacheRoot, repoRoot)

		err = vcsCmd.Create(repoRootDir, rr.Repo)
		if err != nil {
			return nextOps, fmt.Errorf("failed to create repo %q in %q %v", rr.Repo, repoRootDir, err)
		}

	} else {
		vcsCmd = updateVcsCmd(sysVcsCmd)
		err = vcsCmd.Download(repoRootDir)
		if err != nil {
			return nextOps, fmt.Errorf("failed to download repo into %q %v", repoRootDir, err)
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
		fmt.Fprintf(f.Ctx, "Get version %q@%s\n", vpkg.Path, vpkg.Version)
		// Get a list of tags, match to version if possible.
		var tagNames []string
		tagNames, err = vcsCmd.Tags(repoRootDir)
		if err != nil {
			return nextOps, fmt.Errorf("failed to fetch tags %v", err)
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
		vpkg.VersionExact = result.Text
		fmt.Fprintf(f.Ctx, "\tFound exact version %q\n", vpkg.VersionExact)
		err = vcsCmd.TagSync(repoRootDir, result.Text)
		if err != nil {
			return nextOps, fmt.Errorf("failed to sync repo to tag %q %v", result.Text, err)
		}
	case len(revision) > 0:
		fmt.Fprintf(f.Ctx, "Get specific revision %q@%s\n", vpkg.Path, revision)
		// Get specific version.
		vpkg.Version = ""
		vpkg.VersionExact = ""
		err = vcsCmd.RevisionSync(repoRootDir, revision)
		if err != nil {
			return nextOps, fmt.Errorf("failed to sync repo to revision %q %v", revision, err)
		}
	default:
		fmt.Fprintf(f.Ctx, "Get latest revision %q\n", vpkg.Path)
		// Get latest version.
		err = vcsCmd.TagSync(repoRootDir, "")
		if err != nil {
			return nextOps, fmt.Errorf("failed to sync to latest revision %v", err)
		}
	}

	// set op.Src to download dir.
	// /tmp/cache/1/[[github.com/kardianos/govendor]]context
	op.Src = pkgDir
	var deps []string
	op.IgnoreFile, deps, err = f.Ctx.getIgnoreFiles(op.Src)
	if err != nil {
		if os.IsNotExist(err) {
			return nextOps, nil
		}
		return nextOps, fmt.Errorf("failed to get ignore files and deps from %q %v", op.Src, err)
	}

	f.HavePkg[ps.Path] = true

	// Once downloaded, be sure to set the revision and revisionTime
	// in the vendor file package.
	// Find the VCS information.
	system, err := gvvcs.FindVcs(f.CacheRoot, op.Src)
	if err != nil {
		return nextOps, fmt.Errorf("failed to find vcs in %q %v", op.Src, err)
	}
	if system != nil {
		if system.Dirty {
			return nextOps, ErrDirtyPackage{ps.PathOrigin()}
		}
		vpkg.Revision = system.Revision
		if system.RevisionTime != nil {
			vpkg.RevisionTime = system.RevisionTime.UTC().Format(time.RFC3339)
		}
	}

	processDeps := func(deps []string) error {
		// Queue up any missing package deps.
	depLoop:
		for _, dep := range deps {
			dep = strings.TrimSpace(dep)
			if len(dep) == 0 {
				continue
			}

			// Check for deps we already have.
			if f.HavePkg[dep] {
				continue
			}

			for _, test := range f.Ctx.Package {
				if test.Path == dep {
					switch test.Status.Location {
					case LocationVendor, LocationLocal:
						continue depLoop
					}
				}
			}

			// Look for std lib deps
			var yes bool
			yes, err = f.Ctx.isStdLib(dep)
			if err != nil {
				return fmt.Errorf("Failed to check if in stdlib: %v", err)
			}
			if yes {
				continue
			}

			// Look for tree deps.
			if op.Pkg.IncludeTree && strings.HasPrefix(dep, op.Pkg.Path+"/") {
				continue
			}
			version := ""
			hasVersion := false
			revision := ""
			hasOrigin := false
			origin := ""
			for _, vv := range f.Ctx.VendorFile.Package {
				if vv.Remove {
					continue
				}
				if strings.HasPrefix(dep, vv.Path+"/") {
					if len(vv.Origin) > 0 {
						origin = path.Join(vv.PathOrigin(), strings.TrimPrefix(dep, vv.Path))
						hasOrigin = true
					}
					if len(vv.Version) > 0 {
						version = vv.Version
						hasVersion = true
						revision = vv.Revision
						break
					}
					if len(vv.Revision) > 0 {
						revision = vv.Revision
					}
				}
			}

			// Look for tree match in explicit imports
			for _, item := range f.Ctx.TreeImport {
				if item.Path != dep && !strings.HasPrefix(dep, item.Path+"/") {
					continue
				}
				if len(item.Origin) > 0 {
					origin = path.Join(item.PathOrigin(), strings.TrimPrefix(dep, item.Path))
					hasOrigin = true
				}
				if len(item.Version) > 0 {
					version = item.Version
					hasVersion = true
					revision = ""
				}
				break
			}

			f.HavePkg[dep] = true
			dest := filepath.Join(f.Ctx.RootDir, f.Ctx.VendorFolder, dep)

			// Update vendor file with correct Local field.
			vp := f.Ctx.VendorFilePackagePath(dep)
			if vp == nil {
				vp = &vendorfile.Package{
					Add:      true,
					Path:     dep,
					Revision: revision,
					Version:  version,
					Origin:   origin,
				}
				f.Ctx.VendorFile.Package = append(f.Ctx.VendorFile.Package, vp)
			}
			if hasVersion {
				vp.Version = version
			}
			if hasOrigin {
				vp.Origin = origin
			}
			if len(vp.Revision) == 0 {
				vp.Revision = revision
			}
			spec := &pkgspec.Pkg{
				Path:       dep,
				Version:    version,
				HasVersion: hasVersion,
				Origin:     origin,
				HasOrigin:  hasOrigin,
			}
			nextOps = append(nextOps, &Operation{
				Type: OpFetch,
				Pkg:  &Package{Pkg: spec},
				Src:  spec.String(),
				Dest: dest,
			})
		}
		return nil
	}

	err = processDeps(deps)
	if err != nil {
		return nextOps, err
	}

	err = f.Ctx.copyOperation(op, processDeps)
	if err != nil {
		return nextOps, err
	}

	return nextOps, nil
}
