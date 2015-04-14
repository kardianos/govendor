package rewrite

import (
	"os"
	"path/filepath"
)

func findRoot(folder string) (root string, err error) {
	for {
		test := filepath.Join(folder, internalVendor)
		_, err := os.Stat(test)
		if os.IsNotExist(err) == false {
			return folder, nil
		}
		nextFolder := filepath.Clean(filepath.Join(folder, ".."))

		// Check for root folder.
		if nextFolder == folder {
			return "", ErrMissingVendorFile
		}
		folder = nextFolder
	}
}
