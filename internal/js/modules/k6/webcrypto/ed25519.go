package webcrypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
)

// Ed25519KeyGenParams represents the object that should be passed as the algorithm
// paramter into `SubtleCrypto.GenerateKey`, when generating an Ed25519 key pair.
// The Ed25519 key generation expects only the algorithm type as a parameter.
type Ed25519KeyGenParams struct {
	Algorithm
}

var _ KeyGenerator = &Ed25519KeyGenParams{}

func newEd25519KeyGenParams(normalized Algorithm) (KeyGenerator, error) {
	return &Ed25519KeyGenParams{
		Algorithm: normalized,
	}, nil
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
