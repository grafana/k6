package webcrypto

import (
	"crypto/rand"
	"crypto/rsa"
	"reflect"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
)

// RsaHashedKeyAlgorithm represents the RSA key algorithm as defined by the [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#RsaHashedKeyAlgorithm-dictionary
type RsaHashedKeyAlgorithm struct {
	KeyAlgorithm

	// Hash identifies the name of the digest algorithm to use.
	// You can use any of the following:
	//   * [Sha256]
	//   * [Sha384]
	//   * [Sha512]
	Hash any
}

var _ KeyGenerator = &RSAHashedKeyGenParams{}

func newRsaHashedKeyGenParams(
	rt *sobek.Runtime,
	normalized Algorithm,
	params sobek.Value,
) (*RSAHashedKeyGenParams, error) {
	modulusLength, err := traverseObject(rt, params, "modulusLength")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get modulusLength from algorithm parameter")
	}

	publicExponentRaw, err := traverseObject(rt, params, "publicExponent")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get publicExponent from algorithm parameter")
	}

	publicExponent, ok := publicExponentRaw.Export().([]byte)
	if !ok {
		return nil, NewError(OperationError, "publicExponent is not a byte array")
	}

	hashRaw, err := traverseObject(rt, params, "hash")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get hash from algorithm parameter")
	}

	// TODO: hash could be an object if normalized algorithm has it as an object
	hash, err := extractSupportedHash(rt, hashRaw)
	if err != nil {
		return nil, err
	}

	return &RSAHashedKeyGenParams{
		Algorithm:      normalized,
		ModulusLength:  int(modulusLength.ToInteger()),
		PublicExponent: publicExponent,
		Hash:           hash,
	}, nil
}

// TODO: this should be a generic function
// that uses in any place where we need to extract a hash
func extractSupportedHash(rt *sobek.Runtime, v sobek.Value) (any, error) {
	if common.IsNullish(v) {
		return "", NewError(TypeError, "hash is null or undefined")
	}

	// try string first
	if v.ExportType().Kind() == reflect.String {
		if !isHashAlgorithm(v.ToString().String()) {
			return nil, NewError(NotSupportedError, "hash algorithm is not supported "+v.ToString().String())
		}

		return v.ToString().String(), nil
	}

	// otherwise, it should be an object
	name := v.ToObject(rt).Get("name")
	if common.IsNullish(name) {
		return "", NewError(TypeError, "name is null or undefined")
	}

	if !isHashAlgorithm(name.ToString().String()) {
		return nil, NewError(NotSupportedError, "hash algorithm is not supported "+name.ToString().String())
	}

	return Algorithm{Name: name.String()}, nil
}

// GenerateKey generates a new RSA key pair.
func (rsakgp *RSAHashedKeyGenParams) GenerateKey(
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (CryptoKeyGenerationResult, error) {
	var keyPairGenerator func(modulusLength int) (any, any, error)

	var privateKeyUsages, publicKeyUsages []CryptoKeyUsage

	keyPairGenerator = generateRSAKeyPair

	if len(keyUsages) == 0 {
		return nil, NewError(SyntaxError, "key usages cannot be empty")
	}

	// check if the key usages are valid
	// TODO: ensure that this is the best place to do this
	if rsakgp.Algorithm.Name == RSASsaPkcs1v15 || rsakgp.Algorithm.Name == RSAPss {
		for _, usage := range keyUsages {
			switch usage {
			case SignCryptoKeyUsage:
			case VerifyCryptoKeyUsage:
				continue
			default:
				return nil, NewError(SyntaxError, "invalid key usage: "+usage)
			}
		}
	}
	if rsakgp.Algorithm.Name == RSAOaep {
		for _, usage := range keyUsages {
			switch usage {
			case EncryptCryptoKeyUsage:
			case DecryptCryptoKeyUsage:
			case WrapKeyCryptoKeyUsage:
			case UnwrapKeyCryptoKeyUsage:
				continue
			default:
				return nil, NewError(SyntaxError, "invalid key usage: "+usage)
			}
		}
	}

	alg := RsaHashedKeyAlgorithm{
		KeyAlgorithm: KeyAlgorithm{
			Algorithm: rsakgp.Algorithm,
		},
		Hash: rsakgp.Hash,
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
		Usages:      publicKeyUsages,
	}

	var err error
	privateKey.handle, publicKey.handle, err = keyPairGenerator(rsakgp.ModulusLength)
	if err != nil {
		return nil, err
	}

	return &CryptoKeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

func generateRSAKeyPair(
	modulusLength int,
) (any, any, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, modulusLength)
	if err != nil {
		return nil, nil, NewError(OperationError, "could not generate RSA key pair")
	}

	// TODO: support setting of the public exponent

	return privateKey, privateKey.Public(), nil
}
