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
	rt := c.vu.Runtime()
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
	if !IsInstanceOf(rt, typedArray, acceptedTypes...) {
		common.Throw(rt, NewError(TypeMismatchError, "typedArray parameter isn't a TypedArray instance"))
	}

	// 2.
	// The spec quotas and fills by byteLength of the view, not its element
	// count. Using `.length` previously wrote a single random byte into each
	// element, so Uint16/Uint32 arrays (including the documented example)
	// only ever contained values in 0-255.
	// [spec]: https://www.w3.org/TR/WebCryptoAPI/#Crypto-method-getRandomValues
	obj := typedArray.ToObject(rt)
	byteLength := exportNonNegativeInt(obj.Get("byteLength"))
	if byteLength < 0 {
		common.Throw(rt, NewError(TypeMismatchError, "typedArray parameter isn't a TypedArray instance"))
	}

	if byteLength > maxRandomValuesLength {
		common.Throw(
			rt,
			NewError(
				QuotaExceededError,
				fmt.Sprintf("typedArray parameter is too big; maximum byte length is %d", maxRandomValuesLength),
			),
		)
	}

	byteOffset := exportNonNegativeInt(obj.Get("byteOffset"))
	if byteOffset < 0 {
		byteOffset = 0
	}

	ab, ok := obj.Get("buffer").Export().(sobek.ArrayBuffer)
	if !ok || ab.Detached() {
		common.Throw(rt, NewError(TypeMismatchError, "typedArray parameter isn't a TypedArray instance"))
	}

	buf := ab.Bytes()
	end := byteOffset + byteLength
	if byteOffset > int64(len(buf)) || end > int64(len(buf)) {
		common.Throw(rt, NewError(TypeMismatchError, "typedArray parameter isn't a TypedArray instance"))
	}

	// 3.
	// Overwrite the view's bytes in the underlying ArrayBuffer so multi-byte
	// typed arrays receive a full-width cryptographically random value per
	// element. crypto/rand.Read uses /dev/urandom (or equivalent).
	if byteLength > 0 {
		if _, err := rand.Read(buf[byteOffset:end]); err != nil {
			common.Throw(rt, err)
		}
	}

	// Although the input array has been modified in place,
	// the specification stipulates it should also be returned.
	return typedArray
}

// exportNonNegativeInt returns v as an int64, or -1 if v is missing/invalid.
func exportNonNegativeInt(v sobek.Value) int64 {
	if common.IsNullish(v) {
		return -1
	}
	n := v.ToInteger()
	if n < 0 {
		return -1
	}
	return n
}

// maxRandomValuesLength is the maximum view byteLength accepted by getRandomValues.
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
