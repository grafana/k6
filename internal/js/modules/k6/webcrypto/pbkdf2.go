package webcrypto

import (
	"crypto/pbkdf2"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
)

// PBKDF2KeyImportParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.ImportKey`, when generating any password based key: that is, when the
// algorithm is identified as PBKDF2.
type PBKDF2KeyImportParams struct {
	Algorithm
}

func newPBKDF2ImportParams(normalized Algorithm) *PBKDF2KeyImportParams {
	return &PBKDF2KeyImportParams{
		Algorithm: normalized,
	}
}

// PBKDF2KeyAlgorithm is the algorithm for PBKDF2 keys as defined in the [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#dfn-PBKDF2KeyAlgorithm //TODO: update this to something real
type PBKDF2KeyAlgorithm struct {
	Algorithm
}

// Ensure that PBKDF2ImportParams implements the KeyImporter interface.
var (
	_ KeyImporter = &PBKDF2KeyImportParams{}
	_ BitsDeriver = &PBKDF2Params{}
	_ KeyDeriver  = &PBKDF2Params{}
)

// ImportKey represents the PBKDF2 function that imports the PBKDF2 password as a CryptoSecret
func (keyParams PBKDF2KeyImportParams) ImportKey(
	format KeyFormat,
	keyData []byte,
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (*CryptoKey, error) {
	if format != RawKeyFormat {
		return nil, NewError(NotSupportedError, "invalid format: "+format)
	}

	for _, usage := range keyUsages {
		switch usage {
		case DeriveBitsCryptoKeyUsage, DeriveKeyCryptoKeyUsage:
			continue
		default:
			return nil, NewError(SyntaxError, "invalid key usage: "+usage)
		}
	}

	if extractable {
		return nil, NewError(SyntaxError, "invalid value for param extractable ")
	}

	return &CryptoKey{
		Algorithm: PBKDF2KeyAlgorithm(keyParams),
		Type:      SecretCryptoKeyType,
		handle:    keyData,
	}, nil
}

func newPBKDF2DeriveParams(rt *sobek.Runtime, normalized Algorithm, params sobek.Value) (*PBKDF2Params, error) {
	hashValue, err := traverseObject(rt, params, "hash")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get hash from algorithm parameter")
	}

	normalizedHash, err := normalizeAlgorithm(rt, hashValue, OperationIdentifierDeriveBits)
	if err != nil {
		return nil, err
	}

	iterations, err := traverseObject(rt, params, "iterations")
	if err != nil {
		return nil, err
	}

	numberIterations := iterations.ToInteger()
	if numberIterations == 0 {
		return nil, NewError(OperationError, "number of iterations can't be 0")
	}

	salt, err := traverseObject(rt, params, "salt")
	if err != nil {
		return nil, err
	}

	byteSalt, err := common.ToBytes(salt.Export())
	if err != nil {
		return nil, err
	}

	return &PBKDF2Params{
		Name:       normalized.Name,
		Hash:       normalizedHash.Name,
		Iterations: int(numberIterations),
		Salt:       byteSalt,
	}, nil
}

// DeriveBits represents the PBKDF2 function that derives the key as bits from PBKDF2 params
func (keyParams PBKDF2Params) DeriveBits(
	baseKey *CryptoKey,
	length int,
) ([]byte, error) {
	pk, err := validateBaseKey(baseKey, OperationIdentifierDeriveBits)
	if err != nil {
		return nil, err
	}

	hashFn, ok := getHashFn(keyParams.Hash)
	if !ok {
		return nil, NewError(NotSupportedError, "hash function not supported")
	}

	alg, ok := baseKey.Algorithm.(PBKDF2KeyAlgorithm)
	if !ok {
		return nil, NewError(OperationError, "provided baseKey is not a valid algorithm")
	}

	if alg.Name != keyParams.Name {
		return nil, NewError(OperationError,
			"provided basekey algorithm and deriveKey algorithm name dont match "+alg.Name+"!="+keyParams.Name,
		)
	}

	keyLen := length / 8

	dk, err := pbkdf2.Key(hashFn, string(pk), keyParams.Salt, keyParams.Iterations, keyLen)
	if err != nil {
		return nil, err
	}

	return dk, nil
}

// DeriveKey represents the PBKDF2 function that derives a key from the PBKDF2 Parms
func (keyParams PBKDF2Params) DeriveKey(
	baseKey *CryptoKey,
	ki KeyImporter,
	kgl KeyGetLengther,
	keyUsages []CryptoKeyUsage,
	extractable bool,
) (*CryptoKey, error) {
	pk, err := validateBaseKey(baseKey, OperationIdentifierDeriveKey)
	if err != nil {
		return nil, err
	}

	hashFn, ok := getHashFn(keyParams.Hash)
	if !ok {
		return nil, NewError(NotSupportedError, "hash function not supported")
	}

	alg, ok := baseKey.Algorithm.(PBKDF2KeyAlgorithm)
	if !ok {
		return nil, NewError(OperationError, "provided baseKey is not a valid algorithm")
	}

	if alg.Name != keyParams.Name {
		return nil, NewError(OperationError,
			"provided basekey algorithm and deriveKey algorithm name dont match "+alg.Name+"!="+keyParams.Name,
		)
	}

	keyLengthBits := kgl.GetKeyLength()

	if keyLengthBits%8 != 0 {
		return nil, NewError(InvalidAccessError, "provided length of key must be a multiple of 8")
	}

	dk, err := pbkdf2.Key(hashFn, string(pk), keyParams.Salt, keyParams.Iterations, keyLengthBits/8)
	if err != nil {
		return nil, err
	}

	derivedKey, err := ki.ImportKey("raw", dk, extractable, keyUsages)
	if err != nil {
		return nil, err
	}

	return derivedKey, nil
}

func validateBaseKey(baseKey *CryptoKey, usage CryptoKeyUsage) ([]byte, error) {
	err := baseKey.Validate()
	if err != nil {
		return nil, err
	}

	if baseKey.Type != SecretCryptoKeyType {
		return nil, NewError(InvalidAccessError, "algorithm's password key is not a secret key")
	}

	if !baseKey.ContainsUsage(usage) {
		return nil, NewError(InvalidAccessError, "provided baseKey doesn't contain `"+usage+"` usage")
	}

	pk, ok := baseKey.handle.([]byte)
	if !ok {
		return nil, NewError(InvalidAccessError, "provided baseKey is not a valid PBKDF2 Crypto Key")
	}

	return pk, nil
}
