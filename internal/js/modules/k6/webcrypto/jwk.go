package webcrypto

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
)

const (
	// JWKECKeyType represents the elliptic curve key type.
	JWKECKeyType = "EC"

	// JWKOctKeyType represents the symmetric key type.
	JWKOctKeyType = "oct"
)

// JsonWebKey represents a JSON Web Key (JsonWebKey) key.
type JsonWebKey map[string]interface{} //nolint:stylecheck,revive // we name this type JsonWebKey to match the spec

// Set sets a key-value pair in the JWK.
func (jwk *JsonWebKey) Set(key string, value interface{}) {
	(*jwk)[key] = value
}

// symmetricJWK represents a symmetric JWK key.
// It is used to unmarshal symmetric keys from JWK format.
type symmetricJWK struct {
	Kty string `json:"kty"`
	K   string `json:"k"`
}

func (jwk *symmetricJWK) validate() error {
	if jwk.Kty != JWKOctKeyType {
		return fmt.Errorf("invalid key type: %s", jwk.Kty)
	}

	if jwk.K == "" {
		return errors.New("key (k) is required")
	}

	return nil
}

// extractSymmetricJWK extracts the symmetric key from a given JWK key (JSON data).
func extractSymmetricJWK(jsonKeyData []byte) ([]byte, error) {
	sk := symmetricJWK{}
	if err := json.Unmarshal(jsonKeyData, &sk); err != nil {
		return nil, fmt.Errorf("failed to parse symmetric JWK: %w", err)
	}

	if err := sk.validate(); err != nil {
		return nil, fmt.Errorf("invalid symmetric JWK: %w", err)
	}

	skBytes, err := base64URLDecode(sk.K)
	if err != nil {
		return nil, fmt.Errorf("failed to decode symmetric key: %w", err)
	}

	return skBytes, nil
}

// exportSymmetricJWK exports a symmetric key as a map of JWK key parameters.
func exportSymmetricJWK(key *CryptoKey) (*JsonWebKey, error) {
	rawKey, ok := key.handle.([]byte)
	if !ok {
		return nil, errors.New("key's handle isn't a byte slice")
	}

	// wrap result into the object that is expected to be returned
	exported := &JsonWebKey{}

	exported.Set("k", base64URLEncode(rawKey))
	exported.Set("kty", JWKOctKeyType)
	exported.Set("ext", key.Extractable)
	exported.Set("key_ops", key.Usages)

	algV, err := extractAlg(key.Algorithm, len(rawKey))
	if err != nil {
		return nil, fmt.Errorf("failed to extract algorithm: %w", err)
	}
	exported.Set("alg", algV)

	return exported, nil
}

func extractAlg(inAlg any, keyLen int) (string, error) {
	switch alg := inAlg.(type) {
	case hasHash:
		v := alg.hash()
		if len(v) < 4 {
			return "", errors.New("length of hash algorithm is less than 4: " + v)
		}
		return "HS" + v[4:], nil
	case hasAlg:
		v := alg.alg()
		if len(v) < 4 {
			return "", errors.New("length of named algorithm is less than 4: " + v)
		}

		return fmt.Sprintf("A%d%s", (8 * keyLen), v[4:]), nil
	default:
		return "", fmt.Errorf("unsupported algorithm: %v", inAlg)
	}
}

// ecJWK represents an EC JWK key.
// It is used to unmarshal ECDSA and ECDH keys to and from JWK format.
type ecJWK struct {
	// Key type
	Kty string `json:"kty"`
	// Canonical Curve
	Crv string `json:"crv"`
	// X coordinate
	X string `json:"x"`
	// Y coordinate
	Y string `json:"y"`
	// Private scalar
	D string `json:"d"`
}

