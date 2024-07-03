package common

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"

	"github.com/grafana/sobek"
)

// NewRandSource is copied from Sobek's source code:
// https://github.com/grafana/sobek/blob/master/sobek/main.go#L44
// The returned RandSource is NOT safe for concurrent use:
// https://golang.org/pkg/math/rand/#NewSource
func NewRandSource() sobek.RandSource {
	var seed int64
	if err := binary.Read(crand.Reader, binary.LittleEndian, &seed); err != nil {
		panic(fmt.Errorf("could not read random bytes: %w", err))
	}
	return rand.New(rand.NewSource(seed)).Float64 //nolint:gosec
}
