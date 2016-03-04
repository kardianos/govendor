// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
	"github.com/kardianos/govendor/vendorfile"
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

func (ctx *Context) Sync() error {
	outOfDate, err := ctx.VerifyVendor()
	if err != nil {
		return fmt.Errorf("Failed to verify checksums: %v", err)
	}
	// TODO (DT): Create temporary folder to download files from.
	for _, vp := range outOfDate {
		_ = vp
		// TODO (DT): Bundle packages together that have the same revision and share at least one root segment.
	}
	// TODO (DT): For each bundle, download into the temporary folder.
	// Each bundle should go into their own root folder: <temp-dir>/<bundle-index/<package-path>

	// TODO (DT): Copy each vendor file (listed in each bundle) into the vendor folder.
	// Ensure we hash the values and update the vendor file package listing.

	return ctx.WriteVendorFile()
}
