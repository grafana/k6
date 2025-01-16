package webcrypto

import (
	"crypto/rand"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
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
	// Obtain the length of the typed array, and throw a QuotaExceededError if
	// it's too big, as specified in the [spec's] 10.2.1.2 paragraph.
	// [spec]: https://www.w3.org/TR/WebCryptoAPI/#Crypto-method-getRandomValues
	obj := typedArray.ToObject(c.vu.Runtime())
	objLength, ok := obj.Get("length").ToNumber().Export().(int64)
	if !ok {
		common.Throw(c.vu.Runtime(), NewError(TypeMismatchError, "typedArray parameter isn't a TypedArray instance"))
	}

	if objLength > maxRandomValuesLength {
		common.Throw(
			c.vu.Runtime(),
			NewError(
				QuotaExceededError,
				fmt.Sprintf("typedArray parameter is too big; maximum length is %d", maxRandomValuesLength),
			),
		)
	}

	// 3.
	// Create a buffer of a matching size and fill
	// it with random values.
	//
	// We use crypto/rand.Read() here as it will use /dev/urandom or
	// an equivalent on Unix-like systems, and CryptGenRandom()
	// on Windows. This is the recommended way to generate random
	// by the specification.
	randomValues := make([]byte, objLength)
	_, err := rand.Read(randomValues)
	if err != nil {
		common.Throw(c.vu.Runtime(), err)
	}

	for i := int64(0); i < objLength; i++ {
		err := obj.Set(strconv.FormatInt(i, 10), randomValues[i])
		if err != nil {
			common.Throw(c.vu.Runtime(), err)
		}
	}

	// Although the input array has been modified in place,
	// the specification stipulates it should also be returned.
	return typedArray
}

// MaxRandomValues is the maximum number of random values that can be generated
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
