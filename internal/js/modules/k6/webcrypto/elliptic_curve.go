package webcrypto

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"fmt"
	"math/big"

	"github.com/grafana/sobek"
)

const (
	p256Canonical = "P-256"
	p384Canonical = "P-384"
	p521Canonical = "P-521"
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

func newEcKeyImportParams(rt *sobek.Runtime, normalized Algorithm, params sobek.Value) (*EcKeyImportParams, error) {
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
	var importFn func(curve EllipticCurveKind, keyData []byte) (any, CryptoKeyType, error)

	switch {
	case e.Algorithm.Name == ECDH && format == Pkcs8KeyFormat:
		importFn = importECDHPrivateKey
	case e.Algorithm.Name == ECDH && format == RawKeyFormat:
		importFn = importECDHPublicKey
	case e.Algorithm.Name == ECDSA && format == Pkcs8KeyFormat:
		importFn = importECDSAPrivateKey
	case e.Algorithm.Name == ECDSA && format == RawKeyFormat:
		importFn = importECDSAPublicKey
	case e.Algorithm.Name == ECDH && format == SpkiKeyFormat:
		importFn = importECDHSPKIPublicKey
	case e.Algorithm.Name == ECDSA && format == SpkiKeyFormat:
		importFn = importECDSASPKIPublicKey
	case e.Algorithm.Name == ECDSA && format == JwkKeyFormat:
		importFn = importECDSAJWK
	case e.Algorithm.Name == ECDH && format == JwkKeyFormat:
		importFn = importECDHJWK
	default:
		return nil, NewError(NotSupportedError, unsupportedKeyFormatErrorMsg+" "+format+" for algorithm "+e.Algorithm.Name)
	}

	handle, keyType, err := importFn(e.NamedCurve, keyData)
	if err != nil {
		return nil, err
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

func importECDHPublicKey(curve EllipticCurveKind, keyData []byte) (any, CryptoKeyType, error) {
	c, err := pickECDHCurve(curve.String())
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(NotSupportedError, "invalid ECDH curve "+string(curve))
	}

	handle, err := c.NewPublicKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "unable to import ECDH public key data: "+err.Error())
	}

	return handle, PublicCryptoKeyType, nil
}

func importECDHSPKIPublicKey(_ EllipticCurveKind, keyData []byte) (any, CryptoKeyType, error) {
	pk, err := x509.ParsePKIXPublicKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "unable to import ECDH public key data: "+err.Error())
	}

	ecdsaKey, ok := pk.(*ecdsa.PublicKey)
	if !ok {
		return nil, UnknownCryptoKeyType, NewError(DataError, "a public key is not an ECDSA key")
	}

	// try to restore the ECDH key
	key, err := ecdsaKey.ECDH()
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "unable to import key data: "+err.Error())
	}

	return key, PublicCryptoKeyType, nil
}

func importECDSASPKIPublicKey(_ EllipticCurveKind, keyData []byte) (any, CryptoKeyType, error) {
	pk, err := x509.ParsePKIXPublicKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "unable to import ECDH public key data: "+err.Error())
	}

	ecdsaKey, ok := pk.(*ecdsa.PublicKey)
	if !ok {
		return nil, UnknownCryptoKeyType, NewError(DataError, "a public key is not an ECDSA key")
	}

	// try to restore the ECDH key
	return ecdsaKey, PublicCryptoKeyType, nil
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

func (k EllipticCurveKind) String() string {
	return string(k)
}

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

func importECDHPrivateKey(_ EllipticCurveKind, keyData []byte) (any, CryptoKeyType, error) {
	parsedKey, err := x509.ParsePKCS8PrivateKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "unable to import ECDH private key data: "+err.Error())
	}

	// check if the key is an ECDSA key
	ecdsaKey, ok := parsedKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, UnknownCryptoKeyType, NewError(DataError, "a private key is not an ECDSA key")
	}

	// try to restore the ECDH key
	handle, err := ecdsaKey.ECDH()
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "unable to import key data: "+err.Error())
	}

	return handle, PrivateCryptoKeyType, nil
}

func importECDSAPrivateKey(_ EllipticCurveKind, keyData []byte) (any, CryptoKeyType, error) {
	parsedKey, err := x509.ParsePKCS8PrivateKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "unable to import ECDSA private key data: "+err.Error())
	}

	ecdsaKey, ok := parsedKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, UnknownCryptoKeyType, NewError(DataError, "a private key is not an ECDSA key")
	}

	return ecdsaKey, PrivateCryptoKeyType, nil
}

