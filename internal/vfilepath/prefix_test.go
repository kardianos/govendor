package vfilepath

import "testing"

func TestHasPrefixDirTrue(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
	}{
		{
			path:   "/",
			prefix: "/",
		},
		{
			path:   "/foo",
			prefix: "/",
		},
		{
			path:   "/foo",
			prefix: "/foo",
		},
		{
			path:   "/foo/bar",
			prefix: "/foo",
		},
		{
			path:   "foo/bar",
			prefix: "foo",
		},
	}

	for _, test := range tests {
		if !HasPrefixDir(test.path, test.prefix) {
			t.Errorf("%s should have %s as prefix", test.path, test.prefix)
		}
	}
}

func TestHasPrefixDirFalse(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
	}{
		{
			path:   "/",
			prefix: "/foo",
		},
		{
			path:   "/foo-bar",
			prefix: "/foo",
		},
	}

	for _, test := range tests {
		if HasPrefixDir(test.path, test.prefix) {
			t.Errorf("%s should not have %s as prefix", test.path, test.prefix)
		}
	}
}
