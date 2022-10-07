package webcrypto

import (
	"crypto"
	"errors"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules"
)

// FIXME: SubtleCrypto is described as an "interface", should it be a nested module
// with top-level functions instead then? (as opposed to a struct as it is now)

// FIXME: Make sure we cover the complete range of errors, and that their value makes sense

type SubtleCrypto struct {
	vu modules.VU
}

// InvalidAccessError is an error returned when the requested operation is not valid for the provided key
// (invalid encryption algorithm, or invalid key for the specified encryption algorithm)
var InvalidAccessError = errors.New("the requested operation is not valid for the provided key")

// OperationError is an error returned when the operation failed for an operation specific reason
// (algorithm parameters of invalid sizes, or AES-GCM plaintext longer than 2³⁹−256 bytes)
var OperationError = errors.New("the operation failed for an operation specific reason")

// NotSupportedError is an error returned when trying to use an algorithm that is either unknown or isn't
// suitable for derivation, or if the algorithm requested for the derived key doesn't define a key length.
var NotSupportedError = errors.New("the operation failed as the requested algorithm isn't suited for derivation")

// The TypeError object represents an error when an operation could not be performed, typically (but not exclusively) when a value is not of the expected type.
var TypeError = errors.New("the operation failed as it attempted to use an invalid format")

// Encrypt encrypts data.
//
// It takes as its arguments a key to encrypt with, some algorithm-specific
// parameters, and the data to encrypt (also known as "plaintext").
//
// It returns a Promise which will be fulfilled with the encrypted data (also known as "ciphertext").
//
// The `algorithm` parameter should be one of:
//   - an `SubtleCrypto.RSAOaepParams` object
//   - an `SubtleCrypto.AESCtrParams` object
//   - an `SubtleCrypto.AESCbcParams` object
//   - an`SubtleCrypto.AESGcmParams` object
//
// The `key` parameter should be a `CryptoKey` to be used for encryption.
//
// The `data` parameter should contain the data to be encryption.
func (sc *SubtleCrypto) Encrypt(algorithm goja.Value, key CryptoKey, data []byte) *goja.Promise {
	// TODO: implementation
	return nil
}

// Decrypt decrypts some encrypted data.
//
// It takes as arguments a key to decrypt with, some optional extra parameters, and
// the data to decrypt (also known as "ciphertext").
//
// It returns a Promise which will be fulfilled with the decrypted data (also known
// as "plaintext").
//
// Note that if the provided `algorithm` is RSA-OAEP, the `key` parameter should hold
// the `PrivateKey` property of the `CryptoKeyPair`.
//
// The `algorithm` parameter should be one of:
//   - an `SubtleCrypto.RSAOaepParams` object
//   - an `SubtleCrypto.AESCtrParams` object
//   - an `SubtleCrypto.AESCbcParams` object
//   - an `SubtleCrypto.AESGcmParams` object
//
// The `key` parameter should be a `CryptoKey` to be used for decryption.
//
// The `data` parameter should contain the data to be decrypted.
func (sc *SubtleCrypto) Decrypt(algorithm goja.Value, key CryptoKey, data []byte) *goja.Promise {
	// TODO: implementation
	return nil
}

// Sign generates a digital signature.
//
// It takes as its arguments a key to sign with, some algorithm-specific parameters, and the data to sign.
// It returns a Promise which will be fulfilled with the signature.
//
// Note that if the `algorithm` parameter identifies a public-key cryptosystem, the `key` parameter
// should be a private key.
//
// You can use the corresponding `SubtleCrypto.Verify` method to verify the signature.
//
// The `algorithm` parameter should be one of:
//   - the string "RSASSA-PKCS1-v1_5" or an object of the form `{ "name": "RSASSA-PKCS1-v1_5" }`
//   - an `SubtleCrypto.RSAPssParams` object
//   - an `SubtleCrypto.EcdsaParams` object
//   - the string "HMAC" or an object of the form `{ "name": "HMAC" }`
//
// The `key` parameter should be a `CryptoKey` to be used for signing. Note that if
// `algorithm` identifies a public-key cryptosystem, this is the private key.
//
// The `data` parameter should contain the data to be signed.
func (sc *SubtleCrypto) Sign(algorithm goja.Value, key CryptoKey, data []byte) *goja.Promise {
	// TODO: implementation
	return nil
}

