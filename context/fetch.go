// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

type fetcher struct{}

func newFetcher() (*fetcher, error) {
	return &fetcher{}, nil
}

// Fetch repo locally if not already present.
// Transform the fetch op into a copy op.
func (f *fetcher) op(op *Operation) error {
	op.Type = OpCopy
	src := op.Src
	// TODO (DT): set op.Src to download dir.
	op.Src = "true-dir-src from download dir"

	_ = src

	// TODO (DT) Once downloaded, be sure to set the revision and revisionTime
	// in the vendor file package.

	return nil
}
