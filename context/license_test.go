package context

import (
	"testing"
)

func TestGetLastVendorRoot(t *testing.T) {
	tt := []struct {
		From string
		To   string
	}{
		{
			From: "/foo/bar/bean",
			To:   "/foo/bar/bean",
		},
		{
			From: "/foo/bar/bean/vendor/fox/vax/bax",
			To:   "/fox/vax/bax",
		},
	}
	for _, item := range tt {
		got := getLastVendorRoot(item.From)
		if got != item.To {
			t.Errorf("Want: %q, Got: %q", item.To, got)
		}
	}
}