// Verify verifies a digital signature.
//
// It takes as its arguments a key to verify the signature with, some
// algorithm-specific parameters, the signature, and the original signed data.
//
// It returns a Promise which will be fulfilled with a boolean value indicating
// whether the signature is valid.
//
// Note that the `key` parameter should hold the secret key for a symmetric algorithm
// and the public key for a public-key system.
//
// The `algorithm` parameter should be one of:
//   - the string "RSASSA-PKCS1-v1_5" or an object of the form `{ "name": "RSASSA-PKCS1-v1_5" }`
//   - an `SubtleCrypto.RSAPssParams` object
//   - an `SubtleCrypto.EcdsaParams` object
//   - the string "HMAC" or an object of the form `{ "name": "HMAC" }`
//
// The `key` parameter should be a `CryptoKey` to be used for verification. Note that it
// is the secret key for a symmetric algorithm and the public key for a public-key system.
//
// The `signature` parameter should contain the signature to be verified.
//
// The `data` parameter should contain the original signed data.
func (sc *SubtleCrypto) Verify(algorithm goja.Value, key CryptoKey, signature []byte, data []byte) *goja.Promise {
	// TODO: implementation
	return nil
}

// Digest generates a digest of the given data.
//
// A digest is a short fixed-length value derived from some
// variable-length input. Cryptographic digests should exhibit
// collision-resistance, meaning that it's hard to come up with
// two different inputs that have the same digest value.
//
// It takes as its arguments an identifier for the digest algorithm
// to use and the data to digest.
// It returns a Promise which will be fulfilled with the digest.
//
// Supported digest algorithms:
//   - SHA-1 (not to be used in cryptographic applications)
//   - SHA-256
//   - SHA-384
//   - SHA-512
//
// The `data` parameter should contain the data to be digested.
func (sc *SubtleCrypto) Digest(algorithm DigestKind, data interface{}) *goja.Promise {
	promise, resolve, reject := sc.makeHandledPromise()

	bytes, err := ToBytes(data)
	if err != nil {
		reject(err)
		return promise
	}

	go func() {
		switch DigestKind(algorithm) {
		case DigestKindSHA1:
			resolve(crypto.SHA1.New().Sum(bytes))
		case DigestKindSHA256:
			resolve(crypto.SHA256.New().Sum(bytes))
		case DigestKindSHA384:
			resolve(crypto.SHA384.New().Sum(bytes))
		case DigestKindSHA512:
			resolve(crypto.SHA512.New().Sum(bytes))
		default:
			reject(NotSupportedError)
			return
		}
	}()

	return promise
}

// GenerateKey generate a new key (for symmetric algorithms) or key pair (for public-key algorithms).
//
// The generated key will match the algorithm, usages, and extractability given
// as parameters.
//
// It returns a Promise that fulfills with a `SubtleCrypto.CryptoKey` (for symmetric algorithms)
// or a `SubtleCrypto.CryptoKeyPair` (for public-key algorithms).
//
// The `algorithm` parameter should be one of:
//   - for RSASSA-PKCS1-v1_5, RSA-PSS, or RSA-OAEP: pass an `SubtleCrypto.RSAHashedKeyGenParams` object
//   - for ECDSA or ECDH: pass an `SubtleCrypto.ECKeyGenParams` object
//   - an `SubtleCrypto.HMACKeyGenParams` object
//   - for AES-CTR, AES-CBC, AES-GCM, AES-KW: pass an `SubtleCrypto.AESKeyGenParams`
//
// The `extractable` parameter indicates whether it will be possible to export the key
// using `SubtleCrypto.ExportKey` or `SubtleCrypto.WrapKey`.
//
// The `keyUsages` parameter is an array of strings indicating what the key can be used for.
func (sc *SubtleCrypto) GenerateKey(algorithm goja.Value, extractable bool, keyUsages []CryptoKeyUsage) *goja.Promise {
	// TODO: implementation
	return nil
}

