// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rewrite

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/kardianos/vendor/internal/github.com/dchest/safefile"
	"github.com/kardianos/vendor/vendorfile"
)

func writeVendorFile(root string, vf *vendorfile.File) (err error) {
	path := filepath.Join(root, internalVendor)
	perm := os.FileMode(0777)
	fi, err := os.Stat(path)
	if err == nil {
		perm = fi.Mode()
	}

	buf := &bytes.Buffer{}
	err = vf.Marshal(buf)
	if err != nil {
		return
	}
	err = safefile.WriteFile(path, buf.Bytes(), perm)
	return
}

func readVendorFile(root, vendorPath string) (*vendorfile.File, error) {
	path := filepath.Join(root, vendorPath)
	vf := &vendorfile.File{}
	f, err := os.Open(path)
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
