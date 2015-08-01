// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"sort"
)

// Status indicates the status of the import.
type Status byte

func (ls Status) String() string {
	switch ls {
	case StatusUnknown:
		return "?"
	case StatusMissing:
		return "m"
	case StatusStandard:
		return "s"
	case StatusLocal:
		return "l"
	case StatusExternal:
		return "e"
	case StatusUnused:
		return "u"
	case StatusProgram:
		return "p"
	case StatusVendor:
		return "v"
	}
	return ""
}

const (
	// StatusUnknown indicates the status was unable to be obtained.
	StatusUnknown Status = iota
	// StatusMissing indicates import not found in GOROOT or GOPATH.
	StatusMissing
	// StatusStd indicates import found in GOROOT.
	StatusStandard
	// StatusLocal indicates import is part of the local project.
	StatusLocal
	// StatusExternal indicates import is found in GOPATH and not copied.
	StatusExternal
	// StatusUnused indicates import has been copied, but is no longer used.
	StatusUnused
	// StatusProgram indicates the import is a main package but internal or vendor.
	StatusProgram
	// StatusVendor indicates theimport is in the vendor folder.
	StatusVendor
)

// ListItem represents a package in the current project.
type StatusItem struct {
	Status     Status
	Canonical  string
	Local      string
	ImportedBy []string
}

func (li StatusItem) String() string {
	if li.Local == li.Canonical {
		return fmt.Sprintf("%s %s < %q", li.Status, li.Canonical, li.ImportedBy)
	}
	return fmt.Sprintf("%s %s [%s] < %q", li.Status, li.Local, li.Canonical, li.ImportedBy)
}

type statusItemSort []StatusItem

func (li statusItemSort) Len() int      { return len(li) }
func (li statusItemSort) Swap(i, j int) { li[i], li[j] = li[j], li[i] }
func (li statusItemSort) Less(i, j int) bool {
	if li[i].Status != li[j].Status {
		return li[i].Status > li[j].Status
	}
	return li[i].Local < li[j].Local
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
	ctx.updatePackageReferences()
	list := make([]StatusItem, 0, len(ctx.Package))
	for _, pkg := range ctx.Package {
		li := StatusItem{
			Status:     pkg.Status,
			Canonical:  pkg.Canonical,
			Local:      pkg.Local,
			ImportedBy: make([]string, 0, len(pkg.referenced)),
		}
		for _, ref := range pkg.referenced {
			li.ImportedBy = append(li.ImportedBy, ref.Local)
		}
		sort.Strings(li.ImportedBy)
		list = append(list, li)
	}
	// Sort li by Status, then Path.
	sort.Sort(statusItemSort(list))

	return list, nil
}
