package webcrypto

import (
	"crypto/pbkdf2"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
)

// PBKDF2ImportParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.ImportKey`, when generating any password based key: that is, when the
// algorithm is identified as PBKDF2.
type PBKDF2KeyImportParams struct {
	Algorithm
}

func newPBKDF2ImportParams(normalized Algorithm) (*PBKDF2KeyImportParams, error) {
	return &PBKDF2KeyImportParams{
		Algorithm: normalized,
	}, nil
}

func (keyParams PBKDF2KeyImportParams) ImportKey(
	format KeyFormat,
	keyData []byte,
	keyUsages []CryptoKeyUsage,
	extractable bool,
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
		Algorithm: keyParams.Algorithm,
		Type:      SecretCryptoKeyType,
		handle:    keyData,
	}, nil
}

// Ensure that PBKDF2ImportParams implements the KeyImporter interface.
var _ KeyImporter = &PBKDF2KeyImportParams{}
var _ BitsDeriver = &PBKDF2Params{}
var _ KeyDeriver = &PBKDF2Params{}

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

func (keyParams PBKDF2Params) DeriveBits(
	rt *sobek.Runtime,
	baseKey sobek.Value,
	length int,
) ([]byte, error) {

	pk, err := validateBaseKey(rt, baseKey, OperationIdentifierDeriveBits)
	if err != nil {
		return nil, err
	}

	hashFn, ok := getHashFn(keyParams.Hash)
	if !ok {
		return nil, NewError(NotSupportedError, "hash function not supported")
	}

	keyAlgName, err := traverseObject(rt, baseKey, "algorithm", "name")
	if err != nil {
		return nil, err
	}

	if keyAlgName.String() != keyParams.Name {
		return nil, NewError(OperationError, "provided basekey algorithm and deriveKey algorithm name dont match "+keyAlgName.String()+"!="+keyParams.Name)
	}

	keyLen := length / 8

	dk, err := pbkdf2.Key(hashFn, string(pk), keyParams.Salt, keyParams.Iterations, keyLen)
	if err != nil {
		return nil, err
	}

	return dk, nil
}

func (keyParams PBKDF2Params) DeriveKey(
	rt *sobek.Runtime,
	baseKey sobek.Value,
	derivedKeyType sobek.Value,
	ki KeyImporter,
	kgl KeyGetLengther,
	keyUsages []CryptoKeyUsage,
	extractable bool,
) (*CryptoKey, error) {
	pk, err := validateBaseKey(rt, baseKey, OperationIdentifierDeriveKey)
	if err != nil {
		return nil, err
	}

	hashFn, ok := getHashFn(keyParams.Hash)
	if !ok {
		return nil, NewError(NotSupportedError, "hash function not supported")
	}

	keyAlgName, err := traverseObject(rt, baseKey, "algorithm", "name")
	if err != nil {
		return nil, err
	}

	if keyAlgName.String() != keyParams.Name {
		return nil, NewError(InvalidAccessError, "provided basekey algorithm and deriveKey algorithm name dont match "+keyAlgName.String()+"!="+keyParams.Name)
	}

	keyLengthBits, err := kgl.GetKeyLength(rt, derivedKeyType)
	if err != nil {
		return nil, err
	}

	if keyLengthBits%8 != 0 {
		return nil, NewError(InvalidAccessError, "provided length of key must be a multiple of 8")
	}

	dk, err := pbkdf2.Key(hashFn, string(pk), keyParams.Salt, keyParams.Iterations, keyLengthBits/8)
	if err != nil {
		return nil, err
	}

	derivedKey, err := ki.ImportKey("raw", dk, keyUsages, extractable)
	if err != nil {
		return nil, err
	}

	return derivedKey, nil
}

func validateBaseKey(rt *sobek.Runtime, baseKey sobek.Value, usage CryptoKeyUsage) ([]byte, error) {
	var password *CryptoKey

	err := rt.ExportTo(baseKey, &password)
	if err != nil {
		return nil, NewError(InvalidAccessError, "provided baseKey is not a valid CryptoKey")
	}

	err = password.Validate()
	if err != nil {
		return nil, err
	}

	if password.Type != SecretCryptoKeyType {
		return nil, NewError(InvalidAccessError, "algorithm's password key is not a secret key")
	}

	if !password.ContainsUsage(usage) {
		return nil, NewError(InvalidAccessError, "provided baseKey doesn't contain `"+usage+"` usage")
	}

	pk, ok := password.handle.([]byte)
	if !ok {
		return nil, NewError(InvalidAccessError, "provided baseKey is not a valid PBKDF2 Crypto Key")
	}

	return pk, nil
}
