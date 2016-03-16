// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vcs

import (
	"encoding/xml"
	"os/exec"
	"path/filepath"
	"time"

	os "github.com/kardianos/govendor/internal/vos"
)

type VcsSvn struct{}

func (svn VcsSvn) Find(dir string) (*VcsInfo, error) {
	fi, err := os.Stat(filepath.Join(dir, ".svn"))
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

	cmd := exec.Command("svn", "info", "--xml")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return info, svn.parseInfo(output, info)
}
func (svn VcsSvn) parseInfo(output []byte, info *VcsInfo) error {
	var err error
	XX := struct {
		Commit struct {
			Revision     string `xml:"revision,attr"`
			RevisionTime string `xml:"date"`
		} `xml:"entry>commit"`
	}{}
	err = xml.Unmarshal(output, &XX)
	if err != nil {
		return err
	}
	info.Revision = XX.Commit.Revision
	tm, err := time.Parse(time.RFC3339, XX.Commit.RevisionTime)
	if err == nil {
		info.RevisionTime = &tm
	}
	return nil
}
