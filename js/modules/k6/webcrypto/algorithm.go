package webcrypto

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/grafana/sobek"
)

// Algorithm represents
type Algorithm struct {
	Name AlgorithmIdentifier `js:"name"`
}

// AlgorithmIdentifier represents the name of an algorithm.
// As defined by the [specification]
//
// Note that it is defined as an alias of string, instead of a dedicated type,
// to ensure it is handled as a string by sobek.
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

// HashAlgorithmIdentifier represents the name of a hash algorithm.
//
// Note that it is defined as an alias of string, instead of a dedicated type,
// to ensure it is handled as a string under the hood by sobek.
type HashAlgorithmIdentifier = AlgorithmIdentifier

const (
	// SHA1 represents the SHA-1 algorithm.
	SHA1 HashAlgorithmIdentifier = "SHA-1"

	// SHA256 represents the SHA-256 algorithm.
	SHA256 = "SHA-256"

	// SHA384 represents the SHA-384 algorithm.
	SHA384 = "SHA-384"

	// SHA512 represents the SHA-512 algorithm.
	SHA512 = "SHA-512"
)

// OperationIdentifier represents the name of an operation.
//
// Note that it is defined as an alias of string, instead of a dedicated type,
// to ensure it is handled as a string by sobek.
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

// normalizeAlgorithm normalizes the given algorithm following the
// algorithm described in the WebCrypto [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#algorithm-normalization-normalize-an-algorithm
func normalizeAlgorithm(rt *sobek.Runtime, v sobek.Value, op AlgorithmIdentifier) (Algorithm, error) {
	var algorithm Algorithm

	// "if alg is an instance of a DOMString: return the result of the running the
	// normalize algorithm, with the `alg` set to a new Algorithm object whose name
	// attribute is set to alg, and with the op set to op."
	if v.ExportType().Kind() == reflect.String {
		algorithmString, ok := v.Export().(string)
		if !ok {
			return Algorithm{}, NewError(ImplementationError, "algorithm cannot be interpreted as a string")
		}

		algorithmObject := rt.NewObject()
		if err := algorithmObject.Set("name", algorithmString); err != nil {
			return Algorithm{}, NewError(ImplementationError, "unable to transform algorithm string into an object")
		}

		return normalizeAlgorithm(rt, algorithmObject, op)
	}

	if err := rt.ExportTo(v, &algorithm); err != nil {
		return Algorithm{}, NewError(SyntaxError, "algorithm cannot be interpreted as a string or an object")
	}

	algorithm.Name = normalizeAlgorithmName(algorithm.Name)

	if !isRegisteredAlgorithm(algorithm.Name, op) {
		return Algorithm{}, NewError(
			NotSupportedError,
			fmt.Sprintf("algorithm %q doesn't support (in implementation) operation %q", algorithm.Name, op),
		)
	}

	return algorithm, nil
}

func normalizeAlgorithmName(name string) string {
	// Algorithm identifiers are always upper cased.
	// A registered algorithm provided in lower case format, should
	// be considered valid.
	name = strings.ToUpper(name)

	// exception is made for RSASSA-PKCS1-v1_5
	if name == strings.ToUpper(RSASsaPkcs1v15) {
		return RSASsaPkcs1v15
	}

	return name
}

// isRegisteredAlgorithm returns true if the given algorithm name is registered
// for the given operation. As per steps 1. and 5. of the WebCrypto specification's
// "[algorithm normalization]" algorithm.
//
// [algorithm normalization]: https://www.w3.org/TR/WebCryptoAPI/#algorithm-normalization-normalize-an-algorithm
func isRegisteredAlgorithm(algorithmName string, forOperation string) bool {
	switch forOperation {
	case OperationIdentifierDigest:
		return isHashAlgorithm(algorithmName)
	case OperationIdentifierGenerateKey:
		// FIXME: the presence of the hash algorithm here is for HMAC support and should be handled separately
		return isAesAlgorithm(algorithmName) ||
			isHashAlgorithm(algorithmName) ||
			algorithmName == HMAC ||
			isEllipticCurve(algorithmName) ||
			isRSAAlgorithm(algorithmName)
	case OperationIdentifierExportKey, OperationIdentifierImportKey:
		return isAesAlgorithm(algorithmName) ||
			algorithmName == HMAC ||
			isEllipticCurve(algorithmName) ||
			isRSAAlgorithm(algorithmName)
	case OperationIdentifierEncrypt, OperationIdentifierDecrypt:
		return isAesAlgorithm(algorithmName) || algorithmName == RSAOaep
	case OperationIdentifierSign, OperationIdentifierVerify:
		return algorithmName == HMAC || algorithmName == ECDSA || algorithmName == RSAPss || algorithmName == RSASsaPkcs1v15
	default:
		return false
	}
}

func isAesAlgorithm(algorithmName string) bool {
	return algorithmName == AESCbc || algorithmName == AESCtr || algorithmName == AESGcm || algorithmName == AESKw
}

func isHashAlgorithm(algorithmName string) bool {
	return algorithmName == SHA1 || algorithmName == SHA256 || algorithmName == SHA384 || algorithmName == SHA512
}

func isRSAAlgorithm(algorithmName string) bool {
	return algorithmName == RSASsaPkcs1v15 || algorithmName == RSAPss || algorithmName == RSAOaep
}

// hasAlg an internal interface that helps us to identify
// if a given object has an algorithm method.
type hasAlg interface {
	alg() string
}

func isEllipticCurve(algorithmName string) bool {
	return algorithmName == ECDH || algorithmName == ECDSA
}
