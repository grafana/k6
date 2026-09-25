//go:build grpc_e2e && !linux

package grpce2e

import "os/exec"

// setPdeathsig is a no-op on platforms that do not support a parent-death
// signal. On those systems the server is still stopped via t.Cleanup and the
// test's context cancellation. The e2e job runs on Linux.
func setPdeathsig(_ *exec.Cmd) {}
