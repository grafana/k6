/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package crypto

import (
	"context"
	gocrypto "crypto"
	"crypto/dsa"
	"crypto/rand"
	"crypto/rsa"
	"encoding/asn1"
	"math/big"
	"strconv"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/modules/k6/crypto/x509"
	"github.com/pkg/errors"
)

// SigningOptions configures a sign or verify operation
type SigningOptions map[string]string

// Verifier verifies the signature of chunked input
type Verifier struct {
	ctx             *context.Context
	function        gocrypto.Hash
	functionEncoded string
	options         SigningOptions
	plaintext       []byte
}

// Signer produces a signature of chunked input
type Signer struct {
	ctx             *context.Context
	function        gocrypto.Hash
	functionEncoded string
	options         SigningOptions
	plaintext       []byte
}

type dsaSignature struct {
	R *big.Int
	S *big.Int
}

// Verify checks for a valid message signature
func (*Crypto) Verify(
	ctx *context.Context,
	signer x509.PublicKey,
	functionEncoded string,
	plaintextEncoded interface{},
	signatureEncoded interface{},
	options SigningOptions,
) bool {
	function, digest, signature :=
		prepareVerify(ctx, functionEncoded, plaintextEncoded, signatureEncoded)
	verified, err :=
		executeVerify(&signer, function, digest, signature, options)
	if err != nil {
		throw(ctx, err)
	}
	return verified
}

// VerifyString checks for a valid signature of a string message
func (surface *Crypto) VerifyString(
	ctx *context.Context,
	signer x509.PublicKey,
	functionEncoded string,
	plaintextEncoded string,
	signatureEncoded interface{},
	options SigningOptions,
) bool {
	function, digest, signature := prepareVerifyString(
		ctx, functionEncoded, plaintextEncoded, signatureEncoded)
	verified, err :=
		executeVerify(&signer, function, digest, signature, options)
	if err != nil {
		throw(ctx, err)
	}
	return verified
}

// Sign produces a message signature
func (*Crypto) Sign(
	ctx *context.Context,
	signer x509.PrivateKey,
	functionEncoded string,
	plaintextEncoded interface{},
	format string,
	options SigningOptions,
) interface{} {
	function, digest := prepareSign(ctx, functionEncoded, plaintextEncoded)
	signature, err := executeSign(&signer, function, digest, format, options)
	if err != nil {
		throw(ctx, err)
	}
	return signature
}

// SignString products a signature of a string message
func (*Crypto) SignString(
	ctx *context.Context,
	signer x509.PrivateKey,
	functionEncoded string,
	plaintextEncoded string,
	format string,
	options SigningOptions,
) interface{} {
	function, digest :=
		prepareSignString(ctx, functionEncoded, plaintextEncoded)
	signature, err := executeSign(&signer, function, digest, format, options)
	if err != nil {
		throw(ctx, err)
	}
	return signature
}

// CreateVerify creates a chunked verifier
func (*Crypto) CreateVerify(
	ctx *context.Context,
	functionEncoded string,
	options SigningOptions,
) *Verifier {
	function, err := decodeFunction(functionEncoded)
	if err != nil {
		throw(ctx, err)
	}
	return &Verifier{
		ctx:             ctx,
		function:        function,
		functionEncoded: functionEncoded,
		options:         options,
	}
}

// CreateSign creates a chunked signer
func (*Crypto) CreateSign(
	ctx *context.Context,
	functionEncoded string,
	options SigningOptions,
) *Signer {
	function, err := decodeFunction(functionEncoded)
	if err != nil {
		throw(ctx, err)
	}
	return &Signer{
		ctx:             ctx,
		function:        function,
		functionEncoded: functionEncoded,
		options:         options,
	}
}

// Update appends to a verifier plaintext
func (verifier *Verifier) Update(additionEncoded interface{}, format string) {
	addition, err := decodeBinary(additionEncoded, format)
	if err != nil {
		throw(verifier.ctx, err)
	}
	verifier.plaintext = append(verifier.plaintext, addition...)
}

// Verify checks for a valid signature of a verifier plaintext
func (verifier *Verifier) Verify(
	signer x509.PublicKey,
	signatureEncoded interface{},
) bool {
	signature, err := decodeSignature(signatureEncoded)
	if err != nil {
		throw(verifier.ctx, err)
	}
	digest, err := hashPlaintext(
		verifier.ctx,
		verifier.functionEncoded,
		verifier.plaintext,
	)
	if err != nil {
		throw(verifier.ctx, err)
	}
	verified, err := executeVerify(
		&signer,
		verifier.function,
		digest,
		signature,
		verifier.options,
	)
	if err != nil {
		throw(verifier.ctx, err)
	}
	return verified
}

