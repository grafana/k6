//go:build windows
// +build windows

package fasthttp

import (
	"errors"
	"syscall"
)

func isConnectionReset(err error) bool {
	return errors.Is(err, syscall.WSAECONNRESET)
}