// DeriveKey can be used to derive a secret key from a master key.
//
// It takes as arguments some initial key material, the derivation
// algorithm to use, and the desired properties for the key to derive.
// It returns a Promise which will be fulfilled with a CryptoKey object
// representing the new key.
//
// Note that if the `algorithm` parameter is ECDH, the the `baseKey` parameter
// should be a private key. Otherwise, it should be the initial key material for
// the derivation function: for example, for PBKDF2 it might be a password, imported
// as a CryptoKey using `SubtleCrypto.ImportKey`.
//
// The `algorithm` parameter should be one of:
//   - an `SubtleCrypto.ECDHKeyDeriveParams` object
//   - an `SubtleCrypto.HKDFParams` object
//   - an `SubtleCrypto.Pbkdf2Params` object
//
// The `baseKey` parameter should be a CryptoKey object representing the input
// to the derivation algorithm. If `algorithm` is ECDH, then this will be the
// ECDH private key. Otherwise it will be the initial key material for the derivation
// function: for example, for PBKDF2 it might be a password, imported as a `SubtleCrypto.CryptoKey`
// using `SubtleCrypto.ImportKey`.
//
// The `derivedKeyAlgorithm` parameter should be one of:
//   - an `SubtleCrypto.HMACKeyGenParams` object
//   - For AES-CTR, AES-CBC, AES-GCM, AES-KW: pass an `SubtleCrypto.AESKeyGenParams`
//
// The `extractable` parameter indicates whether it will be possible to export the key
// using `SubtleCrypto.ExportKey` or `SubtleCrypto.WrapKey`.
//
// The `keyUsages` parameter is an array of strings indicating what the key can be used for.
func (sc *SubtleCrypto) DeriveKey(
	algorithm goja.Value,
	baseKey CryptoKey,
	derivedKeyAlgorithm goja.Value,
	extractable bool,
	keyUsages []CryptoKeyUsage,
) *goja.Promise {
	// TODO: implementation
	return nil
}

// DeriveBits derives an array of bits from a base key.
//
// It takes as its arguments the base key, the derivation algorithm to use, and the length of the bit string to derive.
// It returns a Promise which will be fulfilled with an ArrayBuffer containing the derived bits.
//
// This method is very similar to `SubtleCrypto.DeriveKey`, except that `SubtleCrypto.DeriveKey` returns
// a `CryptoKey` object rather than an ArrayBuffer. Essentially `SubtleCrypto.DeriveKey` is composed
// of `SubtleCrypto.DeriveBits` followed by `SubtleCrypto.ImportKey`.
//
// This function supports the same derivation algorithms as deriveKey(): ECDH, HKDF, and PBKDF2
//
// Note that if the `algorithm` parameter is ECDH, the `baseKey` parameter should be the ECDH private key.
// Otherwise it should be the initial key material for the derivation function: for example, for PBKDF2 it might
// be a password, imported as a `CryptoKey` using `SubtleCrypto.ImportKey`.
//
// The `algorithm` parameter should be one of:
//   - an `SubtleCrypto.ECDHKeyDeriveParams` object
//   - an `SubtleCrypto.HKDFParams` object
//   - an `SubtleCrypto.PBKDF2Params` object
//
// The `baseKey` parameter should be a `CryptoKey` object representing the input to the derivation algorithm.
// If `algorithm` is ECDH, then this will be the ECDH private key. Otherwise it will be the initial key material
// for the derivation function: for example, for PBKDF2 it might be a password, imported as a `CryptoKey`
// using `SubtleCrypto.ImportKey`.
//
// The `length` parameter is the number of bits to derive. The number should be a multiple of 8.
func (sc *SubtleCrypto) DeriveBits(algorithm goja.Value, baseKey CryptoKey, length int) *goja.Promise {
	// TODO: implementation
	return nil
}

