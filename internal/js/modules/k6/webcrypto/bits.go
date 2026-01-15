package webcrypto

import "github.com/grafana/sobek"

// BitsDeriver is the interface implemented by the parameters used to derive bits
type BitsDeriver interface {
	DeriveBits(privateKey *CryptoKey, length int) ([]byte, error)
}

func newBitsDeriver(rt *sobek.Runtime, normalized Algorithm, algorithm sobek.Value) (BitsDeriver, error) {
	var deriver BitsDeriver
	var err error

	switch normalized.Name {
	case ECDH:
		deriver, err = newECDHKeyDeriveParams(rt, normalized, algorithm)
	case PBKDF2:
		deriver, err = newPBKDF2DeriveParams(rt, normalized, algorithm)
	default:
		return nil, NewError(NotSupportedError, "unsupported algorithm for derive bits: "+normalized.Name)
	}

	if err != nil {
		return nil, err
	}

	return deriver, nil
}
