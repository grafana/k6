package webcrypto

import (
	"fmt"

	"github.com/grafana/sobek"
)

// Encrypter is an interface for encrypting data.
type Encrypter interface {
	Encrypt(plaintext []byte, key CryptoKey) ([]byte, error)
}

// Decrypter is an interface for decrypting data.
type Decrypter interface {
	Decrypt(ciphertext []byte, key CryptoKey) ([]byte, error)
}

// EncryptDecrypter is an interface for encrypting and decrypting data.
type EncryptDecrypter interface {
	Encrypter
	Decrypter
}

// newEncryptDecrypter instantiates an EncryptDecrypter based on the provided
// algorithm and parameters `sobek.Value`.
//
// The returned instance can be used to encrypt/decrypt data using the
// corresponding AES algorithm.
func newEncryptDecrypter(
	rt *sobek.Runtime,
	algorithm Algorithm,
	params sobek.Value,
) (EncryptDecrypter, error) {
	var ed EncryptDecrypter
	var paramsObjectName string
	var err error

	switch algorithm.Name {
	case AESCbc:
		ed = new(AESCBCParams)
		paramsObjectName = "AesCbcParams"
	case AESCtr:
		ed = new(AESCTRParams)
		paramsObjectName = "AesCtrParams"
	case AESGcm:
		ed = new(AESGCMParams)
		paramsObjectName = "AesGcmParams"
	default:
		return nil, NewError(NotSupportedError, "unsupported algorithm")
	}

	if err = rt.ExportTo(params, ed); err != nil {
		errMsg := fmt.Sprintf("invalid algorithm parameters, unable to interpret as %sParams object", paramsObjectName)
		return nil, NewError(SyntaxError, errMsg)
	}

	return ed, nil
}
