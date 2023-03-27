package webcrypto

import (
	"crypto/rand"
)

// AesKeyGenParams represents the object that should be passed as
// the algorithm parameter into `SubtleCrypto.generateKey`, when generating
// an AES key: that is, when the algorithm is identified as any
// of AES-CBC, AES-CTR, AES-GCM, or AES-KW.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-keygen-params
type AesKeyGenParams struct {
	Algorithm

	// The length, in bits, of the key.
	Length int64 `json:"length"`
}

// GenerateKey generates a new AES key.
func (akgp *AesKeyGenParams) GenerateKey(
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (*CryptoKey, error) {
	for _, usage := range keyUsages {
		switch usage {
		case WrapKeyCryptoKeyUsage, UnwrapKeyCryptoKeyUsage:
			continue
		case EncryptCryptoKeyUsage, DecryptCryptoKeyUsage:
			// At the time of writing, the go standard library [doesn't
			// support AES-KW](https://github.com/golang/go/issues/27599), we
			// might want to revisit this in the future.
			if akgp.Algorithm.Name != AESKw {
				continue
			}

			return nil, NewError(0, SyntaxError, "invalid key usage")
		default:
			return nil, NewError(0, SyntaxError, "invalid key usage")
		}
	}

	if akgp.Length != 128 && akgp.Length != 192 && akgp.Length != 256 {
		return nil, NewError(0, OperationError, "invalid key length")
	}

	randomKey := make([]byte, akgp.Length/8)
	if _, err := rand.Read(randomKey); err != nil {
		// 4.
		return nil, NewError(0, OperationError, "could not generate random key")
	}

	// 5. 6. 7. 8. 9.
	key := CryptoKey{}
	key.Type = SecretCryptoKeyType
	key.Algorithm = AesKeyAlgorithm{
		Algorithm: akgp.Algorithm,
		Length:    akgp.Length,
	}

	// 10.
	key.Extractable = extractable

	// 11.
	key.Usages = keyUsages

	// Set key handle to our random key.
	key.handle = randomKey

	// 12.
	return &key, nil
}

// AesKeyAlgorithm is the algorithm for AES keys as defined in the [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#dfn-AesKeyAlgorithm
type AesKeyAlgorithm struct {
	Algorithm

	Length int64 `json:"length"`
}

// exportAESKey exports an AES key to its raw representation.
//
// TODO @oleiade: support JWK format.
func exportAESKey(key *CryptoKey, format KeyFormat) ([]byte, error) {
	if !key.Extractable {
		return nil, NewError(0, InvalidAccessError, "the key is not extractable")
	}

	// 1.
	if key.handle == nil {
		return nil, NewError(0, OperationError, "the key is not valid, no data")
	}

	switch format {
	case RawKeyFormat:
		handle, ok := key.handle.([]byte)
		if !ok {
			return nil, NewError(0, ImplementationError, "exporting key data's bytes failed")
		}

		return handle, nil
	default:
		// FIXME: note that we do not support JWK format, yet.
		return nil, NewError(0, NotSupportedError, "unsupported key format "+format)
	}
}

// importAESKey imports an AES key from its raw representation, and returns a CryptoKey.
//
// TODO @oleiade: support JWK format.
func importAESKey(
	format KeyFormat,
	algorithm Algorithm,
	data []byte,
	keyUsages []CryptoKeyUsage,
) (*CryptoKey, error) {
	for _, usage := range keyUsages {
		switch usage {
		case EncryptCryptoKeyUsage, DecryptCryptoKeyUsage, WrapKeyCryptoKeyUsage, UnwrapKeyCryptoKeyUsage:
			continue
		default:
			return nil, NewError(0, SyntaxError, "invalid key usage: "+usage)
		}
	}

	switch format {
	case RawKeyFormat:
		var (
			has128Bits = len(data) == 16
			has192Bits = len(data) == 24
			has256Bits = len(data) == 32
		)

		if !has128Bits && !has192Bits && !has256Bits {
			return nil, NewError(0, DataError, "invalid key length")
		}
	default:
		return nil, NewError(0, NotSupportedError, "unsupported key format "+format)
	}

	key := &CryptoKey{
		Algorithm: AesKeyAlgorithm{
			Algorithm: algorithm,
			Length:    int64(len(data) * 8),
		},
		Type:   SecretCryptoKeyType,
		handle: data,
	}

	return key, nil
}
