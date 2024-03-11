package webcrypto

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

// JsonWebKey represents a JSON Web Key (JsonWebKey) key.
type JsonWebKey map[string]interface{} //nolint:stylecheck,revive // we name this type JsonWebKey to match the spec

// Set sets a key-value pair in the JWK.
func (jwk *JsonWebKey) Set(key string, value interface{}) {
	(*jwk)[key] = value
}

// extractSymmetricJWK extracts the symmetric key from a given JWK key (JSON data).
func extractSymmetricJWK(jsonKeyData []byte) ([]byte, error) {
	key, err := jwk.ParseKey(jsonKeyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input as JWK key: %w", err)
	}

	// check if the key is a symmetric key
	sk, ok := key.(jwk.SymmetricKey)
	if !ok {
		return nil, errors.New("input isn't a valid JWK symmetric key")
	}

	return sk.Octets(), nil
}

// exportSymmetricJWK exports a symmetric key as a map of JWK key parameters.
func exportSymmetricJWK(key *CryptoKey) (*JsonWebKey, error) {
	rawKey, ok := key.handle.([]byte)
	if !ok {
		return nil, errors.New("key's handle isn't a byte slice")
	}

	sk, err := jwk.FromRaw(rawKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWK key: %w", err)
	}

	// we do marshal and unmarshal to get the map of JWK key parameters
	// where all standard parameters are present, a proper marshaling is done
	m, err := json.Marshal(sk)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JWK key: %w", err)
	}

	// wrap result into the object that is expected to be returned
	exported := &JsonWebKey{}
	err = json.Unmarshal(m, exported)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWK key: %w", err)
	}

	exported.Set("ext", key.Extractable)
	exported.Set("key_ops", key.Usages)

	algV, err := extractAlg(key.Algorithm, len(rawKey))
	if err != nil {
		return nil, fmt.Errorf("failed to extract algorithm: %w", err)
	}
	exported.Set("alg", algV)

	return exported, nil
}

func extractAlg(inAlg any, keyLen int) (string, error) {
	switch alg := inAlg.(type) {
	case hasHash:
		v := alg.hash()
		if len(v) < 4 {
			return "", errors.New("length of hash algorithm is less than 4: " + v)
		}
		return "HS" + v[4:], nil
	case hasAlg:
		v := alg.alg()
		if len(v) < 4 {
			return "", errors.New("length of named algorithm is less than 4: " + v)
		}

		return fmt.Sprintf("A%d%s", (8 * keyLen), v[4:]), nil
	default:
		return "", fmt.Errorf("unsupported algorithm: %v", inAlg)
	}
}
