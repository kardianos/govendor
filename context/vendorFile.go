// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/kardianos/govendor/internal/github.com/dchest/safefile"
	"github.com/kardianos/govendor/internal/pathos"
	"github.com/kardianos/govendor/vendorfile"
)

// WriteVendorFile writes the current vendor file to the context location.
func (ctx *Context) WriteVendorFile() (err error) {
	perm := os.FileMode(0777)
	fi, err := os.Stat(ctx.VendorFilePath)
	if err == nil {
		perm = fi.Mode()
	}

	buf := &bytes.Buffer{}
	err = ctx.VendorFile.Marshal(buf)
	if err != nil {
		return
	}
	dir, _ := filepath.Split(ctx.VendorFilePath)
	err = os.MkdirAll(dir, 0777)
	if err != nil {
		return
	}
	err = safefile.WriteFile(ctx.VendorFilePath, buf.Bytes(), perm)
	return
}

func readVendorFile(vendorFilePath string) (*vendorfile.File, error) {
	vf := &vendorfile.File{}
	f, err := os.Open(vendorFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	err = vf.Unmarshal(f)
	if err != nil {
		return nil, err
	}
	// Determine if local field is relative to GOPATH or vendor file.
	// Change to relative to vendor file as needed.
	folder, _ := filepath.Split(vendorFilePath)
	relToFile := 0
	relToGOPATH := 0
	for _, pkg := range vf.Package {
		p := filepath.Join(folder, pathos.SlashToFilepath(pkg.Local))
		_, err := os.Stat(p)
		if os.IsNotExist(err) {
			relToGOPATH++
			continue
		}
		relToFile++
	}
	if relToFile > relToGOPATH || len(vf.Package) == 0 {
		return vf, nil
	}

	gopathList := strings.Split(os.Getenv("GOPATH"), string(os.PathListSeparator))
	gopath := ""
	for _, gp := range gopathList {
		if pathos.FileHasPrefix(folder, gp) {
			gopath = gp
			break
		}
	}
	if len(gopath) == 0 {
		return vf, nil
	}
	prefix := pathos.FileTrimPrefix(folder, filepath.Join(gopath, "src"))
	if len(prefix) > 0 {
		prefix = prefix[1:]
	}
	for _, pkg := range vf.Package {
		pkg.Local = strings.TrimPrefix(pkg.Local, prefix)
	}

	return vf, nil
}
