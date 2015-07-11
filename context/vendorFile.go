// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	"os"

	"github.com/kardianos/vendor/internal/github.com/dchest/safefile"
	"github.com/kardianos/vendor/vendorfile"
)

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
