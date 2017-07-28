// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/kardianos/govendor/pkgspec"
)

type (
	// Status is the package type, location, and presence indicators.
	Status struct {
		Type     StatusType     // program, package
		Location StatusLocation // vendor, local, external, stdlib
		Presence StatusPresence // missing, unused, tree, excluded

		Not bool // Not indicates boolean operation "not" on above.
	}

	StatusType     byte // StatusType is main or not-main.
	StatusLocation byte // StatusLocation is where the package is.
	StatusPresence byte // StatusPresence is if it can be found or referenced.

	// StatusGroup is the logical filter for status with "and", "not", and grouping.
	StatusGroup struct {
		Status []Status
		Group  []StatusGroup
		And    bool
		Not    bool
	}
)

func (s Status) String() string {
	t := ' '
	l := ' '
	p := ' '
	not := ""
	if s.Not {
		not = "!"
	}
	switch s.Type {
	default:
		panic("Unknown Type type")
	case TypeUnknown:
		t = '_'
	case TypePackage:
		t = ' '
	case TypeProgram:
		t = 'p'
	}
	switch s.Location {
	default:
		panic("Unknown Location type")
	case LocationUnknown:
		l = '_'
	case LocationNotFound:
		l = ' '
	case LocationLocal:
		l = 'l'
	case LocationExternal:
		l = 'e'
	case LocationVendor:
		l = 'v'
	case LocationStandard:
		l = 's'
	}
	switch s.Presence {
	default:
		panic("Unknown Presence type")
	case PresenceUnknown:
		p = '_'
	case PresenceFound:
		p = ' '
	case PresenceMissing:
		p = 'm'
	case PresenceUnused:
		p = 'u'
	case PresenceTree:
		p = 't'
	case PresenceExcluded:
		p = 'x'
	}
	return not + string(t) + string(l) + string(p)
}

func (sg StatusGroup) String() string {
	buf := &bytes.Buffer{}
	if sg.And {
		buf.WriteString("and")
	} else {
		buf.WriteString("or")
	}
	buf.WriteRune('(')
	for i, s := range sg.Status {
		if i != 0 {
			buf.WriteRune(',')
		}
		buf.WriteString(s.String())
	}
	if len(sg.Status) > 0 && len(sg.Group) > 0 {
		buf.WriteRune(',')
	}
	for i, ssg := range sg.Group {
		if i != 0 {
			buf.WriteRune(',')
		}
		buf.WriteString(ssg.String())
	}
	buf.WriteRune(')')
	return buf.String()
}

func (pkgSt Status) Match(filterSt Status) bool {
	// not: true, pkg: A, filter: B
	// true == (A == B) -> true == false -> false
	//
	// not: false, pkg: A, filter: B
	// false == (A == B) -> false == false -> true
	//
	// not: true, pkg: A, filter: A
	// true == (A == A) -> true == true) -> true
	//
	// not: false, pkg: A, filter: A
	// false == (A == A) -> false == true -> false
	if filterSt.Location != LocationUnknown && filterSt.Not == (pkgSt.Location == filterSt.Location) {
		return false
	}
	if filterSt.Type != TypeUnknown && filterSt.Not == (pkgSt.Type == filterSt.Type) {
		return false
	}
	if filterSt.Presence != PresenceUnknown && filterSt.Not == (pkgSt.Presence == filterSt.Presence) {
		return false
	}
	return true
}

func (status Status) MatchGroup(filter StatusGroup) bool {
	or := !filter.And
	for _, fs := range filter.Status {
		if status.Match(fs) == or {
			return or != filter.Not
		}
	}
	for _, fg := range filter.Group {
		if status.MatchGroup(fg) == or {
			return or != filter.Not
		}
	}
	return filter.And
}

const (
	TypeUnknown StatusType = iota // TypeUnknown is unset StatusType.
	TypePackage                   // TypePackage package is a non-main package.
	TypeProgram                   // TypeProgram package is a main package.
)

