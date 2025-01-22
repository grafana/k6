package webcrypto

import (
	"errors"
	"strings"

	"github.com/grafana/sobek"
)

// CryptoKeyGenerationResult represents the result of a key generation operation.
type CryptoKeyGenerationResult interface {
	// IsKeyPair returns true if the result is a key pair, false otherwise.
	IsKeyPair() bool

	// ResolveCryptoKeyPair returns the underlying CryptoKeyPair, if the result is a key pair, error otherwise.
	ResolveCryptoKeyPair() (*CryptoKeyPair, error)

	// ResolveCryptoKey returns the underlying CryptoKey, if the result is a key, error otherwise.
	ResolveCryptoKey() (*CryptoKey, error)
}

// CryptoKeyPair represents a key pair for an asymmetric cryptography algorithm, also known as
// a public-key algorithm.
//
// The Private, and Public generic type parameters define the underlying type holding the private,
// and public key, respectively.
type CryptoKeyPair struct {
	// PrivateKey holds the private key. For encryption and decryption algorithms,
	// this key is used to decrypt. For signing and verification algorithms it is used to sign.
	PrivateKey *CryptoKey `js:"privateKey"`

	// PublicKey holds the public key. For encryption and decryption algorithms,
	// this key is used to encrypt. For signing and verification algorithms it is used to verify.
	PublicKey *CryptoKey `js:"publicKey"`
}

// IsKeyPair .
func (ckp *CryptoKeyPair) IsKeyPair() bool {
	return true
}

// ResolveCryptoKeyPair returns the underlying CryptoKeyPair.
func (ckp *CryptoKeyPair) ResolveCryptoKeyPair() (*CryptoKeyPair, error) {
	return ckp, nil
}

// ResolveCryptoKey returns an error since the underlying type is not a CryptoKey.
func (ckp *CryptoKeyPair) ResolveCryptoKey() (*CryptoKey, error) {
	return nil, errors.New("not a CryptoKey")
}

var _ CryptoKeyGenerationResult = &CryptoKeyPair{}

// CryptoKey represents a cryptographic key obtained from one of the SubtleCrypto
// methods `SubtleCrypto.generateKey`, `SubtleCrypto.DeriveKey`, `SubtleCrypto.ImportKey`,
// or `SubtleCrypto.UnwrapKey`.
type CryptoKey struct {
	// Type holds the type of the key.
	Type CryptoKeyType `js:"type"`

	// Extractable indicates whether or not the key may be extracted
	// using `SubtleCrypto.ExportKey` or `SubtleCrypto.WrapKey`.
	//
	// If the value is `true`, the key may be extracted.
	// If the value is `false`, the key may not be extracted, and
	// `SubtleCrypto.exportKey` and `SubtleCrypto.wrapKey` will fail.
	Extractable bool `js:"extractable"`

	// By the time we access the Algorithm field of CryptoKey, we
	// generally already know what type of algorithm it is, and are
	// really looking to access the specific attributes of that algorithm.
	// Thus, the generic parameter type helps us manipulate the
	// `CryptoKey` type without having to cast the `Algorithm` field.
	Algorithm any `js:"algorithm"`

	// Usages holds the key usages for which this key can be used.
	Usages []CryptoKeyUsage `js:"usages"`

	// handle is an internal slot, holding the underlying key data.
	// See [specification](https://www.w3.org/TR/WebCryptoAPI/#dfnReturnLink-0).
	handle any
}

// IsKeyPair .
func (ck *CryptoKey) IsKeyPair() bool {
	return false
}

// ResolveCryptoKeyPair returns an error since the underlying type is not a CryptoKeyPair.
func (ck *CryptoKey) ResolveCryptoKeyPair() (*CryptoKeyPair, error) {
	return nil, errors.New("not a Crypto Key Pair")
}

// ResolveCryptoKey returns the underlying CryptoKey.
func (ck *CryptoKey) ResolveCryptoKey() (*CryptoKey, error) {
	return ck, nil
}

