package webcrypto

import (
	"crypto/hmac"
	"errors"
	"fmt"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules"
)

// FIXME: SubtleCrypto is described as an "interface", should it be a nested module
// with top-level functions instead then? (as opposed to a struct as it is now)

// FIXME: Make sure we cover the complete range of errors, and that their value makes sense

// SubtleCrypto represents the SubtleCrypto interface of the Web Crypto API.
type SubtleCrypto struct {
	vu modules.VU
}

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
func (sc *SubtleCrypto) Encrypt(algorithm, key, data goja.Value) *goja.Promise {
	rt := sc.vu.Runtime()
	promise, resolve, reject := sc.makeHandledPromise()

	// 2.
	// We obtain a copy of the key data, because we might need to modify it.
	plaintext, err := exportArrayBuffer(rt, data)
	if err != nil {
		reject(err)
		return promise
	}

	// 3.
	normalized, err := normalizeAlgorithm(rt, algorithm, OperationIdentifierEncrypt)
	if err != nil {
		reject(err)
		return promise
	}

	var ck CryptoKey
	if err = rt.ExportTo(key, &ck); err != nil {
		reject(NewError(0, TypeError, "key argument does hold not a valid CryptoKey object"))
		return promise
	}

	keyAlgorithmNameValue, err := traverseObject(rt, key.ToObject(rt), "algorithm", "name")
	if err != nil {
		reject(err)
		return promise
	}

	// 8.
	if normalized.Name != keyAlgorithmNameValue.String() {
		reject(NewError(0, InvalidAccessError, "algorithm name does not match key algorithm name"))
		return promise
	}

	encrypter, err := newEncryptDecrypter(rt, normalized, algorithm)
	if err != nil {
		reject(err)
		return promise
	}

	go func() {
		// 9.
		if !ck.ContainsUsage(EncryptCryptoKeyUsage) {
			reject(NewError(0, InvalidAccessError, "key does not contain the 'encrypt' usage"))
			return
		}

		var ciphertext []byte

		switch normalized.Name {
		case AESCbc, AESCtr, AESGcm:
			// 10.
			ciphertext, err = encrypter.Encrypt(plaintext, ck)
			if err != nil {
				reject(err)
				return
			}

			resolve(rt.NewArrayBuffer(ciphertext))
			return
		default:
			reject(NewError(0, NotSupportedError, fmt.Sprintf("unsupported algorithm %q", normalized.Name)))
			return
		}
	}()

	return promise
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
func (sc *SubtleCrypto) Decrypt(algorithm, key, data goja.Value) *goja.Promise {
	rt := sc.vu.Runtime()
	promise, resolve, reject := sc.makeHandledPromise()

	// 2.
	// We obtain a copy of the key data, because we might need to modify it.
	ciphertext, err := exportArrayBuffer(rt, data)
	if err != nil {
		reject(err)
		return promise
	}

	// 3.
	normalized, err := normalizeAlgorithm(rt, algorithm, OperationIdentifierDecrypt)
	if err != nil {
		reject(err)
		return promise
	}

	var ck CryptoKey
	if err = rt.ExportTo(key, &ck); err != nil {
		reject(NewError(0, InvalidAccessError, "key argument does hold not a valid CryptoKey object"))
		return promise
	}

	keyAlgorithmNameValue, err := traverseObject(rt, key.ToObject(rt), "algorithm", "name")
	if err != nil {
		reject(err)
		return promise
	}

	// 8.
	if normalized.Name != keyAlgorithmNameValue.String() {
		reject(NewError(0, InvalidAccessError, "algorithm name does not match key algorithm name"))
		return promise
	}

	decrypter, err := newEncryptDecrypter(rt, normalized, algorithm)
	if err != nil {
		reject(err)
		return promise
	}

	go func() {
		// 9.
		if !ck.ContainsUsage(DecryptCryptoKeyUsage) {
			reject(NewError(0, InvalidAccessError, "key does not contain the 'decrypt' usage"))
			return
		}

		var plaintext []byte

		switch normalized.Name {
		case AESCbc, AESCtr, AESGcm:
			// 10.
			plaintext, err = decrypter.Decrypt(ciphertext, ck)
			if err != nil {
				reject(err)
				return
			}

			resolve(rt.NewArrayBuffer(plaintext))
		default:
			reject(NewError(0, NotSupportedError, fmt.Sprintf("unsupported algorithm %q", normalized.Name)))
		}
	}()

	return promise
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
func (sc *SubtleCrypto) Sign(algorithm, key, data goja.Value) *goja.Promise {
	rt := sc.vu.Runtime()
	promise, resolve, reject := sc.makeHandledPromise()

	// 2.
	// We obtain a copy of the key data, because we might need to modify it.
	dataToSign, err := exportArrayBuffer(rt, data)
	if err != nil {
		reject(err)
		return promise
	}

	// 3.
	normalized, err := normalizeAlgorithm(rt, algorithm, OperationIdentifierSign)
	if err != nil {
		reject(err)
		return promise
	}

	var ck CryptoKey
	if err = rt.ExportTo(key, &ck); err != nil {
		reject(NewError(0, InvalidAccessError, "key argument does hold not a valid CryptoKey object"))
		return promise
	}

	keyAlgorithmNameValue, err := traverseObject(rt, key.ToObject(rt), "algorithm", "name")
	if err != nil {
		reject(err)
		return promise
	}

	go func() {
		// 8.
		if normalized.Name != keyAlgorithmNameValue.String() {
			reject(NewError(0, InvalidAccessError, "algorithm name does not match key algorithm name"))
			return
		}

		// 9.
		for !ck.ContainsUsage(SignCryptoKeyUsage) {
			reject(NewError(0, InvalidAccessError, "key does not contain the 'sign' usage"))
			return
		}

		// 10.
		switch normalized.Name {
		case HMAC:
			keyAlgorithm, ok := ck.Algorithm.(HmacKeyAlgorithm)
			if !ok {
				reject(NewError(0, InvalidAccessError, "key algorithm does not describe a HMAC key"))
				return
			}

			keyHandle, ok := ck.handle.([]byte)
			if !ok {
				reject(NewError(0, InvalidAccessError, "key handle is of incorrect type"))
				return
			}

			hashFn, err := keyAlgorithm.HashFn()
			if err != nil {
				reject(err)
				return
			}

			hasher := hmac.New(hashFn, keyHandle)
			hasher.Write(dataToSign)

			// 10.
			mac := hasher.Sum(nil)

			resolve(rt.NewArrayBuffer(mac))
		default:
			reject(NewError(0, NotSupportedError, fmt.Sprintf("unsupported algorithm %q", normalized.Name)))
		}
	}()

	return promise
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
func (sc *SubtleCrypto) Verify(algorithm, key, signature, data goja.Value) *goja.Promise {
	rt := sc.vu.Runtime()
	promise, resolve, reject := sc.makeHandledPromise()

	// 2.
	signatureData, err := exportArrayBuffer(sc.vu.Runtime(), signature)
	if err != nil {
		reject(err)
		return promise
	}

	// 3.
	signedData, err := exportArrayBuffer(sc.vu.Runtime(), data)
	if err != nil {
		reject(err)
		return promise
	}

	// 4.
	normalizedAlgorithm, err := normalizeAlgorithm(rt, algorithm, OperationIdentifierVerify)
	if err != nil {
		reject(err)
		return promise
	}

	var ck CryptoKey
	if err = rt.ExportTo(key, &ck); err != nil {
		reject(NewError(0, InvalidAccessError, "key argument does hold not a valid CryptoKey object"))
		return promise
	}

	keyAlgorithmNameValue, err := traverseObject(rt, key, "algorithm", "name")
	if err != nil {
		reject(err)
		return promise
	}

	go func() {
		// 9.
		if normalizedAlgorithm.Name != keyAlgorithmNameValue.String() {
			reject(NewError(0, InvalidAccessError, "algorithm name does not match key algorithm name"))
			return
		}

		// 10.
		for !ck.ContainsUsage(VerifyCryptoKeyUsage) {
			reject(NewError(0, InvalidAccessError, "key does not contain the 'sign' usage"))
			return
		}

		switch normalizedAlgorithm.Name {
		case HMAC:
			keyAlgorithm, ok := ck.Algorithm.(HmacKeyAlgorithm)
			if !ok {
				reject(NewError(0, InvalidAccessError, "key algorithm does not describe a HMAC key"))
				return
			}

			keyHandle, ok := ck.handle.([]byte)
			if !ok {
				reject(NewError(0, InvalidAccessError, "key handle is of incorrect type"))
				return
			}

			hashFn, err := keyAlgorithm.HashFn()
			if err != nil {
				reject(err)
				return
			}

			hasher := hmac.New(hashFn, keyHandle)
			hasher.Write(signedData)

			resolve(hmac.Equal(signatureData, hasher.Sum(nil)))
		default:
			reject(NewError(0, NotSupportedError, fmt.Sprintf("unsupported algorithm %q", normalizedAlgorithm.Name)))
		}
	}()

	return promise
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
func (sc *SubtleCrypto) Digest(algorithm goja.Value, data goja.Value) *goja.Promise {
	promise, resolve, reject := sc.makeHandledPromise()
	rt := sc.vu.Runtime()

	// Validate that the value we received is either an ArrayBuffer, TypedArray, or DataView
	// This uses the technique described in https://github.com/dop251/goja/issues/379#issuecomment-1164441879
	if !IsInstanceOf(sc.vu.Runtime(), data, ArrayBufferConstructor, DataViewConstructor) &&
		!IsTypedArray(sc.vu.Runtime(), data) {
		reject(errors.New("data must be an ArrayBuffer, TypedArray, or DataView"))
		return promise
	}

	// 2.
	// Cast the data to a Goja Object, and, as we're now sure it's
	// either an ArrayBuffer, or a view on an ArrayBuffer, we can
	// get the underlying ArrayBuffer by exporting its buffer property
	// to a Goja ArrayBuffer, and then getting its underlying Go slice
	// by calling the `Bytes()` method.
	//
	// Doing so conviniently also copies the underlying buffer, which
	// is required by the specification.
	// See https://www.w3.org/TR/WebCryptoAPI/#SubtleCrypto-method-digest
	asObject := data.ToObject(rt)
	arrayBuffer, ok := asObject.Get("buffer").Export().(goja.ArrayBuffer)
	if !ok {
		reject(errors.New("could not get ArrayBuffer from data"))
		return promise
	}

	bytes := arrayBuffer.Bytes()

	// The specification explicitly requires us to copy the underlying
	// bytes held by the array buffer
	bytesCopy := make([]byte, len(bytes))
	copy(bytesCopy, bytes)

	// 3.
	normalized, err := normalizeAlgorithm(rt, algorithm, OperationIdentifierDigest)
	if err != nil {
		// "if an error occurred, return a Promise rejected with NormalizedAlgorithm"
		reject(err)
		return promise
	}

	// 6.
	go func() {
		// 6.
		hashFn, ok := getHashFn(normalized.Name)
		if !ok {
			// 7.
			reject(NewError(0, NotSupportedError, "unsupported algorithm: "+normalized.Name))
			return
		}

		// 8.
		hash := hashFn()
		hash.Write(bytes)
		digest := hash.Sum(nil)

		// 9.
		resolve(rt.NewArrayBuffer(digest))
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
	promise, resolve, reject := sc.makeHandledPromise()

	normalized, err := normalizeAlgorithm(sc.vu.Runtime(), algorithm, OperationIdentifierGenerateKey)
	if err != nil {
		reject(err)
		return promise
	}

	keyGenerator, err := newKeyGenerator(sc.vu.Runtime(), normalized, algorithm)
	if err != nil {
		reject(err)
		return promise
	}

	go func() {
		// 7.
		result, err := keyGenerator.GenerateKey(extractable, keyUsages)
		if err != nil {
			reject(err)
			return
		}

		// 8.
		isSecretKey := result.Type == SecretCryptoKeyType
		isPrivateKey := result.Type == PrivateCryptoKeyType
		isUsagesEmpty := len(result.Usages) == 0
		if (isSecretKey || isPrivateKey) && isUsagesEmpty {
			reject(NewError(0, SyntaxError, "usages cannot not be empty for a secret or private CryptoKey"))
			return
		}

		resolve(result)
	}()

	return promise
}

// DeriveKey can be used to derive a secret key from a master key.
//
// It takes as arguments some initial key material, the derivation
// algorithm to use, and the desired properties for the key to derive.
// It returns a Promise which will be fulfilled with a CryptoKey object
// representing the new key.
//
// Note that if the `algorithm` parameter is ECDH, the `baseKey` parameter
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
	baseKey goja.Value,
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
func (sc *SubtleCrypto) DeriveBits(algorithm goja.Value, baseKey goja.Value, length int) *goja.Promise {
	// TODO: implementation
	return nil
}

// ImportKey imports a key: that is, it takes as input a key in an external, portable
// format and gives you a CryptoKey object that you can use in the Web Crypto API.
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
//
// TODO @oleiade: implement support for JWK format
func (sc *SubtleCrypto) ImportKey(
	format KeyFormat,
	keyData goja.Value,
	algorithm goja.Value,
	extractable bool,
	keyUsages []CryptoKeyUsage,
) *goja.Promise {
	rt := sc.vu.Runtime()
	promise, resolve, reject := sc.makeHandledPromise()

	// 2.
	ab, err := exportArrayBuffer(rt, keyData)
	if err != nil {
		reject(err)
		return promise
	}
	keyBytes := make([]byte, len(ab))
	copy(keyBytes, ab)

	// 3.
	normalized, err := normalizeAlgorithm(rt, algorithm, OperationIdentifierImportKey)
	if err != nil {
		reject(err)
		return promise
	}

	ki, err := newKeyImporter(rt, normalized, algorithm)
	if err != nil {
		reject(err)
		return promise
	}

	// 5.
	go func() {
		// 8.
		result, err := ki.ImportKey(format, keyBytes, keyUsages)
		if err != nil {
			reject(err)
			return
		}

		// 9.
		isSecretKey := result.Type == SecretCryptoKeyType
		isPrivateKey := result.Type == PrivateCryptoKeyType
		isUsagesEmpty := len(keyUsages) == 0
		if (isSecretKey || isPrivateKey) && isUsagesEmpty {
			reject(NewError(0, SyntaxError, "usages cannot not be empty for a secret or private CryptoKey"))
			return
		}

		// 10.
		result.Extractable = extractable

		// 11.
		result.Usages = keyUsages

		// 12.
		resolve(result)
	}()

	return promise
}

// ExportKey exports a key: that is, it takes as input a CryptoKey object and gives
// you the key in an external, portable format.
//
// To export a key, the key must have CryptoKey.extractable set to true.
//
// Keys are not exported in an encrypted format: to encrypt keys when exporting
// them use the SubtleCrypto.wrapKey() API instead.
//
// It returns A Promise:
//   - If format was jwk, then the promise fulfills with a JSON object containing the key.
//   - Otherwise the promise fulfills with an ArrayBuffer containing the key.
//
// The `format` parameter identifies the format of the key data.
// The `key` parameter is the key to export, as a CryptoKey object.
//
// TODO @oleiade: implement support for JWK format
func (sc *SubtleCrypto) ExportKey(format KeyFormat, key goja.Value) *goja.Promise {
	rt := sc.vu.Runtime()
	promise, resolve, reject := sc.makeHandledPromise()

	var algorithm Algorithm
	algValue := key.ToObject(rt).Get("algorithm")
	if err := rt.ExportTo(algValue, &algorithm); err != nil {
		reject(NewError(0, SyntaxError, "key is not a valid CryptoKey"))
		return promise
	}

	ck, ok := key.Export().(*CryptoKey)
	if !ok {
		reject(NewError(0, ImplementationError, "unable to extract CryptoKey"))
		return promise
	}

	keyAlgorithmName := key.ToObject(rt).Get("algorithm").ToObject(rt).Get("name").String()
	if algorithm.Name != keyAlgorithmName {
		reject(NewError(0, InvalidAccessError, "algorithm name does not match key algorithm name"))
		return promise
	}

	go func() {
		// 5.
		if !isRegisteredAlgorithm(algorithm.Name, OperationIdentifierExportKey) {
			reject(NewError(0, NotSupportedError, "unsupported algorithm "+algorithm.Name))
			return
		}

		// 6.
		if !ck.Extractable {
			reject(NewError(0, InvalidAccessError, "the key is not extractable"))
			return
		}

		var result []byte
		var err error

		switch keyAlgorithmName {
		case AESCbc, AESCtr, AESGcm:
			result, err = exportAESKey(ck, format)
			if err != nil {
				reject(err)
				return
			}
		case HMAC:
			result, err = exportHmacKey(ck, format)
			if err != nil {
				reject(err)
				return
			}
		default:
			reject(NewError(0, NotSupportedError, "unsupported algorithm "+keyAlgorithmName))
			return
		}

		resolve(rt.NewArrayBuffer(result))
	}()

	return promise
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
func (sc *SubtleCrypto) WrapKey(
	format KeyFormat,
	key goja.Value,
	wrappingKey goja.Value,
	wrapAlgorithm goja.Value,
) *goja.Promise {
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
// The `unwrappedKeyAlgorithm` parameter identifies the algorithm to use to import the unwrapped
// key, and should be one of:
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
	unwrappingKey goja.Value,
	unwrapAlgo goja.Value,
	unwrappedKeyAlgo goja.Value,
	extractable bool,
	keyUsages []CryptoKeyUsage,
) *goja.Promise {
	// TODO: implementation
	return nil
}
