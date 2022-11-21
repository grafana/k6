package webcrypto

import (
	"crypto"
	"fmt"
	"hash"
	"strings"
)

// Algorithm represents
type Algorithm struct {
	Name AlgorithmIdentifier `json:"name"`
}

// Ensure AesKeyGenParams implements the From interface.
var _ From[map[string]interface{}, Algorithm] = Algorithm{}

func (a Algorithm) From(dict map[string]interface{}) (Algorithm, error) {
	algorithm := Algorithm{}
	nameFound := false

	for key, value := range dict {
		if strings.EqualFold(key, "name") {
			name, ok := value.(string)
			if !ok {
				return Algorithm{}, NewWebCryptoError(0, NotSupportedError, "algorithm name is not a string")
			}

			name = strings.ToUpper(name)

			if !IsAlgorithm(name) && !IsHashAlgorithm(name) {
				return Algorithm{}, NewWebCryptoError(0, NotSupportedError, "algorithm name is not supported")
			}

			algorithm.Name = AlgorithmIdentifier(name)
			nameFound = true
			break
		}
	}

	if !nameFound {
		return Algorithm{}, NewWebCryptoError(0, NotSupportedError, "algorithm name is not found")
	}

	return algorithm, nil
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

	// ECDH represents the ECDH algorithm.
	AlgorithmIdentifierECDH = "ECDH"
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
	switch HashAlgorithmIdentifier(algorithm) {
	case Sha1:
		return crypto.SHA1.New, nil
	case Sha256:
		return crypto.SHA256.New, nil
	case Sha384:
		return crypto.SHA384.New, nil
	case Sha512:
		return crypto.SHA512.New, nil
	}

	return nil, NewWebCryptoError(0, ImplementationError, fmt.Sprintf("unsupported hash algorithm: %s", algorithm))
}

// normalize algorithm
// normalizeAlgorithm(algorithm: string | Algorithm, op: string):

// NormalizeAlgorithm normalizes the given algorithm following the algorithm described in the WebCrypto [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#algorithm-normalization-normalize-an-algorithm
func NormalizeAlgorithm(algorithm interface{}, op OperationIdentifier) (interface{}, error) {
	// var initialAlg AlgorithmIdentifier
	var initialAlg map[string]interface{}

	switch alg := algorithm.(type) {
	case string:
		return NormalizeAlgorithm(map[string]interface{}{"name": alg}, op)
	case map[string]interface{}:
		// FIXME: this should call the NewAlgorithmFrom method instead
		if _, ok := alg["name"]; !ok {
			return Algorithm{}, NewWebCryptoError(0, SyntaxError, "algorithm name is not found")
		}

		if _, ok := alg["name"].(string); !ok {
			return Algorithm{}, NewWebCryptoError(0, SyntaxError, "algorithm name is not a string")
		}

		initialAlg = alg
	default:
		return Algorithm{}, NewWebCryptoError(0, ImplementationError, "unsupported algorithm type")
	}

	// 1.
	registeredAlgorithms, ok := supportedAlgorithms[op]
	if !ok {
		return Algorithm{}, NewWebCryptoError(0, ImplementationError, fmt.Sprintf("unsupported operation: %s", op))
	}

	// 2. 3. 4.
	algName := AlgorithmIdentifier(initialAlg["name"].(string))

	// 5.
	var desiredType string
	algNameRegistered := false
	for key, value := range registeredAlgorithms {
		if strings.EqualFold(string(key), string(algName)) {
			algName = key
			desiredType = value
			algNameRegistered = true
			break
		}
	}

	if !algNameRegistered {
		return Algorithm{}, NewWebCryptoError(0, NotSupportedError, fmt.Sprintf("unsupported algorithm name: %s", algName))
	}

	// No further operation is needed if the algorithm does not have a desired type.
	if desiredType == "" {
		return Algorithm{Name: algName}, nil
	}

	// 6.
	// FIXME: the case strings should be constants
	switch desiredType {
	default:
		return Algorithm{}, NewWebCryptoError(0, ImplementationError, fmt.Sprintf("unsupported algorithm type: %s", desiredType))
	}
}

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
		if strings.EqualFold(string(alg), algorithm) {
			return true
		}
	}

	return false
}

func IsHashAlgorithm(algorithm string) bool {
	algorithms := [...]HashAlgorithmIdentifier{
		Sha1,
		Sha256,
		Sha384,
		Sha512,
	}

	for _, alg := range algorithms {
		if strings.EqualFold(string(alg), algorithm) {
			return true
		}
	}

	return false
}

// As defined by the [specification]
// [specification]: https://w3c.github.io/webcrypto/#algorithm-normalization-internal
var supportedAlgorithms = map[OperationIdentifier]map[AlgorithmIdentifier]string{
	OperationIdentifierDigest: {
		AlgorithmIdentifier(Sha1):   "",
		AlgorithmIdentifier(Sha256): "",
		AlgorithmIdentifier(Sha384): "",
		AlgorithmIdentifier(Sha512): "",
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
