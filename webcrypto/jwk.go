package webcrypto

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
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
