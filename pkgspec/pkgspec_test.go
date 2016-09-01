// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkgspec

import "testing"

func TestParse(t *testing.T) {
	list := []struct {
		Spec string
		Str  string
		Pkg  *Pkg
		Err  error
		WD   string
	}{
		{Spec: "abc/def", Pkg: &Pkg{Path: "abc/def"}},
		{Spec: "", Err: ErrEmptyPath},
		{Spec: "::", Err: ErrEmptyPath},
		{Spec: "::foo", Err: ErrEmptyPath},
		{Spec: "abc/def::", Err: ErrEmptyOrigin},
		{Spec: "abc/def::foo/bar/vendor/abc/def"},
		{Spec: "abc/def::foo/bar/vendor/abc/def@"},
		{Spec: "abc/def::foo/bar/vendor/abc/def@v1.2.3", Pkg: &Pkg{Path: "abc/def", HasOrigin: true, Origin: "foo/bar/vendor/abc/def", HasVersion: true, Version: "v1.2.3"}},
		{Spec: "abc/def@", Pkg: &Pkg{Path: "abc/def", HasVersion: true}},
		{Spec: "abc/def@v1.2.3", Pkg: &Pkg{Path: "abc/def", HasVersion: true, Version: "v1.2.3"}},
		{Spec: "./def@v1.2.3", Str: "abc/def@v1.2.3", Pkg: &Pkg{Path: "abc/def", HasVersion: true, Version: "v1.2.3"}, WD: "abc/"},
		{Spec: "abc\\def\\", Str: "abc/def", Pkg: &Pkg{Path: "abc/def"}},
		{Spec: "github.com/aws/aws-sdk-go/aws/client::github.com/aws/aws-sdk-go/aws/client"},
		{Spec: "a/b/vendor/z/y/x", Str: "z/y/x::a/b/vendor/z/y/x"},
		{Spec: "a/b/vendor/z/y/x::a/b/vendor/z/y/x", Err: ErrInvalidPath},
	}

	for _, item := range list {
		pkg, err := Parse(item.WD, item.Spec)
		if err != nil && item.Err != nil {
			if err != item.Err {
				t.Errorf("For %q, got error %q but expected error %q", item.Spec, err, item.Err)
				continue
			}
			continue
		}
		if err == nil && item.Err != nil {
			t.Errorf("For %q, got nil error but expected error %q, %#v", item.Spec, item.Err, pkg)
			continue
		}
		if pkg == nil {
			t.Errorf("For %q, got nil pkg", item.Spec)
			continue
		}
		pkgStr := pkg.String()
		specStr := item.Spec
		if len(item.Str) > 0 {
			specStr = item.Str
		}
		if pkgStr != specStr {
			t.Errorf("For %q, round tripped to %q", specStr, pkgStr)
			continue
		}
		if item.Pkg != nil {
			diffA := pkg.Path != item.Pkg.Path || pkg.Origin != item.Pkg.Origin || pkg.Version != item.Pkg.Version
			diffB := pkg.HasVersion != item.Pkg.HasVersion || pkg.HasOrigin != item.Pkg.HasOrigin || pkg.MatchTree != item.Pkg.MatchTree || pkg.IncludeTree != item.Pkg.IncludeTree
			if diffA || diffB {
				t.Errorf("For %q, pkg detail diff: got %#v", item.Spec, pkg)
			}
		}
	}
}
