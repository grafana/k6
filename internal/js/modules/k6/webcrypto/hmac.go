package webcrypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"

	"github.com/grafana/sobek"
	"gopkg.in/guregu/null.v3"
)

// HMACKeyGenParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.GenerateKey`, when generating an HMAC key.
type HMACKeyGenParams struct {
	Algorithm

	// Hash represents the name of the digest function to use. You can
	// use any of the following: [Sha256], [Sha384],
	// or [Sha512].
	Hash Algorithm `js:"hash"`

	// Length holds (a Number) the length of the key, in bits.
	// If this is omitted, the length of the key is equal to the block size
	// of the hash function you have chosen.
	// Unless you have a good reason to use a different length, omit
	// use the default.
	Length null.Int `js:"length"`
}

func (hkgp HMACKeyGenParams) hash() string {
	return hkgp.Hash.Name
}

// newHMACKeyGenParams creates a new HMACKeyGenParams object, from the normalized
// algorithm, and the params parameters passed by the user.
//
// It handles the logic of extracting the hash algorithm from the params object,
// and normalizing it. It also handles the logic of extracting the length
// attribute from the params object, and setting it to the default value if it's
// not present as described in the hmac `generateKey` [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#hmac-operations
func newHMACKeyGenParams(rt *sobek.Runtime, normalized Algorithm, params sobek.Value) (*HMACKeyGenParams, error) {
	// The specification doesn't explicitly tell us what to do if the
	// hash field is not present, but we assume it's a mandatory field
	// and throw an error if it's not present.
	hashValue, err := traverseObject(rt, params, "hash")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get hash from algorithm parameter")
	}

	// Although the specification doesn't explicitly ask us to do so, we
	// normalize the hash algorithm here, as it shares the same definition
	// as the AlgorithmIdentifier type, and we'll need it later on.
	//
	// Note @oleiade: The specification seems to assume that the normalization
	// algorithm will normalize the existing Algorithm fields, and leave
	// the rest untouched. As we are in the context of a statically typed
	// language, we can't do that, so we need to normalize the hash
	// algorithm here.
	normalizedHash, err := normalizeAlgorithm(rt, hashValue, OperationIdentifierGenerateKey)
	if err != nil {
		return nil, err
	}

	// As the length attribute is optional and as the key generation process
	// differentiates unset from zero-values, we need to handle the case
	// where the length attribute is not present in the params object.
	var length null.Int
	algorithmLengthValue, err := traverseObject(rt, params, "length")
	if err == nil {
		length = null.IntFrom(algorithmLengthValue.ToInteger())
	}

	return &HMACKeyGenParams{
		Algorithm: normalized,
		Hash:      normalizedHash,
		Length:    length,
	}, nil
}

