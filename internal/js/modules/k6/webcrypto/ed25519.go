package webcrypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
)

// Ed25519KeyGenParams represents the object that should be passed as the algorithm
// parameter into `SubtleCrypto.GenerateKey`, when generating an Ed25519 key pair.
// The Ed25519 key generation expects only the algorithm type as a parameter.
type Ed25519KeyGenParams struct {
	Algorithm
}

var _ KeyGenerator = &Ed25519KeyGenParams{}

func newEd25519KeyGenParams(normalized Algorithm) KeyGenerator {
	return &Ed25519KeyGenParams{
		Algorithm: normalized,
	}
}

// GenerateKey generates a new Ed25519 key pair, according to the algorithm
// described in the specification.
//
// [specification]: https://wicg.github.io/webcrypto-secure-curves/#ed25519
func (kgp *Ed25519KeyGenParams) GenerateKey(
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (CryptoKeyGenerationResult, error) {
	rawPublicKey, rawPrivateKey, err := generateEd25519KeyPair(keyUsages)
	if err != nil {
		return nil, err
	}

	privateKey := &CryptoKey{
		Type:        PrivateCryptoKeyType,
		Extractable: extractable,
		Algorithm:   kgp.Algorithm,
		Usages: UsageIntersection(
			keyUsages,
			[]CryptoKeyUsage{SignCryptoKeyUsage},
		),
		handle: rawPrivateKey,
	}

	publicKey := &CryptoKey{
		Type:        PublicCryptoKeyType,
		Extractable: true,
		Algorithm:   kgp.Algorithm,
		Usages: UsageIntersection(
			keyUsages,
			[]CryptoKeyUsage{VerifyCryptoKeyUsage},
		),
		handle: rawPublicKey,
	}

	return &CryptoKeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

func generateEd25519KeyPair(keyUsages []CryptoKeyUsage) (
	ed25519.PublicKey,
	ed25519.PrivateKey,
	error,
) {
	if len(keyUsages) == 0 {
		return nil, nil, NewError(SyntaxError, "Invalid key usage: no key usages provided")
	}

	for _, usage := range keyUsages {
		switch usage {
		case SignCryptoKeyUsage, VerifyCryptoKeyUsage:
			continue
		default:
			return nil, nil, NewError(SyntaxError, fmt.Sprintf("Invalid key usage: %s", usage))
		}
	}

	return ed25519.GenerateKey(rand.Reader)
}

type ed25519SignerVerifier struct{}

func (ed25519SignerVerifier) Sign(key CryptoKey, data []byte) ([]byte, error) {
	if key.Type != PrivateCryptoKeyType {
		return nil, NewError(InvalidAccessError, "Must use private key to sign data")
	}

	keyHandle, ok := key.handle.(ed25519.PrivateKey)
	if !ok {
		return nil, NewError(InvalidAccessError, "Key handle is not an Ed25519 Private Key")
	}

	return ed25519.Sign(keyHandle, data), nil
}

func (ed25519SignerVerifier) Verify(key CryptoKey, signature, data []byte) (bool, error) {
	if key.Type != PublicCryptoKeyType {
		return false, NewError(InvalidAccessError, "Must use public key to verify data")
	}

	keyHandle, ok := key.handle.(ed25519.PublicKey)
	if !ok {
		return false, NewError(InvalidAccessError, "Key handle is not an Ed25519 public key")
	}

	// The specification requires the verification to fail if either the public key or the R
	// component of the signature (its first half) is a small-order point. Go's ed25519.Verify
	// doesn't perform this check, so we do it here.
	if len(signature) != ed25519.SignatureSize {
		return false, nil
	}

	if isEd25519SmallOrderPoint(keyHandle) || isEd25519SmallOrderPoint(signature[:ed25519.PublicKeySize]) {
		return false, nil
	}

	return ed25519.Verify(keyHandle, data, signature), nil
}

// isEd25519SmallOrderPoint returns true if the given encoded point is of small order.
//
// It compares the point against every encoding, canonical or not, of the eight points of small
// order on the Ed25519 curve, as listed in table 3 of "Taming the many EdDSAs" by Chalkias et al.
// (https://eprint.iacr.org/2020/1244). Keeping an exhaustive list avoids having to decode points,
// which Go's standard library doesn't expose outside of its internal packages.
func isEd25519SmallOrderPoint(point []byte) bool {
	smallOrderPoints := [...]string{
		// Canonical encodings
		"0100000000000000000000000000000000000000000000000000000000000000", // order 1
		"ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f", // order 2
		"0000000000000000000000000000000000000000000000000000000000000080", // order 4
		"0000000000000000000000000000000000000000000000000000000000000000", // order 4
		"c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a", // order 8
		"c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac03fa", // order 8
		"26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05", // order 8
		"26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc85", // order 8
		// Non-canonical encodings
		"0100000000000000000000000000000000000000000000000000000000000080", // order 1
		"ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", // order 2
		"eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f", // order 1
		"eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", // order 1
		"edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", // order 4
		"edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f", // order 4
	}

	return slices.Contains(smallOrderPoints[:], hex.EncodeToString(point))
}

// Ed25519ImportParams is an internal placeholder struct for Ed25519 import parameters.
// Although not described by the specification, we define it to be able to implement
// our internal KeyImporter interface.
type Ed25519ImportParams struct {
	Algorithm
}

func newEd25519ImportParams(normalized Algorithm) *Ed25519ImportParams {
	return &Ed25519ImportParams{
		Algorithm: normalized,
	}
}

// ImportKey imports a key according to the algorithm described in the specification.
// [specification]:https://wicg.github.io/webcrypto-secure-curves/#ed25519-operations
func (eip *Ed25519ImportParams) ImportKey(
	format KeyFormat,
	keyData []byte,
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (*CryptoKey, error) {
	return importOKPKey(eip.Algorithm, okpKeyImporters{
		spki:  importEd25519Spki,
		pkcs8: importEd25519Pkcs8,
		jwk:   importEd25519Jwk,
		raw:   importEd25519Raw,
	}, format, keyData, extractable, keyUsages)
}

func importEd25519Spki(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error) {
	for _, usage := range keyUsages {
		switch usage {
		case VerifyCryptoKeyUsage:
			continue
		default:
			return nil, UnknownCryptoKeyType, NewError(SyntaxError, "invalid key usage: "+usage)
		}
	}

	parsedKey, err := x509.ParsePKIXPublicKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "Unable to import Ed25519 public key data: "+err.Error())
	}

	handle, ok := parsedKey.(ed25519.PublicKey)
	if !ok {
		return nil, UnknownCryptoKeyType, NewError(DataError, "given key is not an Ed25519 key")
	}

	return handle, PublicCryptoKeyType, nil
}

func importEd25519Pkcs8(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error) {
	for _, usage := range keyUsages {
		switch usage {
		case SignCryptoKeyUsage:
			continue
		default:
			return nil, UnknownCryptoKeyType, NewError(SyntaxError, "invalid key usage: "+usage)
		}
	}

	parsedKey, err := x509.ParsePKCS8PrivateKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "unable to import Ed25519 private key data: "+err.Error())
	}

	handle, ok := parsedKey.(ed25519.PrivateKey)
	if !ok {
		return nil, UnknownCryptoKeyType, NewError(DataError, "given key is not an Ed25519 key")
	}

	return handle, PrivateCryptoKeyType, nil
}

