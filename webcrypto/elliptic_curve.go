package webcrypto

import (
	"crypto/ecdh"
	"crypto/rand"
	"errors"

	"github.com/dop251/goja"
)

// EcKeyAlgorithm is the algorithm for elliptic curve keys as defined in the [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#EcKeyAlgorithm-dictionary
type EcKeyAlgorithm struct {
	KeyAlgorithm

	// NamedCurve holds (a String) the name of the elliptic curve to use.
	NamedCurve EllipticCurveKind `js:"namedCurve"`
}

// EcKeyImportParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.ImportKey` or `SubtleCrypto.UnwrapKey`, when generating any elliptic-curve-based
// key pair: that is, when the algorithm is identified as either of ECDSA or ECDH.
type EcKeyImportParams struct {
	// Name should be set to AlgorithmKindEcdsa or AlgorithmKindEcdh.
	Name AlgorithmIdentifier `js:"name"`

	// NamedCurve holds (a String) the name of the elliptic curve to use.
	NamedCurve EllipticCurveKind `js:"namedCurve"`
}

// EllipticCurveKind represents the kind of elliptic curve that is being used.
type EllipticCurveKind string

const (
	// EllipticCurveKindP256 represents the P-256 curve.
	EllipticCurveKindP256 EllipticCurveKind = "P-256"

	// EllipticCurveKindP384 represents the P-384 curve.
	EllipticCurveKindP384 EllipticCurveKind = "P-384"

	// EllipticCurveKindP521 represents the P-521 curve.
	EllipticCurveKindP521 EllipticCurveKind = "P-521"

	// TODO: check why this isn't a valid curve
	// EllipticCurveKind25519 represents the Curve25519 curve.
	// EllipticCurveKind25519 EllipticCurveKind = "Curve25519"
)

// IsEllipticCurve returns true if the given string is a valid EllipticCurveKind,
// false otherwise.
func IsEllipticCurve(name string) bool {
	switch name {
	case string(EllipticCurveKindP256):
		return true
	case string(EllipticCurveKindP384):
		return true
	case string(EllipticCurveKindP521):
		return true
	default:
		return false
	}
}

// ECKeyGenParams  represents the object that should be passed as the algorithm
// parameter into `SubtleCrypto.GenerateKey`, when generating any
// elliptic-curve-based key pair: that is, when the algorithm is identified
// as either of AlgorithmKindEcdsa or AlgorithmKindEcdh.
type ECKeyGenParams struct {
	Algorithm

	// NamedCurve holds (a String) the name of the curve to use.
	// You can use any of the following: CurveKindP256, CurveKindP384, or CurveKindP521.
	NamedCurve EllipticCurveKind
}

var _ KeyGenerator = &ECKeyGenParams{}

func newECKeyGenParams(rt *goja.Runtime, normalized Algorithm, params goja.Value) (*ECKeyGenParams, error) {
	namedCurve, err := traverseObject(rt, params, "namedCurve")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get namedCurve from algorithm parameter")
	}

	return &ECKeyGenParams{
		Algorithm:  normalized,
		NamedCurve: EllipticCurveKind(namedCurve.String()),
	}, nil
}

// GenerateKey generates a new ECDH key pair, according to the algorithm
// described in the specification.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#dfn-EcKeyGenParams
func (ecgp *ECKeyGenParams) GenerateKey(
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (CryptoKeyGenerationResult, error) {
	c, err := pickEllipticCurve(ecgp.NamedCurve)
	if err != nil {
		return nil, NewError(NotSupportedError, "invalid elliptic curve "+string(ecgp.NamedCurve))
	}

	if len(keyUsages) == 0 {
		return nil, NewError(SyntaxError, "key usages cannot be empty")
	}

	for _, usage := range keyUsages {
		switch usage {
		case DeriveKeyCryptoKeyUsage, DeriveBitsCryptoKeyUsage:
			continue
		default:
			return nil, NewError(SyntaxError, "invalid key usage")
		}
	}

	// generate a private & public key
	rawPrivateKey, err := c.GenerateKey(rand.Reader)
	if err != nil {
		return nil, NewError(OperationError, "unable to generate a key pair")
	}
	rawPublicKey := rawPrivateKey.PublicKey()

	alg := &EcKeyAlgorithm{
		KeyAlgorithm: KeyAlgorithm{
			Algorithm: ecgp.Algorithm,
		},
		NamedCurve: ecgp.NamedCurve,
	}

	// wrap the keys in CryptoKey objects
	privateKey := CryptoKey{
		Type:        PrivateCryptoKeyType,
		Extractable: extractable,
		Algorithm:   alg,
		Usages: UsageIntersection(
			keyUsages,
			[]CryptoKeyUsage{
				DeriveKeyCryptoKeyUsage,
				DeriveBitsCryptoKeyUsage,
			},
		),
		handle: rawPrivateKey,
	}

	publicKey := CryptoKey{
		Type:        PublicCryptoKeyType,
		Extractable: true,
		Algorithm:   alg,
		Usages:      []CryptoKeyUsage{},
		handle:      rawPublicKey,
	}

	return &CryptoKeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// pickEllipticCurve returns the elliptic curve that corresponds to the given
// EllipticCurveKind.
// If the curve is not supported, an error is returned.
func pickEllipticCurve(k EllipticCurveKind) (ecdh.Curve, error) {
	switch k {
	case EllipticCurveKindP256:
		return ecdh.P256(), nil
	case EllipticCurveKindP384:
		return ecdh.P384(), nil
	case EllipticCurveKindP521:
		return ecdh.P521(), nil
	// TODO: check why this fails
	// case EllipticCurveKind25519:
	// return ecdh.X25519(), nil
	default:
		return nil, errors.New("invalid elliptic curve")
	}
}