// GenerateKey generates a new HMAC key.
func (hkgp *HMACKeyGenParams) GenerateKey(
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (CryptoKeyGenerationResult, error) {
	// 1.
	for _, usage := range keyUsages {
		switch usage {
		case SignCryptoKeyUsage, VerifyCryptoKeyUsage:
			continue
		default:
			return nil, NewError(SyntaxError, "invalid key usage: "+usage)
		}
	}

	// 2.
	// We extract the length attribute from the algorithm object, as it's not
	// part of the normalized algorithm, and as accessing the runtime from the
	// callback below could lead to a race condition.
	if !hkgp.Length.Valid {
		var length bitLength
		switch hkgp.Hash.Name {
		case SHA1:
			length = byteLength(sha1.BlockSize).asBitLength()
		case SHA256:
			length = byteLength(sha256.BlockSize).asBitLength()
		case SHA384:
			length = byteLength(sha512.BlockSize).asBitLength()
		case SHA512:
			length = byteLength(sha512.BlockSize).asBitLength()
		default:
			// This case should never happen, as the normalization algorithm
			// should have thrown an error if the hash algorithm is invalid.
			return nil, NewError(ImplementationError, "invalid hash algorithm: "+hkgp.Hash.Name)
		}

		hkgp.Length = null.IntFrom(int64(length))
	}

	if hkgp.Length.Int64 == 0 {
		return nil, NewError(OperationError, "algorithm's length cannot be 0")
	}

	// 3.
	randomKey := make([]byte, bitLength(hkgp.Length.Int64).asByteLength())
	if _, err := rand.Read(randomKey); err != nil {
		// 4.
		return nil, NewError(OperationError, "failed to generate random key; reason:  "+err.Error())
	}

	// 5.
	key := &CryptoKey{Type: SecretCryptoKeyType, handle: randomKey}

	// 6.
	algorithm := &HMACKeyAlgorithm{}

	// 7.
	algorithm.Name = HMAC
	algorithm.Length = hkgp.Length.Int64

	// 8.
	hash := KeyAlgorithm{}

	// 9.
	hash.Name = hkgp.Hash.Name

	// 10.
	algorithm.Hash = hash

	// 11. 12. 13.
	key.Algorithm = algorithm
	key.Extractable = extractable
	key.Usages = keyUsages

	return key, nil
}

// Ensure that HMACKeyGenParams implements the KeyGenerator interface.
var _ KeyGenerator = &HMACKeyGenParams{}

// HMACKeyAlgorithm represents the algorithm of an HMAC key.
type HMACKeyAlgorithm struct {
	KeyAlgorithm

	// Hash represents the inner hash function to use.
	Hash KeyAlgorithm `js:"hash"`

	// Length represents he length (in bits) of the key.
	Length int64 `js:"length"`
}

func (hka HMACKeyAlgorithm) hash() string {
	return hka.Hash.Name
}

func exportHMACKey(ck *CryptoKey, format KeyFormat) (interface{}, error) {
	// 1.
	if ck.handle == nil {
		return nil, NewError(OperationError, "key data is not accessible")
	}

	// 2.
	bits, ok := ck.handle.([]byte)
	if !ok {
		return nil, NewError(OperationError, "key underlying data is not of the correct type")
	}

	// 4.
	switch format {
	case RawKeyFormat:
		return bits, nil
	case JwkKeyFormat:
		m, err := exportSymmetricJWK(ck)
		if err != nil {
			return nil, NewError(ImplementationError, err.Error())
		}

		return m, nil
	default:
		// FIXME: note that we do not support JWK format, yet #37.
		return nil, NewError(NotSupportedError, "unsupported key format "+format)
	}
}

// HashFn returns the hash function to use for the HMAC key.
func (hka *HMACKeyAlgorithm) HashFn() (func() hash.Hash, error) {
	hashFn, ok := getHashFn(hka.Hash.Name)
	if !ok {
		return nil, NewError(NotSupportedError, fmt.Sprintf("unsupported key hash algorithm %q", hka.Hash.Name))
	}

	return hashFn, nil
}

// HMACImportParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.GenerateKey`, when generating an HMAC key.
type HMACImportParams struct {
	Algorithm

	// Hash represents the name of the digest function to use. You can
	// use any of the following: [Sha256], [Sha384],
	// or [Sha512].
	Hash Algorithm `js:"hash"`

	// Length holds (a Number) the length of the key, in bits.
	// If this is omitted, the length of the key is equal to the block size
	// of the hash function you have chosen.
	// Unless you have a good reason to use a different length, omit
	// use the default.
	Length null.Int `js:"length"`
}

var _ hasHash = (*HMACImportParams)(nil)

func (hip HMACImportParams) hash() string {
	return hip.Hash.Name
}

// newHMACImportParams creates a new HMACImportParams object from the given
// algorithm and params objects.
func newHMACImportParams(rt *sobek.Runtime, normalized Algorithm, params sobek.Value) (*HMACImportParams, error) {
	// The specification doesn't explicitly tell us what to do if the
	// hash field is not present, but we assume it's a mandatory field
	// and throw an error if it's not present.
	hashValue, err := traverseObject(rt, params, "hash")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get hash from algorithm parameter")
	}

	// Although the specification doesn't explicitly ask us to do so, we
	// normalize the hash algorithm here, as it shares the same definition
	// as the AlgorithmIdentifier type, and we'll need it later on.
	//
	// Note @oleiade: The specification seems to assume that the normalization
	// algorithm will normalize the existing Algorithm fields, and leave
	// the rest untouched. As we are in the context of a statically typed
	// language, we can't do that, so we need to normalize the hash
	// algorithm here.
	normalizedHash, err := normalizeAlgorithm(rt, hashValue, OperationIdentifierGenerateKey)
	if err != nil {
		return nil, err
	}

	// As the length attribute is optional and as the key generation process
	// differentiates unset from zero-values, we need to handle the case
	// where the length attribute is not present in the params object.
	var length null.Int
	algorithmLengthValue, err := traverseObject(rt, params, "length")
	if err == nil {
		length = null.IntFrom(algorithmLengthValue.ToInteger())
	}

	return &HMACImportParams{
		Algorithm: normalized,
		Hash:      normalizedHash,
		Length:    length,
	}, nil
}

