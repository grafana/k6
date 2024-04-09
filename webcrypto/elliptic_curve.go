package webcrypto

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
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
	Algorithm

	// NamedCurve holds (a String) the name of the elliptic curve to use.
	NamedCurve EllipticCurveKind `js:"namedCurve"`
}

func newEcKeyImportParams(rt *goja.Runtime, normalized Algorithm, params goja.Value) (*EcKeyImportParams, error) {
	namedCurve, err := traverseObject(rt, params, "namedCurve")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get namedCurve from algorithm parameter")
	}

	return &EcKeyImportParams{
		Algorithm:  normalized,
		NamedCurve: EllipticCurveKind(namedCurve.String()),
	}, nil
}

// Ensure that EcKeyImportParams implements the KeyImporter interface.
var _ KeyImporter = &EcKeyImportParams{}

// ImportKey imports a key according to the algorithm described in the specification.
// https://www.w3.org/TR/WebCryptoAPI/#ecdh-operations
func (e *EcKeyImportParams) ImportKey(
	format KeyFormat,
	keyData []byte,
	_ []CryptoKeyUsage,
) (*CryptoKey, error) {
	var keyType CryptoKeyType
	var handle any

	if format == RawKeyFormat {
		// raw key type is always public
		keyType = PublicCryptoKeyType

		// pick the elliptic curve
		c, err := pickECDHCurve(e.NamedCurve)
		if err != nil {
			return nil, NewError(NotSupportedError, "invalid elliptic curve "+string(e.NamedCurve))
		}

		handle, err = c.NewPublicKey(keyData)
		if err != nil {
			return nil, NewError(DataError, "unable to import key data: "+err.Error())
		}
	}

	if format == Pkcs8KeyFormat {
		// pkcs8 key type is always private
		keyType = PrivateCryptoKeyType

		var err error
		parsedKey, err := x509.ParsePKCS8PrivateKey(keyData)
		if err != nil {
			return nil, NewError(DataError, "unable to import key data: "+err.Error())
		}

		// check if the key is an ECDSA key
		ecdsaKey, ok := parsedKey.(*ecdsa.PrivateKey)
		if !ok {
			return nil, NewError(DataError, "a private key is not an ECDSA key")
		}

		// try to restore the ECDH key
		handle, err = ecdsaKey.ECDH()
		if err != nil {
			return nil, NewError(DataError, "unable to import key data: "+err.Error())
		}
	}

	return &CryptoKey{
		Algorithm: EcKeyAlgorithm{
			KeyAlgorithm: KeyAlgorithm{
				Algorithm: e.Algorithm,
			},
			NamedCurve: e.NamedCurve,
		},
		Type:   keyType,
		handle: handle,
	}, nil
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

// GenerateKey generates a new ECDH/ECDSA key pair, according to the algorithm
// described in the specification.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#dfn-EcKeyGenParams
func (ecgp *ECKeyGenParams) GenerateKey(
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (CryptoKeyGenerationResult, error) {
	var keyPairGenerator func(curve EllipticCurveKind, keyUsages []CryptoKeyUsage) (any, any, error)
	var privateKeyUsages, publicKeyUsages []CryptoKeyUsage

	switch ecgp.Algorithm.Name {
	case ECDH:
		keyPairGenerator = generateECDHKeyPair
		privateKeyUsages = []CryptoKeyUsage{DeriveKeyCryptoKeyUsage, DeriveBitsCryptoKeyUsage}
		publicKeyUsages = []CryptoKeyUsage{}
	case ECDSA:
		keyPairGenerator = generateECDSAKeyPair
		privateKeyUsages = []CryptoKeyUsage{SignCryptoKeyUsage}
		publicKeyUsages = []CryptoKeyUsage{VerifyCryptoKeyUsage}
	default:
		return nil, NewError(NotSupportedError, "unsupported elliptic algorithm: "+ecgp.Algorithm.Name)
	}

	if !isValidEllipticCurve(ecgp.NamedCurve) {
		return nil, NewError(NotSupportedError, "invalid elliptic curve "+string(ecgp.NamedCurve))
	}

	if len(keyUsages) == 0 {
		return nil, NewError(SyntaxError, "key usages cannot be empty")
	}

	alg := &EcKeyAlgorithm{
		KeyAlgorithm: KeyAlgorithm{
			Algorithm: ecgp.Algorithm,
		},
		NamedCurve: ecgp.NamedCurve,
	}

	// wrap the keys in CryptoKey objects
	privateKey := &CryptoKey{
		Type:        PrivateCryptoKeyType,
		Extractable: extractable,
		Algorithm:   alg,
		Usages: UsageIntersection(
			keyUsages,
			privateKeyUsages,
		),
	}

	publicKey := &CryptoKey{
		Type:        PublicCryptoKeyType,
		Extractable: true,
		Algorithm:   alg,
		Usages:      publicKeyUsages,
	}

	var err error
	privateKey.handle, publicKey.handle, err = keyPairGenerator(ecgp.NamedCurve, keyUsages)
	if err != nil {
		return nil, err
	}

	return &CryptoKeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

func generateECDHKeyPair(curve EllipticCurveKind, keyUsages []CryptoKeyUsage) (any, any, error) {
	for _, usage := range keyUsages {
		switch usage {
		case DeriveKeyCryptoKeyUsage, DeriveBitsCryptoKeyUsage:
			continue
		default:
			return nil, nil, NewError(SyntaxError, "invalid key usage")
		}
	}

	c, err := pickECDHCurve(curve)
	if err != nil {
		return nil, nil, NewError(NotSupportedError, err.Error())
	}

	// generate a private & public key
	rawPrivateKey, err := c.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, NewError(OperationError, "unable to generate a ECDH key pair")
	}

	return rawPrivateKey, rawPrivateKey.PublicKey(), nil
}

func generateECDSAKeyPair(curve EllipticCurveKind, keyUsages []CryptoKeyUsage) (any, any, error) {
	for _, usage := range keyUsages {
		switch usage {
		case SignCryptoKeyUsage, VerifyCryptoKeyUsage:
			continue
		default:
			return nil, nil, NewError(SyntaxError, "invalid key usage")
		}
	}

	c, err := pickEllipticCurve(curve)
	if err != nil {
		return nil, nil, NewError(NotSupportedError, err.Error())
	}

	rawPrivateKey, err := ecdsa.GenerateKey(c, rand.Reader)
	if err != nil {
		return nil, nil, NewError(OperationError, "unable to generate a ECDSA key pair")
	}

	return rawPrivateKey, rawPrivateKey.PublicKey, nil
}

// isValidEllipticCurve returns true if the given elliptic curve is supported,
func isValidEllipticCurve(curve EllipticCurveKind) bool {
	return curve == EllipticCurveKindP256 || curve == EllipticCurveKindP384 || curve == EllipticCurveKindP521
}

// pickECDHCurve returns the elliptic curve that corresponds to the given
// EllipticCurveKind.
// If the curve is not supported, an error is returned.
func pickECDHCurve(k EllipticCurveKind) (ecdh.Curve, error) {
	switch k {
	case EllipticCurveKindP256:
		return ecdh.P256(), nil
	case EllipticCurveKindP384:
		return ecdh.P384(), nil
	case EllipticCurveKindP521:
		return ecdh.P521(), nil
	default:
		return nil, errors.New("invalid ECDH curve")
	}
}

func pickEllipticCurve(k EllipticCurveKind) (elliptic.Curve, error) {
	switch k {
	case EllipticCurveKindP256:
		return elliptic.P256(), nil
	case EllipticCurveKindP384:
		return elliptic.P384(), nil
	case EllipticCurveKindP521:
		return elliptic.P521(), nil
	default:
		return nil, errors.New("invalid elliptic curve")
	}
}

func exportECKey(alg string, ck *CryptoKey, format KeyFormat) ([]byte, error) {
	if ck.handle == nil {
		return nil, NewError(OperationError, "key data is not accessible")
	}

	switch format {
	case RawKeyFormat:
		if ck.Type != PublicCryptoKeyType {
			return nil, NewError(InvalidAccessError, "key is not a valid elliptic curve public key")
		}

		bytes, err := extractPublicKeyBytes(alg, ck.handle)
		if err != nil {
			return nil, NewError(OperationError, "unable to extract public key data: "+err.Error())
		}

		return bytes, nil
	case Pkcs8KeyFormat:
		if ck.Type != PrivateCryptoKeyType {
			return nil, NewError(InvalidAccessError, "key is not a valid elliptic curve private key")
		}

		bytes, err := x509.MarshalPKCS8PrivateKey(ck.handle)
		if err != nil {
			return nil, NewError(OperationError, "unable to marshal key to PKCS8 format: "+err.Error())
		}

		return bytes, nil
	default:
		return nil, NewError(NotSupportedError, "unsupported key format "+format)
	}
}

func extractPublicKeyBytes(alg string, handle any) ([]byte, error) {
	if alg == ECDH {
		k, ok := handle.(*ecdh.PublicKey)
		if !ok {
			return nil, NewError(OperationError, "key data isn't a valid elliptic curve public key")
		}

		return k.Bytes(), nil
	}

	if alg == ECDSA {
		k, ok := handle.(ecdsa.PublicKey)
		if !ok {
			return nil, NewError(OperationError, "key data isn't a valid elliptic curve public key")
		}

		return elliptic.Marshal(k.Curve, k.X, k.Y), nil
	}

	return nil, errors.New("unsupported algorithm " + alg)
}

func deriveBitsECDH(privateKey CryptoKey, publicKey CryptoKey) ([]byte, error) {
	pk, ok := privateKey.handle.(*ecdh.PrivateKey)
	if !ok {
		return nil, NewError(InvalidAccessError, "key is not a valid ECDH private key")
	}
	pc, ok := publicKey.handle.(*ecdh.PublicKey)
	if !ok {
		return nil, NewError(InvalidAccessError, "key is not a valid ECDH public key")
	}

	return pk.ECDH(pc)
}
