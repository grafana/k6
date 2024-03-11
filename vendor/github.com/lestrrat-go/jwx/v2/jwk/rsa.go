package jwk

import (
	"crypto"
	"crypto/rsa"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/lestrrat-go/blackmagic"
	"github.com/lestrrat-go/jwx/v2/internal/base64"
	"github.com/lestrrat-go/jwx/v2/internal/pool"
)

func (k *rsaPrivateKey) FromRaw(rawKey *rsa.PrivateKey) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	d, err := bigIntToBytes(rawKey.D)
	if err != nil {
		return fmt.Errorf(`invalid rsa.PrivateKey: %w`, err)
	}
	k.d = d

	l := len(rawKey.Primes)

	if l < 0 /* I know, I'm being paranoid */ || l > 2 {
		return fmt.Errorf(`invalid number of primes in rsa.PrivateKey: need 0 to 2, but got %d`, len(rawKey.Primes))
	}

	if l > 0 {
		p, err := bigIntToBytes(rawKey.Primes[0])
		if err != nil {
			return fmt.Errorf(`invalid rsa.PrivateKey: %w`, err)
		}
		k.p = p
	}

	if l > 1 {
		q, err := bigIntToBytes(rawKey.Primes[1])
		if err != nil {
			return fmt.Errorf(`invalid rsa.PrivateKey: %w`, err)
		}
		k.q = q
	}

	// dp, dq, qi are optional values
	if v, err := bigIntToBytes(rawKey.Precomputed.Dp); err == nil {
		k.dp = v
	}
	if v, err := bigIntToBytes(rawKey.Precomputed.Dq); err == nil {
		k.dq = v
	}
	if v, err := bigIntToBytes(rawKey.Precomputed.Qinv); err == nil {
		k.qi = v
	}

	// public key part
	n, e, err := rsaPublicKeyByteValuesFromRaw(&rawKey.PublicKey)
	if err != nil {
		return fmt.Errorf(`invalid rsa.PrivateKey: %w`, err)
	}
	k.n = n
	k.e = e

	return nil
}

func rsaPublicKeyByteValuesFromRaw(rawKey *rsa.PublicKey) ([]byte, []byte, error) {
	n, err := bigIntToBytes(rawKey.N)
	if err != nil {
		return nil, nil, fmt.Errorf(`invalid rsa.PublicKey: %w`, err)
	}

	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(rawKey.E))
	i := 0
	for ; i < len(data); i++ {
		if data[i] != 0x0 {
			break
		}
	}
	return n, data[i:], nil
}

func (k *rsaPublicKey) FromRaw(rawKey *rsa.PublicKey) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	n, e, err := rsaPublicKeyByteValuesFromRaw(rawKey)
	if err != nil {
		return fmt.Errorf(`invalid rsa.PrivateKey: %w`, err)
	}
	k.n = n
	k.e = e

	return nil
}

func (k *rsaPrivateKey) Raw(v interface{}) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	var d, q, p big.Int // note: do not use from sync.Pool

	d.SetBytes(k.d)
	q.SetBytes(k.q)
	p.SetBytes(k.p)

	// optional fields
	var dp, dq, qi *big.Int
	if len(k.dp) > 0 {
		dp = &big.Int{} // note: do not use from sync.Pool
		dp.SetBytes(k.dp)
	}

	if len(k.dq) > 0 {
		dq = &big.Int{} // note: do not use from sync.Pool
		dq.SetBytes(k.dq)
	}

	if len(k.qi) > 0 {
		qi = &big.Int{} // note: do not use from sync.Pool
		qi.SetBytes(k.qi)
	}

	var key rsa.PrivateKey

	pubk := newRSAPublicKey()
	pubk.n = k.n
	pubk.e = k.e
	if err := pubk.Raw(&key.PublicKey); err != nil {
		return fmt.Errorf(`failed to materialize RSA public key: %w`, err)
	}

	key.D = &d
	key.Primes = []*big.Int{&p, &q}

	if dp != nil {
		key.Precomputed.Dp = dp
	}
	if dq != nil {
		key.Precomputed.Dq = dq
	}
	if qi != nil {
		key.Precomputed.Qinv = qi
	}
	key.Precomputed.CRTValues = []rsa.CRTValue{}

	return blackmagic.AssignIfCompatible(v, &key)
}

