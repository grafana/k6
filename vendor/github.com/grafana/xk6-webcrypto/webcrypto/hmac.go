package webcrypto

import (
	"crypto/rand"
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"

	"github.com/dop251/goja"
	"gopkg.in/guregu/null.v3"
)

// HmacKeyGenParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.GenerateKey`, when generating an HMAC key.
type HmacKeyGenParams struct {
	Algorithm

	// Hash represents the name of the digest function to use. You can
	// use any of the following: [Sha256], [Sha384],
	// or [Sha512].
	Hash Algorithm `json:"hash"`

	// Length holds (a Number) the length of the key, in bits.
	// If this is omitted, the length of the key is equal to the block size
	// of the hash function you have chosen.
	// Unless you have a good reason to use a different length, omit
	// use the default.
	Length null.Int `json:"length"`
}

// newHmacKeyGenParams creates a new HmacKeyGenParams object, from the normalized
// algorithm, and the params parameters passed by the user.
//
// It handles the logic of extracting the hash algorithm from the params object,
// and normalizing it. It also handles the logic of extracting the length
// attribute from the params object, and setting it to the default value if it's
// not present as described in the hmac `generateKey` [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#hmac-operations
func newHmacKeyGenParams(rt *goja.Runtime, normalized Algorithm, params goja.Value) (*HmacKeyGenParams, error) {
	// The specification doesn't explicitly tell us what to do if the
	// hash field is not present, but we assume it's a mandatory field
	// and throw an error if it's not present.
	hashValue, err := traverseObject(rt, params, "hash")
	if err != nil {
		return nil, NewError(0, SyntaxError, "could not get hash from algorithm parameter")
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

	return &HmacKeyGenParams{
		Algorithm: normalized,
		Hash:      normalizedHash,
		Length:    length,
	}, nil
}

// GenerateKey generates a new HMAC key.
func (hkgp *HmacKeyGenParams) GenerateKey(
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (*CryptoKey, error) {
	// 1.
	for _, usage := range keyUsages {
		switch usage {
		case SignCryptoKeyUsage, VerifyCryptoKeyUsage:
			continue
		default:
			return nil, NewError(0, SyntaxError, "invalid key usage: "+usage)
		}
	}

	// 2.
	// We extract the length attribute from the algorithm object, as it's not
	// part of the normalized algorithm, and as accessing the runtime from the
	// callback below could lead to a race condition.
	if !hkgp.Length.Valid {
		switch hkgp.Hash.Name {
		case Sha1:
			hkgp.Length = null.IntFrom(sha1.BlockSize * 8)
		case Sha256:
			hkgp.Length = null.IntFrom(sha256.BlockSize * 8)
		case Sha384:
			hkgp.Length = null.IntFrom(sha512.BlockSize * 8)
		case Sha512:
			hkgp.Length = null.IntFrom(sha512.BlockSize * 8)
		default:
			// This case should never happen, as the normalization algorithm
			// should have thrown an error if the hash algorithm is invalid.
			return nil, NewError(0, ImplementationError, "invalid hash algorithm: "+hkgp.Hash.Name)
		}
	}

	if hkgp.Length.Int64 == 0 {
		return nil, NewError(0, OperationError, "algorithm's length cannot be 0")
	}

	// 3.
	randomKey := make([]byte, hkgp.Length.Int64/8)
	if _, err := rand.Read(randomKey); err != nil {
		// 4.
		return nil, NewError(0, OperationError, "failed to generate random key; reason:  "+err.Error())
	}

	// 5.
	key := &CryptoKey{Type: SecretCryptoKeyType, handle: randomKey}

	// 6.
	algorithm := HmacKeyAlgorithm{}

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

// Ensure that HmacKeyGenParams implements the KeyGenerator interface.
var _ KeyGenerator = &HmacKeyGenParams{}

// HmacKeyAlgorithm represents the algorithm of an HMAC key.
type HmacKeyAlgorithm struct {
	KeyAlgorithm

	// Hash represents the inner hash function to use.
	Hash KeyAlgorithm `json:"hash"`

	// Length represents he length (in bits) of the key.
	Length int64 `json:"length"`
}

func exportHmacKey(ck *CryptoKey, format KeyFormat) ([]byte, error) {
	// 1.
	if ck.handle == nil {
		return nil, NewError(0, OperationError, "key data is not accesible")
	}

	// 2.
	bits, ok := ck.handle.([]byte)
	if !ok {
		return nil, NewError(0, OperationError, "key underlying data is not of the correct type")
	}

	// 4.
	switch format {
	case RawKeyFormat:
		return bits, nil
	default:
		// FIXME: note that we do not support JWK format, yet #37.
		return nil, NewError(0, NotSupportedError, "unsupported key format "+format)
	}
}

// HashFn returns the hash function to use for the HMAC key.
func (hka *HmacKeyAlgorithm) HashFn() (func() hash.Hash, error) {
	hashFn, ok := getHashFn(hka.Hash.Name)
	if !ok {
		return nil, NewError(0, NotSupportedError, fmt.Sprintf("unsupported key hash algorithm %q", hka.Hash.Name))
	}

	return hashFn, nil
}

// HmacImportParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.GenerateKey`, when generating an HMAC key.
type HmacImportParams struct {
	Algorithm

	// Hash represents the name of the digest function to use. You can
	// use any of the following: [Sha256], [Sha384],
	// or [Sha512].
	Hash Algorithm `json:"hash"`

	// Length holds (a Number) the length of the key, in bits.
	// If this is omitted, the length of the key is equal to the block size
	// of the hash function you have chosen.
	// Unless you have a good reason to use a different length, omit
	// use the default.
	Length null.Int `json:"length"`
}

// newHmacImportParams creates a new HmacImportParams object from the given
// algorithm and params objects.
func newHmacImportParams(rt *goja.Runtime, normalized Algorithm, params goja.Value) (*HmacImportParams, error) {
	// The specification doesn't explicitly tell us what to do if the
	// hash field is not present, but we assume it's a mandatory field
	// and throw an error if it's not present.
	hashValue, err := traverseObject(rt, params, "hash")
	if err != nil {
		return nil, NewError(0, SyntaxError, "could not get hash from algorithm parameter")
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

	return &HmacImportParams{
		Algorithm: normalized,
		Hash:      normalizedHash,
		Length:    length,
	}, nil
}

// ImportKey imports a key from raw key data. It implements the KeyImporter interface.
func (hip *HmacImportParams) ImportKey(
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
			return nil, NewError(0, SyntaxError, "invalid key usage: "+usage)
		}
	}

	// 3.
	var hash KeyAlgorithm

	// 4.
	switch format {
	case RawKeyFormat:
		hash = KeyAlgorithm{Algorithm{Name: hip.Hash.Name}}
	default:
		return nil, NewError(0, NotSupportedError, "unsupported key format "+format)
	}

	// 5. 6.
	length := int64(len(keyData) * 8)
	if length == 0 {
		return nil, NewError(0, DataError, "key length cannot be 0")
	}

	// 7.
	if hip.Length.Valid {
		if hip.Length.Int64 > length {
			return nil, NewError(0, DataError, "key length cannot be greater than the length of the imported data")
		}

		if hip.Length.Int64 < length {
			return nil, NewError(0, DataError, "key length cannot be less than the length of the imported data")
		}

		length = hip.Length.Int64
	}

	// 8.
	key := CryptoKey{
		Type:   SecretCryptoKeyType,
		handle: keyData[:length/8],
	}

	// 9.
	algorithm := HmacKeyAlgorithm{}

	// 10.
	algorithm.Name = HMAC

	// 11.
	algorithm.Length = length

	// 12.
	algorithm.Hash = hash

	// 13.
	key.Algorithm = algorithm

	return &key, nil
}

// Ensure that HmacImportParams implements the KeyImporter interface.
var _ KeyImporter = &HmacImportParams{}
