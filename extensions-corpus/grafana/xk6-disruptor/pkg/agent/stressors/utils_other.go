//go:build !linux
// +build !linux

package stressors

import (
	"time"
)

// CPUTime is only supported in linux
// This implementation is a workaround to avoid unit tests failing in darwin until the project
// is restructured to separate the linux agent from the multi-platform k6 extension
func CPUTime() time.Duration {
	panic("unsupported platform")
}