// Raw takes the values stored in the Key object, and creates the
// corresponding *rsa.PublicKey object.
func (k *rsaPublicKey) Raw(v interface{}) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	var key rsa.PublicKey

	n := pool.GetBigInt()
	e := pool.GetBigInt()
	defer pool.ReleaseBigInt(e)

	n.SetBytes(k.n)
	e.SetBytes(k.e)

	key.N = n
	key.E = int(e.Int64())

	return blackmagic.AssignIfCompatible(v, &key)
}

func makeRSAPublicKey(v interface {
	makePairs() []*HeaderPair
}) (Key, error) {
	newKey := newRSAPublicKey()

	// Iterate and copy everything except for the bits that should not be in the public key
	for _, pair := range v.makePairs() {
		switch pair.Key {
		case RSADKey, RSADPKey, RSADQKey, RSAPKey, RSAQKey, RSAQIKey:
			continue
		default:
			//nolint:forcetypeassert
			key := pair.Key.(string)
			if err := newKey.Set(key, pair.Value); err != nil {
				return nil, fmt.Errorf(`failed to set field %q: %w`, key, err)
			}
		}
	}

	return newKey, nil
}

func (k *rsaPrivateKey) PublicKey() (Key, error) {
	return makeRSAPublicKey(k)
}

func (k *rsaPublicKey) PublicKey() (Key, error) {
	return makeRSAPublicKey(k)
}

// Thumbprint returns the JWK thumbprint using the indicated
// hashing algorithm, according to RFC 7638
func (k rsaPrivateKey) Thumbprint(hash crypto.Hash) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	var key rsa.PrivateKey
	if err := k.Raw(&key); err != nil {
		return nil, fmt.Errorf(`failed to materialize RSA private key: %w`, err)
	}
	return rsaThumbprint(hash, &key.PublicKey)
}

func (k rsaPublicKey) Thumbprint(hash crypto.Hash) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	var key rsa.PublicKey
	if err := k.Raw(&key); err != nil {
		return nil, fmt.Errorf(`failed to materialize RSA public key: %w`, err)
	}
	return rsaThumbprint(hash, &key)
}

func rsaThumbprint(hash crypto.Hash, key *rsa.PublicKey) ([]byte, error) {
	buf := pool.GetBytesBuffer()
	defer pool.ReleaseBytesBuffer(buf)

	buf.WriteString(`{"e":"`)
	buf.WriteString(base64.EncodeUint64ToString(uint64(key.E)))
	buf.WriteString(`","kty":"RSA","n":"`)
	buf.WriteString(base64.EncodeToString(key.N.Bytes()))
	buf.WriteString(`"}`)

	h := hash.New()
	if _, err := buf.WriteTo(h); err != nil {
		return nil, fmt.Errorf(`failed to write rsaThumbprint: %w`, err)
	}
	return h.Sum(nil), nil
}

func validateRSAKey(key interface {
	N() []byte
	E() []byte
}, checkPrivate bool) error {
	if len(key.N()) == 0 {
		// Ideally we would like to check for the actual length, but unlike
		// EC keys, we have nothing in the key itself that will tell us
		// how many bits this key should have.
		return fmt.Errorf(`missing "n" value`)
	}
	if len(key.E()) == 0 {
		return fmt.Errorf(`missing "e" value`)
	}
	if checkPrivate {
		if priv, ok := key.(interface{ D() []byte }); ok {
			if len(priv.D()) == 0 {
				return fmt.Errorf(`missing "d" value`)
			}
		} else {
			return fmt.Errorf(`missing "d" value`)
		}
	}

	return nil
}

func (k *rsaPrivateKey) Validate() error {
	if err := validateRSAKey(k, true); err != nil {
		return NewKeyValidationError(fmt.Errorf(`jwk.RSAPrivateKey: %w`, err))
	}
	return nil
}

func (k *rsaPublicKey) Validate() error {
	if err := validateRSAKey(k, false); err != nil {
		return NewKeyValidationError(fmt.Errorf(`jwk.RSAPublicKey: %w`, err))
	}
	return nil
}
