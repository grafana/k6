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

// chance returns true with a `percentage` chance, otherwise false.
// the `percentage` argument is expected to be
// within 0 <= percentage <= 100 range.
func chance(percentage int) bool {
	//nolint:gosec
	return rand.Intn(100) < percentage
}
