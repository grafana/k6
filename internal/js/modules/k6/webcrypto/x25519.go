package webcrypto

import (
	"crypto/ecdh"
	"crypto/x509"
	"encoding/json"
)

// X25519KeyGenParams represents the object that should be passed as the algorithm
// parameter into `SubtleCrypto.GenerateKey`, when generating an X25519 key pair.
// The X25519 key generation expects only the algorithm type as a parameter.
// The WebCrypto spec expects the X25519 algorithm defined as a standalone algorithm,
// and not as a curve, so we implement the generation here separately from the other ECDH
// curves.
type X25519KeyGenParams struct {
	Algorithm
}

var _ KeyGenerator = &X25519KeyGenParams{}

func newX25519KeyGenParams(normalized Algorithm) *X25519KeyGenParams {
	return &X25519KeyGenParams{Algorithm: normalized}
}

// GenerateKey generates a new X25519 key pair, according to the algorithm
// described in the specification.
//
// [specification]: https://wicg.github.io/webcrypto-secure-curves/#x25519
func (p *X25519KeyGenParams) GenerateKey(
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (CryptoKeyGenerationResult, error) {
	if len(keyUsages) == 0 {
		return nil, NewError(SyntaxError, "Invalid key usage: no key usages provided")
	}

	privateHandle, publicHandle, err := generateECDHKeyPair(X25519, keyUsages)
	if err != nil {
		return nil, err
	}

	private := &CryptoKey{
		Type:        PrivateCryptoKeyType,
		Extractable: extractable,
		Algorithm:   p.Algorithm,
		Usages:      UsageIntersection(keyUsages, []CryptoKeyUsage{DeriveKeyCryptoKeyUsage, DeriveBitsCryptoKeyUsage}),
		handle:      privateHandle,
	}

	public := &CryptoKey{
		Type:        PublicCryptoKeyType,
		Extractable: true,
		Algorithm:   p.Algorithm,
		Usages:      []CryptoKeyUsage{},
		handle:      publicHandle,
	}

	return &CryptoKeyPair{
		PrivateKey: private,
		PublicKey:  public,
	}, nil
}

// X25519ImportParams is an internal placeholder struct for X25519 import parameters.
// Although not described by the specification, we define it to be able to implement
// our internal KeyImporter interface.
type X25519ImportParams struct {
	Algorithm
}

func newX25519ImportParams(normalized Algorithm) *X25519ImportParams {
	return &X25519ImportParams{
		Algorithm: normalized,
	}
}

// ImportKey imports a key according to the algorithm described in the specification.
// [specification]: https://wicg.github.io/webcrypto-secure-curves/#x25519-operations
func (p *X25519ImportParams) ImportKey(
	format KeyFormat,
	keyData []byte,
	keyUsages []CryptoKeyUsage,
) (*CryptoKey, error) {
	var importFn func(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error)

	switch format {
	case SpkiKeyFormat:
		importFn = importX25519Spki
	case Pkcs8KeyFormat:
		importFn = importX25519Pkcs8
	case JwkKeyFormat:
		importFn = importX25519Jwk
	case RawKeyFormat:
		importFn = importX25519Raw
	default:
		return nil, NewError(NotSupportedError, unsupportedKeyFormatErrorMsg+" "+format+" for algorithm "+p.Algorithm.Name)
	}

	handle, keyType, err := importFn(keyData, keyUsages)
	if err != nil {
		return nil, err
	}

	return &CryptoKey{
		Algorithm: p.Algorithm,
		handle:    handle,
		Type:      keyType,
	}, nil
}

func importX25519Spki(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error) {
	if len(keyUsages) != 0 {
		return nil, UnknownCryptoKeyType, NewError(SyntaxError, "usages must be empty for X25519 in SPKI format")
	}

	parsedKey, err := x509.ParsePKIXPublicKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "Unable to import X25519 public key data: "+err.Error())
	}

	handle, ok := parsedKey.(*ecdh.PublicKey)
	if !ok {
		return nil, UnknownCryptoKeyType, NewError(DataError, "given key is not an X25519 key")
	}

	return &handle, PublicCryptoKeyType, nil
}