func importEd25519Jwk(keyData []byte, keyUsages []CryptoKeyUsage, extractable bool) (any, CryptoKeyType, error) {
	var jwkKey alg25519JWK
	if err := json.Unmarshal(keyData, &jwkKey); err != nil {
		return nil,
			UnknownCryptoKeyType,
			NewError(DataError, "failed to parse input as Ed25519 JWK key: "+err.Error())
	}

	if err := jwkKey.validateAlg25519JWK(keyUsages, extractable, "Ed25519"); err != nil {
		return nil, UnknownCryptoKeyType, err
	}

	x, err := base64URLDecode(jwkKey.X)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "failed to decode public key: "+err.Error())
	}

	if len(x) != ed25519.PublicKeySize {
		return nil, UnknownCryptoKeyType, NewError(DataError, fmt.Sprintf(
			"invalid Ed25519 public key length: got %d, want %d", len(x), ed25519.PublicKeySize,
		))
	}

	// If the 'd' field is not present, the key is public, so return the public key
	if jwkKey.D == nil {
		return ed25519.PublicKey(x), PublicCryptoKeyType, nil
	}

	// As defined in RFC 8037, section 2, the 'd' field holds the 32 bytes private key (the seed
	// in Go's terminology), not the 64 bytes expanded form used by Go's ed25519.PrivateKey.
	d, err := base64URLDecode(*jwkKey.D)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "failed to decode private key: "+err.Error())
	}

	if len(d) != ed25519.SeedSize {
		return nil, UnknownCryptoKeyType, NewError(DataError, fmt.Sprintf(
			"invalid Ed25519 private key length: got %d, want %d", len(d), ed25519.SeedSize,
		))
	}

	privateKey := ed25519.NewKeyFromSeed(d)
	if !bytes.Equal(privateKey[ed25519.SeedSize:], x) {
		return nil, UnknownCryptoKeyType, NewError(DataError, "the 'x' field does not match the private key 'd'")
	}

	return privateKey, PrivateCryptoKeyType, nil
}

