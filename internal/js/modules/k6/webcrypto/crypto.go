package webcrypto

import (
	"crypto/rand"
	"encoding/binary"
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

	// 1.
	// A missing, null or undefined argument used to reach the constructor
	// lookup below and take the whole process down with a nil pointer
	// dereference. Browsers and Node throw a TypeError the script can
	// catch instead (#6320).
	if common.IsNullish(typedArray) {
		panic(rt.NewTypeError("typedArray parameter is missing, null or undefined"))
	}

	// 2.
	// Identify and fill the view through its Go-side representation rather
	// than its JS-visible properties. Exporting one of the IntegerArray
	// types yields a slice aliasing the view's storage, so writing through
	// it updates the JS array in place, and every element receives as many
	// random bits as its width allows (#6318). Because the element count
	// and width come from the view itself, overridden properties such as
	// `length` or `constructor` can neither crash the process nor change
	// how many bytes are written (#6320).
	//
	// The quota is specified on the view's byteLength, not its element
	// count ([spec's] 10.2.1.2 paragraph): a Uint32Array(65536) asks for
	// 256 KiB of randomness and is rejected the same way it is in browsers.
	// [spec]: https://www.w3.org/TR/WebCryptoAPI/#Crypto-method-getRandomValues
	switch view := typedArray.Export().(type) {
	case []byte: // Uint8Array, Uint8ClampedArray
		c.throwIfViewTooLong(rt, len(view), 1)
		fillRandomBytes(view)
	case []int8: // Int8Array
		c.throwIfViewTooLong(rt, len(view), 1)
		fillRandomBytes(view)
	case []int16: // Int16Array
		c.throwIfViewTooLong(rt, len(view), 2)
		scatterRandomInt16s(view)
	case []uint16: // Uint16Array
		c.throwIfViewTooLong(rt, len(view), 2)
		scatterRandomUint16s(view)
	case []int32: // Int32Array
		c.throwIfViewTooLong(rt, len(view), 4)
		scatterRandomInt32s(view)
	case []uint32: // Uint32Array
		c.throwIfViewTooLong(rt, len(view), 4)
		scatterRandomUint32s(view)
	default:
		common.Throw(rt, NewError(TypeMismatchError, "typedArray parameter isn't a TypedArray instance"))
	}

	// Although the input array has been modified in place,
	// the specification stipulates it should also be returned.
	return typedArray
}

// throwIfViewTooLong enforces the spec's 65536-byte quota on getRandomValues
// input views, throwing a QuotaExceededError the script can catch.
func (c *Crypto) throwIfViewTooLong(rt *sobek.Runtime, elements, elementSize int) {
	if elements*elementSize > maxRandomValuesLength {
		common.Throw(
			rt,
			NewError(
				QuotaExceededError,
				fmt.Sprintf(
					"typedArray parameter is too big; maximum length is %d bytes",
					maxRandomValuesLength,
				),
			),
		)
	}
}

// fillRandomBytes overwrites an 8-bit view's storage with fresh random bytes.
// Signedness is irrelevant at this width: the bits are copied verbatim.
func fillRandomBytes[E ~int8 | ~uint8](view []E) {
	buf := make([]byte, len(view))
	_, _ = rand.Read(buf)
	for i, b := range buf {
		view[i] = E(b)
	}
}

// scatterRandomInt16s fills each 16-bit element with 16 fresh random bits.
// The byte order used to assemble elements is an implementation detail: it is
// only required to be consistent, since the result is uniformly random either
// way.
func scatterRandomInt16s(view []int16) {
	buf := make([]byte, 2*len(view))
	_, _ = rand.Read(buf)
	for i := range view {
		view[i] = int16(binary.LittleEndian.Uint16(buf[2*i:]))
	}
}

// scatterRandomUint16s fills each 16-bit element with 16 fresh random bits.
func scatterRandomUint16s(view []uint16) {
	buf := make([]byte, 2*len(view))
	_, _ = rand.Read(buf)
	for i := range view {
		view[i] = binary.LittleEndian.Uint16(buf[2*i:])
	}
}

// scatterRandomInt32s fills each 32-bit element with 32 fresh random bits.
func scatterRandomInt32s(view []int32) {
	buf := make([]byte, 4*len(view))
	_, _ = rand.Read(buf)
	for i := range view {
		view[i] = int32(binary.LittleEndian.Uint32(buf[4*i:]))
	}
}

// scatterRandomUint32s fills each 32-bit element with 32 fresh random bits.
func scatterRandomUint32s(view []uint32) {
	buf := make([]byte, 4*len(view))
	_, _ = rand.Read(buf)
	for i := range view {
		view[i] = binary.LittleEndian.Uint32(buf[4*i:])
	}
}

// MaxRandomValues is the maximum number of random bytes that can be requested
// from a single getRandomValues call.
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
