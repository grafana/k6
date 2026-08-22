package webcrypto

import (
	"bytes"
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

	// The BufferSource fields above (iv, counter, additionalData, label)
	// alias the script's view or buffer after ExportTo, and the encrypter
	// reads them later, in the callback goroutine — after the synchronous
	// call has already returned its promise. Snapshot them now so a script
	// reusing or mutating the buffer while the promise is pending cannot
	// change (or race with) the bytes actually used, as the specification's
	// "get a copy of the bytes" steps require (#6319).
	snapshotParams(ed)

	return ed, nil
}

// snapshotParams replaces every BufferSource field of the encrypt/decrypt
// algorithm parameters with a copy, detaching the parameters from any storage
// the script can still reach.
func snapshotParams(ed EncryptDecrypter) {
	switch p := ed.(type) {
	case *AESCBCParams:
		p.Iv = bytes.Clone(p.Iv)
	case *AESCTRParams:
		p.Counter = bytes.Clone(p.Counter)
	case *AESGCMParams:
		p.Iv = bytes.Clone(p.Iv)
		p.AdditionalData = bytes.Clone(p.AdditionalData)
	case *RSAOaepParams:
		p.Label = bytes.Clone(p.Label)
	}
}
