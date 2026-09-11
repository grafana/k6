//go:build grpc_e2e && linux

package grpce2e

import (
	"os/exec"
	"syscall"
)

// setPdeathsig asks the kernel to send SIGKILL to the child when the parent
// (test) process dies. This backstops t.Cleanup for cases where cleanups do not
// run, such as a `go test -timeout` panic, so the helper server is not orphaned.
func setPdeathsig(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
