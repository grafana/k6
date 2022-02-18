//go:build !linux
// +build !linux

package chromium

import "os/exec"

// KillAfterParent kills the child process when the parent process dies.
func KillAfterParent(cmd *exec.Cmd) {
}
