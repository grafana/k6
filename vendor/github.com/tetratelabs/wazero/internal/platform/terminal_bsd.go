//go:build darwin || freebsd

package platform

import "syscall"

const ioctlReadTermios = syscall.TIOCGETA
