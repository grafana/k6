//go:build !(darwin || linux || freebsd || windows)

package platform

func isTerminal(fd uintptr) bool {
	return false
}