// Validate checks if the key is valid.
func (ck *CryptoKey) Validate() error {
	if ck.Type != PrivateCryptoKeyType && ck.Type != PublicCryptoKeyType && ck.Type != SecretCryptoKeyType {
		return errors.New("invalid key type")
	}

	return nil
}

var _ CryptoKeyGenerationResult = &CryptoKey{}

// ContainsUsage returns true if the key contains the specified usage.
func (ck *CryptoKey) ContainsUsage(usage CryptoKeyUsage) bool {
	return contains(ck.Usages, usage)
}

// CryptoKeyType represents the type of a key.
//
// Note that it is defined as an alias of string, instead of a dedicated type,
// to ensure it is handled as a string by sobek.
type CryptoKeyType = string

const (
	// UnknownCryptoKeyType that we set when we don't know the type of the key.
	UnknownCryptoKeyType CryptoKeyType = "unknown"

	// SecretCryptoKeyType carries the information that a key is a secret key
	// to use with a symmetric algorithm.
	SecretCryptoKeyType CryptoKeyType = "secret"

	// PrivateCryptoKeyType carries the information that a key is the private half
	// of an asymmetric key pair.
	PrivateCryptoKeyType CryptoKeyType = "private"

	// PublicCryptoKeyType carries the information that a key is the public half
	// of an asymmetric key pair.
	PublicCryptoKeyType CryptoKeyType = "public"
)

// CryptoKeyUsage represents the usage of a key.
//
// Note that it is defined as an alias of string, instead of a dedicated type,
// to ensure it is handled as a string by sobek.
type CryptoKeyUsage = string

const (
	// EncryptCryptoKeyUsage indicates that the key may be used to encrypt messages.
	EncryptCryptoKeyUsage CryptoKeyUsage = "encrypt"

	// DecryptCryptoKeyUsage indicates that the key may be used to decrypt messages.
	DecryptCryptoKeyUsage CryptoKeyUsage = "decrypt"

	// SignCryptoKeyUsage indicates that the key may be used to sign messages.
	SignCryptoKeyUsage CryptoKeyUsage = "sign"

	// VerifyCryptoKeyUsage indicates that the key may be used to verify signatures.
	VerifyCryptoKeyUsage CryptoKeyUsage = "verify"

	// DeriveKeyCryptoKeyUsage indicates that the key may be used to derive a new key.
	DeriveKeyCryptoKeyUsage CryptoKeyUsage = "deriveKey"

	// DeriveBitsCryptoKeyUsage indicates that the key may be used to derive bits.
	DeriveBitsCryptoKeyUsage CryptoKeyUsage = "deriveBits"

	// WrapKeyCryptoKeyUsage indicates that the key may be used to wrap another key.
	WrapKeyCryptoKeyUsage CryptoKeyUsage = "wrapKey"

	// UnwrapKeyCryptoKeyUsage indicates that the key may be used to unwrap another key.
	UnwrapKeyCryptoKeyUsage CryptoKeyUsage = "unwrapKey"
)

// KeyAlgorithm represents the algorithm used to generate a cryptographic key.
type KeyAlgorithm struct {
	Algorithm
}

// KeyGenerator is the interface implemented by the algorithms used to generate
// cryptographic keys.
type KeyGenerator interface {
	GenerateKey(extractable bool, keyUsages []CryptoKeyUsage) (CryptoKeyGenerationResult, error)
}

