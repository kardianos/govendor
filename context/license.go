// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
	"github.com/pkg/errors"
)

type License struct {
	Path     string
	Filename string
	Text     string
}

type LicenseSort []License

func (list LicenseSort) Len() int {
	return len(list)
}
func (list LicenseSort) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}
func (list LicenseSort) Less(i, j int) bool {
	a, b := list[i], list[j]
	if a.Path == b.Path {
		return a.Filename < b.Filename
	}
	return a.Path < b.Path
}

type licenseSearchType byte

const (
	licensePrefix licenseSearchType = iota
	licenseSubstring
	licenseSuffix
)

type licenseSearch struct {
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
var licenses = []licenseSearch{
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

var licenseNotExt = []string{
	".go",
	".c",
	".h",
	".cpp",
	".hpp",
}

func isLicenseFile(name string) bool {
	cname := strings.ToLower(name)
	for _, X := range licenseNotExt {
		if filepath.Ext(name) == X {
			return false
		}
	}
	for _, L := range licenses {
		if L.Search.Test(cname, L.Text) {
			return true
		}
	}
	return false
}

// licenseWalk starts in a folder and searches up the folder tree
// for license like files. Found files are reported to the found function.
func licenseWalk(root, startIn string, found func(folder, name string) error) error {
	folder := startIn
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

			err = found(folder, name)
			if err != nil {
				return err
			}
		}

		if len(folder) <= len(root) {
			return nil
		}

		nextFolder := filepath.Clean(filepath.Join(folder, ".."))

		if nextFolder == folder {
			return nil
		}
		folder = nextFolder
	}
	panic("licenseFind loop limit")
}

// licenseCopy starts the search in the parent of "startIn" folder.
// Looks in all sub-folders until root is reached. The root itself is not
// searched.
func licenseCopy(root, startIn, vendorRoot, pkgPath string) error {
	addTo, _ := pathos.TrimCommonSuffix(pathos.SlashToFilepath(pkgPath), startIn)
	startIn = filepath.Clean(filepath.Join(startIn, ".."))
	return licenseWalk(root, startIn, func(folder, name string) error {
		srcPath := filepath.Join(folder, name)
		trimTo := pathos.FileTrimPrefix(getLastVendorRoot(folder), root)

		/*
			Path: "golang.org/x/tools/go/vcs"
			Root: "/tmp/govendor-cache280388238/1"
			StartIn: "/tmp/govendor-cache280388238/1/go/vcs"
			addTo: "golang.org/x/tools"
			$PROJ/vendor + addTo + pathos.FileTrimPrefix(folder, root) + "LICENSE"
		*/
		destPath := filepath.Join(vendorRoot, addTo, trimTo, name)

		// Only copy if file does not exist.
		_, err := os.Stat(srcPath)
		if err != nil {
			return errors.Errorf("Source license path doesn't exist %q", srcPath)
		}
		destDir, _ := filepath.Split(destPath)
		os.MkdirAll(destDir, 0777)
		return errors.Wrapf(copyFile(destPath, srcPath, nil), "copyFile dest=%q src=%q", destPath, srcPath)
	})
}

func getLastVendorRoot(s string) string {
	w := strings.Replace(s, "\\", "/", -1)
	ix := strings.LastIndex(w, "/vendor/")
	if ix < 0 {
		return s
	}
	return s[ix+len("/vendor"):]
}

// LicenseDiscover looks for license files in a given path.
func LicenseDiscover(root, startIn, overridePath string, list map[string]License) error {
	return licenseWalk(root, startIn, func(folder, name string) error {
		ipath := pathos.SlashToImportPath(strings.TrimPrefix(folder, root))
		if len(overridePath) > 0 {
			ipath = overridePath
		}
		if _, found := list[ipath]; found {
			return nil
		}
		p := filepath.Join(folder, name)
		text, err := ioutil.ReadFile(p)
		if err != nil {
			return fmt.Errorf("Failed to read license file %q %v", p, err)
		}
		key := path.Join(ipath, name)
		list[key] = License{
			Path:     ipath,
			Filename: name,
			Text:     string(text),
		}
		return nil
	})
}
