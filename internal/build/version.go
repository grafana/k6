// Package build handles information and processes about the k6 binary's build.
package build

// Version contains the current version of k6
// represented using Semantic Versioning expression.
const Version = "2.2.0"

// BuildOrigin records how this binary was built, e.g. "release", and a build command sets it with
// -ldflags "-X go.k6.io/k6/v2/internal/build.BuildOrigin=release".
var BuildOrigin string //nolint:gochecknoglobals // the Go linker can only set a package-level variable
