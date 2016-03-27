// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package vcs gets version control information from the file system.
package vcs

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/kardianos/govendor/internal/pathos"
)

// VcsInfo returns information about a given repo.
type VcsInfo struct {
	Dirty        bool
	Revision     string
	RevisionTime *time.Time
}

// Vcs represents a version control system.
type Vcs interface {
	// Return nil VcsInfo if unable to determine VCS from directory.
	Find(dir string) (*VcsInfo, error)
}

var vcsRegistry = []Vcs{
	VcsGit{},
	VcsHg{},
	VcsSvn{},
	VcsBzr{},
}
var registerSync = sync.Mutex{}

// RegisterVCS adds a new VCS to use.
func RegisterVCS(vcs Vcs) {
	registerSync.Lock()
	defer registerSync.Unlock()

	vcsRegistry = append(vcsRegistry, vcs)
}

const looplimit = 10000

// FindVcs determines the version control information given a package dir and
// lowest root dir.
func FindVcs(root, packageDir string) (info *VcsInfo, err error) {
	if !filepath.IsAbs(root) {
		return nil, nil
	}
	if !filepath.IsAbs(packageDir) {
		return nil, nil
	}
	path := packageDir
	for i := 0; i <= looplimit; i++ {
		for _, vcs := range vcsRegistry {
			info, err = vcs.Find(path)
			if err != nil {
				return nil, err
			}
			if info != nil {
				return info, nil
			}
		}

		nextPath := filepath.Clean(filepath.Join(path, ".."))
		// Check for root.
		if nextPath == path {
			return nil, nil
		}
		if pathos.FileHasPrefix(nextPath, root) == false {
			return nil, nil
		}
		path = nextPath
	}
	panic("loop limit")
}
