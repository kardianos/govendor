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
Tasks that are not planned at this time, but could be done in the future.
 * Speed up working with multiple packages at once by altering the rewrite API.
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
