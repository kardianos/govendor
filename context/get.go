// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"go/build"
	"io"
	"os"
	"path/filepath"

	"github.com/kardianos/govendor/pkgspec"
	"golang.org/x/tools/go/vcs"
)

func Get(logger io.Writer, pkgspecName string, insecure bool) (*pkgspec.Pkg, error) {
	// Get the GOPATHs.
	gopathList := filepath.SplitList(build.Default.GOPATH)
	gopath := gopathList[0]

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ps, err := pkgspec.Parse(cwd, pkgspecName)
	if err != nil {
		return nil, err
	}
	return ps, get(logger, filepath.Join(gopath, "src"), ps, insecure)
}

func get(logger io.Writer, gopath string, ps *pkgspec.Pkg, insecure bool) error {
	pkgDir := filepath.Join(gopath, ps.Path)
	sysVcsCmd, repoRoot, err := vcs.FromDir(pkgDir, gopath)
	var vcsCmd *VCSCmd
	repoRootDir := filepath.Join(gopath, repoRoot)
	if err != nil {
		rr, err := vcs.RepoRootForImportPath(ps.PathOrigin(), false)
		if err != nil {
			return err
		}
		if !insecure && !vcsIsSecure(rr.Repo) {
			return fmt.Errorf("repo remote not secure")
		}

		vcsCmd = updateVcsCmd(rr.VCS)
		repoRoot = rr.Root
		repoRootDir = filepath.Join(gopath, repoRoot)

		err = vcsCmd.Create(repoRootDir, rr.Repo)
		if err != nil {
			return fmt.Errorf("failed to create repo %q in %q %v", rr.Repo, repoRootDir, err)
		}

	} else {
		vcsCmd = updateVcsCmd(sysVcsCmd)
		err = vcsCmd.Download(repoRootDir)
		if err != nil {
			return fmt.Errorf("failed to download repo into %q %v", repoRootDir, err)
		}
	}
	err = os.MkdirAll(filepath.Join(repoRootDir, "vendor"), 0777)
	if err != nil {
		return err
	}
	ctx, err := NewContext(repoRootDir, filepath.Join("vendor", vendorFilename), "vendor", false)
	if err != nil {
		return err
	}
	ctx.Insecure = insecure
	ctx.Logger = logger
	statusList, err := ctx.Status()
	if err != nil {
		return err
	}
	added := make(map[string]bool, len(statusList))
	for _, item := range statusList {
		switch item.Status.Location {
		case LocationExternal, LocationNotFound:
			if added[item.Pkg.Path] {
				continue
			}
			ctx.ModifyImport(item.Pkg, Fetch)
			added[item.Pkg.Path] = true
		}
	}
	defer ctx.WriteVendorFile()
	return ctx.Alter()
}
