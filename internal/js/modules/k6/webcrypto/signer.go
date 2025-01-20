package webcrypto

import "github.com/grafana/sobek"

// SignerVerifier .
type SignerVerifier interface {
	Sign(key CryptoKey, dataToSign []byte) ([]byte, error)
	Verify(key CryptoKey, signature, dataToVerify []byte) (bool, error)
}

func newSignerVerifier(rt *sobek.Runtime, normalized Algorithm, params sobek.Value) (SignerVerifier, error) {
	switch normalized.Name {
	case HMAC:
		return &hmacSignerVerifier{}, nil
	case ECDSA:
		return newECDSAParams(rt, normalized, params)
	case RSASsaPkcs1v15:
		return &rsaSsaPkcs1v15SignerVerifier{}, nil
	case RSAPss:
		return newRSAPssParams(rt, normalized, params)
	default:
		return nil, NewError(NotSupportedError, "unsupported algorithm for signing/verifying: "+normalized.Name)
	}
}
