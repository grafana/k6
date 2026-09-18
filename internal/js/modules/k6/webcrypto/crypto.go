package webcrypto

import (
	"crypto/rand"
	"fmt"

	"github.com/google/uuid"
	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

// Crypto represents the Crypto interface of the Web Crypto API.
type Crypto struct {
	vu modules.VU

	Subtle    *SubtleCrypto `js:"subtle"`
	CryptoKey *CryptoKey    `js:"CryptoKey"`
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
func (c *Crypto) GetRandomValues(typedArray sobek.Value) sobek.Value {
	// The typedArray parameter is not optional in the specification's WebIDL
	// definition, so reject a missing or nullish one before IsInstanceOf gets
	// to dereference it. This is an ECMAScript TypeError rather than one of the
	// WebCrypto error names, which is what browsers and Node throw here, and
	// what a nullish argument already produced before this check existed.
	if common.IsNullish(typedArray) {
		panic(c.vu.Runtime().NewTypeError("typedArray parameter is required"))
	}

	acceptedTypes := []JSType{
		Int8ArrayConstructor,
		Uint8ArrayConstructor,
		Uint8ClampedArrayConstructor,
		Int16ArrayConstructor,
		Uint16ArrayConstructor,
		Int32ArrayConstructor,
		Uint32ArrayConstructor,
	}

	// 1.
	if !IsInstanceOf(c.vu.Runtime(), typedArray, acceptedTypes...) {
		common.Throw(c.vu.Runtime(), NewError(TypeMismatchError, "typedArray parameter isn't a TypedArray instance"))
	}

	// 2.
	// The spec's quota is on the view's byteLength, not its element count.
	// ExportTo returns the ArrayBuffer bytes this view covers, so filling
	// that slice in place also gives 16- and 32-bit arrays their full width
	// instead of one random byte per element.
	// [spec]: https://www.w3.org/TR/WebCryptoAPI/#Crypto-method-getRandomValues
	var view []byte
	err := c.vu.Runtime().ExportTo(typedArray, &view)
	if err != nil {
		common.Throw(c.vu.Runtime(), NewError(TypeMismatchError, "typedArray parameter isn't a TypedArray instance"))
	}

	if int64(len(view)) > maxRandomValuesLength {
		common.Throw(
			c.vu.Runtime(),
			NewError(
				QuotaExceededError,
				fmt.Sprintf("typedArray parameter is too big; maximum length is %d", maxRandomValuesLength),
			),
		)
	}

	// 3.
	// We use crypto/rand.Read() here as it will use /dev/urandom or
	// an equivalent on Unix-like systems, and CryptGenRandom()
	// on Windows. This is the recommended way to generate random
	// by the specification.
	_, err = rand.Read(view)
	if err != nil {
		common.Throw(c.vu.Runtime(), err)
	}

	// Although the input array has been modified in place,
	// the specification stipulates it should also be returned.
	return typedArray
}

// maxRandomValuesLength is the maximum byteLength of a typed array that
// getRandomValues will fill, per the Web Crypto API.
const maxRandomValuesLength = 65536

// RandomUUID returns a [RFC4122] compliant v4 UUID string.
//
// It implements the Web Crypto API's Crypto.randomUUID() method, as
// specified in [Web Crypto API's specification] Level 10, section 10.1.2.
// The UUID is generated using a cryptographically secure random number generator.
//
// [RFC4122]: https://tools.ietf.org/html/rfc4122
// [Web Crypto API's specification]: https://w3c.github.io/webcrypto/#Crypto-method-randomUUID
func (c *Crypto) RandomUUID() string {
	return uuid.New().String()
}
