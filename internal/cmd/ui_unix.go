//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || zos
// +build darwin dragonfly freebsd linux netbsd openbsd zos

package cmd

import (
	"os"
	"syscall"
)

func getWinchSignal() os.Signal {
	return syscall.SIGWINCH
}
