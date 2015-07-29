// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vcs

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/kardianos/govendor/internal/pathos"
)

type VcsInfo struct {
	Dirty        bool
	Revision     string
	RevisionTime *time.Time
}

type Vcs interface {
	// Return nil VcsInfo if unable to determine VCS from directory.
	Find(dir string) (*VcsInfo, error)
}

var VcsRegistry = []Vcs{
	VcsGit{},
	VcsHg{},
	VcsBzr{},
}
var registerSync = sync.Mutex{}

func RegisterVCS(vcs Vcs) {
	registerSync.Lock()
	registerSync.Unlock()

	// TODO: Ensure unique. Panic on duplicate. Maybe make into a map.
	VcsRegistry = append(VcsRegistry, vcs)
}

const looplimit = 10000

func FindVcs(root, packageDir string) (info *VcsInfo, err error) {
	path := packageDir
	for i := 0; i <= looplimit; i++ {
		for _, vcs := range VcsRegistry {
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
