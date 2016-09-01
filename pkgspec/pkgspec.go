// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package pkgspec parses the package specification string
package pkgspec

import (
	"errors"
	"path"
	"strings"
)

const (
	TreeIncludeSuffix = "/^"
	TreeMatchSuffix   = "/..."
)

const (
	originMatch   = "::"
	versionMatch  = "@"
	vendorSegment = "/vendor/"
)

var (
	ErrEmptyPath   = errors.New("Empty package path")
	ErrEmptyOrigin = errors.New("Empty origin specified")
	ErrInvalidPath = errors.New("Path contains a vendor folder and a origin")
)

// Parse a package spec according to:
// package-spec = <path>[{/...|/^}][::<origin>][@[<version-spec>]]
func Parse(currentGoPath, s string) (*Pkg, error) {
	// Clean up the import path before
	s = strings.Trim(s, "/\\ \t")
	if len(s) == 0 {
		return nil, ErrEmptyPath
	}
	s = strings.Replace(s, `\`, `/`, -1)

	originIndex := strings.Index(s, originMatch)
	versionIndex := strings.LastIndex(s, versionMatch)

	if originIndex == 0 {
		return nil, ErrEmptyPath
	}

	// Don't count the origin if it is after the "@" symbol.
	if originIndex > versionIndex && versionIndex > 0 {
		originIndex = -1
	}

	pkg := &Pkg{
		Path:      s,
		HasOrigin: (originIndex >= 0),
	}

	if versionIndex > 0 {
		pkg.Path = s[:versionIndex]
		pkg.Version = s[versionIndex+len(versionMatch):]
		pkg.HasVersion = true
	}
	if originIndex > 0 {
		pkg.Path = s[:originIndex]
		endOrigin := len(s)
		if versionIndex > 0 {
			endOrigin = versionIndex
		}
		pkg.Origin = s[originIndex+len(originMatch) : endOrigin]
		if len(pkg.Origin) == 0 {
			return nil, ErrEmptyOrigin
		}
	}
	// Look for vendor folder in package path.
	// This is allowed in origin, but not path.
	vendorIndex := strings.LastIndex(pkg.Path, vendorSegment)
	if vendorIndex >= 0 {
		if len(pkg.Origin) > 0 {
			return nil, ErrInvalidPath
		}
		pkg.Origin = pkg.Path
		pkg.Path = pkg.Path[vendorIndex+len(vendorSegment):]
	}

	if strings.HasSuffix(pkg.Path, TreeMatchSuffix) {
		pkg.MatchTree = true
		pkg.Path = strings.TrimSuffix(pkg.Path, TreeMatchSuffix)
	} else if strings.HasSuffix(pkg.Path, TreeIncludeSuffix) {
		pkg.IncludeTree = true
		pkg.Path = strings.TrimSuffix(pkg.Path, TreeIncludeSuffix)
	}
	if strings.HasPrefix(pkg.Path, ".") && len(currentGoPath) != 0 {
		currentGoPath = strings.Replace(currentGoPath, `\`, `/`, -1)
		currentGoPath = strings.TrimPrefix(currentGoPath, "/")
		pkg.Path = path.Join(currentGoPath, pkg.Path)
	}

	return pkg, nil
}
