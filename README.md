## Vendor tool for Go
Follows the recommendation to use import path re-writes and avoid GOPATH
changes and go tool changes. Uses the following vendor file specification:
https://github.com/kardianos/vendor-spec .

*Work in progress*

### FAQ
Q: Why not just use godeps?

A: Godeps is a great tool. We currently use it. However it's primary designed
to work with GOPATH.
It doesn't support advanced re-writes, the re-writes don't work on Windows,
and it places vendor packages under an even longer URL. The meta-data file it
writes is decent, but needs work.

...

Q: Why vendor?

A: It's a business requirement.

...

Q: Why re-write import paths?

A: See https://github.com/kardianos/vendor-spec#a-rational-for-package-copying-and-import-path-re-writing .

...

Q: Seriously, can't we rely on github to stay around and just pin the version?

A: I've recently replaced an application written in VB6 written
sometime around 1998. It's 2015. Before I replaced it I maintained it for
several years. Some of the hardest aspects of keeping it running was finding
and installing the external COM libraries and DLLs. The companies that offered
the components are still around, but they don't offer the components the
program used anymore. This wouldn't be nearly the issue it was if the components
were kept along with the rest of the source code.

This is not a unique story. Companies that are more then a few years old *will*
have legacy programs. Libraries *will* drop off the face of the Earth.
Maintenance programmers will not be happy when they discover
the component they need disappeared 5 years ago.

...

Q: What if we just copied the vendor code to the repository, then copy them to
the GOPATH locations after a fetch? I don't like the idea of import re-writes.

A: If you setup one project per GOPATH it would mostly work ("go get"
wouldn't work). However, if two projects that shared the same GOPATH
relied on the same vendor package, but different versions, then one vendor package
would clobber the other vendor package. This approach doesn't work when scaling
a company code base.
