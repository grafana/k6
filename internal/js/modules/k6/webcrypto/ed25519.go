package webcrypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
)

// Ed25519KeyGenParams represents the object that should be passed as the algorithm
// paramter into `SubtleCrypto.GenerateKey`, when generating any X25519 key pair.
// The X25519 key generation expects only the algorithm type as a parameter.
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
	rawPrivateKey, rawPublicKey, err := generateEd25519KeyPair(keyUsages)
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
