package rewrite

import (
	"os"
	"path/filepath"
	"strings"
)

var srcDir = string(filepath.Separator) + "src" + string(filepath.Separator)

func findGOPATH(folder string) (gopath, importPath string, err error) {
	folder = filepath.Clean(folder)
	all := os.Getenv("GOPATH")
	list := strings.Split(all, string(filepath.ListSeparator))
	for _, item := range list {
		gopath = filepath.Clean(item)
		if strings.HasPrefix(folder, gopath) {
			importPath = strings.TrimPrefix(folder[len(gopath):], srcDir)
			return gopath, importPath, nil
		}
	}
	return "", "", ErrNotInGOPATH
}

func findRoot(folder string) (root string, err error) {
	for {
		test := filepath.Join(folder, internalVendor)
		_, err := os.Stat(test)
		if os.IsNotExist(err) == false {
			return folder, nil
		}
		nextFolder := filepath.Clean(filepath.Join(folder, ".."))

		// Check for root folder.
		// TODO: Also check for GOPATH root.
		if nextFolder == folder {
			return "", ErrMissingVendorFile
		}
		folder = nextFolder
	}
}
