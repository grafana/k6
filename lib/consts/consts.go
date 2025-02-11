// Package consts houses some constants needed across k6
package consts

import "go.k6.io/k6/internal/build"

// Version contains the current semantic version of k6.
//
// Deprecated: alias to support the legacy versioning API. Use the new version package,
// it will be removed as soon as the external services stop to depend on it.
const Version = build.Version