// Update appends to a signer plaintext
func (signer *Signer) Update(additionEncoded interface{}, format string) {
	addition, err := decodeBinary(additionEncoded, format)
	if err != nil {
		throw(signer.ctx, err)
	}
	signer.plaintext = append(signer.plaintext, addition...)
}

// Sign produces a signature of a signer plaintext
func (signer *Signer) Sign(
	key x509.PrivateKey,
	format string,
) interface{} {
	digest, err := hashPlaintext(
		signer.ctx,
		signer.functionEncoded,
		signer.plaintext,
	)
	if err != nil {
		throw(signer.ctx, err)
	}
	signature, err := executeSign(
		&key,
		signer.function,
		digest,
		format,
		signer.options,
	)
	if err != nil {
		throw(signer.ctx, err)
	}
	return signature
}

func prepareVerify(
	ctx *context.Context,
	functionEncoded string,
	plaintextEncoded interface{},
	signatureEncoded interface{},
) (gocrypto.Hash, []byte, []byte) {
	function, err := decodeFunction(functionEncoded)
	if err != nil {
		throw(ctx, err)
	}
	plaintext, err := decodePlaintext(plaintextEncoded)
	if err != nil {
		throw(ctx, err)
	}
	digest, err := hashPlaintext(ctx, functionEncoded, plaintext)
	if err != nil {
		throw(ctx, err)
	}
	signature, err := decodeSignature(signatureEncoded)
	if err != nil {
		throw(ctx, err)
	}
	return function, digest, signature
}

func prepareVerifyString(
	ctx *context.Context,
	functionEncoded string,
	plaintextEncoded string,
	signatureEncoded interface{},
) (gocrypto.Hash, []byte, []byte) {
	function, err := decodeFunction(functionEncoded)
	if err != nil {
		throw(ctx, err)
	}
	plaintext := []byte(plaintextEncoded)
	digest, err := hashPlaintext(ctx, functionEncoded, plaintext)
	if err != nil {
		throw(ctx, err)
	}
	signature, err := decodeSignature(signatureEncoded)
	if err != nil {
		throw(ctx, err)
	}
	return function, digest, signature
}

func executeVerify(
	signer *x509.PublicKey,
	function gocrypto.Hash,
	digest []byte,
	signature []byte,
	options SigningOptions,
) (bool, error) {
	var verified bool = false
	var err error = nil
	switch signer.Type {
	case "RSA":
		verified, err =
			verifyRSA(signer.RSA, function, digest, signature, options)
	case "DSA":
		verified, err = verifyDSA(signer.DSA, digest, signature)
	default:
		err = errors.New("invalid public key")
	}
	if err != nil {
		return false, err
	}
	return verified, nil
}

func verifyRSA(
	signer *rsa.PublicKey,
	function gocrypto.Hash,
	digest []byte,
	signature []byte,
	options SigningOptions,
) (bool, error) {
	switch options["type"] {
	case "":
		return verifyPKCS(signer, function, digest, signature), nil
	case "pss":
		return verifyPSS(signer, function, digest, signature, options), nil
	default:
		err := errors.New("unsupported type: " + options["type"])
		return false, err
	}
}

func verifyPKCS(
	signer *rsa.PublicKey,
	function gocrypto.Hash,
	digest []byte,
	signature []byte,
) bool {
	err := rsa.VerifyPKCS1v15(signer, function, digest, signature)
	if err != nil {
		return false
	}
	return true
}

func verifyPSS(
	signer *rsa.PublicKey,
	function gocrypto.Hash,
	digest []byte,
	signature []byte,
	options SigningOptions,
) bool {
	config := decodePssOptions(options)
	err := rsa.VerifyPSS(signer, function, digest, signature, &config)
	if err != nil {
		return false
	}
	return true

}

func verifyDSA(
	signer *dsa.PublicKey,
	digest []byte,
	signatureDer []byte,
) (bool, error) {
	var signature dsaSignature
	_, err := asn1.Unmarshal(signatureDer, &signature)
	if err != nil {
		return false, err
	}
	verified := dsa.Verify(signer, digest, signature.R, signature.S)
	return verified, nil
}

func prepareSign(
	ctx *context.Context,
	functionEncoded string,
	plaintextEncoded interface{},
) (gocrypto.Hash, []byte) {
	function, err := decodeFunction(functionEncoded)
	if err != nil {
		throw(ctx, err)
	}
	plaintext, err := decodePlaintext(plaintextEncoded)
	if err != nil {
		throw(ctx, err)
	}
	digest, err := hashPlaintext(ctx, functionEncoded, plaintext)
	if err != nil {
		throw(ctx, err)
	}
	return function, digest
}

func prepareSignString(
	ctx *context.Context,
	functionEncoded string,
	plaintextEncoded string,
) (gocrypto.Hash, []byte) {
	function, err := decodeFunction(functionEncoded)
	if err != nil {
		throw(ctx, err)
	}
	plaintext := []byte(plaintextEncoded)
	digest, err := hashPlaintext(ctx, functionEncoded, plaintext)
	if err != nil {
		throw(ctx, err)
	}
	return function, digest
}