func (jwk *ecJWK) validate() error {
	if jwk.Kty != JWKECKeyType {
		return fmt.Errorf("invalid key type: %s", jwk.Kty)
	}

	if jwk.Crv == "" {
		return errors.New("curve is required")
	}

	if jwk.X == "" {
		return errors.New("coordinate X is required")
	}

	if jwk.Y == "" {
		return errors.New("coordinate Y is required")
	}

	return nil
}

// encodeCurveBigInt encodes the private scalar D of an ECDSA key with padding.
func encodeCurveBigInt(data *big.Int, curveBits int) string {
	// Determine the expected byte length for the curve
	byteLength := curveBits / 8
	// Add one more byte if the bits are not a multiple of 8
	if curveBits%8 != 0 {
		byteLength++
	}

	dBytes := data.Bytes()
	dPadded := padLeft(dBytes, byteLength) // Pad if necessary

	return base64URLEncode(dPadded)
}

// padLeft pads the byte slice with zeros to the left to ensure it has a specific length.
func padLeft(bytes []byte, size int) []byte {
	padding := make([]byte, size-len(bytes))
	return append(padding, bytes...) //nolint:makezero // we need to pad with zeros
}

func exportECJWK(key *CryptoKey) (interface{}, error) {
	exported := &JsonWebKey{}
	exported.Set("kty", JWKECKeyType)

	var x, y, d *big.Int
	var curveParams *elliptic.CurveParams

	switch k := key.handle.(type) {
	case *ecdsa.PrivateKey:
		x = k.X
		y = k.Y
		d = k.D
		curveParams = k.Curve.Params()
	case *ecdsa.PublicKey:
		x = k.X
		y = k.Y
		curveParams = k.Curve.Params()
	case *ecdh.PrivateKey:
		ecdsaKey, err := convertECDHtoECDSAKey(k)
		if err != nil {
			return nil, fmt.Errorf("failed to convert ECDH key to ECDSA key: %w", err)
		}

		x = ecdsaKey.X
		y = ecdsaKey.Y
		d = ecdsaKey.D
		curveParams = ecdsaKey.Curve.Params()
	case *ecdh.PublicKey:
		ecdsaKey, err := convertPublicECDHtoECDSA(k)
		if err != nil {
			return nil, fmt.Errorf("failed to convert ECDH key to ECDSA key: %w", err)
		}

		x = ecdsaKey.X
		y = ecdsaKey.Y
		curveParams = ecdsaKey.Curve.Params()
	default:
		return nil, errors.New("key's handle isn't an ECDSA/ECDH public/private key")
	}

	exported.Set("crv", curveParams.Name)
	curveBits := curveParams.BitSize

	exported.Set("x", base64URLEncode(x.Bytes()))
	exported.Set("y", base64URLEncode(y.Bytes()))

	if d != nil {
		exported.Set("d", encodeCurveBigInt(d, curveBits))
	}

	return exported, nil
}

func importECDSAJWK(_ EllipticCurveKind, jsonKeyData []byte) (any, CryptoKeyType, error) {
	var jwkKey ecJWK
	if err := json.Unmarshal(jsonKeyData, &jwkKey); err != nil {
		return nil, UnknownCryptoKeyType, fmt.Errorf("failed to parse input as EC JWK key: %w", err)
	}

	if err := jwkKey.validate(); err != nil {
		return nil, UnknownCryptoKeyType, fmt.Errorf("invalid EC JWK key: %w", err)
	}

	crv, err := pickEllipticCurve(jwkKey.Crv)
	if err != nil {
		return nil, UnknownCryptoKeyType, fmt.Errorf("failed to parse elliptic curve: %w", err)
	}

	x, err := base64URLDecode(jwkKey.X)
	if err != nil {
		return nil, UnknownCryptoKeyType, fmt.Errorf("failed to decode X coordinate: %w", err)
	}

	y, err := base64URLDecode(jwkKey.Y)
	if err != nil {
		return nil, UnknownCryptoKeyType, fmt.Errorf("failed to decode Y coordinate: %w", err)
	}

	pk := &ecdsa.PublicKey{
		Curve: crv,
		X:     new(big.Int).SetBytes(x),
		Y:     new(big.Int).SetBytes(y),
	}

	// if the key is a public key, return it
	if jwkKey.D == "" {
		return pk, PublicCryptoKeyType, nil
	}

	d, err := base64URLDecode(jwkKey.D)
	if err != nil {
		return nil, UnknownCryptoKeyType, fmt.Errorf("failed to decode D: %w", err)
	}

	return &ecdsa.PrivateKey{
		PublicKey: *pk,
		D:         new(big.Int).SetBytes(d),
	}, PrivateCryptoKeyType, nil
}

