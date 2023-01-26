package tracing

import (
	"math/rand"
)

// randHexStringRunes returns a random string of n hex characters.
//
// Note that this function uses a non-cryptographic random number generator.
func randHexString(n int) string {
	hexRunes := []rune("123456789abcdef")

	b := make([]rune, n)
	for i := range b {
		b[i] = hexRunes[rand.Intn(len(hexRunes))] //nolint:gosec
	}

	return string(b)
}
