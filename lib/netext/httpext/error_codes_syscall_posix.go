//go:build !windows
// +build !windows

package httpext

import (
	"net"
	"os"
)

func getOSSyscallErrorCode(e *net.OpError, se *os.SyscallError) (errCode, string) {
	return 0, ""
}