func importECDHJWK(_ EllipticCurveKind, jsonKeyData []byte) (any, CryptoKeyType, error) {
	// first we do try to parse the key as ECDSA key
	key, _, err := importECDSAJWK(EllipticCurveKindP256, jsonKeyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, fmt.Errorf("failed to parse input as ECDH key: %w", err)
	}

	switch key := key.(type) {
	case *ecdsa.PrivateKey:
		ecdhKey, err := key.ECDH()
		if err != nil {
			return nil, UnknownCryptoKeyType, fmt.Errorf("failed to convert ECDSA key to ECDH key: %w", err)
		}

		return ecdhKey, PrivateCryptoKeyType, nil
	case *ecdsa.PublicKey:
		ecdhKey, err := key.ECDH()
		if err != nil {
			return nil, UnknownCryptoKeyType, fmt.Errorf("failed to convert ECDSA key to ECDH key: %w", err)
		}

		return ecdhKey, PublicCryptoKeyType, nil
	default:
		return nil, UnknownCryptoKeyType, errors.New("input isn't a valid ECDH key")
	}
}

type rsaJWK struct {
	Kty string `json:"kty"`          // Key Type
	N   string `json:"n"`            // Modulus
	E   string `json:"e"`            // Exponent
	D   string `json:"d,omitempty"`  // Private exponent
	P   string `json:"p,omitempty"`  // First prime factor
	Q   string `json:"q,omitempty"`  // Second prime factor
	Dp  string `json:"dp,omitempty"` // Exponent1
	Dq  string `json:"dq,omitempty"` // Exponent2
	Qi  string `json:"qi,omitempty"` // Coefficient
}

func (jwk *rsaJWK) validate() error {
	if jwk.Kty != "RSA" {
		return fmt.Errorf("invalid key type: %s", jwk.Kty)
	}

	if jwk.N == "" {
		return errors.New("modulus (n) is required")
	}

	if jwk.E == "" {
		return errors.New("exponent (e) is required")
	}

	// TODO: consider validating the other fields in future
	return nil
}

