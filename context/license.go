// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
)

type licenseSearchType byte

const (
	licensePrefix licenseSearchType = iota
	licenseSubstring
	licenseSuffix
)

type license struct {
	Text   string
	Search licenseSearchType
}

func (t licenseSearchType) Test(filename, test string) bool {
	switch t {
	case licensePrefix:
		return strings.HasPrefix(filename, test)
	case licenseSubstring:
		return strings.Contains(filename, test)
	case licenseSuffix:
		return strings.HasSuffix(filename, test)
	}
	return false
}

type licenseTest interface {
	Test(filename, test string) bool
}

// licenses lists the filenames to copy over to the vendor folder.
var licenses = []license{
	{Text: "license", Search: licensePrefix},
	{Text: "unlicense", Search: licensePrefix},
	{Text: "copying", Search: licensePrefix},
	{Text: "copyright", Search: licensePrefix},
	{Text: "copyright", Search: licensePrefix},
	{Text: "legal", Search: licenseSubstring},
	{Text: "notice", Search: licenseSubstring},
	{Text: "disclaimer", Search: licenseSubstring},
	{Text: "patent", Search: licenseSubstring},
	{Text: "third-party", Search: licenseSubstring},
	{Text: "thirdparty", Search: licenseSubstring},
}

func isLicenseFile(name string) bool {
	cname := strings.ToLower(name)
	for _, L := range licenses {
		if L.Search.Test(cname, L.Text) {
			return true
		}
	}
	return false
}

// licenseCopy starts the search in the parent of "startIn" folder.
// Looks in all sub-folders until root is reached. The root itself is not
// searched.
func licenseCopy(root, startIn, vendorRoot string) error {
	folder := filepath.Clean(filepath.Join(startIn, ".."))
	for i := 0; i <= looplimit; i++ {
		dir, err := os.Open(folder)
		if err != nil {
			return err
		}

		fl, err := dir.Readdir(-1)
		dir.Close()
		if err != nil {
			return err
		}
		for _, fi := range fl {
			name := fi.Name()
			if name[0] == '.' {
				continue
			}
			if fi.IsDir() {
				continue
			}
			if !isLicenseFile(name) {
				continue
			}

			srcPath := filepath.Join(folder, name)
			destPath := filepath.Join(vendorRoot, pathos.FileTrimPrefix(getLastVendorRoot(folder), root), name)

			// Only copy if file does not exist.
			_, err := os.Stat(destPath)
			if err == nil {
				continue
			}

			err = copyFile(destPath, srcPath, nil)
			if err != nil {
				return err
			}
		}

		nextFolder := filepath.Clean(filepath.Join(folder, ".."))

		if nextFolder == folder {
			return nil
		}
		if pathos.FileStringEquals(root, nextFolder) {
			return nil
		}
		folder = nextFolder
	}
	panic("copyLicense loop limit")
}

func getLastVendorRoot(s string) string {
	w := strings.Replace(s, "\\", "/", -1)
	ix := strings.LastIndex(w, "/vendor/")
	if ix < 0 {
		return s
	}
	return s[ix+len("/vendor"):]
}
