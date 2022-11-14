package webcrypto

import (
	"errors"
	"fmt"
	"strings"
)

// AlgorithmIdentifier represents the name of an algorithm.
// As defined by the [specification]
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#algorithm-dictionary
type AlgorithmIdentifier string

const (
	// Sha1 represents the SHA-1 algorithm.
	Sha1 AlgorithmIdentifier = "SHA-1"

	// Sha256 represents the SHA-256 algorithm.
	Sha256 = "SHA-256"

	// Sha384 represents the SHA-384 algorithm.
	Sha384 = "SHA-384"

	// Sha512 represents the SHA-512 algorithm.
	Sha512 = "SHA-512"

	// RSASsaPkcs1V15 represents the RSA-SHA1 algorithm.
	RSASsaPkcs1V15 = "RSASSA-PKCS1-v1_5"

	// RSAPss represents the RSA-PSS algorithm.
	RSAPss = "RSA-PSS"

	// RSAOaep represents the RSA-OAEP algorithm.
	RSAOaep = "RSA-OAEP"

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
	AlgorithmIdentifierECDH = "ECDH"
)

// NormalizeAlgorithm normalizes the given algorithm following the algorithm described in the WebCrypto [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#algorithm-normalization-normalize-an-algorithm
func NormalizeAlgorithm(algorithm interface{}, op OperationIdentifier) (Algorithm, error) {
	var initialAlg Algorithm

	switch alg := algorithm.(type) {
	default:
		return Algorithm{}, fmt.Errorf("%w; reason: unsupported type argument", ErrNormalizationFailed)
	case string:
		return NormalizeAlgorithm(Algorithm{Name: AlgorithmIdentifier(alg)}, op)
	case map[string]interface{}:
		if _, ok := alg["name"]; !ok {
			return Algorithm{}, fmt.Errorf("%w; reason: provided Object argument lacks 'name' property", ErrNormalizationFailed)
		}

		if _, ok := alg["name"].(string); !ok {
			return Algorithm{}, fmt.Errorf("%w; reason: provided Object argument's 'name' property is not a string", ErrNormalizationFailed)
		}

		initialAlg = Algorithm{Name: AlgorithmIdentifier(alg["name"].(string))}

	}

	// 1.
	registeredAlgorithms, ok := supportedAlgorithms[op]
	if !ok {
		return Algorithm{}, fmt.Errorf("%w; reason: invalid operation name: %s", NotSupportedError, op)
	}

	// 2. 3. 4.
	algName := initialAlg.Name

	// 5.
	var desiredType *string
	for key, value := range registeredAlgorithms {
		if strings.EqualFold(string(key), string(algName)) {
			algName = key
			desiredType = value
		}
	}

	// No further operation is needed if the algorithm does not have a desired type.
	if desiredType == nil {
		return Algorithm{Name: algName}, nil
	}

	// 6.
	// TODO: Implement the normalization algorithm for the rest of the operations
	// and algorithms when needed.

	return Algorithm{}, nil
}

// ErrNormalizationFailed is an error that is returned when the normalizeAlgorithm
// operation fails.
var ErrNormalizationFailed = errors.New("failed to normalize algorithm")

func IsSupportedAlgorithm(algorithm string) bool {
	switch AlgorithmIdentifier(algorithm) {
	case Sha1, Sha256, Sha384, Sha512:
		return true
	}

	return false
}

var supportedAlgorithms = map[OperationIdentifier]map[AlgorithmIdentifier]*string{
	OperationIdentifierDigest: {
		Sha1:   nil,
		Sha256: nil,
		Sha384: nil,
		Sha512: nil,
	},
}

// Algorithm represents
type Algorithm struct {
	Name AlgorithmIdentifier
}

// OperationIdentifier represents the name of an operation.
type OperationIdentifier string

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