// ImportKey imports a key: that is, it takes as input a key in an external, portable format and gives you a CryptoKey object that
// you can use in the Web Crypto API. returns a Promise that fulfills with a CryptoKey corresponding
//
// It returns a Promise that fulfills with the imported key as a CryptoKey object.
//
// The `format` parameter identifies the format of the key data.
//
// The `keyData` parameter is the key data, in the format specified by the `format` parameter.
//
// The `algorithm` parameter should be one of:
//   - for RSASSA-PKCS1-v1_5, RSA-PSS or RSA-OAEP: pass an `SubtleCrypto.RSAHashedImportParams` object
//   - for ECDSA or ECDH: pass an `SubtleCrypto.EcKeyImportParams` object
//   - an `SubtleCrypto.HMACImportParams` object
//   - for AES-CTR, AES-CBC, AES-GCM or AES-KW pass the string identifying
//     the algorithm or an object of the form `{ name: ALGORITHM }`, where
//     `ALGORITHM` is the name of the algorithm.
//   - for PBKDF2: pass the string "PBKDF2"
//   - for HKDF: pass the string "HKDF"
func (sc *SubtleCrypto) ImportKey(
	format KeyFormat,
	keyData []byte,
	algorithm goja.Value,
	extractable bool,
	keyUsages []CryptoKeyUsage,
) *goja.Promise {
	// TODO: implementation
	return nil
}

// ExportKey exports a key: that is, it takes as input a CryptoKey object and gives you the key in an external, portable format.
//
// To export a key, the key must have CryptoKey.extractable set to true.
//
// Keys are not exported in an encrypted format: to encrypt keys when exporting them use the SubtleCrypto.wrapKey() API instead.
//
// It returns A Promise:
//   - If format was jwk, then the promise fulfills with a JSON object containing the key.
//   - Otherwise the promise fulfills with an ArrayBuffer containing the key.
//
// The `format` parameter identifies the format of the key data.
// The `key` parameter is the key to export, as a CryptoKey object.
func (sc *SubtleCrypto) ExportKey(format KeyFormat, key CryptoKey) *goja.Promise {
	// TODO
	return nil
}

// WrapKey  "wraps" a key.
//
// This means that it exports the key in an external, portable format, then encrypts the exported key.
// Wrapping a key helps protect it in untrusted environments, such as inside an otherwise unprotected data
// store or in transmission over an unprotected network.
//
// As with `SubtleCrypto.ExportKey`, you specify an export format for the key.
// To export a key, it must have `CryptoKey.Extractable` set to true.
//
// But because `SubtleCrypto.WrapKey“ also encrypts the key to be imported, you
// also need to pass in the key that must be used to encrypt it. This is sometimes called the "wrapping key".
//
// The inverse of `SubtleCrypto.WrapKey` is `SubtleCrypto.UnwrapKey`: while `SubtleCrypto.WrapKey“ is composed
// of export + encrypt, unwrapKey is composed of import + decrypt.
//
// It returns a Promise that fulfills with an ArrayBuffer containing the encrypted exported key.
//
// The `format` parameter identifies the format of the key data.
// The `key` parameter is the key to export, as a CryptoKey object.
// The `wrappingKey` parameter is the key to use to encrypt the exported key. The key **must** have
// the `wrapKey` usage flag set.
// The `wrapAlgorithm` parameter identifies the algorithm to use to encrypt the exported key, and should be one of:
//   - an `SubtleCrypto.RSAOaepParams` object
//   - an `SubtleCrypto.AesCtrParams` object
//   - an `SubtleCrypto.AesCbcParams` object
//   - an `SubtleCrypto.AesGcmParams` object
//   - for the AES-KW algorithm, pass the string "AES-KW", or an object of the form `{ name: "AES-KW" }`
func (sc *SubtleCrypto) WrapKey(format KeyFormat, key CryptoKey, wrappingKey CryptoKey, wrapAlgorithm goja.Value) *goja.Promise {
	// TODO: implementation
	return nil
}

