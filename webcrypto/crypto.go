package webcrypto

import "go.k6.io/k6/js/modules"

// Crypto represents the Crypto interface of the Web Crypto API.
type Crypto struct {
	vu modules.VU

	Subtle    *SubtleCrypto      `json:"subtle"`
	CryptoKey *CryptoKey[[]byte] `json:"CryptoKey"`
}

// GetRandomValues lets you get cryptographically strong random values.
// As defined by the Web Crypto API's Crypto.getRandomValues() method
// [specifications].
//
// Do not generate keys using the getRandomValues method. Use the generateKey method instead.
//
// The array given as the parameter is filled with random numbers (random in
// its cryptographic sense, not in its statistical sense).
//
// To guarantee enough performance, this implementation is not using a truly
// random number generator, but is using a pseudo-random number generator
// seeded with a value with enough entropy. We are using the golang
// crypto/rand package, which uses the operating system's random number
// generator.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#Crypto-method-getRandomValues
func (c *Crypto) GetRandomValues() {
	// TODO
}

// RandomUUID returns a [RFC4122] compliant v4 UUID string.
//
// It implements the Web Crypto API's Crypto.randomUUID() method, as
// specified in [Web Crypto API's specification] Level 10, section 10.1.2.
// The UUID is generated using a cryptographically secure random number generator.
//
// [RFC4122]: https://tools.ietf.org/html/rfc4122
// [Web Crypto API's specification]: https://w3c.github.io/webcrypto/#Crypto-method-randomUUID
func (c *Crypto) RandomUUID() {
	// TODO
}
