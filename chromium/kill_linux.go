//go:build linux
// +build linux

package chromium

import (
	"os/exec"
	"syscall"
)

// KillAfterParent kills the child process when the parent process dies.
func KillAfterParent(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
