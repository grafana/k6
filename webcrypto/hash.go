package webcrypto

import (
	"crypto"
	"hash"
)

// getHashFn returns the hash function associated with the given name.
//
// It returns a generator function, that can be used to create a new
// hash.Hash instance.
func getHashFn(name string) (func() hash.Hash, bool) {
	switch name {
	case Sha1:
		return crypto.SHA1.New, true
	case Sha256:
		return crypto.SHA256.New, true
	case Sha384:
		return crypto.SHA384.New, true
	case Sha512:
		return crypto.SHA512.New, true
	default:
		return nil, false
	}
}
