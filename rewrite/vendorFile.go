package rewrite

// VendorFile is the structure of the vendor file.
type VendorFile struct {
	// The name of the tool last used to write this file.
	// This is not necessarily the name of the executable as that will
	// vary based on platform.
	Tool string

	List []struct {
		// Import path. Example "rsc.io/pdf".
		// go get <Import> should fetch the remote package.
		//
		// If Import ends in "/..." the tool should manage all packages below
		// the import as well.
		Import string

		// Package path relative to "internal" folder.
		// Examples: "rsc.io/pdf", "pdf".
		// If Local is an empty string, the tool should assume the path is
		// relative to GOPATH and the package is not currently copied
		// locally.
		//
		// Local should not contain a trailing "/...".
		// Local should always use forward slashes and must not contain the
		// path elements "." or "..".
		Local string

		// The version of the package. This field must be persisted by all
		// tools, but not all tools will interpret this field.
		// The value of Version should be a single value that can be used
		// to fetch the same or similar version.
		// Examples: "abc104...438ade0", "v1.3.5"
		Version string

		// VersionTime is the time the version was created. The time should be
		// parsed and written in the "time.RFC3339" format.
		VersionTime string
	}
}
