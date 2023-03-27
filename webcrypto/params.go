package webcrypto

// From is an interface representing the ability to produce
// an instance from a given generic input. It is an attempt
// to create a contract around construction of objects from
// others.
type From[Input, Output any] interface {
	// From produces an output of type Output from the
	// content of the given input.
	From(Input) (Output, error)
}

// AESCbcParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.Encrypt`, `SubtleCrypto.Decrypt`, `SubtleCrypto.WrapKey`, or
// `SubtleCrypto.UnwrapKey`, when using the AES-CBC algorithm.
type AESCbcParams struct {
	// Name should be set to AES-CBC.
	Name string

	// Iv holds (an ArrayBuffer, a TypedArray, or a DataView) the initialization vector.
	// Must be 16 bytes, unpredictable, and preferably cryptographically random.
	// However, it need not be secret (for example, it may be transmitted unencrypted along with the ciphertext).
	Iv []byte
}

// AESCtrParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.Encrypt`, `SubtleCrypto.Decrypt`, `SubtleCrypto.WrapKey`, or
// `SubtleCrypto.UnwrapKey`, when using the AES-CTR algorithm.
type AESCtrParams struct {
	// Name should be set to AES-CTR.
	Name string

	// Counter holds (an ArrayBuffer, a TypedArray, or a DataView) the initial value of the counter block.
	// This must be 16 bytes long (the AES block size). The rightmost length bits of this block are used
	// for the counter, and the rest is used for the nonce.
	//
	// For example, if length is set to 64, then the first half of counter is
	// the nonce and the second half is used for the counter.
	Counter []byte

	// Length holds (a Number) the number of bits in the counter block that are used for the actual counter.
	// The counter must be big enough that it doesn't wrap: if the message is n blocks and the counter is m bits long, then
	// the following must be true: n <= 2^m.
	//
	// The NIST SP800-38A standard, which defines CTR, suggests that the counter should occupy half of the counter
	// block (see Appendix B.2), so for AES it would be 64.
	Length int
}

// AESGcmParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.Encrypt`, `SubtleCrypto.Decrypt`, `SubtleCrypto.WrapKey`, or
// `SubtleCrypto.UnwrapKey`, when using the AES-GCM algorithm.
// FIXME: How do we ensure that tagLength defaults to 128 in the context of usage within k6?
type AESGcmParams struct {
	// Name should be set to AES-GCM.
	Name string

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
	Iv []byte

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
	AdditionalData []byte

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
	TagLength int
}

// AESKeyGenParams represents the object that should be passed as
// the algorithm parameter into `SubtleCrypto.generateKey`, when generating
// an AES key: that is, when the algorithm is identified as any
// of AES-CBC, AES-CTR, AES-GCM, or AES-KW.
type AESKeyGenParams struct {
	// Name should be set to `AES-CBC`, `AES-CTR`, `AES-GCM`, or `AES-KW`.
	Name AlgorithmIdentifier

	// Length holds (a Number) the length of the key, in bits.
	Length int
}

// AESKwParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.Encrypt`, `SubtleCrypto.Decrypt`, `SubtleCrypto.WrapKey`, or
// `SubtleCrypto.UnwrapKey`, when using the AES-KW algorithm.
type AESKwParams struct {
	// Name should be set to AlgorithmKindAesKw.
	Name AlgorithmIdentifier
}

// The ECDSAParams represents the object that should be passed as the algorithm
// parameter into `SubtleCrypto.Sign` or `SubtleCrypto.Verify“ when using the
// ECDSA algorithm.
type ECDSAParams struct {
	// Name should be set to AlgorithmKindEcdsa.
	Name AlgorithmIdentifier

	// Hash identifies the name of the digest algorithm to use.
	// You can use any of the following:
	//   * [Sha256]
	//   * [Sha384]
	//   * [Sha512]
	Hash AlgorithmIdentifier
}

