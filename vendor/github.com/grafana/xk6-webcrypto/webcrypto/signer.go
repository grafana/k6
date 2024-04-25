package webcrypto

import "github.com/dop251/goja"

// SignerVerifier .
type SignerVerifier interface {
	Sign(key CryptoKey, dataToSign []byte) ([]byte, error)
	Verify(key CryptoKey, signature, dataToVerify []byte) (bool, error)
}

func newSignerVerifier(rt *goja.Runtime, normalized Algorithm, params goja.Value) (SignerVerifier, error) {
	switch normalized.Name {
	case HMAC:
		return &hmacSignerVerifier{}, nil
	case ECDSA:
		return newECDSAParams(rt, normalized, params)
	default:
		return nil, NewError(NotSupportedError, "unsupported algorithm for signing/verifying: "+normalized.Name)
	}
}
