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

// AESKwParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.Encrypt`, `SubtleCrypto.Decrypt`, `SubtleCrypto.WrapKey`, or
// `SubtleCrypto.UnwrapKey`, when using the AES-KW algorithm.
type AESKwParams struct {
	// Name should be set to AlgorithmKindAesKw.
	Name AlgorithmIdentifier
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
	Algorithm

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
	Hash Algorithm
}

// RSAHashedImportParams represents the object that should be passed as the
// algorithm parameter into `SubtleCrypto.ImportKey` or `SubtleCrypto.UnwrapKey`, when
// importing any RSA-based key pair: that is, when the algorithm is identified as any
// of RSASSA-PKCS1-v1_5, RSA-PSS, or RSA-OAEP.
type RSAHashedImportParams struct {
	Algorithm

	// Hash represents the name of the digest function to use.
	// Note that although you can technically pass SHA-1 here, this is strongly
	// discouraged as it is considered vulnerable.
	Hash Algorithm
}

// RSAOaepParams represents the object that should be passed as the algorithm parameter
// into `SubtleCrypto.Encrypt`, `SubtleCrypto.Decrypt`, `SubtleCrypto.WrapKey`, or
// `SubtleCrypto.UnwrapKey`, when using the RSA_OAEP algorithm.
type RSAOaepParams struct {
	Algorithm

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
	Algorithm Algorithm

	// SaltLength holds (a Number) the length of the random salt to use, in bytes.
	// RFC 3447 says that "typical salt lengths" are either 0 or the length of the output
	// of the digest algorithm selected when this key was generated. For instance,
	// when using the SHA256 digest algorithm, the salt length could be 32.
	SaltLength int
}

// RSASsaPkcs1v15Params represents the object that should be passed as the algorithm
type RSASsaPkcs1v15Params struct {
	// Name should be set to AlgorithmKindRsassaPkcs1v15.
	Name AlgorithmIdentifier
}
