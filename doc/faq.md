# govendor FAQ

#### Q: How do I test only my packages?
A: Run `govendor test +local`.

#### Q: How do I build install all my vendor packages?
A: Run `govendor install +vendor,^program`.

#### Q: How do I pull all my dependencies from network remotes?
A: Run `govendor fetch +out`.

#### Q: I have a working program with dependencies in $GOPATH. I want to vendor now.
A: Run `govendor add +external`.

#### Q: I have copied dependencies into "vendor". I want to update from $GOPATH.
A: Run `govendor update +vendor`.

#### Q: I'm getting missing packages from `appengine` but I don't care about `appengine`. How do I ignore these packages?
A: Edit the `vendor/vendor.json` file. Update the "ignore" field to include "appengine".
If you are already ignoring tests, it will look like: `"ignore": "test appengine",`.

#### Q: I have modified a package in $GOPATH and I want to try the changes in vendor without committing them.
A: Run `govendor update -uncommitted <updated-package-import-path>`.

#### Q: I've forked a package and I haven't upstreamed the changes yet. What should I do?
A: Assuming you've pushed your changes to an accessable repository, run 
`govendor fetch github.com/normal/pkg::github.com/myfork/pkg`. This will
fetch from "myfork" but place package in "normal".

#### Q: I have C files or HTML resources in sub-folders. How do I ensure they are copied as well?
A: Run either `govendor fetch github.com/dep/pkg/^` or `govendor add github.com/dep/pkg/^`.
This is the same as using the `-tree` argument.

#### Q: How do I prevent vendor source from being checked in?
A: Add `vendor/*/` to your ignore file.

#### Q: How do I populate the vendor folder if it has not been checked in?
A: Run `govendor sync`.
