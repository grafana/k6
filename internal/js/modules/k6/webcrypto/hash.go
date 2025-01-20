package webcrypto

import (
	"crypto"
	"hash"
	"reflect"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
)

// getHashFn returns the hash function associated with the given name.
//
// It returns a generator function, that can be used to create a new
// hash.Hash instance.
func getHashFn(name string) (func() hash.Hash, bool) {
	switch name {
	case SHA1:
		return crypto.SHA1.New, true
	case SHA256:
		return crypto.SHA256.New, true
	case SHA384:
		return crypto.SHA384.New, true
	case SHA512:
		return crypto.SHA512.New, true
	default:
		return nil, false
	}
}

// hasHash an internal interface that helps us to identify
// if a given object has a hash method.
type hasHash interface {
	hash() string
}

func mapHashFn(hash AlgorithmIdentifier) (crypto.Hash, error) {
	unknownHash := crypto.Hash(0)

	switch hash {
	case SHA1:
		return crypto.SHA1, nil
	case SHA256:
		return crypto.SHA256, nil
	case SHA384:
		return crypto.SHA384, nil
	case SHA512:
		return crypto.SHA512, nil
	default:
		return unknownHash, NewError(NotSupportedError, "hash algorithm is not supported "+hash)
	}
}

// extractHash tries to extract the hash from the given parameters.
func extractHash(rt *sobek.Runtime, params sobek.Value) (Algorithm, error) {
	v, err := traverseObject(rt, params, "hash")
	if err != nil {
		return Algorithm{}, NewError(SyntaxError, "could not get hash from algorithm parameter")
	}

	if common.IsNullish(v) {
		return Algorithm{}, NewError(TypeError, "hash is null or undefined")
	}

	var hashName string
	if v.ExportType().Kind() == reflect.String {
		// try string first
		if !isHashAlgorithm(v.ToString().String()) {
			return Algorithm{}, NewError(NotSupportedError, "hash algorithm is not supported "+v.ToString().String())
		}

		hashName = v.ToString().String()
	} else {
		// otherwise, it should be an object
		name := v.ToObject(rt).Get("name")
		if common.IsNullish(name) {
			return Algorithm{}, NewError(TypeError, "hash name is null or undefined")
		}

		hashName = name.ToString().String()
	}

	if !isHashAlgorithm(hashName) {
		return Algorithm{}, NewError(NotSupportedError, "hash algorithm is not supported "+hashName)
	}

	return Algorithm{Name: hashName}, nil
}
