// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	ros "os"
	"path/filepath"
	"strings"

	"github.com/dchest/safefile"
	"github.com/kardianos/govendor/vendorfile"

	os "github.com/kardianos/govendor/internal/vos"
)

// WriteVendorFile writes the current vendor file to the context location.
func (ctx *Context) WriteVendorFile() (err error) {
	perm := ros.FileMode(0666)
	fi, err := os.Stat(ctx.VendorFilePath)
	if err == nil {
		perm = fi.Mode()
	}

	ctx.VendorFile.RootPath = ctx.RootImportPath

	buf := &bytes.Buffer{}
	err = ctx.VendorFile.Marshal(buf)
	if err != nil {
		return
	}
	err = buf.WriteByte('\n')
	if err != nil {
		return
	}
	dir, _ := filepath.Split(ctx.VendorFilePath)
	err = os.MkdirAll(dir, 0777)
	if err != nil {
		return
	}

	for i := range ctx.VendorFile.Package {
		vp := ctx.VendorFile.Package[i]
		vp.Add = false
	}

	err = safefile.WriteFile(ctx.VendorFilePath, buf.Bytes(), perm)
	if err == nil {
		for _, vp := range ctx.VendorFile.Package {
			vp.Add = false
		}
	}

	return
}

func readVendorFile(vendorRoot, vendorFilePath string) (*vendorfile.File, error) {
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
	// Remove any existing origin field if the prefix matches the
	// context package root. This fixes a previous bug introduced in the file,
	// that is now fixed.
	for _, row := range vf.Package {
		row.Origin = strings.TrimPrefix(row.Origin, vendorRoot)
	}

	return vf, nil
}
