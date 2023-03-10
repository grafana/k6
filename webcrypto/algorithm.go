package webcrypto

import (
	"crypto"
	"fmt"
	"hash"
	"reflect"
	"strings"

	"github.com/dop251/goja"
)

// Algorithm represents
type Algorithm struct {
	Name AlgorithmIdentifier `json:"name"`
}

// NormalizedName returns the normalized algorithm identifier.
// It implements the NormalizedIdentifier interface.
func (a Algorithm) NormalizedName() AlgorithmIdentifier {
	return a.Name
}

// AlgorithmIdentifier represents the name of an algorithm.
// As defined by the [specification]
//
// Note that it is defined as an alias of string, instead of a dedicated type,
// to ensure it is handled as a string by goja.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#algorithm-dictionary
type AlgorithmIdentifier = string

const (
	// RSASsaPkcs1v15 represents the RSA-SHA1 algorithm.
	RSASsaPkcs1v15 = "RSASSA-PKCS1-v1_5"

	// RSAPss represents the RSA-PSS algorithm.
	RSAPss = "RSA-PSS"

	// RSAOaep represents the RSA-OAEP algorithm.
	RSAOaep = "RSA-OAEP"

	// HMAC represents the HMAC algorithm.
	HMAC = "HMAC"

	// AESCtr represents the AES-CTR algorithm.
	AESCtr = "AES-CTR"

	// AESCbc represents the AES-CBC algorithm.
	AESCbc = "AES-CBC"

	// AESGcm represents the AES-GCM algorithm.
	AESGcm = "AES-GCM"

	// AESKw represents the AES-KW algorithm.
	AESKw = "AES-KW"

	// ECDSA represents the ECDSA algorithm.
	ECDSA = "ECDSA"

	// ECDH represents the ECDH algorithm.
	ECDH = "ECDH"
)

// NormalizeAlgorithmName returns the normalized algorithm name.
//
// As the algorithm name is case-insensitive, we normalize it to
// our internal representation.
func NormalizeAlgorithmName(name string) AlgorithmIdentifier {
	algorithms := [...]AlgorithmIdentifier{
		// RSA
		RSASsaPkcs1v15,
		RSAPss,
		RSAOaep,

		// HMAC
		HMAC,

		// AES
		AESCtr,
		AESCbc,
		AESGcm,
		AESKw,

		// ECDSA
		ECDSA,

		// ECDH
		ECDH,
	}

	for _, alg := range algorithms {
		if strings.EqualFold(name, alg) {
			return alg
		}
	}

	return name
}

// HashAlgorithmIdentifier represents the name of a hash algorithm.
//
// Note that it is defined as an alias of string, instead of a dedicated type,
// to ensure it is handled as a string under the hood by goja.
type HashAlgorithmIdentifier = AlgorithmIdentifier

const (
	// Sha1 represents the SHA-1 algorithm.
	Sha1 HashAlgorithmIdentifier = "SHA-1"

	// Sha256 represents the SHA-256 algorithm.
	Sha256 = "SHA-256"

	// Sha384 represents the SHA-384 algorithm.
	Sha384 = "SHA-384"

	// Sha512 represents the SHA-512 algorithm.
	Sha512 = "SHA-512"
)

// Hasher returns the appropriate hash.Hash for the given algorithm.
func Hasher(algorithm HashAlgorithmIdentifier) (func() hash.Hash, error) {
	switch algorithm {
	case Sha1:
		return crypto.SHA1.New, nil
	case Sha256:
		return crypto.SHA256.New, nil
	case Sha384:
		return crypto.SHA384.New, nil
	case Sha512:
		return crypto.SHA512.New, nil
	}

	return nil, NewError(0, ImplementationError, fmt.Sprintf("unsupported hash algorithm: %s", algorithm))
}

// NormalizedAlgorithm represents a normalized algorithm.
type NormalizedAlgorithm interface {
	NormalizedName() AlgorithmIdentifier
}

// normalize algorithm
// normalizeAlgorithm(algorithm: string | Algorithm, op: string):