func importECDSAPublicKey(curve EllipticCurveKind, keyData []byte) (any, CryptoKeyType, error) {
	c, err := pickEllipticCurve(curve.String())
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(NotSupportedError, "invalid elliptic curve "+string(curve))
	}

	x, y := elliptic.Unmarshal(c, keyData) //nolint:staticcheck // we need to use the Unmarshal function
	if x == nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "unable to import ECDSA public key data")
	}

	return &ecdsa.PublicKey{
		Curve: c,
		X:     x,
		Y:     y,
	}, PublicCryptoKeyType, nil
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

func newECKeyGenParams(rt *sobek.Runtime, normalized Algorithm, params sobek.Value) (*ECKeyGenParams, error) {
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
		return nil, NewError(NotSupportedError, "elliptic curve "+string(ecgp.NamedCurve)+" is not supported")
	}

	if len(keyUsages) == 0 {
		return nil, NewError(SyntaxError, "key usages cannot be empty")
	}

	alg := EcKeyAlgorithm{
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
		Usages: UsageIntersection(
			publicKeyUsages,
			keyUsages,
		),
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

	c, err := pickECDHCurve(curve.String())
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

	c, err := pickEllipticCurve(curve.String())
	if err != nil {
		return nil, nil, NewError(NotSupportedError, err.Error())
	}

	rawPrivateKey, err := ecdsa.GenerateKey(c, rand.Reader)
	if err != nil {
		return nil, nil, NewError(OperationError, "unable to generate a ECDSA key pair")
	}

	return rawPrivateKey, &rawPrivateKey.PublicKey, nil
}

// isValidEllipticCurve returns true if the given elliptic curve is supported,
func isValidEllipticCurve(curve EllipticCurveKind) bool {
	return curve == EllipticCurveKindP256 || curve == EllipticCurveKindP384 || curve == EllipticCurveKindP521
}

func pickECDHCurve(k string) (ecdh.Curve, error) {
	switch k {
	case p256Canonical:
		return ecdh.P256(), nil
	case p384Canonical:
		return ecdh.P384(), nil
	case p521Canonical:
		return ecdh.P521(), nil
	default:
		return nil, errors.New("invalid ECDH curve")
	}
}

func pickEllipticCurve(k string) (elliptic.Curve, error) {
	switch k {
	case p256Canonical:
		return elliptic.P256(), nil
	case p384Canonical:
		return elliptic.P384(), nil
	case p521Canonical:
		return elliptic.P521(), nil
	default:
		return nil, errors.New("invalid elliptic curve " + k)
	}
}

