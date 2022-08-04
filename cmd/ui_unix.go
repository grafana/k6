//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd
// +build darwin dragonfly freebsd linux netbsd openbsd

package cmd

import (
	"os"
	"syscall"
)

func getWinchSignal() os.Signal {
	return syscall.SIGWINCH
}