// NormalizeAlgorithm normalizes the given algorithm following the algorithm described in the WebCrypto [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#algorithm-normalization-normalize-an-algorithm
func NormalizeAlgorithm(rt *goja.Runtime, algorithm goja.Value, op OperationIdentifier) (NormalizedAlgorithm, error) {
	// 1.
	registeredAlgorithms, ok := supportedAlgorithms[op]
	if !ok {
		return Algorithm{}, NewError(0, ImplementationError, fmt.Sprintf("unsupported operation: %s", op))
	}

	// 2.
	var initialAlg Algorithm
	switch algorithm.ExportType().Kind() {
	case reflect.String:
		obj := rt.NewObject()
		err := obj.Set("name", algorithm.Export())
		if err != nil {
			// 3.
			return Algorithm{}, NewError(0, ImplementationError, "unable to convert the string argument to an object")
		}
		return NormalizeAlgorithm(rt, obj, op)
	case reflect.Map, reflect.Struct:
		err := rt.ExportTo(algorithm, &initialAlg)
		if err != nil {
			// 3.
			return Algorithm{}, NewError(0, SyntaxError, "algorithm object is not a valid algorithm")
		}
	default:
		return Algorithm{}, NewError(0, SyntaxError, "unsupported algorithm type")
	}

	// 4.
	algName := initialAlg.Name

	// 5.
	var desiredType string
	algNameRegistered := false
	for key, value := range registeredAlgorithms {
		if strings.EqualFold(key, algName) {
			algName = key
			desiredType = value
			algNameRegistered = true
			break
		}
	}

	if !algNameRegistered {
		return Algorithm{}, NewError(0, NotSupportedError, fmt.Sprintf("unsupported algorithm name: %s", algName))
	}

	// No further operation is needed if the algorithm does not have a desired type.
	if desiredType == "" {
		return Algorithm{Name: algName}, nil
	}

	// 6.
	err := NewError(0, ImplementationError, fmt.Sprintf("unsupported algorithm type: %s", desiredType))
	return Algorithm{}, err
}

// As defined by the [specification]
// [specification]: https://w3c.github.io/webcrypto/#algorithm-normalization-internal
//
//nolint:gochecknoglobals
var supportedAlgorithms = map[OperationIdentifier]map[AlgorithmIdentifier]string{
	OperationIdentifierDigest: {
		Sha1:   "",
		Sha256: "",
		Sha384: "",
		Sha512: "",
	},
	OperationIdentifierGenerateKey: {
		RSASsaPkcs1v15: "RsaHashedKeyGenParams",
		RSAPss:         "RsaHashedKeyGenParams",
		RSAOaep:        "RsaHashedKeyGenParams",
		ECDSA:          "EcKeyGenParams",
		ECDH:           "EcKeyGenParams",
		HMAC:           "HmacKeyGenParams",
		AESCtr:         "AesKeyGenParams",
		AESCbc:         "AesKeyGenParams",
		AESGcm:         "AesKeyGenParams",
		AESKw:          "AesKeyGenParams",
	},
}

// IsAlgorithm returns true if the given algorithm is supported by the library.
func IsAlgorithm(algorithm string) bool {
	algorithms := [...]AlgorithmIdentifier{
		// RSA
		RSASsaPkcs1v15,
		RSAPss,
		RSAOaep,

		// HMAC
		HMAC,

		// AES
		AESCtr,
		AESCbc,
		AESGcm,
		AESKw,

		// ECDSA
		ECDSA,

		// ECDH
		ECDH,
	}

	for _, alg := range algorithms {
		if strings.EqualFold(alg, algorithm) {
			return true
		}
	}

	return false
}

// IsHashAlgorithm returns true if the given cryptographic hash algorithm
// name is valid and supported by the library.
func IsHashAlgorithm(algorithm string) bool {
	algorithms := [...]HashAlgorithmIdentifier{
		Sha1,
		Sha256,
		Sha384,
		Sha512,
	}

	for _, alg := range algorithms {
		if strings.EqualFold(alg, algorithm) {
			return true
		}
	}

	return false
}

// OperationIdentifier represents the name of an operation.
//
// Note that it is defined as an alias of string, instead of a dedicated type,
// to ensure it is handled as a string by goja.
type OperationIdentifier = string

const (
	// OperationIdentifierSign represents the sign operation.
	OperationIdentifierSign OperationIdentifier = "sign"

	// OperationIdentifierVerify represents the verify operation.
	OperationIdentifierVerify OperationIdentifier = "verify"

	// OperationIdentifierEncrypt represents the encrypt operation.
	OperationIdentifierEncrypt OperationIdentifier = "encrypt"

	// OperationIdentifierDecrypt represents the decrypt operation.
	OperationIdentifierDecrypt OperationIdentifier = "decrypt"

	// OperationIdentifierDeriveBits represents the deriveBits operation.
	OperationIdentifierDeriveBits OperationIdentifier = "deriveBits"

	// OperationIdentifierDeriveKey represents the deriveKey operation.
	OperationIdentifierDeriveKey OperationIdentifier = "deriveKey"

	// OperationIdentifierWrapKey represents the wrapKey operation.
	OperationIdentifierWrapKey OperationIdentifier = "wrapKey"

	// OperationIdentifierUnwrapKey represents the unwrapKey operation.
	OperationIdentifierUnwrapKey OperationIdentifier = "unwrapKey"

	// OperationIdentifierImportKey represents the importKey operation.
	OperationIdentifierImportKey OperationIdentifier = "importKey"

	// OperationIdentifierExportKey represents the exportKey operation.
	OperationIdentifierExportKey OperationIdentifier = "exportKey"

	// OperationIdentifierGenerateKey represents the generateKey operation.
	OperationIdentifierGenerateKey OperationIdentifier = "generateKey"

	// OperationIdentifierDigest represents the digest operation.
	OperationIdentifierDigest OperationIdentifier = "digest"
)