func importRSAJWK(jsonKeyData []byte) (any, CryptoKeyType, int, error) {
	var jwk rsaJWK
	if err := json.Unmarshal(jsonKeyData, &jwk); err != nil {
		return nil, UnknownCryptoKeyType, 0, fmt.Errorf("failed to parse input as RSA JWK key: %w", err)
	}

	if err := jwk.validate(); err != nil {
		return nil, UnknownCryptoKeyType, 0, fmt.Errorf("invalid RSA JWK key: %w", err)
	}

	// decode the various key components
	nBytes, err := base64URLDecode(jwk.N)
	if err != nil {
		return nil, UnknownCryptoKeyType, 0, fmt.Errorf("failed to decode modulus: %w", err)
	}
	eBytes, err := base64URLDecode(jwk.E)
	if err != nil {
		return nil, UnknownCryptoKeyType, 0, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// convert exponent to an integer
	eInt := new(big.Int).SetBytes(eBytes).Int64()
	pubKey := rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(eInt),
	}

	// if the private exponent is missing, return the public key
	if jwk.D == "" {
		return pubKey, PublicCryptoKeyType, pubKey.N.BitLen(), nil
	}

	dBytes, err := base64URLDecode(jwk.D)
	if err != nil {
		return nil, UnknownCryptoKeyType, 0, fmt.Errorf("failed to decode private exponent: %w", err)
	}
	pBytes, err := base64URLDecode(jwk.P)
	if err != nil {
		return nil, UnknownCryptoKeyType, 0, fmt.Errorf("failed to decode first prime factor: %w", err)
	}
	qBytes, err := base64URLDecode(jwk.Q)
	if err != nil {
		return nil, UnknownCryptoKeyType, 0, fmt.Errorf("failed to decode second prime factor: %w", err)
	}
	dpBytes, err := base64URLDecode(jwk.Dp)
	if err != nil {
		return nil, UnknownCryptoKeyType, 0, fmt.Errorf("failed to decode first exponent: %w", err)
	}
	dqBytes, err := base64URLDecode(jwk.Dq)
	if err != nil {
		return nil, UnknownCryptoKeyType, 0, fmt.Errorf("failed to decode second exponent: %w", err)
	}
	qiBytes, err := base64URLDecode(jwk.Qi)
	if err != nil {
		return nil, UnknownCryptoKeyType, 0, fmt.Errorf("failed to decode coefficient: %w", err)
	}

	privKey := &rsa.PrivateKey{
		PublicKey: pubKey,
		D:         new(big.Int).SetBytes(dBytes),
		Primes: []*big.Int{
			new(big.Int).SetBytes(pBytes),
			new(big.Int).SetBytes(qBytes),
		},
		Precomputed: rsa.PrecomputedValues{
			Dp:   new(big.Int).SetBytes(dpBytes),
			Dq:   new(big.Int).SetBytes(dqBytes),
			Qinv: new(big.Int).SetBytes(qiBytes),
		},
	}

	err = privKey.Validate()
	if err != nil {
		return nil, UnknownCryptoKeyType, 0, fmt.Errorf("failed to validate private key: %w", err)
	}

	return privKey, PrivateCryptoKeyType, pubKey.N.BitLen(), nil
}

func exportRSAJWK(key *CryptoKey) (interface{}, error) {
	exported := &JsonWebKey{}
	exported.Set("kty", "RSA")

	switch rsaKey := key.handle.(type) {
	case *rsa.PrivateKey:
		exported.Set("n", base64URLEncode(rsaKey.N.Bytes()))
		exported.Set("e", base64URLEncode(big.NewInt(int64(rsaKey.E)).Bytes()))
		exported.Set("d", base64URLEncode(rsaKey.D.Bytes()))
		exported.Set("p", base64URLEncode(rsaKey.Primes[0].Bytes()))
		exported.Set("q", base64URLEncode(rsaKey.Primes[1].Bytes()))
		exported.Set("dp", base64URLEncode(rsaKey.Precomputed.Dp.Bytes()))
		exported.Set("dq", base64URLEncode(rsaKey.Precomputed.Dq.Bytes()))
		exported.Set("qi", base64URLEncode(rsaKey.Precomputed.Qinv.Bytes()))
	case *rsa.PublicKey:
		exported.Set("n", base64URLEncode(rsaKey.N.Bytes()))
		exported.Set("e", base64URLEncode(big.NewInt(int64(rsaKey.E)).Bytes()))
	case rsa.PublicKey:
		exported.Set("n", base64URLEncode(rsaKey.N.Bytes()))
		exported.Set("e", base64URLEncode(big.NewInt(int64(rsaKey.E)).Bytes()))
	default:
		return nil, fmt.Errorf("key's handle isn't an RSA public/private key, got: %T", key.handle)
	}

	return exported, nil
}

type alg25519JWK struct {
	// Key type
	Kty string `json:"kty"`
	// Canonical Curve
	Crv string `json:"crv"`
	// Public Key Use
	Use string `json:"use"`
	// Private scalar
	KeyOps []CryptoKeyUsage `json:"key_ops"`
	// Public key
	X string `json:"x"`
	// Private key
	D string `json:"d"`
	// Extractable
	Ext *bool `json:"ext"`
}

