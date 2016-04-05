// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package migrate

import (
	"errors"
	"path/filepath"
)

func init() {
	register("gb", sysGb{})
}

type sysGb struct{}

func (sys sysGb) Check(root string) (system, error) {
	if hasDirs(root, "src", filepath.Join("vendor", "src")) {
		return sys, nil
	}
	return nil, nil
}
func (sysGb) Migrate(root string) error {
	// Move files from "src" to first GOPATH.
	// Move vendor files from "vendor/src" to "vendor".
	// Translate "vendor/manifest" to vendor.json file.
	return errors.New("Migrate gb not implemented")
}
