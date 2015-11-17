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

type VcsBzr struct{}

func (VcsBzr) Find(dir string) (*VcsInfo, error) {
	fi, err := os.Stat(filepath.Join(dir, ".bzr"))
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

	cmd := exec.Command("bzr", "status")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	if string(output) != "" {
		info.Dirty = true
	}

	cmd = exec.Command("bzr", "log", "-r-1")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "revno:") {
			info.Revision = strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "revno:")), " ")[0]
		} else if strings.HasPrefix(line, "timestamp:") {
			tm, err := time.Parse("Mon 2006-01-02 15:04:05 -0700", strings.TrimSpace(strings.TrimPrefix(line, "timestamp:")))
			if err != nil {
				return nil, err
			}
			info.RevisionTime = &tm
		}
	}
	return info, nil
}
