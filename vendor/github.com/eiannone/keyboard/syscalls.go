// +build !windows,!linux

package keyboard

import (
	"golang.org/x/sys/unix"
)

const (
	ioctl_GETATTR = unix.TIOCGETA
	ioctl_SETATTR = unix.TIOCSETA
)
