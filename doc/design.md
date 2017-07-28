# Vendor Tool
*Vendor tool that follows the vendor-spec*


## sub-commands "fetch" and sync

```
# The "fetch" package-spec will take a version identifier.
package-spec = <path>[@[version]]

# If "package-spec" includes an "@" but not a version, the version
# is removed.

# There is no provision to specify a version (@) with a status.

# Update or add package(s). Optionally update or the version spec.
govendor fetch [-tree] [-insecure] [+status] [package-spec]`

# Example: Set a version specifier
govendor fetch github.com/kardianos/osext@v1

# Fetching an existing package (named or with status) without
# a "@" will update to HEAD if no version or the latest matching
# revision if there is a version.
govendor fetch github.com/kardianos/osext

# Example: Remove any version specifier
govendor fetch github.com/kardianos/osext@

# Sync reads the vendor.json file and updates the vendor dir to match.
# Only hits the network if files out of date.
govendor sync
```

New field in vendor-spec "checksumSHA1", sums the head content of all
files in the package listing. Only sum the first 500 KB.
"filename:<filename>,size:<number of bytes>,first N bytes".

New field in vendor-spec "version". Add the version as specified after the
"@" in the version spec. When pulling in dependencies, this field is merged
into the rest of the project vendor-spec. Conflicts are resolved by dev.

These checksums are re-computed after an update or fetch. They are
checked after a sync. They are used to determine what needs a sync and
what does not need a sync during the `govendor sync` command.

The go command wrapper commands all run `govendor sync` before
executing. Thus they ensure the dependencies are all up to date before running.
Normally no network action is needed, just a verify.
Example `govendor install +local` would first run the "sync" command
before running the go install command on all local packages.

Sub-command "fetch" will need to get any new dependencies recursivly as well.

### Version spec

Versions in the package-spec are a special prefix matching that
checks vcs branches and tags.
When "version = v1" then the following would all match: v1.4, v1.8, v1.12.
The following would *not* match: v10, foo-v1.4, v1-40

After matching acceptable labels, they must be sorted and a single label
returned. Of the following: "v1.4, v1.8, v1.12, v1.12-beta --> v1.12 would 
be chosen.

There is no precedence between branches and tags, they are both searched for
labels and sorted all together to find the correct match. In case of two
labels with exactly the same, one from branch, one from tag, choose the branch.

In the go repo: "version = release-branch.go1" would currently return
the branch: "release-branch.go1.6".

### Updates happen

Versions from dependencies are copied into the project vendor-spec file.
If there is a conflict, the developer is asked which version (or different
version) will be used.

Make interactive, but separate out asking from CLI in API.

### Setting the origin

If may be benefitial to be able to set the origin when fetching packages. A
possible design would look like this:
`package-spec = <path>[::<origin>][{/...|/^}][@[<version-spec>]]`

### Summary

Support fetching versions, but version ranges are not facy, just a spiced up
prefix specifier. Users are encuraged to track major versions.

Sub-commands add and update only ever fetch from GOPATH. Sub-commands fetch
and sync work with remote repositories.

If you don't want to check in dependencies, add "vendor/*/" in the .gitignore.

## In-progress, uncommitted flag

Might support a new flag in add/update called "-uncommitted" which bypasses
checks for commited check. However it also doesn't update the revision field
and it doesn't update the checksum field.

When `govendor status` is ran, it will show that is package is out-of-date.
