package webcrypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"math/big"

	"github.com/grafana/sobek"
)

// RsaHashedKeyAlgorithm represents the RSA key algorithm as defined by the [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#RsaHashedKeyAlgorithm-dictionary
type RsaHashedKeyAlgorithm struct {
	KeyAlgorithm

	ModulusLength int `js:"modulusLength"`

	Hash Algorithm
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

	hash, err := extractHash(rt, params)
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

// GenerateKey generates a new RSA key pair.
func (rsakgp *RSAHashedKeyGenParams) GenerateKey(
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (CryptoKeyGenerationResult, error) {
	var privateKeyUsages, publicKeyUsages []CryptoKeyUsage

	publicExponent := int(new(big.Int).SetBytes(rsakgp.PublicExponent).Int64())
	if err := validatePublicExponent(publicExponent); err != nil {
		return nil, NewError(
			OperationError,
			fmt.Sprintf("invalid public exponent: %s", err),
		)
	}

	if len(keyUsages) == 0 {
		return nil, NewError(SyntaxError, "key usages cannot be empty")
	}

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
		privateKeyUsages = []CryptoKeyUsage{DecryptCryptoKeyUsage}
		publicKeyUsages = []CryptoKeyUsage{EncryptCryptoKeyUsage}
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
		ModulusLength: rsakgp.ModulusLength,
		KeyAlgorithm: KeyAlgorithm{
			Algorithm: rsakgp.Algorithm,
		},
		Hash: rsakgp.Hash,
	}

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
	privateKey.handle, publicKey.handle, err = generateRSAKeyPair(rsakgp.ModulusLength, publicExponent)
	if err != nil {
		return nil, err
	}

	return &CryptoKeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// validatePublicExponent validates the public exponent.
// it's done same way how golang's rsa package does it + additional check for evenness.
func validatePublicExponent(e int) error {
	if e%2 == 0 {
		return errors.New("public exponent is even")
	}

	if e < 2 {
		return errors.New("public exponent too small")
	}
	if e > 1<<31-1 {
		return errors.New("public exponent too large")
	}

	return nil
}

func generateRSAKeyPair(
	modulusLength int,
	publicExponent int,
) (any, any, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, modulusLength)
	if err != nil {
		return nil, nil, NewError(OperationError, "could not generate RSA key pair")
	}

	privateKey.PublicKey.E = publicExponent

	// validate the key pair, since we are setting the public exponent manually
	if err := privateKey.Validate(); err != nil {
		return nil, nil, NewError(OperationError, "could not validate RSA key pair, check public exponent: "+err.Error())
	}

	return privateKey, privateKey.Public(), nil
}

func exportRSAKey(ck *CryptoKey, format KeyFormat) (interface{}, error) {
	if ck.handle == nil {
		return nil, NewError(OperationError, "key data is not accessible")
	}

	switch format {
	case SpkiKeyFormat:
		if ck.Type != PublicCryptoKeyType {
			return nil, NewError(InvalidAccessError, fmt.Sprintf(errMsgNotExpectedPublicKey, "RSA", ck.handle))
		}

		bytes, err := x509.MarshalPKIXPublicKey(ck.handle)
		if err != nil {
			return nil, NewError(OperationError, "unable to marshal key to SPKI format: "+err.Error())
		}

		return bytes, nil
	case Pkcs8KeyFormat:
		if ck.Type != PrivateCryptoKeyType {
			return nil, NewError(InvalidAccessError, fmt.Sprintf(errMsgNotExpectedPrivateKey, "RSA", ck.handle))
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
	hash, err := extractHash(rt, params)
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
func (rhkip *RSAHashedImportParams) ImportKey(
	format KeyFormat,
	keyData []byte,
	usages []CryptoKeyUsage,
) (*CryptoKey, error) {
	var importFn func(keyData []byte) (any, CryptoKeyType, int, error)

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

	handle, keyType, modusLength, err := importFn(keyData)
	if err != nil {
		return nil, err
	}

	return &CryptoKey{
		Algorithm: RsaHashedKeyAlgorithm{
			ModulusLength: modusLength,
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

func importRSAPrivateKey(keyData []byte) (any, CryptoKeyType, int, error) {
	parsedKey, err := x509.ParsePKCS8PrivateKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, 0, NewError(DataError, "unable to import RSA private key data: "+err.Error())
	}

	privateKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, UnknownCryptoKeyType, 0, NewError(DataError, fmt.Sprintf(errMsgNotExpectedPrivateKey, "RSA", privateKey))
	}

	return privateKey, PrivateCryptoKeyType, privateKey.PublicKey.N.BitLen(), nil
}

func importRSAPublicKey(keyData []byte) (any, CryptoKeyType, int, error) {
	parsedKey, err := x509.ParsePKIXPublicKey(keyData)
	if err != nil {
		return nil, UnknownCryptoKeyType, 0, NewError(DataError, "unable to import RSA public key data: "+err.Error())
	}

	publicKey, ok := parsedKey.(*rsa.PublicKey)
	if !ok {
		return nil, UnknownCryptoKeyType, 0, NewError(DataError, fmt.Sprintf(errMsgNotExpectedPublicKey, "RSA", publicKey))
	}

	return publicKey, PublicCryptoKeyType, publicKey.N.BitLen(), nil
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
		return nil, NewError(InvalidAccessError, fmt.Sprintf(errMsgNotExpectedPrivateKey, "RSA", key.handle))
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
		return false, NewError(InvalidAccessError, fmt.Sprintf(errMsgNotExpectedPublicKey, "RSA", key.handle))
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
		return unk, NewError(InvalidAccessError, "key algorithm is not a RSA hashed key algorithm")
	}

	return mapHashFn(rsaHashedAlg.Hash.Name)
}

func newRSAPssParams(rt *sobek.Runtime, normalized Algorithm, params sobek.Value) (*RSAPssParams, error) {
	saltLength, err := traverseObject(rt, params, "saltLength")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get saltLength from algorithm parameter")
	}

	return &RSAPssParams{
		Algorithm:  normalized,
		SaltLength: int(saltLength.ToInteger()),
	}, nil
}

var _ SignerVerifier = &RSAPssParams{}

// Sign signs the given data.
func (rsasv *RSAPssParams) Sign(key CryptoKey, data []byte) ([]byte, error) {
	rsaKey, ok := key.handle.(*rsa.PrivateKey)
	if !ok {
		return nil, NewError(InvalidAccessError, fmt.Sprintf(errMsgNotExpectedPrivateKey, "RSA", key.handle))
	}

	hash, err := extractHashFromRSAKey(key)
	if err != nil {
		return nil, err
	}

	hashedData := hash.New()
	hashedData.Write(data)

	signature, err := rsa.SignPSS(rand.Reader, rsaKey, hash, hashedData.Sum(nil), &rsa.PSSOptions{
		SaltLength: rsasv.SaltLength,
	})
	if err != nil {
		return nil, NewError(OperationError, "could not sign data: "+err.Error())
	}

	return signature, nil
}

// Verify verifies the signature of the given data.
func (rsasv *RSAPssParams) Verify(key CryptoKey, signature []byte, data []byte) (bool, error) {
	rsaKey, ok := key.handle.(*rsa.PublicKey)
	if !ok {
		return false, NewError(InvalidAccessError, fmt.Sprintf(errMsgNotExpectedPublicKey, "RSA", key.handle))
	}

	hash, err := extractHashFromRSAKey(key)
	if err != nil {
		return false, err
	}

	hashedData := hash.New()
	hashedData.Write(data)

	err = rsa.VerifyPSS(rsaKey, hash, hashedData.Sum(nil), signature, &rsa.PSSOptions{
		SaltLength: rsasv.SaltLength,
	})
	return err == nil, nil
}

// Encrypt .
func (rsaoaep *RSAOaepParams) Encrypt(plaintext []byte, key CryptoKey) ([]byte, error) {
	rsaKey, ok := key.handle.(*rsa.PublicKey)
	if !ok {
		return nil, NewError(InvalidAccessError, fmt.Sprintf(errMsgNotExpectedPublicKey, "RSA", key.handle))
	}

	hash, err := extractHashFromRSAKey(key)
	if err != nil {
		return nil, err
	}

	ciphertext, err := rsa.EncryptOAEP(hash.New(), rand.Reader, rsaKey, plaintext, rsaoaep.Label)
	if err != nil {
		return nil, NewError(OperationError, "could not encrypt data: "+err.Error())
	}

	return ciphertext, nil
}

// Decrypt .
func (rsaoaep *RSAOaepParams) Decrypt(ciphertext []byte, key CryptoKey) ([]byte, error) {
	rsaKey, ok := key.handle.(*rsa.PrivateKey)
	if !ok {
		return nil, NewError(InvalidAccessError, fmt.Sprintf(errMsgNotExpectedPrivateKey, "RSA", key.handle))
	}

	hash, err := extractHashFromRSAKey(key)
	if err != nil {
		return nil, err
	}

	plaintext, err := rsa.DecryptOAEP(hash.New(), rand.Reader, rsaKey, ciphertext, rsaoaep.Label)
	if err != nil {
		return nil, NewError(OperationError, "could not decrypt data: "+err.Error())
	}

	return plaintext, nil
}
