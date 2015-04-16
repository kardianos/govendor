## Vendor tool for Go
Follows the recommendation to use import path re-writes and avoid GOPATH
changes and go tool changes. Uses the following vendor file specification:
https://github.com/kardianos/vendor-spec . This vendor tool aims to aid in the establishment a final vendor file
specification and be a useful tool.

Tasks that are planned:
 * Proper inspection of source vendor files (see rewrite/path.go findLocalImportPath).
 * Remove the package "import" comment from copied re-written packages.
    This comment will be enforced starting in go1.5 for user packages.

Tasks that are not planned at this time, but could be done in the future.
 * Speed up working with multiple packages at once by altering the rewrite API.
 * "Transactional" re-writes (rename temp files all at once).
 * Command to check for newer versions, either in GOPATH or remote repo.

### FAQ
Q: Why not use an existing tool?

A: I do not know of an existing tool that works on all platforms and
is designed from the ground up to support vendoring and import re-writes.
I also wanted a test bed to test the proposed vendor-spec.

------------

Q: Why this design and not X?

A: See https://github.com/kardianos/vendor-spec#faq .
