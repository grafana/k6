//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd && !solaris && !illumos

package pool

import (
	"errors"
	"net"
)

// errUnexpectedRead is placeholder error variable for non-unix build constraints
var errUnexpectedRead = errors.New("unexpected read from socket")

func connCheck(_ net.Conn) error {
	return nil
}

// since we can't check for data on the socket, we just assume there is some
func maybeHasData(_ net.Conn) bool {
	return true
}