const (
	LocationUnknown  StatusLocation = iota // LocationUnknown is unset StatusLocation.
	LocationNotFound                       // LocationNotFound package is not to be found (use PresenceMissing).
	LocationStandard                       // LocationStandard package is in the standard library.
	LocationLocal                          // LocationLocal package is in a project, not in a vendor folder.
	LocationExternal                       // LocationExternal package is not in a project, in GOPATH.
	LocationVendor                         // LocationVendor package is in a vendor folder.
)

const (
	PresenceUnknown  StatusPresence = iota // PresenceUnknown is unset StatusPresence.
	PresenceFound                          // PresenceFound package exists.
	PresenceMissing                        // PresenceMissing package is referenced but not found.
	PresenceUnused                         // PresenceUnused package is found locally but not referenced.
	PresenceTree                           // PresenceTree package is in vendor folder, in a tree, but not referenced.
	PresenceExcluded                       // PresenceExcluded package exists, but should not be vendored.
)

// ListItem represents a package in the current project.
type StatusItem struct {
	Status       Status
	Pkg          *pkgspec.Pkg
	VersionExact string
	Local        string
	ImportedBy   []*Package
}

func (li StatusItem) String() string {
	if li.Local == li.Pkg.Path {
		return fmt.Sprintf("%s %s < %q", li.Status, li.Pkg.Path, li.ImportedBy)
	}
	return fmt.Sprintf("%s %s [%s] < %q", li.Status, li.Local, li.Pkg.Path, li.ImportedBy)
}

type statusItemSort []StatusItem

func (li statusItemSort) Len() int      { return len(li) }
func (li statusItemSort) Swap(i, j int) { li[i], li[j] = li[j], li[i] }
func (li statusItemSort) Less(i, j int) bool {
	if li[i].Status.Location != li[j].Status.Location {
		return li[i].Status.Location > li[j].Status.Location
	}
	return li[i].Local < li[j].Local
}

// Status obtains the current package status list.
func (ctx *Context) updateStatusCache() error {
	var err error
	if !ctx.loaded || ctx.dirty {
		err = ctx.loadPackage()
		if err != nil {
			return err
		}
	}
	ctx.updatePackageReferences()
	list := make([]StatusItem, 0, len(ctx.Package))
	for _, pkg := range ctx.Package {
		version := ""
		versionExact := ""
		if vp := ctx.VendorFilePackagePath(pkg.Path); vp != nil {
			version = vp.Version
			versionExact = vp.VersionExact
		}

		origin := ""
		if pkg.Origin != pkg.Path {
			origin = pkg.Origin
		}
		if len(pkg.Origin) == 0 && pkg.Path != pkg.Local {
			origin = pkg.Local
		}

		li := StatusItem{
			Status:       pkg.Status,
			Pkg:          &pkgspec.Pkg{Path: pkg.Path, IncludeTree: pkg.IncludeTree, Origin: origin, Version: version, FilePath: pkg.Dir},
			Local:        pkg.Local,
			VersionExact: versionExact,
			ImportedBy:   make([]*Package, 0, len(pkg.referenced)),
		}
		for _, ref := range pkg.referenced {
			li.ImportedBy = append(li.ImportedBy, ref)
		}
		sort.Sort(packageList(li.ImportedBy))
		list = append(list, li)
	}
	// Sort li by Status, then Path.
	sort.Sort(statusItemSort(list))

	ctx.statusCache = list
	return nil
}

// Status obtains the current package status list.
func (ctx *Context) Status() ([]StatusItem, error) {
	var err error
	if !ctx.loaded || ctx.dirty {
		err = ctx.loadPackage()
		if err != nil {
			return nil, err
		}
	}
	if ctx.statusCache == nil {
		err = ctx.updateStatusCache()
		if err != nil {
			return nil, err
		}
	}
	return ctx.statusCache, nil
}
