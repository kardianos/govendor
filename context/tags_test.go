// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"strings"
	"testing"
)

func TestTagComplex(t *testing.T) {
	list := []struct {
		ignoreList string
		file       []string
		buildTags  string
		ignored    bool
	}{
		{
			ignoreList: "mips appengine test",
			file:       []string{"mips", "test"},
			buildTags:  "",
			ignored:    true,
		},
		{
			ignoreList: "",
			file:       []string{},
			buildTags:  "ignore",
			ignored:    true,
		},
		{
			ignoreList: "",
			file:       []string{},
			buildTags:  "",
			ignored:    false,
		},
		{
			ignoreList: "test",
			file:       []string{"mips"},
			buildTags:  "amd64",
			ignored:    false,
		},
		{
			ignoreList: "mips appengine test",
			file:       []string{},
			buildTags:  "",
			ignored:    false,
		},
		{
			ignoreList: "mips appengine test",
			file:       []string{},
			buildTags:  "mips,appengine",
			ignored:    true,
		},
		{
			ignoreList: "appengine test",
			file:       []string{},
			buildTags:  "mips,appengine",
			ignored:    true,
		},
		{
			ignoreList: "appengine test solaris",
			file:       []string{},
			buildTags:  "!windows",
			ignored:    false,
		},
		{
			ignoreList: "appengine test solaris",
			file:       []string{},
			buildTags:  "darwin dragonfly freebsd linux nacl netbsd openbsd solaris",
			ignored:    false,
		},
		{
			ignoreList: "darwin dragonfly freebsd linux nacl netbsd openbsd solaris",
			file:       []string{},
			buildTags:  "darwin dragonfly freebsd linux nacl netbsd openbsd solaris",
			ignored:    true,
		},
		{
			ignoreList: "test",
			file:       []string{"test"},
			buildTags:  "go1.8",
			ignored:    true,
		},
		{
			ignoreList: "",
			file:       []string{"!go1.8"},
			buildTags:  "go1.8",
			ignored:    true,
		},
	}

	run := -1

	for index, item := range list {
		if run >= 0 && run != index {
			continue
		}
		ignore := strings.Fields(item.ignoreList)
		ts := &TagSet{}
		for _, f := range item.file {
			ts.AddFileTag(f)
		}
		ts.AddBuildTags(item.buildTags)

		ignored := ts.IgnoreItem(ignore...)

		if ignored != item.ignored {
			t.Errorf("index %d wanted ignored=%t, got ignored=%t: ignore=%q build=%v", index, item.ignored, ignored, item.ignoreList, ts)
		}
	}
}
