package webcrypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"fmt"
)

// Ed25519KeyGenParams represents the object that should be passed as the algorithm
// paramter into `SubtleCrypto.GenerateKey`, when generating an Ed25519 key pair.
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

func (kgp *Ed25519KeyGenParams) GenerateKey(extractable bool, keyUsages []CryptoKeyUsage) (CryptoKeyGenerationResult, error) {
	rawPublicKey, rawPrivateKey, err := generateEd25519KeyPair(keyUsages)
	if err != nil {
		return nil, err
	}

	alg := KeyAlgorithm{
		Algorithm: kgp.Algorithm,
	}
	privateKey := &CryptoKey{
		Type:        PrivateCryptoKeyType,
		Extractable: extractable,
		Algorithm:   alg,
		Usages: UsageIntersection(
			keyUsages,
			[]CryptoKeyUsage{SignCryptoKeyUsage},
		),
		handle: rawPrivateKey,
	}

	publicKey := &CryptoKey{
		Type:        PublicCryptoKeyType,
		Extractable: true,
		Algorithm:   alg,
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

func generateEd25519KeyPair(keyUsages []CryptoKeyUsage) (ed25519.PublicKey, ed25519.PrivateKey, error) {
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

	// TODO: verify that the ed25519 library conducts small-order checks, if not add them here

	return ed25519.Verify(keyHandle, data, signature), nil
}

// Ed25519 is an internal placeholder struct for Ed25519 import parameters.
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

func (eip *Ed25519ImportParams) ImportKey(format KeyFormat, keyData []byte, keyUsages []CryptoKeyUsage) (*CryptoKey, error) {
	var importFn func(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error)

	switch format {
	case SpkiKeyFormat:
		importFn = importEd25519Spki
	case Pkcs8KeyFormat:
		importFn = importEd25519Pkcs8
	case JwkKeyFormat:
		importFn = importEd25519Jwk
	default:
		return nil, NewError(NotSupportedError, unsupportedKeyFormatErrorMsg+" "+format+" for algorithm "+eip.Algorithm.Name)
	}

	handle, keyType, err := importFn(keyData, keyUsages)
	if err != nil {
		return nil, err
	}

	return &CryptoKey{
		Algorithm: eip.Algorithm,
		handle:    handle,
		Type:      keyType,
	}, nil
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

	handle, ok := parsedKey.(*ed25519.PublicKey)
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

	handle, ok := parsedKey.(*ed25519.PrivateKey)
	if !ok {
		return nil, UnknownCryptoKeyType, NewError(DataError, "given key is not an Ed25519 key")
	}

	return handle, PrivateCryptoKeyType, nil
}

func importEd25519Jwk(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error) {
	var jwkKey ed25519JWK
	if err := json.Unmarshal(keyData, &jwkKey); err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "failed to parse input as Ed25519 JWK key: "+err.Error())
	}

	if err := jwkKey.validate(keyUsages); err != nil {
		return nil, UnknownCryptoKeyType, err
	}

	// If the 'd' field is not present, the key is public, so return the public key
	if jwkKey.D == "" {
		xBytes, err := base64URLDecode(jwkKey.X)
		if err != nil {
			return nil, UnknownCryptoKeyType, NewError(DataError, "failed to decode public key: "+err.Error())
		}

		publicKey := ed25519.PublicKey(xBytes)

		return publicKey, PublicCryptoKeyType, nil
	}

	dBytes, err := base64URLDecode(jwkKey.D)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "failed to decode private key: "+err.Error())
	}

	privateKey := ed25519.PrivateKey(dBytes)
	return privateKey, PrivateCryptoKeyType, nil
}
