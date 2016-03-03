// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"testing"
)

func TestIsVersion(t *testing.T) {
	list := []struct {
		Text      string
		IsVersion bool
	}{
		{"10", true},
		{"86811224500e65b549265b273109f2166a35fe63", false},
		{"v1", true},
		{"v1.2-beta", true},
		{"1", true},
		{"3242", false},
		{"2ea995a", false},
	}

	for _, item := range list {
		is := isVersion(item.Text)
		if is != item.IsVersion {
			t.Errorf("For %q, got %v, want %v", item.Text, is, item.IsVersion)
		}
	}
}