func exportAlg25519JWK(key *CryptoKey) (*JsonWebKey, error) {
	exported := &JsonWebKey{}
	exported.Set("kty", "OKP")
	exported.Set("crv", key.Algorithm.(Algorithm).Name)
	switch alg25519Key := key.handle.(type) {
	case ed25519.PublicKey:
		exported.Set("x", base64URLEncode([]byte(alg25519Key)))
	case ed25519.PrivateKey:
		exported.Set("x", base64URLEncode([]byte(alg25519Key.Public().(ed25519.PublicKey))))
		exported.Set("d", base64URLEncode([]byte(alg25519Key)))
	case *ecdh.PublicKey:
		exported.Set("x", base64URLEncode(alg25519Key.Bytes()))
	case *ecdh.PrivateKey:
		exported.Set("x", base64URLEncode(alg25519Key.Public().(*ecdh.PublicKey).Bytes()))
		exported.Set("d", base64URLEncode(alg25519Key.Bytes()))
	default:
		return nil, fmt.Errorf("key's handle isn't an Ed25519 public/private key, got: %T", key.handle)
	}
	exported.Set("key_ops", key.Usages)
	exported.Set("ext", key.Extractable)

	return exported, nil
}

// validateKeyOps validates that the key_ops field on the JWK does not conflict
// with the given usages according to the JWK specification
func validateKeyOps(keyOps []CryptoKeyUsage, keyUsages []CryptoKeyUsage) error {
	if len(keyOps) == 0 {
		return nil
	}

	// Check for duplicates in keyOps
	seen := make(map[CryptoKeyUsage]bool)
	for _, op := range keyOps {
		if seen[op] {
			return NewError(DataError, "duplicate key operation values are not allowed in key_ops")
		}
		seen[op] = true
	}

	// Validate allowed operation combinations
	hasSign := false
	hasVerify := false
	hasEncrypt := false
	hasDecrypt := false
	hasWrapKey := false
	hasUnwrapKey := false
	hasDerive := false // covers both deriveKey and deriveBits

	for _, op := range keyOps {
		switch op {
		case SignCryptoKeyUsage:
			hasSign = true
		case VerifyCryptoKeyUsage:
			hasVerify = true
		case EncryptCryptoKeyUsage:
			hasEncrypt = true
		case DecryptCryptoKeyUsage:
			hasDecrypt = true
		case WrapKeyCryptoKeyUsage:
			hasWrapKey = true
		case UnwrapKeyCryptoKeyUsage:
			hasUnwrapKey = true
		case DeriveKeyCryptoKeyUsage, DeriveBitsCryptoKeyUsage:
			hasDerive = true
		default:
			// Spec allows for other values, so we don't error here
			continue
		}
	}

	// Check for invalid combinations
	validCombos := (hasSign && hasVerify && !hasEncrypt && !hasDecrypt && !hasWrapKey && !hasUnwrapKey && !hasDerive) ||
		(hasEncrypt && hasDecrypt && !hasSign && !hasVerify && !hasWrapKey && !hasUnwrapKey && !hasDerive) ||
		(hasWrapKey && hasUnwrapKey && !hasSign && !hasVerify && !hasEncrypt && !hasDecrypt && !hasDerive) ||
		(hasDerive && !hasSign && !hasVerify && !hasEncrypt && !hasDecrypt && !hasWrapKey && !hasUnwrapKey) ||
		// Single operation cases
		(hasSign && !hasVerify && !hasEncrypt && !hasDecrypt && !hasWrapKey && !hasUnwrapKey && !hasDerive) ||
		(hasVerify && !hasSign && !hasEncrypt && !hasDecrypt && !hasWrapKey && !hasUnwrapKey && !hasDerive) ||
		(hasEncrypt && !hasDecrypt && !hasSign && !hasVerify && !hasWrapKey && !hasUnwrapKey && !hasDerive) ||
		(hasDecrypt && !hasEncrypt && !hasSign && !hasVerify && !hasWrapKey && !hasUnwrapKey && !hasDerive) ||
		(hasWrapKey && !hasUnwrapKey && !hasSign && !hasVerify && !hasEncrypt && !hasDecrypt && !hasDerive) ||
		(hasUnwrapKey && !hasWrapKey && !hasSign && !hasVerify && !hasEncrypt && !hasDecrypt && !hasDerive)

	if !validCombos {
		return NewError(DataError, "invalid combination of key operations. Only sign/verify, encrypt/decrypt, or wrapKey/unwrapKey pairs are allowed, or single derive operations")
	}

	// Verify that all requested usages are present in keyOps
	for _, usage := range keyUsages {
		if !slices.Contains(keyOps, usage) {
			return NewError(DataError, fmt.Sprintf("requested usage '%s' is not present in key_ops", usage))
		}
	}

	return nil
}

