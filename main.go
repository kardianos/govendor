// vendor tool to copy external source code to the local repository.
package main

// github.com/dchest/safefile

import (
	"github.com/dchest/safefile"
	kp "gopkg.in/alecthomas/kingpin.v1"
	"strings"
)

var (
	initCmd = kp.Command("init", "create a new internal folder and vendor file")

	add       = kp.Command("add", "vendor a new package")
	addImport = add.Arg("import", "import path").Required().String()

	update       = kp.Command("update", "update an existing package")
	updateImport = update.Arg("import", "import path").Required().String()

	remove       = kp.Command("remove", "un-vendor existing package")
	removeImport = remove.Arg("import", "import path").Required().String()

	list       = kp.Command("list", "list imports for packages")
	listStatus = list.Arg("status", "[e]xternal, [i]nternal, [u]nused").String()
)

func main() {
	switch kp.Parse() {
	case "init":
	case "add":
	case "update":
	case "remove":
	case "list":
		switch {
		case len(*listStatus) == 0:
		case strings.HasPrefix("external", *listStatus):
		case strings.HasPrefix("internal", *listStatus):
		case strings.HasPrefix("unused", *listStatus):
		default:
			kp.UsageErrorf("Unknown status to print: %s", *listStatus)
		}
	default:
		kp.Usage()
	}
}

func write() error {
	return safefile.WriteFile("foo.go", []byte{}, 0777)
}