func importEd25519Raw(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error) {
	for _, usage := range keyUsages {
		switch usage {
		case VerifyCryptoKeyUsage:
			continue
		default:
			return nil,
				UnknownCryptoKeyType,
				NewError(SyntaxError,
					fmt.Sprintf("invalid key usage: %s. Only 'verify' is valid for raw Ed25519 keys",
						usage,
					),
				)
		}
	}

	if len(keyData) != ed25519.PublicKeySize {
		return nil,
			UnknownCryptoKeyType,
			NewError(DataError,
				fmt.Sprintf("invalid Ed25519 public key length: got %d, want %d",
					len(keyData),
					ed25519.PublicKeySize,
				),
			)
	}

	handle := ed25519.PublicKey(keyData)
	return handle, PublicCryptoKeyType, nil
}

func exportEd25519Key(key *CryptoKey, format KeyFormat) (any, error) {
	if !key.Extractable {
		return nil, NewError(InvalidAccessError, "the key is not extractable")
	}

	if key.handle == nil {
		return nil, NewError(OperationError, "the key is not valid, no data")
	}

	switch format {
	case SpkiKeyFormat:
		return exportEd25519Spki(key)
	case Pkcs8KeyFormat:
		return exportEd25519Pkcs8(key)
	case JwkKeyFormat:
		return exportAlg25519JWK(key)
	case RawKeyFormat:
		return exportEd25519Raw(key)
	default:
		return nil, NewError(NotSupportedError, unsupportedKeyFormatErrorMsg+" "+format+" for algorithm Ed25519")
	}
}

func exportEd25519Spki(key *CryptoKey) ([]byte, error) {
	if key.Type != PublicCryptoKeyType {
		return nil, NewError(InvalidAccessError, "Must use public key to export as SPKI")
	}

	handle, ok := key.handle.(ed25519.PublicKey)
	if !ok {
		return nil, NewError(InvalidAccessError, "Key handle is not an Ed25519 public key")
	}

	bytes, err := x509.MarshalPKIXPublicKey(handle)
	if err != nil {
		return nil, NewError(OperationError, "unable to marshal key to SPKI format: "+err.Error())
	}

	return bytes, nil
}

func exportEd25519Pkcs8(key *CryptoKey) ([]byte, error) {
	if key.Type != PrivateCryptoKeyType {
		return nil, NewError(InvalidAccessError, "Must use private key to export as PKCS8")
	}

	handle, ok := key.handle.(ed25519.PrivateKey)
	if !ok {
		return nil, NewError(InvalidAccessError, "Key handle is not an Ed25519 private key")
	}

	bytes, err := x509.MarshalPKCS8PrivateKey(handle)
	if err != nil {
		return nil, NewError(OperationError, "unable to marshal key to PKCS8 format: "+err.Error())
	}

	return bytes, nil
}

func exportEd25519Raw(key *CryptoKey) ([]byte, error) {
	if key.Type != PublicCryptoKeyType {
		return nil, NewError(InvalidAccessError, "Must use public key to export as raw")
	}

	handle, ok := key.handle.(ed25519.PublicKey)
	if !ok {
		return nil, NewError(InvalidAccessError, "Key handle is not an Ed25519 public key")
	}

	// The exported buffer is handed to the script, so it must not share memory with the key.
	return bytes.Clone(handle), nil
}
