# Implementation of design

*Each point in a section should be done in order they appear on the list.*

## Fetch and sync sub-commands

 - [x] Factor out govendor/run.go into govendor/runner package.
 - [x] Create a package interface to ask questions and get answers.
  * Ask multiple choice question.
  * Choice can also be "other".
  * Can validate other option.
  * If only other and no multiple choice, simply prompt.
 - [x] In govendor main package, add in a CLI interface to ask questions and get answers.
 - [x] Stub out fetch and sync sub-commands.
 - [x] Update parsing package-spec to include:
  * version-spec
  * origin
 - [x] Add fields to the vendorfile package:
  * version
  * checksumSHA1
 - [x] Have "add" and "update" start populating checksum field. Add tests.
 - [x] Add a label matcher function, return 0 or 1 labels. Add tests. 
		Each potential label should have: Source {branch, tag}, Name string.
		Do not integrate yet.
 - [x] Add a function to decide if a version is a label or revision.
		A revision will either be a valid base64 string or a number without
		any letters and greater then 100. A version will be anything else.
		A revision might be a short hash or long hash.
 - [x] Move existing commands to use the pkg-spec parser.
 - [x] Add common code to verify package's checksum, report package or folder trees that are not valid.
 - [x] Implement the sync command (sync only looks at revision field).
		Might be able to use
		https://godoc.org/golang.org/x/tools/go/vcs#Cmd.CreateAtRev .
		Will need to download into a separate directory then copy packages
		over. Will also need to use checksum to determine which packages
		need to be fetched. If no checksum is present, fetch package
		and write new checksum.
 - [ ] Implement the fetch command when fetch specifies a revision.
 - [ ] Add fetching the version from version-spec. For git try to rely
		on standard git command, but also might look into
		"github.com/src-d/go-git" for inspecting versions remotely.

## Vendor package with outstanding changes

 - [ ] Add new field "uncommitted bool" to vendorfile.
 - [ ] Add new flag to add/update "-uncommitted" that allows copying
		uncommitted changes over. Still check for uncommitted changegs
		and only apply `"uncommitted": true` to packages that actually do
		have uncommitted changes.
 - [ ] Add a new status called "+(?something?)" (maybe "outstanding"?).
		Reports all packages with uncommitted changes.
 - [ ] Create an example git hook, see what it takes to check for uncommitted
		changes. Maybe add "-should-not" to `govendor list` that returns
		non-zero if any results are returned.