// ImportKey imports a key from raw key data. It implements the KeyImporter interface.
func (hip *HMACImportParams) ImportKey(
	format KeyFormat,
	keyData []byte,
	keyUsages []CryptoKeyUsage,
) (*CryptoKey, error) {
	// 2.
	for _, usage := range keyUsages {
		switch usage {
		case SignCryptoKeyUsage, VerifyCryptoKeyUsage:
			continue
		default:
			return nil, NewError(SyntaxError, "invalid key usage: "+usage)
		}
	}

	// 3.
	if format != RawKeyFormat && format != JwkKeyFormat {
		return nil, NewError(NotSupportedError, "unsupported key format "+format)
	}

	hash := KeyAlgorithm{Algorithm{Name: hip.Hash.Name}}

	// 4.
	if format == JwkKeyFormat {
		var err error
		keyData, err = extractSymmetricJWK(keyData)
		if err != nil {
			return nil, NewError(DataError, err.Error())
		}
	}

	// 5. 6.
	length := byteLength(len(keyData)).asBitLength()
	if length == 0 {
		return nil, NewError(DataError, "key length cannot be 0")
	}

	// 7.
	if hip.Length.Valid && hip.Length.Int64 != int64(length) {
		return nil, NewError(DataError, "key length cannot be different from the length of the imported data")
	}

	// 8.
	key := CryptoKey{
		Type:   SecretCryptoKeyType,
		handle: keyData[:length.asByteLength()],
	}

	// 9.
	algorithm := HMACKeyAlgorithm{}

	// 10.
	algorithm.Name = HMAC

	// 11.
	algorithm.Length = int64(length)

	// 12.
	algorithm.Hash = hash

	// 13.
	key.Algorithm = algorithm

	return &key, nil
}

// Ensure that HMACImportParams implements the KeyImporter interface.
var _ KeyImporter = &HMACImportParams{}

type hmacSignerVerifier struct{}

// Sign .
func (hmacSignerVerifier) Sign(key CryptoKey, data []byte) ([]byte, error) {
	keyAlgorithm, ok := key.Algorithm.(hasHash)
	if !ok {
		return nil, NewError(InvalidAccessError, "key algorithm does not describe a HMAC key")
	}

	keyHandle, ok := key.handle.([]byte)
	if !ok {
		return nil, NewError(InvalidAccessError, "key handle is of incorrect type")
	}

	hashFn, ok := getHashFn(keyAlgorithm.hash())
	if !ok {
		return nil, NewError(NotSupportedError, "unsupported hash algorithm "+keyAlgorithm.hash())
	}

	hasher := hmac.New(hashFn, keyHandle)
	hasher.Write(data)

	return hasher.Sum(nil), nil
}

// Verify .
func (hmacSignerVerifier) Verify(key CryptoKey, signature, data []byte) (bool, error) {
	keyAlgorithm, ok := key.Algorithm.(hasHash)
	if !ok {
		return false, NewError(InvalidAccessError, "key algorithm does not describe a HMAC key")
	}

	keyHandle, ok := key.handle.([]byte)
	if !ok {
		return false, NewError(InvalidAccessError, "key handle is of incorrect type")
	}

	hashFn, ok := getHashFn(keyAlgorithm.hash())
	if !ok {
		return false, NewError(InvalidAccessError, "key handle is of incorrect type")
	}

	hasher := hmac.New(hashFn, keyHandle)
	hasher.Write(data)

	return hmac.Equal(signature, hasher.Sum(nil)), nil
}
