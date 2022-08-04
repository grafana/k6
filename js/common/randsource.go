package common

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"

	"github.com/dop251/goja"
)

// NewRandSource is copied from goja's source code:
// https://github.com/dop251/goja/blob/master/goja/main.go#L44
// The returned RandSource is NOT safe for concurrent use:
// https://golang.org/pkg/math/rand/#NewSource
func NewRandSource() goja.RandSource {
	var seed int64
	if err := binary.Read(crand.Reader, binary.LittleEndian, &seed); err != nil {
		panic(fmt.Errorf("could not read random bytes: %w", err))
	}
	return rand.New(rand.NewSource(seed)).Float64 //nolint:gosec
}
