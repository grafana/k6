package webcrypto

import (
	"fmt"
	"reflect"

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
// corresponding algorithm.
func newEncryptDecrypter(
	rt *sobek.Runtime,
	algorithm Algorithm,
	params sobek.Value,
) (EncryptDecrypter, error) {
	var ed EncryptDecrypter
	var err error

	switch algorithm.Name {
	case AESCbc:
		ed = new(AESCBCParams)
	case AESCtr:
		ed = new(AESCTRParams)
	case AESGcm:
		ed = new(AESGCMParams)
	case RSAOaep:
		ed = new(RSAOaepParams)
	default:
		return nil, NewError(NotSupportedError, "unsupported algorithm "+algorithm.Name)
	}

	if err = rt.ExportTo(params, ed); err != nil {
		structType := reflect.TypeOf(ed)

		errMsg := fmt.Sprintf("invalid algorithm parameters, unable to interpret as %q object", structType.Name())
		return nil, NewError(SyntaxError, errMsg)
	}

	return ed, nil
}
