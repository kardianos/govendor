## Vendor tool for Go
Follows the recommendation to use import path re-writes and avoid GOPATH
changes and go tool changes. Uses the following vendor file specification:
https://github.com/kardianos/vendor-spec .

### Status
The vendor tool will copy packages locally and re-write the imports.
It will also record the imports used, both from where they are from
and where they are stored. For git and mercurial repos it will also
record the version and time of the source.

Tasks that are planned:
 * Proper inspection of source vendor files.

Tasks that are NOT planned at this time.
 * "Transactional" re-writes (rename temp files all at once).
 * CLI convenience tools such as add all external or remove all unsued.
 * CLI checks that only shows what it would do if it ran.
 * CLI ouput of diffs to stdout, no file re-writes.

### FAQ
Q: Why not use an existing tool?

A: I do not know of an existing tool that works on all platforms and
is designed from the ground up to support vendoring and import re-writes.
I also wanted a test bed to test the proposed vendor-spec.

------------

Q: Why this design and not X?

A: See https://github.com/kardianos/vendor-spec#faq .