// UnwrapKey "unwraps" a key.
//
// This means that it takes as its input a key that has been exported and then
// encrypted (also called "wrapped"). It decrypts the key and then imports it, returning
// a `CryptoKey` object that can be used in the Web Crypto API.
//
// As with `SubtleCrypto.ImportKey`, you specify the key's import format and other attributes
// of the key to import details such as whether it is extractable, and which operations it can be used for.
//
// But because `SubtleCrypto.UnwrapKey` also decrypts the key to be imported, you also need to pass
// in the key that must be used to decrypt it. This is sometimes called the "unwrapping key".
//
// The inverse of `SubtleCrypto.UnwrapKey` is `SubtleCrypto.WrapKey`: while `SubtleCrypto.UnwrapKey` is composed
// of decrypt + import, `Subtle.WrapKey` is composed of encrypt + export.
//
// It returns a Promise that fulfills with the unwrapped key as a CryptoKey object.
//
// The `format` parameter identifies the format of the key data.
//
// The `wrappedKey` parameter is the key to unwrap.
//
// The `unwrappingKey` parameter is the key to use to decrypt the wrapped key. The key **must** have
//
// the `unwrapKey` usage flag set.
//
// The `unwrapAlgorithm` parameter identifies the algorithm to use to decrypt the wrapped key, and should be one of:
//   - an `SubtleCrypto.RSAOaepParams` object
//   - an `SubtleCrypto.AesCtrParams` object
//   - an `SubtleCrypto.AesCbcParams` object
//   - an `SubtleCrypto.AesGcmParams` object
//   - for the AES-KW algorithm, pass the string "AES-KW", or an object of the form `{ name: "AES-KW" }`
//
// The `unwrappedKeyAlgorithm` parameter identifies the algorithm to use to import the unwrapped key, and should be one of:
//   - for RSASSA-PKCS1-v1_5, RSA-PSS or RSA-OAEP: pass an `SubtleCrypto.RSAHashedImportParams` object
//   - for ECDSA or ECDH: pass an `SubtleCrypto.EcKeyImportParams` object
//   - for HMAC: pass an `SubtleCrypto.HMACImportParams` object
//   - for AES-CTR, AES-CBC, AES-GCM or AES-KW pass the string identifying the algorithm or an object of the form
//     `{ name: ALGORITHM }`, where `ALGORITHM` is the name of the algorithm.
//
// The `extractable` parameter identifies whether the key is extractable.
//
// The `keyUsages` parameter identifies the operations that the key can be used for.
func (sc *SubtleCrypto) UnwrapKey(
	format KeyFormat,
	wrappedKey []byte,
	unwrappingKey CryptoKey,
	unwrapAlgo goja.Value,
	unwrappedKeyAlgo goja.Value,
	extractable bool,
	keyUsages []CryptoKeyUsage) *goja.Promise {
	// TODO: implementation
	return nil
}

// makeHandledPromise will create a promise and return its resolve and reject methods,
// wrapped in such a way that it will block the eventloop from exiting before they are
// called even if the promise isn't resolved by the time the current script ends executing.
// func (sc *SubtleCrypto[C, E, D, S, DK, KD, W]) makeHandledPromise() (*goja.Promise, func(interface{}), func(interface{})) {
func (sc *SubtleCrypto) makeHandledPromise() (*goja.Promise, func(interface{}), func(interface{})) {
	runtime := sc.vu.Runtime()
	callback := sc.vu.RegisterCallback()
	p, resolve, reject := runtime.NewPromise()

	return p, func(i interface{}) {
			// more stuff
			callback(func() error {
				resolve(i)
				return nil
			})
		}, func(i interface{}) {
			// more stuff
			callback(func() error {
				reject(i)
				return nil
			})
		}
}
