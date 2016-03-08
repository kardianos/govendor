// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkgspec

import "bytes"

type Pkg struct {
	Path        string
	Origin      string
	IncludeTree bool
	MatchTree   bool
	HasVersion  bool
	Version     string

	Uncommitted bool
}

func (pkg *Pkg) String() string {
	buf := &bytes.Buffer{}
	buf.WriteString(pkg.Path)
	if pkg.IncludeTree {
		buf.WriteString(TreeIncludeSuffix)
	} else if pkg.MatchTree {
		buf.WriteString(TreeMatchSuffix)
	}
	if len(pkg.Origin) > 0 {
		buf.WriteString(originMatch)
		buf.WriteString(pkg.Origin)
	}
	if pkg.HasVersion {
		buf.WriteString(versionMatch)
		if len(pkg.Version) > 0 {
			buf.WriteString(pkg.Version)
		}
	}
	return buf.String()
}
