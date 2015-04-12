## Vendor Tool
*Vendor tool that follows the vendor-spec*

Tool field: github.com/kardianos/vendor

Commands:
 * init : create an internal directory and vendor file. Error if they both exist.
 * add <import> : Vendor the import from GOPATH.
 * update <import> : Update an existing vendor code from GOPATH.
 * remove <import> : Remove the vendor source and change import paths back to GOPATH.
 * list [status] : list all vendor packages, both copied and external. If not
    status is given then prefix with a letter and a space depending on the
	package status. If a status is given then omit the status per line and only
	show that status. Don’t do exact match on status, check for prefix match.
   - “e” package is external and has NOT been copied, “external”.
   - “i” package is internal and has been copied, “internal”.
   - “u” package is internal, but is not referenced. “unused”.

Invoke the command in the directory that contains the “internal” directory to vendor into.

Get the version and version time by invoking version control commands sequentially until one works in this order: git, hg, bzr, svn.

Features that may be added later:
 * Invoke tool anywhere in directory tree under root directory that contains “internal” directory with vendor file.
 * Auto add and remove packages based on use.
 * Check for updates to packages remotely.
 * Update packages from remote source.
 * Update to specific version.
 * Move sub-vendor packages to the top level.
 * Recognize standard library vendors add “Branch” flag. maybe.