// ECKeyGenParams  represents the object that should be passed as the algorithm
// parameter into `SubtleCrypto.GenerateKey`, when generating any
// elliptic-curve-based key pair: that is, when the algorithm is identified
// as either of AlgorithmKindEcdsa or AlgorithmKindEcdh.
type ECKeyGenParams struct {
	// Name should be set to AlgorithmKindEcdsa or AlgorithmKindEcdh.
	Name AlgorithmIdentifier

	// NamedCurve holds (a String) the name of the curve to use.
	// You can use any of the following: CurveKindP256, CurveKindP384, or CurveKindP521.
	NamedCurve EllipticCurveKind
}

// ECKeyImportParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.ImportKey` or `SubtleCrypto.UnwrapKey`, when generating any elliptic-curve-based
// key pair: that is, when the algorithm is identified as either of ECDSA or ECDH.
type ECKeyImportParams struct {
	// Name should be set to AlgorithmKindEcdsa or AlgorithmKindEcdh.
	Name AlgorithmIdentifier

	// NamedCurve holds (a String) the name of the elliptic curve to use.
	NamedCurve EllipticCurveKind
}

// HKDFParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.DeriveKey`, when using the HKDF algorithm.
type HKDFParams struct {
	// Name should be set to AlgorithmKindHkdf.
	Name AlgorithmIdentifier

	// Hash should be set to the name of the digest algorithm to use.
	// You can use any of the following:
	//   * [Sha256]
	//   * [Sha384]
	//   * [Sha512]
	Hash AlgorithmIdentifier

	// Salt to use. The HKDF specification states that adding
	// salt "adds significantly to the strength of HKDF".
	// Ideally, the salt is a random or pseudo-random value with
	// the same length as the output of the digest function.
	// Unlike the input key material passed into `SubtleCrypto.DeriveKey`,
	// salt does not need to be kept secret.
	Salt []byte

	// Info holds application-specific contextual information.
	// This is used to bind the derived key to an application or
	// context, and enables you to derive different keys for different
	// contexts while using the same input key material.
	//
	// It's important that this should be independent of the input key material itself.
	// This property is required but may be an empty buffer.
	Info []byte
}

// HMACSignatureParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.Sign`, when using the HMAC algorithm.
type HMACSignatureParams struct {
	// Name should be set to AlgorithmKindHmac.
	Name AlgorithmIdentifier
}

// HMACKeyGenParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.GenerateKey`, when generating an HMAC key.
type HMACKeyGenParams struct {
	// Name should be set to AlgorithmKindHmac.
	Name AlgorithmIdentifier

	// FIXME: SHA-1 should be supported here
	// Hash represents the name of the digest function to use. You can
	// use any of the following: [Sha256], [Sha384],
	// or [Sha512].
	Hash AlgorithmIdentifier

	// FIXME: what's the default value?
	// Length holds (a Number) the length of the key, in bits.
	// If this is omitted, the length of the key is equal to the block size
	// of the hash function you have chosen.
	// Unless you have a good reason to use a different length, omit
	// use the default.
	Length int
}

// HMACImportParams represents the object that should be passed as the
// algorithm parameter into `SubtleCrypto.ImportKey` or `SubtleCrypto.UnwrapKey`, when
// generating a key for the HMAC algorithm.
type HMACImportParams struct {
	// Name should be set to AlgorithmKindHmac.
	Name AlgorithmIdentifier

	// Hash represents the name of the digest function to use.
	Hash AlgorithmIdentifier
}

