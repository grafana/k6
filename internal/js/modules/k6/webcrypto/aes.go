package webcrypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/grafana/sobek"
)

// AESKeyGenParams represents the object that should be passed as
// the algorithm parameter into `SubtleCrypto.generateKey`, when generating
// an AES key: that is, when the algorithm is identified as any
// of AES-CBC, AES-CTR, AES-GCM, or AES-KW.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-keygen-params
type AESKeyGenParams struct {
	Algorithm

	// The length, in bits, of the key.
	Length bitLength `js:"length"`
}

var _ hasAlg = (*AESKeyGenParams)(nil)

func (akgp AESKeyGenParams) alg() string {
	return akgp.Name
}

// newAESKeyGenParams creates a new AESKeyGenParams object, from the
// normalized algorithm, and the algorithm parameters.
//
// It handles the logic involved in handling the `length` attribute,
// which is not part of the normalized algorithm.
func newAESKeyGenParams(rt *sobek.Runtime, normalized Algorithm, params sobek.Value) (*AESKeyGenParams, error) {
	// We extract the length attribute from the params object, as it's not
	// part of the normalized algorithm, and as accessing the runtime from the
	// callback below could lead to a race condition.
	algorithmLengthValue, err := traverseObject(rt, params, "length")
	if err != nil {
		return nil, NewError(SyntaxError, "could not get length from algorithm parameter")
	}

	algorithmLength := algorithmLengthValue.ToInteger()

	return &AESKeyGenParams{
		Algorithm: normalized,
		Length:    bitLength(algorithmLength),
	}, nil
}

