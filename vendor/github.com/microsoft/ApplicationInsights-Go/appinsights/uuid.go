package appinsights

import (
	crand "crypto/rand"
	"encoding/binary"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

// uuidGenerator is a wrapper for satori/go.uuid, used for a few reasons:
//   - Avoids build failures due to version differences when a project imports us but
//     does not respect our vendoring. (satori/go.uuid#77, #71, #66, ...)
//   - Avoids error output when creaing new UUID's: if the crypto reader fails,
//     this will fallback on the standard library PRNG, since this is never used
//     for a sensitive application.
//   - Uses io.ReadFull to guarantee fully-populated UUID's (satori/go.uuid#73)
type uuidGenerator struct {
	sync.Mutex
	fallbackRand *rand.Rand
	reader       io.Reader
}

var uuidgen *uuidGenerator = newUuidGenerator(crand.Reader)

// newUuidGenerator creates a new uuiGenerator with the specified crypto random reader.
func newUuidGenerator(reader io.Reader) *uuidGenerator {
	// Setup seed for fallback random generator
	var seed int64
	b := make([]byte, 8)
	if _, err := io.ReadFull(reader, b); err == nil {
		seed = int64(binary.BigEndian.Uint64(b))
	} else {
		// Otherwise just use the timestamp
		seed = time.Now().UTC().UnixNano()
	}

	return &uuidGenerator{
		reader:       reader,
		fallbackRand: rand.New(rand.NewSource(seed)),
	}
}

// newUUID generates a new V4 UUID
func (gen *uuidGenerator) newUUID() uuid.UUID {
	u := uuid.UUID{}
	if _, err := io.ReadFull(gen.reader, u[:]); err != nil {
		gen.fallback(&u)
	}

	u.SetVersion(uuid.V4)
	u.SetVersion(uuid.VariantRFC4122)
	return u
}

// fallback populates the specified UUID with the standard library's PRNG
func (gen *uuidGenerator) fallback(u *uuid.UUID) {
	gen.Lock()
	defer gen.Unlock()

	// This does not fail as per documentation
	gen.fallbackRand.Read(u[:])
}

// newUUID generates a new V4 UUID
func newUUID() uuid.UUID {
	return uuidgen.newUUID()
}
