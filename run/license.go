// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/kardianos/govendor/context"
)

var defaultLicenseTemplate = `{{range $index, $t := .}}{{if ne $index 0}}~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
{{end}}{{.Filename}} - {{.Path}}
{{.Text}}{{end}}
`

func License(w io.Writer, subCmdArgs []string) (HelpMessage, error) {
	flags := flag.NewFlagSet("license", flag.ContinueOnError)
	flags.SetOutput(nullWriter{})
	outputFilename := flags.String("o", "", "output")
	templateFilename := flags.String("template", "", "custom template file")
	err := flags.Parse(subCmdArgs)
	if err != nil {
		return MsgLicense, err
	}
	args := flags.Args()

	templateText := defaultLicenseTemplate
	if len(*templateFilename) > 0 {
		text, err := ioutil.ReadFile(*templateFilename)
		if err != nil {
			return MsgNone, err
		}
		templateText = string(text)
	}
	t, err := template.New("").Parse(templateText)
	if err != nil {
		return MsgNone, err
	}
	output := w
	if len(*outputFilename) > 0 {
		f, err := os.Create(*outputFilename)
		if err != nil {
			return MsgNone, err
		}
		defer f.Close()
		output = f
	}

	ctx, err := context.NewContextWD(context.RootVendorOrWD)
	if err != nil {
		return checkNewContextError(err)
	}
	cgp, err := currentGoPath(ctx)
	if err != nil {
		return MsgNone, err
	}
	f, err := parseFilter(cgp, args)
	if err != nil {
		return MsgLicense, err
	}
	if len(f.Import) == 0 {
		insertListToAllNot(&f.Status, normal)
	} else {
		insertListToAllNot(&f.Status, all)
	}

	list, err := ctx.Status()
	if err != nil {
		return MsgNone, err
	}
	var licenseList context.LicenseSort
	var lmap = make(map[string]context.License, 9)

	err = context.LicenseDiscover(filepath.Clean(filepath.Join(ctx.Goroot, "..")), ctx.Goroot, " go", lmap)
	if err != nil {
		return MsgNone, fmt.Errorf("Failed to discover license for Go %q %v", ctx.Goroot, err)
	}

	for _, item := range list {
		if f.HasStatus(item) == false {
			continue
		}
		if len(f.Import) != 0 && f.FindImport(item) == nil {
			continue
		}
		err = context.LicenseDiscover(ctx.RootGopath, filepath.Join(ctx.RootGopath, item.Local), "", lmap)
		if err != nil {
			return MsgNone, fmt.Errorf("Failed to discover license for %q %v", item.Local, err)
		}
	}
	for _, l := range lmap {
		licenseList = append(licenseList, l)
	}
	sort.Sort(licenseList)

	return MsgNone, t.Execute(output, licenseList)
}