func newKeyGenerator(rt *sobek.Runtime, normalized Algorithm, params sobek.Value) (KeyGenerator, error) {
	var kg KeyGenerator
	var err error

	switch normalized.Name {
	case AESCbc, AESCtr, AESGcm, AESKw:
		kg, err = newAESKeyGenParams(rt, normalized, params)
	case HMAC:
		kg, err = newHMACKeyGenParams(rt, normalized, params)
	case ECDH, ECDSA:
		kg, err = newECKeyGenParams(rt, normalized, params)
	case RSASsaPkcs1v15, RSAPss, RSAOaep:
		kg, err = newRsaHashedKeyGenParams(rt, normalized, params)
	default:
		validAlgorithms := []string{AESCbc, AESCtr, AESGcm, AESKw, HMAC, ECDH, ECDSA, RSASsaPkcs1v15, RSAPss, RSAOaep}
		return nil, NewError(
			NotImplemented,
			"unsupported key generation algorithm '"+normalized.Name+"', "+
				"accepted values are: "+strings.Join(validAlgorithms, ", "),
		)
	}

	if err != nil {
		return nil, err
	}

	return kg, nil
}

// KeyImporter is the interface implemented by the parameters used to import
// cryptographic keys.
type KeyImporter interface {
	ImportKey(format KeyFormat, keyData []byte, keyUsages []CryptoKeyUsage) (*CryptoKey, error)
}

func newKeyImporter(rt *sobek.Runtime, normalized Algorithm, params sobek.Value) (KeyImporter, error) {
	var ki KeyImporter
	var err error

	switch normalized.Name {
	case AESCbc, AESCtr, AESGcm, AESKw:
		ki = newAESImportParams(normalized)
	case HMAC:
		ki, err = newHMACImportParams(rt, normalized, params)
	case ECDH, ECDSA:
		ki, err = newEcKeyImportParams(rt, normalized, params)
	case RSASsaPkcs1v15, RSAPss, RSAOaep:
		ki, err = newRsaHashedImportParams(rt, normalized, params)
	default:
		return nil, errors.New("key import not implemented for algorithm " + normalized.Name)
	}

	if err != nil {
		return nil, err
	}

	return ki, nil
}

// UsageIntersection returns the intersection of two slices of CryptoKeyUsage.
//
// It implements the algorithm described in the [specification] to
// determine the intersection of two slices of CryptoKeyUsage.
//
// [specification]: https://w3c.github.io/webcrypto/#concept-usage-intersection
func UsageIntersection(a, b []CryptoKeyUsage) []CryptoKeyUsage {
	var intersection []CryptoKeyUsage

	for _, usage := range a {
		// Note that the intersection algorithm is case-sensitive.
		// It is also expected to return the occurrence in the a slice "as-is".
		if contains(b, usage) && !contains(intersection, usage) {
			intersection = append(intersection, usage)
		}
	}

	return intersection
}

func contains[T comparable](container []T, element T) bool {
	for _, e := range container {
		if e == element {
			return true
		}
	}

	return false
}

// KeyFormat represents the format of a CryptoKey.
//
// Note that it is defined as an alias of string, instead of a dedicated type,
// to ensure it is handled as a string by sobek.
type KeyFormat = string

const (
	// RawKeyFormat indicates that the key is in raw format.
	RawKeyFormat KeyFormat = "raw"

	// Pkcs8KeyFormat indicates that the key is in PKCS#8 format.
	Pkcs8KeyFormat KeyFormat = "pkcs8"

	// SpkiKeyFormat indicates that the key is in SubjectPublicKeyInfo format.
	SpkiKeyFormat KeyFormat = "spki"

	// JwkKeyFormat indicates that the key is in JSON Web Key format.
	JwkKeyFormat KeyFormat = "jwk"
)

// KeyLength holds the length of the key, in bits.
//
// Note that it is defined as an alias of uint16, instead of a dedicated type,
// to ensure it is handled as a number by sobek.
type KeyLength = uint16

const (
	// KeyLength128 represents a 128 bits key length.
	KeyLength128 KeyLength = 128

	// KeyLength192 represents a 192 bits key length.
	KeyLength192 KeyLength = 192

	// KeyLength256 represents a 256 bits key length.
	KeyLength256 KeyLength = 256

	// KeyLength384 represents a 384 bits key length.
	KeyLength384 KeyLength = 384

	// KeyLength512 represents a 512 bits key length.
	KeyLength512 KeyLength = 512
)
