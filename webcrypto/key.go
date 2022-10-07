package webcrypto

import "github.com/dop251/goja"

// CryptoKeyPair represents a key pair for an asymmetric cryptography algorithm, also known as
// a public-key algorithm.
type CryptoKeyPair struct {
	// PrivateKey holds the private key. For encryption and decryption algorithms,
	// this key is used to decrypt. For signing and verification algorithms it is used to sign.
	PrivateKey CryptoKey

	// PublicKey holds the public key. For encryption and decryption algorithms,
	// this key is used to encrypt. For signing and verification algorithms it is used to verify.
	PublicKey CryptoKey
}

// CryptoKey represents a cryptographic key obtained from one of the SubtleCrypto
// methods `SubtleCrypto.generateKey`, `SubtleCrypto.DeriveKey`, `SubtleCrypto.ImportKey`,
// or `SubtleCrypto.UnwrapKey`.
type CryptoKey struct {
	// Type holds the type of the key.
	Type CryptoKeyType

	// Extractable indicates whether or not the key may be extracted
	// using `SubtleCrypto.ExportKey` or `SubtleCrypto.WrapKey`.
	//
	// If the value is `true`, the key may be extracted.
	// If the value is `false`, the key may not be extracted, and
	// `SubtleCrypto.exportKey` and `SubtleCrypto.wrapKey` will fail.
	Extractable bool

	// Algorithm holds the algorithm for which this key can be used
	// and any associated extra parameters.
	//
	// The value of this property is an object that is specific to the
	// algorithm used to generate the key. Possible values should be castable
	// to the following types:
	//   - AESKeyGenParams
	//   - RSAHashedKeyGenParams
	//   - ECKeyGenParams
	//   - HMACKeyGenParams
	Algorithm goja.Value

	// Usages indicates what can be done with the key.
	Usages []CryptoKeyUsage
}

type CryptoKeyAlgorithm interface {
	AESKeyGenParams | RSAHashedKeyGenParams | ECKeyGenParams | HMACKeyGenParams
}

type CryptoKeyType string

const (
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

type CryptoKeyUsage string

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

type KeyFormat string

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
