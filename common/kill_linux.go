//go:build linux
// +build linux

package common

import (
	"os/exec"
	"syscall"
)

// killAfterParent kills the child process when the parent process dies.
func killAfterParent(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
