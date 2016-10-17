# govendor whitepaper

`go get -u github.com/kardianos/govendor`

## Primary Workflows
 * Lock in existing revisions from GOPATH. Check vendor source into source control.
	Use `govendor add/update` to update from GOPATH.
 * Fetch directly from network locations into vendor folder. Check in vendor source
	into source control. Use `govendor fetch` to update from remote.
 * Fetch directly from network location into vendor folder. Do not check vendor
	source into source control. Run `govendor sync` after pull or clone to get
	correct vendor packages. Add `vendor/*/` in your vcs ignore file.

## Secondary Workflows

 * Working on pkg A, want to test it in cmd B (in a different repo) before checking
	it in. Run `govendor add pkgA -uncommitted` in cmd B project. This will update
	the files, but not update the checksum or revision.
	This way `govendor sync` will revert the files and `govendor status` will show
	this package as out-of-date.
 * Fetch from a repository fork. The package-spec used in the command line can
	specify an origin: `<import-path>[::<origin-path]` for examples
	`github.com/bob/stuff/…::github.com/alice/stuff` will record the packages as
	from bob, but record the origin from alice.
 * Specify a version range and update within that range. Use `govendor fetch` to
	update to the latest revision while still remaining on the specified version
	range. To specify a specific version, don’t edit a file, just use the command
	line `govendor fetch github.com/alice/watch/…@v1`. Version ranges specify a
	major version and an optional minimum minor and patch versions. It is
	recommended to just specify a major version.

## Always-on Features

 * Packages are always copied into the top most vendor folder. That is, they are 
	"flattened" into a single level.
 * Cross platform. Tested on Linux, Windows, OS X.
 * Introduces the concept of "status". To vendor all dependencies that currently
	live in $GOPATH, run `govendor add +external`. To remove all dependencies in
	the vendor folder run `govendor remove +vendor`.
 * Side steps `./…` debate regarding vendor folder by introducing a concept of
	"status". Running `govendor test +local` will test all non-local packages.

## Optional Features

 * You can choose to copy only packages you use into the vendor folder. You can
	also choose to copy the directory and all subfolders under it with path suffix
	"/^" or "-tree" command argument.

## Package Developers

 * Package developers may use govendor. They may choose to just check in the vendor
	metadata file or they may choose to copy everything.
 * Package developers are encouraged to tag releases using semver:
	`[<optional-prefix>-]v<major-number>.<minor-number>.<patch-number>[-<pre-release-label>]`
	examples: `ssh-v1.0.1-alpha`, `ssh-v.11.0`, `v2.0.1`.

## Unsupported Features

 * VCS specific features such as git-submodules. These do not play nice with
	different version control systems in the large and don’t work well when
	flattening the vendor directory.
 * Complex version ranges, such as upper bounding version ranges or ignoring a
	specific version.
 * Use of a separate “design file” just containing direct dependencies of the
	project with versions. Use the tool instead and let it write what you required
	down.

## Rejected Arguments

 * Everyone else does “X”. Thus we should too.
   - Other languages don’t directly link the import path to the physical remote
	path. We’ve paid for it, we should use it. Thus our dependency system will look
		slightly different, though it should still be recognizable.
   - I’m a go developer. If I can take the time to learn a language, I can take the
	time to learn a slightly different tool.
 *  Repositories should be the unit of vendoring, not packages.
   - In Go, the developer works on packages. Repos are largely transparent to
	import paths and should remain transparent.
   - Should there be licensing or packaging concerns, developers have the option of
	choosing to vendor repo trees.
 * We should use a separate “design file” that just contains a list of direct
	dependencies and their versions.
   - In go, it is incredibly easy to dynamically derive and list what is used
	directly by a project and what is transitively required.
   - This isn’t a broken design, but requiring a human to do what a machine can
	easily do seems like a poor design.
