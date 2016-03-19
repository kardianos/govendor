// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vcs

import (
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	os "github.com/kardianos/govendor/internal/vos"
)

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
	info := &VcsInfo{}

	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = dir
	err = cmd.Run()
	if err != nil {
		info.Dirty = true
	}

	cmd = exec.Command("git", "show", "--pretty=format:%H@%ai", "-s")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	line := strings.TrimSpace(string(output))
	ss := strings.Split(line, "@")
	info.Revision = ss[0]
	tm, err := time.Parse("2006-01-02 15:04:05 -0700", ss[1])
	if err != nil {
		return nil, err
	}
	info.RevisionTime = &tm
	return info, nil
}
