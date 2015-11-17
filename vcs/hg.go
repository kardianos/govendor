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
	info := &VcsInfo{}

	cmd := exec.Command("hg", "identify", "-i")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	rev := strings.TrimSpace(string(output))
	if strings.HasSuffix(rev, "+") {
		info.Dirty = true
		rev = strings.TrimSuffix(rev, "+")
	}

	cmd = exec.Command("hg", "log", "-r", rev)
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "changeset:") {
			ss := strings.Split(line, ":")
			info.Revision = strings.TrimSpace(ss[len(ss)-1])
		}
		if strings.HasPrefix(line, "date:") {
			line = strings.TrimPrefix(line, "date:")
			tm, err := time.Parse("Mon Jan 02 15:04:05 2006 -0700", strings.TrimSpace(line))
			if err == nil {
				info.RevisionTime = &tm
			}
		}
	}
	return info, nil
}
