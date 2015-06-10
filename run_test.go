// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Fatal("TODO: Create test.")
	// 1. Setup on-disk GOPATH and env var.
	// 2. Populate GOPATH with example packages.
	// 3. Set current working directory to project root.
	// 4. Run vendor command(s).
	// 5. Inspect project workspace for desired result.

}

func testCmd(t *testing.T, expectedOutput, argLine string) {
	output := &bytes.Buffer{}
	args := append([]string{"testing"}, strings.Split(argLine, " ")...)
	printHelp, err := run(output, args)
	if err != nil {
		t.Fatal(err)
	}
	if printHelp == true {
		t.Fatal("Printed help")
	}
	if output.String() != expectedOutput {
		t.Fatal("Got", output.String())
	}
}
