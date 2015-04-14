package rewrite

import (
	"os"
	"runtime"
	"testing"
)

func TestFindGOPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	orig := os.Getenv("GOPATH")
	defer os.Setenv("GOPATH", orig)

	os.Setenv("GOPATH", "/a/b")
	gopath, importPath, err := findGOPATH("/a/b/src/c/d")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(gopath)
	t.Log(importPath)
}
