## Vendor tool for Go
Supports the GO15VENDOREXPERIMENT environment flag. When set imports are not
rewritten and are copied into the "vendor" folder.

Follows the recommendation to use import path re-writes and avoid GOPATH
changes and go tool changes. Uses the following vendor file specification:
https://github.com/kardianos/vendor-spec . This vendor tool aims to aid in the
establishment a final vendor file specification and be a useful tool.

### What this vendor tool features:
 * flattens dependency tree to single level
 * Can ignore test files and other build tags
 * Tested cross platform support
 * Import path re-writes
 * Package import comment removal
 * Inspection of the current state package locations
 * Handles packages, not directory trees
 * Handles the simple and complex cases
 * Use "..." to also handle packages in sub-folders
 * Handle packages based on their status

### Usage
```
govendor: copy go packages locally and optionally re-write imports.
govendor init
govendor list [-v] [+status] [import-path-filter]
govendor {add, update, remove} [-n] [+status] [import-path-filter]
govendor migrate [auto, godep, internal]

	init
		create a vendor file if it does not exist.
	
	add
		copy one or more packages into the internal folder and re-write paths.
	
	update
		update one or more packages from GOPATH into the internal folder.
	
	remove
		remove one or more packages from the internal folder and re-write packages to vendor paths.

	migrate
		change from a one schema to use the vendor folder.

Expanding "..."
	A package import path may be expanded to other paths that
	show up in "govendor list" be ending the "import-path" with "...".
	NOTE: this uses the import tree from "vendor list" and NOT the file system.

Flags
	-n		print actions but do not run them

Status list:
	external - package does not share root path
	vendor - vendor folder; copied locally
	unused - the package has been copied locally, but isn't used
	local - shares the root path and is not a vendor package
	missing - referenced but not found in GOROOT or GOPATH
	std - standard library package
	program - package is a main package
	---
	all - all of the above status

Status can be referenced by their initial letters.
	"st" == "std"
	"e" == "external"

Ignoring files with build tags:
	The "vendor.json" file contains a string field named "ignore".
	It may contain a space separated list of build tags to ignore when
	listing and copying files. By default the init command adds the
	the "test" tag to the ignore list.
	
Example:
	govendor add github.com/kardianos/osext
	govendor update github.com/kardianos/...
	govendor add +external
	govendor update +ven github.com/company/project/... bitbucket.org/user/pkg
	govendor remove +vendor
	govendor list +ext +std

To opt use the standard vendor directory:
set GO15VENDOREXPERIMENT=1

When GO15VENDOREXPERIMENT=1 imports are copied to the vendor directory without
rewriting their import paths.
```

For example "govendor list +external" will tell you if there are any packages which
live outside the project.

Before doing the commands add, update, or remove, all package dependencies are
discovered. The commands will only act on discovered dependencies. Commands will
never alter packages outside the project directory.

The GO15VENDOREXPERIMENT=1 flag will be honored. If present the vendor file will
be placed into the project root and vendor package will be placed into "vendor"
folder. Import paths will not be rewriten in this case.

When copying packages locally, vendored dependencies of dependencies are always
copied to the "top" level in the internal package, so it also gets rid of the
extra package layers.

The project must be within a GOPATH.

### Examples
```
# Add external packages.
govendor add +external

# Add a specific package.
govendor add github.com/kardianos/osext

# Update vendor packages.
govendor update +vendor

# Revert back to normal GOPATH packages.
govendor remove +vendor

# List package.
govendor list
```

### Ignoring build tags
Ignoring build tags is opt-out and is designed to be the opposite of the build
file directives which are opt-in when specified. Typically a developer will
want to support cross platform builds, but selectively opt out of tags, tests,
and architectures as desired.

To ignore additional tags edit the "vendor.json" file and add tag to the vendor
"ignore" file field. The field uses spaces to separate tags to ignore.
For example the following will ignore both test and appengine files.
```
{
	"ignore": "test appengine",
}
```
