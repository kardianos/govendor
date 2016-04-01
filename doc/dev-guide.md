# Go Developer Guide

 * Always check-in the "vendor/vendor.json" file
 * Do not check-in vendor sources if you expect an external package to import it.
 * Do check-in vendor sources for main packages.
 * Main packages should vendor their own common dependencies.
 * Release with semver, do not break compatibility within a major version.
 * If you choose to release with tags or branches, keep them up-to-date.

## Always check in the "vendor/vendor.json" file

You can add the ignore rule `vendor/*/` to ignore source files.
This way a consumer of your package has a chance at reproducing your package tests
if something appears to break later.


## Do not check-in vendor sources if you expect an external package to import it

The way `go get` currently works is to download repositories into $GOPATH without
modification. This is fine, but if a "library" repository contains a vendor folder,
it is likely it will be unable to be used unless the consumers also vendor
the dependencies.

## Do check-in vendor sources for main packages

Reproducible builds are important. Repositories can and do disappear.
Pull them into your own repository under the vendor folder. Your
maintainers 15 years from now will thank you.

## Release with semver, do not break compatibility within a major version

Release with semver: `v<major>.<minor>.<patch>[-<pre-release>]`.

 * Increment major: break existing API.
 * Increment minor: add API, no breaks to existing API.
 * Increment patch: no api changes, bug fixes.
 * Tag pre-release: use to prepare for a later release with the same numbers.

`govendor` will also handle path prefixes, for example `ssh-v1.0.2-beta1`
can be used in govendor with `govendor fetch my/util/ssh@ssh-v1`.


## If you choose to release with tags or branches, keep them up-to-date

It is completely plausible to do work in branches, then only merge the branch
to master when the branch is stable. This effectively releases software.

If you choose to also tag revisions or release to a dedicated branch (like a
branch named "v1"), ensure HEAD never gets too far ahead of the release.
If it does so it renders the release obsolete and it stops being used.

