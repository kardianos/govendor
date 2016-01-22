## Vendor tool for Go
Imports are copied into the "vendor" folder.

Uses the following vendor file specification:
https://github.com/kardianos/vendor-spec . This vendor tool aims to aid in the
establishment a final vendor file specification and be a useful tool.

If you require import path rewrites checkout the "rewrite" branch archived for
that purpose.

### What this vendor tool features:
 * flattens dependency tree to single level
 * Can ignore test files and other build tags
 * Tested cross platform support
 * Package import comment removal
 * Inspection of the current state package locations
 * Handles packages, not directory trees
 * Handles the simple and complex cases
 * Use "..." to also handle packages in sub-folders
 * Handle packages based on their status

### Usage
```
govendor: copy go packages locally. Uses vendor folder.
govendor init
	Creates a vendor file if it does not exist.

govendor list [options] [+<status>] [import-path-filter]
	List all dependencies and packages in folder tree.
	Options:
		-v           verbose listing, show dependencies of each package
		-no-status   do not prefix status to list, package names only

govendor {add, update, remove} [options] [+status] [import-path-filter]
	add    - Copy one or more packages into the vendor folder.
	update - Update one or more packages from GOPATH into the vendor folder.
	remove - Remove one or more packages from the vendor folder.
	Options:
		-n           dry run and print actions that would be taken
		-tree        copy package(s) and all sub-folders under each package
		
		The following may be replaced with something else in the future.
		-short       if conflict, take short path 
		-long        if conflict, take long path

govendor migrate [auto, godep, internal]
	Change from a one schema to use the vendor folder. Default to auto detect.

Expanding "..."
	A package import path may be expanded to other paths that
	show up in "govendor list" be ending the "import-path" with "...".
	NOTE: this uses the import tree from "vendor list" and NOT the file system.

Flags
	-n		print actions but do not run them
	-short	chooses the shorter path in case of conflict
	-long	chooses the longer path in case of conflict
	
"import-path-filter" arguements:
	May be a literal individual package:
		github.com/user/supercool
		github.com/user/supercool/anotherpkg
	
	Match on any exising Go package that the project uses under "supercool"
		github.com/user/supercool/...
		
	Match the package "supercool" and also copy all sub-folders.
	Will copy non-Go files and Go packages that aren't used.
		github.com/user/supercool/^
	
	Same as specifying:
	-tree github.com/user/supercool

Status list used in "+<status>" arguments:
	external - package does not share root path
	vendor - vendor folder; copied locally
	unused - the package has been copied locally, but isn't used
	local - shares the root path and is not a vendor package
	missing - referenced but not found in GOROOT or GOPATH
	std - standard library package
	program - package is a main package
	---
	outside - external + missing
	all - all of the above status

Status can be referenced by their initial letters.
	"st" == "std"
	"e" == "external"

Ignoring files with build tags:
	The "vendor.json" file contains a string field named "ignore".
	It may contain a space separated list of build tags to ignore when
	listing and copying files. By default the init command adds the
	the "test" tag to the ignore list.

If using go1.5, ensure you set GO15VENDOREXPERIMENT=1
```

For example "govendor list +external" will tell you if there are any packages which
live outside the project.

Before doing the commands add, update, or remove, all package dependencies are
discovered. The commands will only act on discovered dependencies. Commands will
never alter packages outside the project directory.

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

# Add a package tree.
govendor add -tree github.com/mattn/go-sqlite3
    or
govendor add github.com/mattn/go-sqlite3/^

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
