//go:build !windows

package platform

// SanitizeSeparator sanitizes the path separator in the given buffer.
// This does nothing on the non-windows platforms.
func SanitizeSeparator([]byte) {}