func importX25519Pkcs8(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error) {
	for _, usage := range keyUsages {
		switch usage {
		case DeriveKeyCryptoKeyUsage, DeriveBitsCryptoKeyUsage:
			continue
		default:
			return nil, UnknownCryptoKeyType, NewError(SyntaxError, "invalid key usage: "+usage)
		}
	}

	parsedKey, err := x509.ParsePKCS8PrivateKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "Unable to import X25519 private key data: "+err.Error())
	}

	handle, ok := parsedKey.(*ecdh.PrivateKey)
	if !ok {
		return nil, UnknownCryptoKeyType, NewError(DataError, "given key is not an X25519 key")
	}

	return &handle, PrivateCryptoKeyType, nil
}

func importX25519Jwk(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error) {
	var jwkKey alg25519JWK
	if err := json.Unmarshal(keyData, &jwkKey); err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "failed to parse input as X25519 JWK key: "+err.Error())
	}

	if err := jwkKey.validateAlg25519JWK(keyUsages, "X25519"); err != nil {
		return nil, UnknownCryptoKeyType, err
	}

	// If the 'd' field is not present, the key is public, so return the public key
	if jwkKey.D == "" {
		xBytes, err := base64URLDecode(jwkKey.X)
		if err != nil {
			return nil, UnknownCryptoKeyType, NewError(DataError, "failed to decode public key: "+err.Error())
		}

		publicKey, err := ecdh.X25519().NewPublicKey(xBytes)
		if err != nil {
			return nil, UnknownCryptoKeyType, NewError(DataError, "failed to create X25519 public key: "+err.Error())
		}
		return publicKey, PublicCryptoKeyType, nil
	}

	dBytes, err := base64URLDecode(jwkKey.D)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "failed to decode private key: "+err.Error())
	}

	privateKey, err := ecdh.X25519().NewPrivateKey(dBytes)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "failed to create X25519 private key: "+err.Error())
	}
	return privateKey, PrivateCryptoKeyType, nil
}

func importX25519Raw(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error) {
	if len(keyUsages) != 0 {
		return nil, UnknownCryptoKeyType, NewError(SyntaxError, "usages must be empty for X25519 in raw format")
	}

	return keyData, PublicCryptoKeyType, nil
}

func exportX25519Key(key *CryptoKey, format KeyFormat) (any, error) {
	if !key.Extractable {
		return nil, NewError(InvalidAccessError, "the key is not extractable")
	}

	if key.handle == nil {
		return nil, NewError(OperationError, "the key is not valid, no data")
	}

	switch format {
	case SpkiKeyFormat:
		return exportX25519Spki(key)
	case Pkcs8KeyFormat:
		return exportX25519Pkcs8(key)
	case JwkKeyFormat:
		return exportAlg25519JWK(key)
	case RawKeyFormat:
		return exportX25519Raw(key)
	default:
		return nil, NewError(NotSupportedError, unsupportedKeyFormatErrorMsg+" "+format+" for algorithm X25519")
	}
}

func exportX25519Spki(key *CryptoKey) ([]byte, error) {
	if key.Type != PublicCryptoKeyType {
		return nil, NewError(InvalidAccessError, "Must use public key to export as SPKI")
	}

	handle, ok := key.handle.(*ecdh.PublicKey)
	if !ok {
		return nil, NewError(InvalidAccessError, "Key handle is not an X25519 public key")
	}

	bytes, err := x509.MarshalPKIXPublicKey(handle)
	if err != nil {
		return nil, NewError(OperationError, "unable to marshal key to SPKI format: "+err.Error())
	}

	return bytes, nil
}

func exportX25519Pkcs8(key *CryptoKey) ([]byte, error) {
	if key.Type != PrivateCryptoKeyType {
		return nil, NewError(InvalidAccessError, "Must use private key to export as PKCS8")
	}

	handle, ok := key.handle.(*ecdh.PrivateKey)
	if !ok {
		return nil, NewError(InvalidAccessError, "Key handle is not an X25519 private key")
	}

	return x509.MarshalPKCS8PrivateKey(handle)
}

func exportX25519Raw(key *CryptoKey) ([]byte, error) {
	if key.Type != PublicCryptoKeyType {
		return nil, NewError(InvalidAccessError, "Must use public key to export as raw")
	}

	handle, ok := key.handle.(*ecdh.PublicKey)
	if !ok {
		return nil, NewError(InvalidAccessError, "Key handle is not an X25519 public key")
	}

	return handle.Bytes(), nil
}
