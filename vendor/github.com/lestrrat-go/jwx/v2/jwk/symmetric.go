package jwk

import (
	"crypto"
	"fmt"

	"github.com/lestrrat-go/blackmagic"
	"github.com/lestrrat-go/jwx/v2/internal/base64"
)

func (k *symmetricKey) FromRaw(rawKey []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(rawKey) == 0 {
		return fmt.Errorf(`non-empty []byte key required`)
	}

	k.octets = rawKey

	return nil
}

// Raw returns the octets for this symmetric key.
// Since this is a symmetric key, this just calls Octets
func (k *symmetricKey) Raw(v interface{}) error {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return blackmagic.AssignIfCompatible(v, k.octets)
}

// Thumbprint returns the JWK thumbprint using the indicated
// hashing algorithm, according to RFC 7638
func (k *symmetricKey) Thumbprint(hash crypto.Hash) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	var octets []byte
	if err := k.Raw(&octets); err != nil {
		return nil, fmt.Errorf(`failed to materialize symmetric key: %w`, err)
	}

	h := hash.New()
	fmt.Fprint(h, `{"k":"`)
	fmt.Fprint(h, base64.EncodeToString(octets))
	fmt.Fprint(h, `","kty":"oct"}`)
	return h.Sum(nil), nil
}

func (k *symmetricKey) PublicKey() (Key, error) {
	newKey := newSymmetricKey()

	for _, pair := range k.makePairs() {
		//nolint:forcetypeassert
		key := pair.Key.(string)
		if err := newKey.Set(key, pair.Value); err != nil {
			return nil, fmt.Errorf(`failed to set field %q: %w`, key, err)
		}
	}
	return newKey, nil
}

func (k *symmetricKey) Validate() error {
	if len(k.Octets()) == 0 {
		return NewKeyValidationError(fmt.Errorf(`jwk.SymmetricKey: missing "k" field`))
	}
	return nil
}
