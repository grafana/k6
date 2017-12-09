package version

import "fmt"

// Version string, k6 uses semantic versioning (http://semver.org/)
type Version struct {
	Major uint
	Minor uint
	Patch uint
}

// Current represents the current version and can be shared across packages
var Current = Version{Major: 0, Minor: 18, Patch: 2}

// Full returns the full semantic version as a string major.minor.patch
func Full() string {
	return fmt.Sprintf("%d.%d.%d", Current.Major, Current.Minor, Current.Patch)
}
