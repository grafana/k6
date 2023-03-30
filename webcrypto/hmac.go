package webcrypto

import (
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
