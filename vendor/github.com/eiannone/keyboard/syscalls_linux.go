package keyboard

import (
	"golang.org/x/sys/unix"
)

const (
	ioctl_GETATTR = unix.TCGETS
	ioctl_SETATTR = unix.TCSETS
)
