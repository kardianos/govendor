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