func executeSign(
	signer *x509.PrivateKey,
	function gocrypto.Hash,
	digest []byte,
	format string,
	options SigningOptions,
) (interface{}, error) {
	var signature []byte
	var err error
	switch signer.Type {
	case "RSA":
		signature, err = signRSA(signer.RSA, function, digest, options)
	default:
		err = errors.New("invalid private key")
	}
	if err != nil {
		return "", err
	}
	encoded, err := encodeBinary(signature, format)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func signRSA(
	signer *rsa.PrivateKey,
	function gocrypto.Hash,
	digest []byte,
	options SigningOptions,
) ([]byte, error) {
	switch options["type"] {
	case "":
		return signPKCS(signer, function, digest)
	case "pss":
		return signPSS(signer, function, digest, options)
	default:
		err := errors.New("unsupported type: " + options["type"])
		return nil, err
	}
}

func signPKCS(
	signer *rsa.PrivateKey,
	function gocrypto.Hash,
	digest []byte,
) ([]byte, error) {
	signature, err := rsa.SignPKCS1v15(rand.Reader, signer, function, digest)
	if err != nil {
		err = errors.Wrap(err, "failed to sign message")
		return nil, err
	}
	return signature, nil
}

func signPSS(
	signer *rsa.PrivateKey,
	function gocrypto.Hash,
	digest []byte,
	options SigningOptions,
) ([]byte, error) {
	config := decodePssOptions(options)
	signature, err :=
		rsa.SignPSS(rand.Reader, signer, function, digest, &config)
	if err != nil {
		err = errors.Wrap(err, "failed to sign message")
		return nil, err
	}
	return signature, nil
}

func decodeInt(encoded string) int {
	decoded, err := strconv.ParseInt(encoded, 10, 64)
	if err != nil {
		return 0
	}
	return int(decoded)
}

func decodePssOptions(options SigningOptions) rsa.PSSOptions {
	return rsa.PSSOptions{
		SaltLength: decodeInt(options["saltLength"]),
	}
}

// Uncomment cases to enable as hash functions are added
func decodeFunction(encoded string) (gocrypto.Hash, error) {
	switch encoded {
	case "md4":
		return gocrypto.MD4, nil
	case "md5":
		return gocrypto.MD5, nil
	case "sha1":
		return gocrypto.SHA1, nil
	// case "sha224":
	// 	return gocrypto.SHA224, nil
	case "sha256":
		return gocrypto.SHA256, nil
	case "sha384":
		return gocrypto.SHA384, nil
	case "sha512":
		return gocrypto.SHA512, nil
	// case "md5sha1":
	// 	return gocrypto.MD5SHA1, nil
	case "ripemd160":
		return gocrypto.RIPEMD160, nil
	// case "sha3_224":
	// 	return gocrypto.SHA3_224, nil
	// case "sha3_256":
	// 	return gocrypto.SHA3_256, nil
	// case "sha3_384":
	// 	return gocrypto.SHA3_384, nil
	// case "sha3_512":
	// 	return gocrypto.SHA3_512, nil
	case "sha512_224":
		return gocrypto.SHA512_224, nil
	case "sha512_256":
		return gocrypto.SHA512_256, nil
	// case "blake2s_256":
	// 	return gocrypto.BLAKE2s_256, nil
	// case "blake2b_256":
	// 	return gocrypto.BLAKE2b_256, nil
	// case "blake2b_384":
	// 	return gocrypto.BLAKE2b_384, nil
	// case "blake2b_512":
	// 	return gocrypto.BLAKE2b_512, nil
	default:
		err := errors.New("unsupported hash function: " + encoded)
		return 0, err
	}
}

func hashPlaintext(
	ctx *context.Context,
	function string,
	plaintext []byte,
) ([]byte, error) {
	hasher := makeHasher(ctx, function)
	hasher.Update(plaintext)
	abstracted := hasher.Digest("binary")
	digest, ok := abstracted.([]byte)
	if !ok {
		err := errors.New("could not hash data: invalid return value")
		return nil, err
	}
	return digest, nil
}

func decodeSignature(encoded interface{}) ([]byte, error) {
	decoded, err := decodeBinaryDetect(encoded)
	if err != nil {
		err = errors.Wrap(err, "could not decode signature")
		return nil, err
	}
	return decoded, nil
}

func decodePlaintext(encoded interface{}) ([]byte, error) {
	decoded, err := decodeBinaryDetect(encoded)
	if err != nil {
		err = errors.Wrap(err, "could not decode data")
		return nil, err
	}
	return decoded, nil
}

func throw(ctx *context.Context, err error) {
	common.Throw(common.GetRuntime(*ctx), err)
}
