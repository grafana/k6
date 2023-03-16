//go:build !windows

package platform

import "syscall"

func Rename(from, to string) error {
	if from == to {
		return nil
	}
	return syscall.Rename(from, to)
}
