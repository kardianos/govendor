# Kingpin - A Go (golang) command line and flag parser [![Build Status](https://travis-ci.org/alecthomas/kingpin.png)](https://travis-ci.org/alecthomas/kingpin)

## Features

- POSIX-style short flag combining.
- Parsed, type-safe flags.
- Parsed, type-safe positional arguments.
- Support for required flags and required positional arguments
- Callbacks per command, flag and argument.
- Help output that isn't as ugly as sin.

## Versions

Kingpin uses [gopkg.in](https://gopkg.in/alecthomas/kingpin.v1) for versioning.

Usage:

```go
import "gopkg.in/alecthomas/kingpin.v1"
```

## Changes

- *2015-01-23* -- Stable v1.3.4 release.
    - Support "--" for separating flags from positional arguments.
    - Support loading flags from files (ParseWithFileExpansion()). Use @FILE as an argument.
    - Add post-app and post-cmd validation hooks. This allows arbitrary validation to be added.
    - A bunch of improvements to help usage and formatting.
    - Support arbitrarily nested sub-commands.

- *2014-07-08* -- Stable v1.2.0 release.
    - Pass any value through to `Strings()` when final argument.
      Allows for values that look like flags to be processed.
    - Allow `--help` to be used with commands.
    - Support `Hidden()` flags.
    - Parser for [units.Base2Bytes](https://github.com/alecthomas/units)
      type. Allows for flags like `--ram=512MB` or `--ram=1GB`.
    - Add an `Enum()` value, allowing only one of a set of values
      to be selected. eg. `Flag(...).Enum("debug", "info", "warning")`.

- *2014-06-27* -- Stable v1.1.0 release.
    - Bug fixes.
    - Always return an error (rather than panicing) when misconfigured.
    - `OpenFile(flag, perm)` value type added, for finer control over opening files.
    - Significantly improved usage formatting.

- *2014-06-19* -- Stable v1.0.0 release.
    - Support [cumulative positional](#consuming-all-remaining-arguments) arguments.
    - Return error rather than panic when there are fatal errors not caught by
      the type system. eg. when a default value is invalid.
    - Use gokpg.in.

- *2014-06-10* -- Place-holder streamlining.
    - Renamed `MetaVar` to `PlaceHolder`.
    - Removed `MetaVarFromDefault`. Kingpin now uses [heuristics](#place-holders-in-help)
      to determine what to display.

## Simple Example

Kingpin can be used for simple flag+arg applications like so:

```
$ ping --help
usage: ping [<flags>] <ip> [<count>]

Flags:
  --debug            Enable debug mode.
  --help             Show help.
  -t, --timeout=5s   Timeout waiting for ping.

Args:
  <ip>        IP address to ping.
  [<count>]   Number of packets to send
$ ping 1.2.3.4 5
Would ping: 1.2.3.4 with timeout 5s and count 0
```

From the following source:

```go
package main

import (
  "fmt"

  "gopkg.in/alecthomas/kingpin.v1"
)

var (
  debug   = kingpin.Flag("debug", "Enable debug mode.").Bool()
  timeout = kingpin.Flag("timeout", "Timeout waiting for ping.").Default("5s").OverrideDefaultFromEnvar("PING_TIMEOUT").Short('t').Duration()
  ip      = kingpin.Arg("ip", "IP address to ping.").Required().IP()
  count   = kingpin.Arg("count", "Number of packets to send").Int()
)

func main() {
  kingpin.Version("0.0.1")
  kingpin.Parse()
  fmt.Printf("Would ping: %s with timeout %s and count %d", *ip, *timeout, *count)
}
```

## Complex Example

Kingpin can also produce complex command-line applications with global flags,
subcommands, and per-subcommand flags, like this:

```
$ chat --help
usage: chat [<flags>] <command> [<flags>] [<args> ...]

A command-line chat application.

Flags:
  --help              Show help.
  --debug             Enable debug mode.
  --server=127.0.0.1  Server address.

Commands:
  help [<command>]
    Show help for a command.

  register <nick> <name>
    Register a new user.

  post [<flags>] <channel> [<text>]
    Post a message to a channel.

$ chat help post
usage: chat [<flags>] post [<flags>] <channel> [<text>]

Post a message to a channel.

Flags:
  --image=IMAGE  Image to post.

Args:
  <channel>  Channel to post to.
  [<text>]   Text to post.

$ chat post --image=~/Downloads/owls.jpg pics
...
```

From this code:

```go
package main

import (
  "os"
  "strings"
  "gopkg.in/alecthomas/kingpin.v1"
)

var (
  app      = kingpin.New("chat", "A command-line chat application.")
  debug    = app.Flag("debug", "Enable debug mode.").Bool()
  serverIP = app.Flag("server", "Server address.").Default("127.0.0.1").IP()

  register     = app.Command("register", "Register a new user.")
  registerNick = register.Arg("nick", "Nickname for user.").Required().String()
  registerName = register.Arg("name", "Name of user.").Required().String()

  post        = app.Command("post", "Post a message to a channel.")
  postImage   = post.Flag("image", "Image to post.").File()
  postChannel = post.Arg("channel", "Channel to post to.").Required().String()
  postText    = post.Arg("text", "Text to post.").Strings()
)

func main() {
  switch kingpin.MustParse(app.Parse(os.Args[1:])) {
  // Register user
  case register.FullCommand():
    println(*registerNick)

  // Post message
  case post.FullCommand():
    if *postImage != nil {
    }
    text := strings.Join(*postText, " ")
    println("Post:", text)
  }
}
```

## Reference Documentation

### Help

Second to parsing, providing the user with useful help is probably the most
important thing a command-line parser does.

Since 1.3.x, Kingpin uses a bunch of heuristics to display help. For example,
`--help` should generally "just work" without much thought from users.

### Sub-commands

Kingpin supports nested sub-commands, with separate flag and positional
arguments per sub-command. Note that positional arguments may only occur after
sub-commands.

For example:

```go
var (
  deleteCommand     = kingpin.Command("delete", "Delete an object.")
  deleteUserCommand = deleteCommand.Command("user", "Delete a user.")
  deleteUserUIDFlag = deleteUserCommand.Flag("uid", "Delete user by UID rather than username.")
  deleteUserUsername = deleteUserCommand.Arg("username", "Username to delete.")
  deletePostCommand = deleteCommand.Command("post", "Delete a post.")
)

func main() {
  switch kingpin.Parse() {
  case "delete user":
  case "delete post":
  }
}
```

### Custom Parsers

Kingpin supports both flag and positional argument parsers for converting to
Go types. For example, some included parsers are `Int()`, `Float()`,
`Duration()` and `ExistingFile()`.

Parsers conform to Go's [`flag.Value`](http://godoc.org/flag#Value)
interface, so any existing implementations will work.

For example, a parser for accumulating HTTP header values might look like this:

```go
type HTTPHeaderValue http.Header

func (h *HTTPHeaderValue) Set(value string) error {
  parts := strings.SplitN(value, ":", 2)
  if len(parts) != 2 {
    return fmt.Errorf("expected HEADER:VALUE got '%s'", value)
  }
  (*http.Header)(h).Add(parts[0], parts[1])
  return nil
}

func (h *HTTPHeaderValue) String() string {
  return ""
}
```

As a convenience, I would recommend something like this:

```go
func HTTPHeader(s Settings) (target *http.Header) {
  target = new(http.Header)
  s.SetValue((*HTTPHeaderValue)(target))
  return
}
```

You would use it like so:

```go
headers = HTTPHeader(kingpin.Flag("header", "Add a HTTP header to the request.").Short('H'))
```

### Default Values

The default value is the zero value for a type. This can be overridden with
the `Default(value)` function on flags and arguments. This function accepts a
string, which is parsed by the value itself, so it *must* be compliant with
the format expected.

### Place-holders in Help

The place-holder value for a flag is the value used in the help to describe
the value of a non-boolean flag.

The value provided to PlaceHolder() is used if provided, then the value
provided by Default() if provided, then finally the capitalised flag name is
used.

Here are some examples of flags with various permutations:

    --name=NAME           // Flag(...).String()
    --name="Harry"        // Flag(...).Default("Harry").String()
    --name=FULL-NAME      // flag(...).PlaceHolder("FULL-NAME").Default("Harry").String()

### Consuming all remaining arguments

A common command-line idiom is to use all remaining arguments for some
purpose. eg. The following command accepts an arbitrary number of
IP addresses as positional arguments:

    ./cmd ping 10.1.1.1 192.168.1.1

Kingpin supports this by having `Value` provide a `IsCumulative() bool`
function. If this function exists and returns true, the value parser will be
called repeatedly for every remaining argument.

Examples of this are the `Strings()` and `StringMap()` values.

To implement the above example we might do something like this:

```go
type ipList []net.IP

func (i *ipList) Set(value string) error {
  if ip := net.ParseIP(value); ip == nil {
    return fmt.Errorf("'%s' is not an IP address", value)
  } else {
    *i = append(*i, ip)
    return nil
  }
}

func (i *ipList) String() string {
  return ""
}

func (i *ipList) IsCumulative() bool {
  return true
}

func IPList(s Settings) (target *[]net.IP) {
  target = new([]net.IP)
  s.SetValue((*ipList)(target))
  return
}
```

And use it like so:

```go
ips := IPList(kingpin.Arg("ips", "IP addresses to ping."))
```
