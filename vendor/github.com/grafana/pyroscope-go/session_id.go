package pyroscope

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"hash/fnv"
	"math/rand"
	"os"
	"sync"
)

const sessionIDLabelName = "__session_id__"

type sessionID uint64

func (s sessionID) String() string {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(s))
	return hex.EncodeToString(b[:])
}

func newSessionID() sessionID { return globalSessionIDGenerator.newSessionID() }

var globalSessionIDGenerator = newSessionIDGenerator()

type sessionIDGenerator struct {
	sync.Mutex
	src *rand.Rand
}

func (gen *sessionIDGenerator) newSessionID() sessionID {
	var b [8]byte
	gen.Lock()
	_, _ = gen.src.Read(b[:])
	gen.Unlock()
	return sessionID(binary.LittleEndian.Uint64(b[:]))
}

func newSessionIDGenerator() *sessionIDGenerator {
	s, ok := sessionIDHostSeed()
	if !ok {
		s = sessionIDRandSeed()
	}
	return &sessionIDGenerator{src: rand.New(rand.NewSource(s))}
}

func sessionIDRandSeed() int64 {
	var rndSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rndSeed)
	return rndSeed
}

var hostname = os.Hostname

func sessionIDHostSeed() (int64, bool) {
	v, err := hostname()
	if err != nil {
		return 0, false
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(v))
	return int64(h.Sum64()), true
}
