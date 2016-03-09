// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
	"github.com/kardianos/govendor/vendorfile"

	"golang.org/x/tools/go/vcs"
)

func skipperTree(name string, dir bool) bool {
	return false
}
func skipperPackage(name string, dir bool) bool {
	return dir
}

func (ctx *Context) VerifyVendor() (outOfDate []*vendorfile.Package, err error) {
	vf := ctx.VendorFile
	root := filepath.Join(ctx.RootDir, ctx.VendorFolder)
	add := func(vp *vendorfile.Package) {
		outOfDate = append(outOfDate, vp)
	}
	for _, vp := range vf.Package {
		if vp.Remove {
			continue
		}
		if len(vp.Path) == 0 {
			continue
		}
		if len(vp.ChecksumSHA1) == 0 {
			add(vp)
			continue
		}
		fp := filepath.Join(root, pathos.SlashToFilepath(vp.Path))
		h := sha1.New()
		sk := skipperPackage
		if vp.Tree {
			sk = skipperTree
		}
		err = getHash(root, fp, h, sk)
		if err != nil {
			return
		}
		checksum := base64.StdEncoding.EncodeToString(h.Sum(nil))
		if vp.ChecksumSHA1 != checksum {
			add(vp)
		}
	}
	return
}

func getHash(root, fp string, h hash.Hash, skipper func(name string, isDir bool) bool) error {
	rel := pathos.FileTrimPrefix(fp, root)
	rel = pathos.SlashToImportPath(rel)
	rel = strings.Trim(rel, "/")

	h.Write([]byte(rel))

	dir, err := os.Open(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("Failed to open dir %q: %v", fp, err)
	}
	filelist, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		return fmt.Errorf("Failed to read dir %q: %v", fp, err)
	}
	sort.Sort(fileInfoSort(filelist))
	for _, fi := range filelist {
		if skipper(fi.Name(), fi.IsDir()) {
			continue
		}
		p := filepath.Join(fp, fi.Name())
		if fi.IsDir() {
			err = getHash(root, p, h, skipper)
			if err != nil {
				return err
			}
			continue
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		h.Write([]byte(fi.Name()))
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// similarSegments compares two paths and determines if they have
// similar prefix segments. For example github.com/kardianos/rdb and
// github.com/kardianos/govendor have 2 similar segments.
func similarSegments(p1, p2 string) (string, int) {
	seg1 := strings.Split(p1, "/")
	seg2 := strings.Split(p2, "/")

	ct := len(seg1)
	if len(seg2) < ct {
		ct = len(seg2)
	}

	similar := &bytes.Buffer{}
	for i := 0; i < ct; i++ {
		if seg1[i] != seg2[i] {
			return similar.String(), i
		}
		if i != 0 {
			similar.WriteRune('/')
		}
		similar.WriteString(seg1[i])
	}
	return similar.String(), ct
}

type remoteFailure struct {
	Path string
	Err  error
}

func (fail remoteFailure) Error() string {
	return fmt.Sprintf("Failed for %q: %v", fail.Path, fail.Err)
}

type remoteFailureList []remoteFailure

func (list remoteFailureList) Error() string {
	if len(list) == 0 {
		return "(no remote failure)"
	}
	buf := &bytes.Buffer{}
	buf.WriteString("Remotes failed for:\n")
	for _, item := range list {
		buf.WriteString("\t")
		buf.WriteString(item.Error())
		buf.WriteString("\n")
	}
	return buf.String()
}

type syncBundle struct {
	Packages []*vendorfile.Package
	Root     string // Root directory file path.
	Revision string
	Segment  string // Base path segment for similar origin.
	RepoRoot string // The part of the import path where the repo is cloned.
	Err      error
}

// Sync checks for outdated packages in the vendor folder and fetches the
// correct revision from the remote.
func (ctx *Context) Sync() (err error) {
	outOfDate, err := ctx.VerifyVendor()
	if err != nil {
		return fmt.Errorf("Failed to verify checksums: %v", err)
	}
	// Create temporary folder to download files from.
	tempRoot, err := ioutil.TempDir(os.TempDir(), "govendor-cache")
	if err != nil {
		return err
	}
	defer func() {
		err = os.RemoveAll(tempRoot)
	}()
	bundles := make([]*syncBundle, 0, len(outOfDate))
outer:
	for _, vp := range outOfDate {
		// Bundle packages together that have the same revision and share at least one root segment.
		if len(vp.Revision) == 0 {
			continue
		}
		from := vp.Path
		if len(vp.Origin) > 0 {
			from = vp.Origin
		}
		for _, b := range bundles {
			if b.Revision == vp.Revision {
				similar, number := similarSegments(b.Root, from)
				if number >= 2 {
					b.Root = similar
					b.Packages = append(b.Packages, vp)
					continue outer
				}
			}
		}
		// No existing bundle found. Add a new bundle.
		add := &syncBundle{
			Packages: []*vendorfile.Package{vp},
			Segment:  from,
			Revision: vp.Revision,
			Root:     filepath.Join(tempRoot, fmt.Sprintf("%d", len(bundles))),
		}
		bundles = append(bundles, add)
		err = os.MkdirAll(add.Root, 0700)
		if err != nil {
			return err
		}
	}

	// For each bundle, download into the temporary folder.
	// Each bundle should go into their own root folder: <temp-dir>/<bundle-index>/<package-path>
	for _, b := range bundles {
		rr, err := vcs.RepoRootForImportPath(b.Segment, false)
		if err != nil {
			// Support failing a download and continuing with the rest.
			b.Err = err
			continue
		}
		b.RepoRoot = rr.Root
		// TODO (DT): clone into persistent cache.
		err = rr.VCS.CreateAtRev(b.Root, rr.Repo, b.Revision)
		if err != nil {
			// Support failing a download and continuing with the rest.
			b.Err = err
			continue
		}
	}

	rem := remoteFailureList{}
	updatedVendorFile := false

	// Copy each vendor file (listed in each bundle) into the vendor folder.
	// Ensure we hash the values and update the vendor file package listing.
	h := sha1.New()
	for _, b := range bundles {
		if b.Err != nil {
			rem = append(rem, remoteFailure{
				Path: b.Segment,
				Err:  err,
			})
			continue
		}
		for _, vp := range b.Packages {
			from := vp.Path
			if len(vp.Origin) != 0 {
				from = vp.Origin
			}
			from = strings.Trim(pathos.FileTrimPrefix(b.RepoRoot, from), "/")
			dest := filepath.Join(ctx.RootDir, ctx.VendorFolder, pathos.SlashToFilepath(vp.Path))
			// Path handling with single sub-packages and differing origins need to be properly handled.
			src := filepath.Join(b.Root, from)

			// Scan go files for files that should be ignored based on tags and filenames.
			ignoreFiles, _, err := ctx.getIngoreFiles(src)
			if err != nil {
				return err
			}
			// Need to ensure we copy files from "b.Root/<import-path>" for the following command.
			ctx.CopyPackage(dest, src, b.Root, vp.Path, ignoreFiles, vp.Tree, h)
			checksum := h.Sum(nil)
			h.Reset()
			vp.ChecksumSHA1 = base64.StdEncoding.EncodeToString(checksum)
			updatedVendorFile = true
		}
	}

	// Only write a vendor file if something changes.
	if updatedVendorFile {
		err = ctx.WriteVendorFile()
		if err != nil {
			return err
		}
	}

	// Return network errors here.
	if len(rem) > 0 {
		return rem
	}

	return nil
}
