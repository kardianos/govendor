// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rewrite

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type VcsInfo struct {
	Version     string
	VersionTime *time.Time
}

type Vcs interface {
	// Return nil VcsInfo if unable to determine VCS from directory.
	Find(dir string) (*VcsInfo, error)
}

var VcsRegistry = []Vcs{
	VcsGit{},
	VcsHg{},
}

func FindVcs(root, packageDir string) (info *VcsInfo, err error) {
	path := packageDir
	for {
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
		if strings.HasPrefix(nextPath, root) == false {
			return nil, nil
		}
		path = nextPath
	}
}

type VcsGit struct{}

func (VcsGit) Find(dir string) (*VcsInfo, error) {
	fi, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if fi.IsDir() == false {
		return nil, nil
	}

	// Get info.
	cmd := exec.Command("git", "log", "-n", "1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	info := &VcsInfo{}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "commit ") {
			ss := strings.Split(line, " ")
			info.Version = strings.TrimSpace(ss[len(ss)-1])
		}
		if strings.HasPrefix(line, "Date:") {
			line = strings.TrimPrefix(line, "Date:")
			tm, err := time.Parse("Mon Jan 02 15:04:05 2006 -0700", strings.TrimSpace(line))
			if err == nil {
				info.VersionTime = &tm
			} else {
				return nil, err
			}
		}
	}
	return info, nil
}

type VcsHg struct{}

func (VcsHg) Find(dir string) (*VcsInfo, error) {
	fi, err := os.Stat(filepath.Join(dir, ".hg"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if fi.IsDir() == false {
		return nil, nil
	}

	// Get info.
	cmd := exec.Command("hg", "log", "-l", "1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	info := &VcsInfo{}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "changeset:") {
			ss := strings.Split(line, ":")
			info.Version = strings.TrimSpace(ss[len(ss)-1])
		}
		if strings.HasPrefix(line, "date:") {
			line = strings.TrimPrefix(line, "date:")
			tm, err := time.Parse("Mon Jan 02 15:04:05 2006 -0700", strings.TrimSpace(line))
			if err == nil {
				info.VersionTime = &tm
			}
		}
	}
	return info, nil
}
