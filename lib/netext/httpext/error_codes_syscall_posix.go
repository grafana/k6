//go:build !windows
// +build !windows

package httpext

import (
	"net"
	"os"
)

func getOSSyscallErrorCode(_ *net.OpError, _ *os.SyscallError) (errCode, string) {
	return 0, ""
}
