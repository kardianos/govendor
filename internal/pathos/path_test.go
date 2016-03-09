// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pathos

import (
	"testing"
)

func TestTrimCommonSuffix(t *testing.T) {
	list := []struct {
		slash          rune
		base, suffix   string
		result, common string
	}{
		{slash: '/', base: "/a/b/c", suffix: "/x/y/b/c", result: "/a", common: "b/c"},
		{slash: '/', base: "/tmp/vendor_272718190/src/co2/go/pk1/", suffix: "co2/go/pk1", result: "/tmp/vendor_272718190/src", common: "co2/go/pk1"},
		{slash: '\\', base: `d:\bob\alice\noob`, suffix: `c:\tmp\foo\alice\noob`, result: `d:\bob`, common: `alice\noob`},
	}

	for _, item := range list {
		slashSep = item.slash
		got, common := TrimCommonSuffix(item.base, item.suffix)
		if got != item.result || common != item.common {
			t.Errorf("For %#v got %q, common: %q", item, got, common)
		}
	}
}