func exportECKey(ck *CryptoKey, format KeyFormat) (interface{}, error) {
	if ck.handle == nil {
		return nil, NewError(OperationError, "key data is not accessible")
	}

	alg, ok := ck.Algorithm.(EcKeyAlgorithm)
	if !ok {
		return nil, NewError(InvalidAccessError, "key algorithm is not a valid EC algorithm")
	}

	switch format {
	case RawKeyFormat:
		if ck.Type != PublicCryptoKeyType {
			return nil, NewError(InvalidAccessError, "key is not a valid elliptic curve public key")
		}

		bytes, err := extractPublicKeyBytes(alg.Name, ck.handle)
		if err != nil {
			return nil, NewError(OperationError, "unable to extract public key data: "+err.Error())
		}

		return bytes, nil
	case SpkiKeyFormat:
		if ck.Type != PublicCryptoKeyType {
			return nil, NewError(InvalidAccessError, "key is not a valid elliptic curve public key")
		}

		bytes, err := x509.MarshalPKIXPublicKey(ck.handle)
		if err != nil {
			return nil, NewError(OperationError, "unable to marshal key to SPKI format: "+err.Error())
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
	case JwkKeyFormat:
		return exportECJWK(ck)
	default:
		return nil, NewError(NotSupportedError, unsupportedKeyFormatErrorMsg+" "+format)
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
		k, ok := handle.(*ecdsa.PublicKey)
		if !ok {
			return nil, NewError(OperationError, "key data isn't a valid elliptic curve public key")
		}

		return elliptic.Marshal(k.Curve, k.X, k.Y), nil //nolint:staticcheck // we need to use the Marshal function
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

// The ECDSAParams represents the object that should be passed as the algorithm
// parameter into `SubtleCrypto.Sign` or `SubtleCrypto.Verify“ when using the
// ECDSA algorithm.
type ECDSAParams struct {
	// Name should be set to AlgorithmKindEcdsa.
	Name AlgorithmIdentifier

	// Hash identifies the name of the digest algorithm to use.
	// You can use any of the following:
	//   * [Sha256]
	//   * [Sha384]
	//   * [Sha512]
	Hash Algorithm
}

func newECDSAParams(rt *sobek.Runtime, normalized Algorithm, params sobek.Value) (*ECDSAParams, error) {
	hashValue, err := traverseObject(rt, params, "hash")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get hash from algorithm parameter")
	}

	normalizedHash, err := normalizeAlgorithm(rt, hashValue, OperationIdentifierGenerateKey)
	if err != nil {
		return nil, err
	}

	return &ECDSAParams{
		Name: normalized.Name,
		Hash: normalizedHash,
	}, nil
}

// Sign .
func (edsa *ECDSAParams) Sign(key CryptoKey, data []byte) ([]byte, error) {
	if key.Type != PrivateCryptoKeyType {
		return nil, NewError(InvalidAccessError, "key is not a valid ECDSA private key")
	}

	k, ok := key.handle.(*ecdsa.PrivateKey)
	if !ok {
		return nil, NewError(InvalidAccessError, "key is not a valid ECDSA private key")
	}

	// TODO: explicitly check if the hash algorithm is defined
	hashFn, ok := getHashFn(edsa.Hash.Name)
	if !ok {
		return nil, NewError(NotSupportedError, "unsupported hash algorithm: "+edsa.Hash.Name)
	}

	hasher := hashFn()
	hasher.Write(data)

	r, s, err := ecdsa.Sign(rand.Reader, k, hasher.Sum(nil))
	if err != nil {
		return nil, NewError(OperationError, "unable to sign data: "+err.Error())
	}

	bitSize := k.Curve.Params().BitSize
	n := (bitSize + 7) / 8

	rBytes := ensureLength(r.Bytes(), n)
	sBytes := ensureLength(s.Bytes(), n)

	return append(rBytes, sBytes...), nil
}

// Helper function to ensure the byte slice has length n
// prepending it with zeros if necessary.
func ensureLength(b []byte, n int) []byte {
	if len(b) == n {
		return b
	}
	result := make([]byte, n)
	copy(result[n-len(b):], b)
	return result
}

// Verify .
func (edsa *ECDSAParams) Verify(key CryptoKey, signature []byte, data []byte) (bool, error) {
	if key.Type != PublicCryptoKeyType {
		return false, NewError(InvalidAccessError, "key is not a valid ECDSA public key")
	}

	k, ok := key.handle.(*ecdsa.PublicKey)
	if !ok {
		return false, NewError(InvalidAccessError, "key is not a valid ECDSA public key")
	}

	hashFn, ok := getHashFn(edsa.Hash.Name)
	if !ok {
		return false, NewError(NotSupportedError, "unsupported hash algorithm: "+edsa.Hash.Name)
	}

	bitSize := k.Curve.Params().BitSize
	n := (bitSize + 7) / 8

	if len(signature) != 2*n {
		return false, nil
	}

	hasher := hashFn()
	hasher.Write(data)

	r := new(big.Int).SetBytes(signature[:n])
	s := new(big.Int).SetBytes(signature[n:])

	return ecdsa.Verify(k, hasher.Sum(nil), r, s), nil
}

func convertECDHtoECDSAKey(k *ecdh.PrivateKey) (*ecdsa.PrivateKey, error) {
	pk, err := convertPublicECDHtoECDSA(k.PublicKey())
	if err != nil {
		return nil, err
	}

	return &ecdsa.PrivateKey{
		PublicKey: *pk,
		D:         new(big.Int).SetBytes(k.Bytes()),
	}, nil
}

func convertPublicECDHtoECDSA(k *ecdh.PublicKey) (*ecdsa.PublicKey, error) {
	var crv elliptic.Curve
	switch k.Curve() {
	case ecdh.P256():
		crv = elliptic.P256()
	case ecdh.P384():
		crv = elliptic.P384()
	case ecdh.P521():
		crv = elliptic.P521()
	default:
		return nil, errors.New("curve not supported for converting to ECDSA key")
	}

	x, y := elliptic.Unmarshal(crv, k.Bytes()) //nolint:staticcheck // we need to use the Unmarshal function
	if x == nil {
		return nil, fmt.Errorf("unable to convert ECDH public key to ECDSA public key, curve: %s", crv.Params().Name)
	}

	return &ecdsa.PublicKey{
		Curve: crv,
		X:     x,
		Y:     y,
	}, nil
}

func ensureKeysUseSameCurve(k1, k2 CryptoKey) error {
	ecAlg1, ok1 := k1.Algorithm.(EcKeyAlgorithm)
	ecAlg2, ok2 := k2.Algorithm.(EcKeyAlgorithm)
	if !ok1 || !ok2 {
		return errors.New("keys are not valid elliptic curve keys")
	}

	if ecAlg1.NamedCurve != ecAlg2.NamedCurve {
		return errors.New("keys have different curves " + string(ecAlg1.NamedCurve) + " and " + string(ecAlg2.NamedCurve))
	}

	return nil
}
