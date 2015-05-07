## Vendor tool for Go
Follows the recommendation to use import path re-writes and avoid GOPATH
changes and go tool changes. Uses the following vendor file specification:
https://github.com/kardianos/vendor-spec . This vendor tool aims to aid in the establishment a final vendor file
specification and be a useful tool.

### What this vendor tool features:
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
vendor init
vendor list [status]
vendor {add, update, remove} [-status] <import-path or status>

	init
		create a vendor file if it does not exist.
	
	add
		copy one or more packages into the internal folder and re-write paths.
	
	update
		update one or more packages from GOPATH into the internal folder.
	
	remove
		remove one or more packages from the internal folder and re-write packages to vendor paths.

Expanding "..."
	A package import path may be expanded to other paths that
	show up in "vendor list" be ending the "import-path" with "...".
	NOTE: this uses the import tree from "vendor list" and NOT the file system.

Status list:
	external - package does not share root path
	internal - in vendor file; copied locally
	unused - the package has been copied locally, but isn't used
	local - shares the root path and is not a vendor package
	missing - referenced but not found in GOROOT or GOPATH
	std - standard library package

Status can be referenced by their initial letters.
	"st" == "std"
	"e" == "external"
	
Example:
	vendor add github.com/kardianos/osext
	vendor update github.com/kardianos/...
	vendor add -status external
	vendor update -status ext
	vendor remove -status internal
```

For example "vendor list external" will tell you if there are any packages which
live outside the project. The stats "duplicate" is also planned as well, in case
someone references a package in GOPATH that is already vendored. A pre-commit
hook "vendor list ext dup" and only allow commit if it returns empty.

Before doing the commands add, update, or remove, all package dependencies are
discovered. The commands will only act on discovered dependencies. Commands will
never alter packages outside the project directory.

When doing this analysis it also records the both the current location (relative
to the GOPATH env) and also any noted original vendor path found in the
"internal/vendor.json" file. Not only does it use the current projects vendor
file, but it also searches each external dependency for a vendor file.
This is why the go team wanted to establish a standard location, name, structure,
and semantics for such a vendor file.

When copying packages locally, vendored dependencies of dependencies are always
copied to the "top" level in the internal package, so it also gets rid of the
extra package layers. For an example of what a I mean here look at
https://github.com/kardianos/vendor-spec#example how the context package is
venored.

The project must be within a GOPATH.

### Examples
```
# Add external packages.
vendor add -status external

# Add a specific package.
vendor add github.com/kardianos/osext

# Update vendor packages.
vendor update -status internal

# Revert back to normal GOPATH packages.
vendor remove -status internal

# List package.
vendor list
```

### Status
Tasks that are planned.
 * Add a duplicate status in cases where a person adds the wrong path.
 * Add flag "-short" to the init command and "Short" field in the vendor file.
    Keeps paths unique, but removes extra folders.
 * Speed up working with multiple packages at once by altering the rewrite API.

Tasks that are not planned at this time, but could be done in the future.
 * "Transactional" re-writes (rename temp files all at once).
 * Command to check for newer versions, either in GOPATH or remote repo.
 * A -v verbose flag to print what it is doing.

### FAQ
Q: Why not use an existing tool?

A: I do not know of an existing tool that works on all platforms and
is designed from the ground up to support vendoring and import re-writes.
I also wanted a test bed to test the proposed vendor-spec.

------------

Q: Why this design and not X?

A: See https://github.com/kardianos/vendor-spec#faq .

------------

Q: Why do we need a standard vendor file format?

A: So many different tools, such as godoc.org and vendoring tools, can correctly
identify vendor packages.
