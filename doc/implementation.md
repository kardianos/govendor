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
		any letters and greater than 100. A version will be anything else.
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
 - [x] Implement the fetch command when fetch specifies a revision.
 - [x] Recursive fetch.
 - [x] Use a persistent cache ($GOPATH/src-cache) to download into (fetch and sync).
 - [x] Add fetching the version from version-spec. For git try to rely
		on standard git command, but also might look into
		"github.com/src-d/go-git" for inspecting versions remotely.
 - [x] Change fetch and sync to download into ORIGIN, not PATH.
 - [x] Record chosen version in vendor file.
 - [x] When fetching dependencies, if it is a new package, see if there exists
		another package in the same repo and use that revision and version.
 - [x] Add version info to list output.
 - [x] Add svn to internal revision finder.
 - [ ] Respect fetched repos vendor files for versions and revisions.
 - [ ] Handle version and revision conflicts.

## Vendor package with outstanding changes

 - [x] Add new flag to add/update "-uncommitted" that allows copying
		uncommitted changes over. Still check for uncommitted changegs
		and only apply `"uncommitted": true` to packages that actually do
		have uncommitted changes.

## Update Migrations

 - [x] Re-add rewrite code for migrations.
 - [ ] gb github.com/constabulary/gb
 - [ ] govend https://github.com/govend/govend
 - [x] godep (ensure it is still working)
 - [ ] gvt https://github.com/FiloSottile/gvt
 - [x] glock https://github.com/robfig/glock

## TODO

 - [x] Read os.Stdin, split by space (strings.Fields) and append to args.
 - [ ] Add "-imports" to "list" sub-command. Shows the direct dependencies of the selected packages (not the selected packages).
		`govendor list +local -imports +vendor` outputs all the vendor packages directly imported into local packages.
 - [x] Add RootPath (rootPath) to vendor.json file.
 - [x] Setup binary releases on tags.
 - [x] Fix getting the wrong revision on vendor from vendor (sync bug) https://gist.github.com/pkieltyka/502d27d02d7dc84c6202 .
 - [x] Write developer's guide to packages and vendoring document.
 - [x] Implement `govendor get` which downloads stated package into $GOPATH, puts
		all dependencies into a vendor folder, and runs `govendor install +local`.
