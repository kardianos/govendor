# Vendor Tool
*Vendor tool that follows the vendor-spec*

## sub-command "fetch"

`govendor fetch [-tree] [+status] [package-spec]`

Example usage:
```
# Get all missing packages, recursivly.
govendor fetch +m

# Get all 
govendor fetch -tree github.com/mattn/sqlite
```


## option "-tree"

signal with "<package path>/^"
Can tree root not have go files? It shouldn't need go files, implementation might be faulty.


## rework status
type:     program (main), !program (package)
location: vendor, local, external (gopath), standard lib (goroot)
status:   missing, un-used, tree (un-used but in a tree)

other sub-commands
	govendor build -o tools/bin/ +vendor,program
	govendor install +vendor,!program
	govendor test +local
