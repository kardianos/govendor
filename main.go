// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// vendor tool to copy external source code to the local repository.
package main

import (
	"fmt"
	"os"
)

func main() {
	printHelp, err := run(os.Stdout, os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	if printHelp {
		fmt.Fprint(os.Stderr, help)
	}
	if printHelp || err != nil {
		os.Exit(1)
	}
}
