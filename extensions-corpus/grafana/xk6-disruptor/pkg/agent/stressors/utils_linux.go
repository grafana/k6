//go:build linux
// +build linux

package stressors

import (
	"syscall"
	"time"
)

// CPUTime returns the CPU time consumed by the current thread in nanoseconds
func CPUTime() time.Duration {
	usage := new(syscall.Rusage)
	_ = syscall.Getrusage(syscall.RUSAGE_THREAD, usage)
	return time.Duration(usage.Utime.Nano()+usage.Stime.Nano()) * time.Nanosecond
}
