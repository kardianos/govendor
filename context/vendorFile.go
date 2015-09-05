// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/kardianos/govendor/internal/github.com/dchest/safefile"
	"github.com/kardianos/govendor/vendorfile"
)

// WriteVendorFile writes the current vendor file to the context location.
func (ctx *Context) WriteVendorFile() (err error) {
	perm := os.FileMode(0666)
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
	return vf, nil
}