// GenerateKey generates a new AES key, according to the algorithm
// described in the specification.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-keygen-params
func (akgp *AESKeyGenParams) GenerateKey(
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (CryptoKeyGenerationResult, error) {
	for _, usage := range keyUsages {
		switch usage {
		case WrapKeyCryptoKeyUsage, UnwrapKeyCryptoKeyUsage:
			continue
		case EncryptCryptoKeyUsage, DecryptCryptoKeyUsage:
			// At the time of writing, the go standard library [doesn't
			// support AES-KW](https://github.com/golang/go/issues/27599), we
			// might want to revisit this in the future.
			if akgp.Algorithm.Name != AESKw {
				continue
			}

			return nil, NewError(SyntaxError, "invalid key usage")
		default:
			return nil, NewError(SyntaxError, "invalid key usage")
		}
	}

	if akgp.Length != 128 && akgp.Length != 192 && akgp.Length != 256 {
		return nil, NewError(OperationError, "invalid key length")
	}

	randomKey := make([]byte, akgp.Length.asByteLength())
	if _, err := rand.Read(randomKey); err != nil {
		// 4.
		return nil, NewError(OperationError, "could not generate random key")
	}

	// 5. 6. 7. 8. 9.
	key := CryptoKey{}
	key.Type = SecretCryptoKeyType
	key.Algorithm = &AESKeyAlgorithm{
		Algorithm: akgp.Algorithm,
		Length:    int64(akgp.Length),
	}

	// 10.
	key.Extractable = extractable

	// 11.
	key.Usages = keyUsages

	// Set key handle to our random key.
	key.handle = randomKey

	// 12.
	return &key, nil
}

// Ensure that AESKeyGenParams implements the KeyGenerator interface.
var _ KeyGenerator = &AESKeyGenParams{}

// AESKeyAlgorithm is the algorithm for AES keys as defined in the [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#dfn-AESKeyAlgorithm
type AESKeyAlgorithm struct {
	Algorithm

	Length int64 `js:"length"`
}

var _ hasAlg = (*AESKeyAlgorithm)(nil)

func (aka AESKeyAlgorithm) alg() string {
	return aka.Name
}

// exportAESKey exports an AES key to its raw representation.
func exportAESKey(key *CryptoKey, format KeyFormat) (interface{}, error) {
	if !key.Extractable {
		return nil, NewError(InvalidAccessError, "the key is not extractable")
	}

	// 1.
	if key.handle == nil {
		return nil, NewError(OperationError, "the key is not valid, no data")
	}

	switch format {
	case RawKeyFormat:
		handle, ok := key.handle.([]byte)
		if !ok {
			return nil, NewError(ImplementationError, "exporting key data's bytes failed")
		}

		return handle, nil
	case JwkKeyFormat:
		m, err := exportSymmetricJWK(key)
		if err != nil {
			return nil, NewError(ImplementationError, err.Error())
		}

		return m, nil

	default:
		return nil, NewError(NotSupportedError, unsupportedKeyFormatErrorMsg+" "+format)
	}
}

// AESImportParams is an internal placeholder struct for AES import parameters.
// Although not described by the specification, we define it to be able to implement
// our internal KeyImporter interface.
type AESImportParams struct {
	Algorithm
}

func newAESImportParams(normalized Algorithm) *AESImportParams {
	return &AESImportParams{
		Algorithm: normalized,
	}
}

// ImportKey imports an AES key from its raw representation.
// It implements the KeyImporter interface.
func (aip *AESImportParams) ImportKey(
	format KeyFormat,
	keyData []byte,
	keyUsages []CryptoKeyUsage,
) (*CryptoKey, error) {
	for _, usage := range keyUsages {
		switch usage {
		case EncryptCryptoKeyUsage, DecryptCryptoKeyUsage, WrapKeyCryptoKeyUsage, UnwrapKeyCryptoKeyUsage:
			continue
		default:
			return nil, NewError(SyntaxError, "invalid key usage: "+usage)
		}
	}

	// only raw and jwk formats are supported for HMAC
	if format != RawKeyFormat && format != JwkKeyFormat {
		return nil, NewError(NotSupportedError, unsupportedKeyFormatErrorMsg+" "+format)
	}

	// if the key is in JWK format, we need to extract the symmetric key from it
	if format == JwkKeyFormat {
		var err error
		keyData, err = extractSymmetricJWK(keyData)
		if err != nil {
			return nil, NewError(DataError, err.Error())
		}
	}

	// check the key length
	if !isAESBitsLengthValid(len(keyData)) {
		return nil, NewError(DataError, fmt.Sprintf("invalid key length %v bytes", len(keyData)))
	}

	key := &CryptoKey{
		Algorithm: AESKeyAlgorithm{
			Algorithm: aip.Algorithm,
			Length:    int64(byteLength(len(keyData)).asBitLength()),
		},
		Type:   SecretCryptoKeyType,
		handle: keyData,
	}

	return key, nil
}

// isAESBitsLengthValid returns true if the given length is a valid AES key length.
// As per the [specification].
func isAESBitsLengthValid(length int) bool {
	return length == 16 || length == 24 || length == 32
}

// Ensure that AESImportParams implements the KeyImporter interface.
var _ KeyImporter = &AESImportParams{}

// AESCBCParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.Encrypt`, `SubtleCrypto.Decrypt`, `SubtleCrypto.WrapKey`, or
// `SubtleCrypto.UnwrapKey`, when using the AES-CBC algorithm.
//
// As defined in the [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-cbc-params
type AESCBCParams struct {
	Algorithm

	// Name should be set to AES-CBC.
	Name string `js:"name"`

	// Iv holds (an ArrayBuffer, a TypedArray, or a DataView) the initialization vector.
	// Must be 16 bytes, unpredictable, and preferably cryptographically random.
	// However, it need not be secret (for example, it may be transmitted unencrypted along with the ciphertext).
	Iv []byte `js:"iv"`
}

// Encrypt encrypts the given plaintext using the AES-CBC algorithm, and returns the ciphertext.
// Implements the WebCryptoAPI `encrypt` method's [specification] for the AES-CBC algorithm.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-cbc
func (acp *AESCBCParams) Encrypt(plaintext []byte, key CryptoKey) ([]byte, error) {
	// 1.
	// Note that aes.BlockSize stands for the `k` variable as per the specification.
	if len(acp.Iv) != aes.BlockSize {
		return nil, NewError(OperationError, "iv length is not 16 bytes")
	}

	// 2.
	paddedPlainText, err := pKCS7Pad(plaintext, aes.BlockSize)
	if err != nil {
		return nil, NewError(OperationError, "could not pad plaintext")
	}

	keyHandle, ok := key.handle.([]byte)
	if !ok {
		return nil, NewError(ImplementationError, "could not get key handle")
	}

	// 3.
	block, err := aes.NewCipher(keyHandle)
	if err != nil {
		return nil, NewError(OperationError, "could not create cipher")
	}

	ciphertext := make([]byte, len(paddedPlainText))
	cbc := cipher.NewCBCEncrypter(block, acp.Iv)
	cbc.CryptBlocks(ciphertext, paddedPlainText)

	return ciphertext, nil
}

// Decrypt decrypts the given ciphertext using the AES-CBC algorithm, and returns the plaintext.
// Implements the WebCryptoAPI's `decrypt` method's [specification] for the AES-CBC algorithm.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-cbc
func (acp *AESCBCParams) Decrypt(ciphertext []byte, key CryptoKey) ([]byte, error) {
	// 1.
	if len(acp.Iv) != aes.BlockSize {
		return nil, NewError(OperationError, "iv length is invalid, should be 16 bytes")
	}

	keyHandle, ok := key.handle.([]byte)
	if !ok {
		return nil, NewError(OperationError, "invalid key handle")
	}

	// 2.
	block, err := aes.NewCipher(keyHandle)
	if err != nil {
		return nil, NewError(OperationError, "could not create AES cipher")
	}

	paddedPlainText := make([]byte, len(ciphertext))
	cbc := cipher.NewCBCDecrypter(block, acp.Iv)
	cbc.CryptBlocks(paddedPlainText, ciphertext)

	// 3.
	p := paddedPlainText[len(paddedPlainText)-1]
	if p == 0 || p > aes.BlockSize {
		return nil, NewError(OperationError, "invalid padding")
	}

	// 4.
	if !bytes.HasSuffix(paddedPlainText, bytes.Repeat([]byte{p}, int(p))) {
		return nil, NewError(OperationError, "invalid padding")
	}

	// 5.
	plaintext := paddedPlainText[:len(paddedPlainText)-int(p)]

	return plaintext, nil
}

// Ensure that AESCBCParams implements the EncryptDecrypter interface.
var _ EncryptDecrypter = &AESCBCParams{}

// AESCTRParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.Encrypt`, `SubtleCrypto.Decrypt`, `SubtleCrypto.WrapKey`, or
// `SubtleCrypto.UnwrapKey`, when using the AES-CTR algorithm.
//
// As defined in the [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-ctr-params
type AESCTRParams struct {
	Algorithm

	// Counter holds (an ArrayBuffer, a TypedArray, or a DataView) the initial value of the counter block.
	// This must be 16 bytes long (the AES block size). The rightmost length bits of this block are used
	// for the counter, and the rest is used for the nonce.
	//
	// For example, if length is set to 64, then the first half of counter is
	// the nonce and the second half is used for the counter.
	Counter []byte `js:"counter"`

	// Length holds (a Number) the number of bits in the counter block that are used for the actual counter.
	// The counter must be big enough that it doesn't wrap: if the message is n blocks and the counter is m bits long, then
	// the following must be true: n <= 2^m.
	//
	// The NIST SP800-38A standard, which defines CTR, suggests that the counter should occupy half of the counter
	// block (see Appendix B.2), so for AES it would be 64.
	Length int `js:"length"`
}

// Encrypt encrypts the given plaintext using the AES-CTR algorithm, and returns the ciphertext.
// Implements the WebCryptoAPI's `encrypt` method's [specification] for the AES-CTR algorithm.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-ctr
func (acp *AESCTRParams) Encrypt(plaintext []byte, key CryptoKey) ([]byte, error) {
	// 1.
	// Note that aes.BlockSize stands for the `k` variable as per the specification.
	if len(acp.Counter) != aes.BlockSize {
		return nil, NewError(OperationError, "counter length is not 16 bytes")
	}

	// 2.
	if acp.Length <= 0 || acp.Length > 128 {
		return nil, NewError(OperationError, "invalid counter length, out of the 0 < x < 128 bounds")
	}

	keyHandle, ok := key.handle.([]byte)
	if !ok {
		return nil, NewError(ImplementationError, "could not get key handle")
	}

	// 3.
	block, err := aes.NewCipher(keyHandle)
	if err != nil {
		return nil, NewError(OperationError, "could not create cipher")
	}

	ciphertext := make([]byte, len(plaintext))
	ctr := cipher.NewCTR(block, acp.Counter)
	ctr.XORKeyStream(ciphertext, plaintext)

	return ciphertext, nil
}

// Decrypt decrypts the given ciphertext using the AES-CTR algorithm, and returns the plaintext.
// Implements the WebCryptoAPI's `decrypt` method's [specification] for the AES-CTR algorithm.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-ctr
func (acp *AESCTRParams) Decrypt(ciphertext []byte, key CryptoKey) ([]byte, error) {
	// 1.
	if len(acp.Counter) != aes.BlockSize {
		return nil, NewError(OperationError, "counter length is invalid, should be 16 bytes")
	}

	// 2.
	if acp.Length <= 0 || acp.Length > 128 {
		return nil, NewError(OperationError, "invalid length, should be within 1 <= length <= 128 bounds")
	}

	keyHandle, ok := key.handle.([]byte)
	if !ok {
		return nil, NewError(OperationError, "invalid key handle")
	}

	// 3.
	block, err := aes.NewCipher(keyHandle)
	if err != nil {
		return nil, NewError(OperationError, "could not create AES cipher")
	}

	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCTR(block, acp.Counter)
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// Ensure that AESCTRParams implements the EncryptDecrypter interface.
var _ EncryptDecrypter = &AESCTRParams{}

// AESGCMParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.Encrypt`, `SubtleCrypto.Decrypt`, `SubtleCrypto.WrapKey`, or
// `SubtleCrypto.UnwrapKey`, when using the AES-GCM algorithm.
// As defined in the [specification].
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-gcm-params
type AESGCMParams struct {
	Algorithm

	// Iv holds (an ArrayBuffer, a TypedArray, or a DataView) with the initialization vector.
	// This must be unique for every encryption operation carried out with a given key.
	//
	// Put another way: never reuse an IV with the same key.
	// The AES-GCM specification recommends that the IV should be 96 bits long, and
	// typically contains bits from a random number generator.
	//
	// Section 8.2 of the specification outlines methods for constructing IVs.
	// Note that the IV does not have to be secret, just unique: so it is OK, for example, to
	// transmit it in the clear alongside the encrypted message.
	Iv []byte `js:"iv"`

	// AdditionalData (an ArrayBuffer, a TypedArray, or a DataView) contains additional data that will
	// not be encrypted but will be authenticated along with the encrypted data.
	//
	// If additionalData is given here then the same data must be given in the corresponding call
	// to decrypt(): if the data given to the decrypt() call does not match the original data, the
	// decryption will throw an exception.
	// This gives you a way to authenticate associated data without having to encrypt it.
	//
	// The bit length of additionalData must be smaller than 2^64 - 1.
	//
	// The additionalData property is optional and may be omitted without compromising the
	// security of the encryption operation.
	AdditionalData []byte `js:"additionalData"`

	// TagLength (a Number) determines the size in bits of the authentication tag generated in
	// the encryption operation and used for authentication in the corresponding decryption.
	//
	// According to the Web Crypto specification this must have one of the
	// following values: 32, 64, 96, 104, 112, 120, or 128.
	// The AES-GCM specification recommends that it should be 96, 104, 112, 120 or 128, although
	// 32 or 64 bits may be acceptable
	// in some applications: Appendix C of the specification provides additional guidance here.
	//
	// tagLength is optional and defaults to 128 if it is not specified.
	TagLength bitLength `js:"tagLength"`
}

// Encrypt encrypts the given plaintext using the AES-GCM algorithm, and returns the ciphertext.
// Implements the WebCryptoAPI's `encrypt` method's [specification] for the AES-GCM algorithm.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-gcm
func (agp *AESGCMParams) Encrypt(plaintext []byte, key CryptoKey) ([]byte, error) {
	// 1.
	// As described in section 8 of AES-GCM [NIST SP800-38D].
	// [NIST SP800-38D] https://nvlpubs.nist.gov/nistpubs/Legacy/SP/nistspecialpublication800-38d.pdf
	if uint64(len(plaintext)) > maxAESGCMPlaintextLength {
		return nil, NewError(OperationError, "plaintext length is too long")
	}

	// 2.
	// As described in section 8 of AES-GCM [NIST SP800-38D].
	// [NIST SP800-38D] https://nvlpubs.nist.gov/nistpubs/Legacy/SP/nistspecialpublication800-38d.pdf
	//
	// NOTE @oleiade: that the spec targets to support multiple IV lengths
	// but go only supports 12 bytes IVs. We therefore are diverging from the
	// spec here, and have adjusted the test suite accordingly.
	if len(agp.Iv) != 12 {
		return nil, NewError(NotSupportedError, "only 12 bytes long iv are supported")
	}

	// 3.
	// As described in section 8 of AES-GCM [NIST SP800-38D].
	// [NIST SP800-38D] https://nvlpubs.nist.gov/nistpubs/Legacy/SP/nistspecialpublication800-38d.pdf
	if agp.AdditionalData != nil && (uint64(len(agp.AdditionalData)) > maxAESGcmAdditionalDataLength) {
		return nil, NewError(OperationError, "additional data length is too long")
	}

	// 4.
	var tagLength bitLength
	if agp.TagLength == 0 {
		tagLength = 128
	} else {
		switch agp.TagLength {
		case 96, 104, 112, 120, 128:
			tagLength = agp.TagLength
		case 32, 64:
			// Go's GCM implementation does not support 32 or 64 bit tag lengths.
			return nil, NewError(NotSupportedError, "tag length 32 and 64 are not supported")
		default:
			return nil, NewError(OperationError, "invalid tag length, should be one of 96, 104, 112, 120, 128")
		}
	}

	keyHandle, ok := key.handle.([]byte)
	if !ok {
		return nil, NewError(ImplementationError, "could not get key data")
	}

	// 6.
	block, err := aes.NewCipher(keyHandle)
	if err != nil {
		return nil, NewError(OperationError, "could not create cipher")
	}

	gcm, err := cipher.NewGCMWithTagSize(block, int(tagLength.asByteLength()))
	if err != nil {
		return nil, NewError(ImplementationError, "could not create cipher")
	}

	// The Golang AES GCM cipher only supports a Nonce/Iv length of 12 bytes,
	// as opposed to the looser requirements of the Web Crypto API spec.
	if len(agp.Iv) != gcm.NonceSize() {
		return nil, NewError(NotSupportedError, "only 12 bytes long iv are supported")
	}

	// 7. 8.
	// Note that the `Seal` operation adds the tag component at the end of
	// the ciphertext.
	return gcm.Seal(nil, agp.Iv, plaintext, agp.AdditionalData), nil
}

// Decrypt decrypts the given ciphertext using the AES-GCM algorithm, and returns the plaintext.
// Implements the WebCryptoAPI's `decrypt` method's [specification] for the AES-GCM algorithm.
//
// [specification]: https://www.w3.org/TR/WebCryptoAPI/#aes-gcm
func (agp *AESGCMParams) Decrypt(ciphertext []byte, key CryptoKey) ([]byte, error) {
	// 1.
	var tagLength bitLength
	if agp.TagLength == 0 {
		tagLength = 128
	} else {
		switch agp.TagLength {
		case 96, 104, 112, 120, 128:
			tagLength = agp.TagLength
		case 32, 64:
			// Go's AES GCM implementation does not support 32 or 64 bit tag lengths.
			return nil, NewError(OperationError, "invalid tag length, should be within 96 <= length <= 128 bounds")
		default:
			return nil, NewError(OperationError, "invalid tag length, accepted values are 96, 104, 112, 120, 128")
		}
	}

	// 2.
	// Note that we multiply the length of the ciphertext by 8, in order
	// to get the length in bits.
	if byteLength(len(ciphertext)).asBitLength() < tagLength {
		return nil, NewError(OperationError, "ciphertext is too short")
	}

	// 3.
	if len(agp.Iv) < 1 || uint64(len(agp.Iv)) > maxAESGcmIvLength {
		return nil, NewError(OperationError, "iv length is too long")
	}

	// 4.
	if agp.AdditionalData != nil && uint64(len(agp.AdditionalData)) > maxAESGcmAdditionalDataLength {
		return nil, NewError(OperationError, "additional data is too long")
	}

	// 5. 6. are not necessary as Go's AES GCM implementation perform those steps for us

	keyHandle, ok := key.handle.([]byte)
	if !ok {
		return nil, NewError(OperationError, "invalid key handle")
	}

	// 7. 8.
	block, err := aes.NewCipher(keyHandle)
	if err != nil {
		return nil, NewError(OperationError, "could not create AES cipher")
	}

	gcm, err := cipher.NewGCMWithTagSize(block, int(tagLength.asByteLength()))
	if err != nil {
		return nil, NewError(OperationError, "could not create GCM cipher")
	}

	// The Golang AES GCM cipher only supports a Nonce/Iv length of 12 bytes,
	plaintext, err := gcm.Open(nil, agp.Iv, ciphertext, agp.AdditionalData)
	if err != nil {
		return nil, NewError(OperationError, "could not decrypt ciphertext")
	}

	return plaintext, nil
}

// maxAESGCMPlaintextLength holds the value (2 ^ 39) - 256 as specified in
// The [Web Crypto API spec] for the AES-GCM algorithm encryption operation.
//
// [Web Crypto API spec]: https://www.w3.org/TR/WebCryptoAPI/#aes-gcm-encryption-operation
const maxAESGCMPlaintextLength uint64 = (1 << 39) - 256

// maxAESGcmAdditionalDataLength holds the value 2 ^ 64 - 1 as specified in
// the [Web Crypto API spec] for the AES-GCM algorithm encryption operation.
//
// [Web Crypto API spec]: https://www.w3.org/TR/WebCryptoAPI/#aes-gcm-encryption-operation
const maxAESGcmAdditionalDataLength uint64 = (1 << 64) - 1

// maxAESGcmIvLength holds the value 2 ^ 64 - 1 as specified in
// the [Web Crypto API spec] for the AES-GCM algorithm encryption operation.
//
// [Web Crypto API spec]: https://www.w3.org/TR/WebCryptoAPI/#aes-gcm-encryption-operation
const maxAESGcmIvLength uint64 = (1 << 64) - 1

var (
	// ErrInvalidBlockSize is returned when the given block size is invalid.
	ErrInvalidBlockSize = errors.New("invalid block size")

	// ErrInvalidPkcs7Data is returned when the given data is invalid.
	ErrInvalidPkcs7Data = errors.New("invalid PKCS7 data")
)

// pKCS7Padding adds PKCS7 padding to the given plaintext.
// It implements section 10.3 of [RFC 2315].
//
// [RFC 2315]: https://www.rfc-editor.org/rfc/rfc2315#section-10.3
func pKCS7Pad(plaintext []byte, blockSize int) ([]byte, error) {
	if blockSize <= 0 {
		return nil, ErrInvalidBlockSize
	}

	if len(plaintext) == 0 {
		return nil, ErrInvalidPkcs7Data
	}

	l := len(plaintext)
	padding := blockSize - (l % blockSize)
	paddingText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(plaintext, paddingText...), nil
}

// unsupportedKeyFormatErrorMsg is the error message returned when an unsupported
// key format is passed to a function.
//
// This is defined as a constant to cater to linter warnings.
const unsupportedKeyFormatErrorMsg = "unsupported key format"
