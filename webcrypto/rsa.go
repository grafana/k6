package webcrypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
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

func (h RsaHashedKeyAlgorithm) hash() (string, error) {
	switch v := h.Hash.(type) {
	case string:
		return v, nil
	case Algorithm:
		return v.Name, nil
	default:
		return "", errors.New("unsupported hash type")
	}
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
// CONSIDER: replacing it with extractHashFn or extractHash
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
		privateKeyUsages = []CryptoKeyUsage{SignCryptoKeyUsage}
		publicKeyUsages = []CryptoKeyUsage{VerifyCryptoKeyUsage}
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
		Usages:      UsageIntersection(keyUsages, privateKeyUsages),
	}

	publicKey := &CryptoKey{
		Type:        PublicCryptoKeyType,
		Extractable: true,
		Algorithm:   alg,
		Usages:      UsageIntersection(keyUsages, publicKeyUsages),
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

func exportRSAKey(ck *CryptoKey, format KeyFormat) (interface{}, error) {
	if ck.handle == nil {
		return nil, NewError(OperationError, "key data is not accessible")
	}

	switch format {
	case SpkiKeyFormat:
		if ck.Type != PublicCryptoKeyType {
			return nil, NewError(InvalidAccessError, "key is not a valid RSA public key")
		}

		bytes, err := x509.MarshalPKIXPublicKey(ck.handle)
		if err != nil {
			return nil, NewError(OperationError, "unable to marshal key to SPKI format: "+err.Error())
		}

		return bytes, nil
	case Pkcs8KeyFormat:
		if ck.Type != PrivateCryptoKeyType {
			return nil, NewError(InvalidAccessError, "key is not a valid RSA private key")
		}

		bytes, err := x509.MarshalPKCS8PrivateKey(ck.handle)
		if err != nil {
			return nil, NewError(OperationError, "unable to marshal key to PKCS8 format: "+err.Error())
		}

		return bytes, nil
	case JwkKeyFormat:
		return exportRSAJWK(ck)
	default:
		return nil, NewError(NotSupportedError, unsupportedKeyFormatErrorMsg+" "+format)
	}
}

func newRsaHashedImportParams(
	rt *sobek.Runtime,
	normalized Algorithm,
	params sobek.Value,
) (*RSAHashedImportParams, error) {
	hashRaw, err := traverseObject(rt, params, "hash")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get hash from algorithm parameter")
	}

	// TODO: hash could be an object if normalized algorithm has it as an object
	hash, err := extractSupportedHash(rt, hashRaw)
	if err != nil {
		return nil, err
	}

	return &RSAHashedImportParams{
		Algorithm: normalized,
		Hash:      hash,
	}, nil
}

// Ensure that RSAHashedImportParams implements the KeyImporter interface.
var _ KeyImporter = &RSAHashedImportParams{}

// ImportKey imports a key according to the algorithm described in the specification.
// https://www.w3.org/TR/WebCryptoAPI/#ecdh-operations
func (rhkip *RSAHashedImportParams) ImportKey(
	format KeyFormat,
	keyData []byte,
	usages []CryptoKeyUsage,
) (*CryptoKey, error) {
	var importFn func(keyData []byte) (any, CryptoKeyType, error)

	switch {
	case format == Pkcs8KeyFormat:
		importFn = importRSAPrivateKey
	case format == SpkiKeyFormat:
		importFn = importRSAPublicKey
	case format == JwkKeyFormat:
		importFn = importRSAJWK
	default:
		return nil, NewError(
			NotSupportedError,
			unsupportedKeyFormatErrorMsg+" "+format+" for algorithm "+rhkip.Algorithm.Name,
		)
	}

	handle, keyType, err := importFn(keyData)
	if err != nil {
		return nil, err
	}

	return &CryptoKey{
		Algorithm: RsaHashedKeyAlgorithm{
			KeyAlgorithm: KeyAlgorithm{
				Algorithm: rhkip.Algorithm,
			},
			Hash: rhkip.Hash,
		},
		Type:   keyType,
		Usages: usages,
		handle: handle,
	}, nil
}

func importRSAPrivateKey(keyData []byte) (any, CryptoKeyType, error) {
	parsedKey, err := x509.ParsePKCS8PrivateKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "unable to import ECDH private key data: "+err.Error())
	}

	// check if the key is an RSA key
	privateKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, UnknownCryptoKeyType, NewError(DataError, "a private key is not an ECDSA key")
	}

	return privateKey, PrivateCryptoKeyType, nil
}

func importRSAPublicKey(keyData []byte) (any, CryptoKeyType, error) {
	parsedKey, err := x509.ParsePKIXPublicKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, NewError(DataError, "unable to import ECDH public key data: "+err.Error())
	}

	// check if the key is an RSA key
	publicKey, ok := parsedKey.(*rsa.PublicKey)
	if !ok {
		return nil, UnknownCryptoKeyType, NewError(DataError, "a public key is not an ECDSA key")
	}

	return publicKey, PublicCryptoKeyType, nil
}

type rsaSsaPkcs1v15SignerVerifier struct{}

var _ SignerVerifier = &rsaSsaPkcs1v15SignerVerifier{}

func (rsasv *rsaSsaPkcs1v15SignerVerifier) Sign(key CryptoKey, data []byte) ([]byte, error) {
	hash, err := extractHashFromRSAKey(key)
	if err != nil {
		return nil, err
	}

	hashedData := hash.New()
	hashedData.Write(data)

	rsaKey, ok := key.handle.(*rsa.PrivateKey)
	if !ok {
		return nil, NewError(InvalidAccessError, "key is not an RSA private key")
	}

	signature, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, hash, hashedData.Sum(nil))
	if err != nil {
		return nil, NewError(OperationError, "could not sign data: "+err.Error())
	}

	return signature, nil
}

func (rsasv *rsaSsaPkcs1v15SignerVerifier) Verify(key CryptoKey, signature []byte, data []byte) (bool, error) {
	hash, err := extractHashFromRSAKey(key)
	if err != nil {
		return false, err
	}

	hashedData := hash.New()
	hashedData.Write(data)

	rsaKey, ok := key.handle.(*rsa.PublicKey)
	if !ok {
		return false, NewError(InvalidAccessError, "key is not an RSA public key")
	}

	err = rsa.VerifyPKCS1v15(rsaKey, hash, hashedData.Sum(nil), signature)
	if err != nil {
		return false, nil //nolint:nilerr
	}

	return true, nil
}

func extractHashFromRSAKey(key CryptoKey) (crypto.Hash, error) {
	unk := crypto.Hash(0)

	rsaHashedAlg, ok := key.Algorithm.(RsaHashedKeyAlgorithm)
	if !ok {
		return unk, NewError(InvalidAccessError, "key algorithm is not an RSA hashed key algorithm")
	}

	hashName, err := rsaHashedAlg.hash()
	if err != nil {
		return unk, err
	}

	return mapHashFn(hashName)
}