func validateEd25519Usages(keyUsages []CryptoKeyUsage, private bool) error {
	if private {
		for _, usage := range keyUsages {
			switch usage {
			case SignCryptoKeyUsage:
				continue
			default:
				return NewError(SyntaxError, fmt.Sprintf("invalid key usage: %s. Only 'sign' is valid for private Ed25519 keys", usage))
			}
		}
	} else {
		for _, usage := range keyUsages {
			switch usage {
			case VerifyCryptoKeyUsage:
				continue
			default:
				return NewError(SyntaxError, fmt.Sprintf("invalid key usage: %s. Only 'verify' is valid for public Ed25519 keys", usage))
			}
		}
	}

	return nil
}

func validateX25519Usages(keyUsages []CryptoKeyUsage, private bool) error {
	if private {
		for _, usage := range keyUsages {
			switch usage {
			case DeriveKeyCryptoKeyUsage, DeriveBitsCryptoKeyUsage:
				continue
			default:
				return NewError(SyntaxError, fmt.Sprintf("invalid key usage: %s. Only 'deriveKey' and 'deriveBits' are valid for private X25519 keys", usage))
			}
		}
	} else {
		if len(keyUsages) != 0 {
			return NewError(SyntaxError, "usages must be empty for public X25519 keys in JWK format")
		}
	}

	return nil
}

func (jwk *alg25519JWK) validateAlg25519JWK(keyUsages []CryptoKeyUsage, algorithm string) error {
	private := jwk.D != ""
	if algorithm == "Ed25519" {
		err := validateEd25519Usages(keyUsages, private)
		if err != nil {
			return err
		}
	} else if algorithm == "X25519" {
		err := validateX25519Usages(keyUsages, private)
		if err != nil {
			return err
		}
	}

	if jwk.Kty != "OKP" {
		return NewError(DataError, fmt.Sprintf("invalid 'kty': %s. kty value must be 'OKP' for %s keys", jwk.Kty, algorithm))
	}

	if jwk.Crv != algorithm {
		return NewError(DataError, fmt.Sprintf("invalid 'crv': %s. crv value must be %s", jwk.Crv, algorithm))
	}

	if jwk.X == "" {
		return NewError(DataError, fmt.Sprintf("invalid 'x': x field is required for all %s keys", algorithm))
	}

	if private && jwk.D == "" {
		return NewError(DataError, fmt.Sprintf("invalid 'd': d field is required for private %s keys", algorithm))
	}

	if len(keyUsages) != 0 && jwk.Use != "" && jwk.Use != "sig" {
		return NewError(DataError, fmt.Sprintf("invalid 'use': %s. use field must be 'sig' in the JWK if usages are supplied ", jwk.Use))
	}

	if err := validateKeyOps(jwk.KeyOps, keyUsages); err != nil {
		return err
	}

	// TODO: pass extractable down from JS params and validate properly
	return nil
}