// PBKDF2Params represents the object that should be passed as the algorithm
// parameter into `SubtleCrypto.DeriveKey`, when using the PBKDF2 algorithm.
type PBKDF2Params struct {
	// Name should be set to AlgorithmKindPbkdf2.
	Name AlgorithmIdentifier

	// FIXME: should also include SHA-1, unfortunately
	// Hash identifies the name of the digest algorithm to use.
	// You can use any of the following:
	//   * [Sha256]
	//   * [Sha384]
	//   * [Sha512]
	Hash AlgorithmIdentifier

	// Salt should hold a random or pseudo-random value of at
	// least 16 bytes. Unlike the input key material passed into
	// `SubtleCrypto.DeriveKey`, salt does not need to be kept secret.
	Salt []byte

	// Iterations the number of times the hash function will be executed
	// in `SubtleCrypto.DeriveKey`. This determines how computationally
	// expensive (that is, slow) the `SubtleCrypto.DeriveKey` operation will be.
	//
	// In this context, slow is good, since it makes it more expensive for an
	// attacker to run a dictionary attack against the keys.
	// The general guidance here is to use as many iterations as possible,
	// subject to keeping an acceptable level of performance for your application.
	Iterations int
}

// RSAHashedKeyGenParams represents the object that should be passed as the algorithm
// parameter into `SubtleCrypto.GenerateKey`, when generating an RSA key pair.
type RSAHashedKeyGenParams struct {
	// Name should be set to AlgorithmKindRsassPkcs1v15,
	// AlgorithmKindRsaPss, or AlgorithmKindRsaOaep.
	Name AlgorithmIdentifier

	// ModulusLength holds (a Number) the length of the RSA modulus, in bits.
	// This should be at least 2048. Some organizations are now recommending
	// that it should be 4096.
	ModulusLength int

	// PublicExponent holds (a Uint8Array) the public exponent to use.
	// Unless you have a good reason to use something else, use 65537 here.
	PublicExponent []byte

	// Hash represents the name of the digest function to use. You can
	// use any of the following: DigestKindSha256, DigestKindSha384,
	// or DigestKindSha512.
	Hash string
}

// RSAHashedImportParams represents the object that should be passed as the
// algorithm parameter into `SubtleCrypto.ImportKey` or `SubtleCrypto.UnwrapKey`, when
// importing any RSA-based key pair: that is, when the algorithm is identified as any
// of RSASSA-PKCS1-v1_5, RSA-PSS, or RSA-OAEP.
type RSAHashedImportParams struct {
	// Name should be set to AlgorithmKindRsassPkcs1v15,
	// AlgorithmKindRsaPss, or AlgorithmKindRsaOaep depending
	// on the algorithm you want to use.
	Name string

	// Hash represents the name of the digest function to use.
	// Note that although you can technically pass SHA-1 here, this is strongly
	// discouraged as it is considered vulnerable.
	Hash AlgorithmIdentifier
}

// RSAOaepParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.Encrypt`, `SubtleCrypto.Decrypt`, `SubtleCrypto.WrapKey`, or
// `SubtleCrypto.UnwrapKey`, when using the RSA_OAEP algorithm.
type RSAOaepParams struct {
	// Name should be set to "RSA-OAEP"
	Name string

	// Label holds (an ArrayBuffer, a TypedArray, or a DataView) an array of bytes that does not
	// itself need to be encrypted but which should be bound to the ciphertext.
	// A digest of the label is part of the input to the encryption operation.
	//
	// Unless your application calls for a label, you can just omit this argument
	// and it will not affect the security of the encryption operation.
	Label []byte
}

// RSAPssParams represents the object that should be passed as the algorithm
// parameter into `SubtleCrypto.Sign` or `SubtleCrypto.Verify`, when using the
// RSA-PSS algorithm.
type RSAPssParams struct {
	// Name should be set to AlgorithmKindRsaPss.
	Name AlgorithmIdentifier

	// SaltLength holds (a Number) the length of the random salt to use, in bytes.
	// RFC 3447 says that "typical salt lengths" are either 0 or the length of the output
	// of the digest algorithm selected whe this key was generated. For instance,
	// when using the SHA256 digest algorithm, the salt length could be 32.
	SaltLength int
}

// RSASsaPkcs1v15Params represents the object that should be passed as the algorithm
type RSASsaPkcs1v15Params struct {
	// Name should be set to AlgorithmKindRsassaPkcs1v15.
	Name AlgorithmIdentifier
}
