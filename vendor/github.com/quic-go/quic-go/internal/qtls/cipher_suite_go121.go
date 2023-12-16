//go:build go1.21

package qtls

import (
	"crypto"
	"crypto/cipher"
	"crypto/tls"
	"fmt"
	"unsafe"
)

type cipherSuiteTLS13 struct {
	ID     uint16
	KeyLen int
	AEAD   func(key, fixedNonce []byte) cipher.AEAD
	Hash   crypto.Hash
}

//go:linkname cipherSuiteTLS13ByID crypto/tls.cipherSuiteTLS13ByID
func cipherSuiteTLS13ByID(id uint16) *cipherSuiteTLS13

//go:linkname cipherSuitesTLS13 crypto/tls.cipherSuitesTLS13
var cipherSuitesTLS13 []unsafe.Pointer

//go:linkname defaultCipherSuitesTLS13 crypto/tls.defaultCipherSuitesTLS13
var defaultCipherSuitesTLS13 []uint16

//go:linkname defaultCipherSuitesTLS13NoAES crypto/tls.defaultCipherSuitesTLS13NoAES
var defaultCipherSuitesTLS13NoAES []uint16

var cipherSuitesModified bool

// SetCipherSuite modifies the cipherSuiteTLS13 slice of cipher suites inside qtls
// such that it only contains the cipher suite with the chosen id.
// The reset function returned resets them back to the original value.
func SetCipherSuite(id uint16) (reset func()) {
	if cipherSuitesModified {
		panic("cipher suites modified multiple times without resetting")
	}
	cipherSuitesModified = true

	origCipherSuitesTLS13 := append([]unsafe.Pointer{}, cipherSuitesTLS13...)
	origDefaultCipherSuitesTLS13 := append([]uint16{}, defaultCipherSuitesTLS13...)
	origDefaultCipherSuitesTLS13NoAES := append([]uint16{}, defaultCipherSuitesTLS13NoAES...)
	// The order is given by the order of the slice elements in cipherSuitesTLS13 in qtls.
	switch id {
	case tls.TLS_AES_128_GCM_SHA256:
		cipherSuitesTLS13 = cipherSuitesTLS13[:1]
	case tls.TLS_CHACHA20_POLY1305_SHA256:
		cipherSuitesTLS13 = cipherSuitesTLS13[1:2]
	case tls.TLS_AES_256_GCM_SHA384:
		cipherSuitesTLS13 = cipherSuitesTLS13[2:]
	default:
		panic(fmt.Sprintf("unexpected cipher suite: %d", id))
	}
	defaultCipherSuitesTLS13 = []uint16{id}
	defaultCipherSuitesTLS13NoAES = []uint16{id}

	return func() {
		cipherSuitesTLS13 = origCipherSuitesTLS13
		defaultCipherSuitesTLS13 = origDefaultCipherSuitesTLS13
		defaultCipherSuitesTLS13NoAES = origDefaultCipherSuitesTLS13NoAES
		cipherSuitesModified = false
	}
}
