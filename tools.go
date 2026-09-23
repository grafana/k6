//go:build tools

// This file pins the code-generation tools used by `make generate` into k6's
// own module graph, so they build against k6's pinned golang.org/x/tools
// version instead of the (Go-version-incompatible) one from each tool's
// go.mod. See #6399: the mstoykov/enumer fork pins golang.org/x/tools v0.35.0,
// which cannot read the export data produced by Go 1.27 and newer.
//
// The `tools` build tag keeps this file out of normal builds; `go mod tidy`
// still sees the imports and keeps the requirements in go.mod.
package main

import (
	_ "github.com/mstoykov/enumer"
)
