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
	"crypto/rand"
	"crypto/rsa"
	"hash"

	"github.com/loadimpact/k6/js/modules/k6/crypto/x509"
	"github.com/pkg/errors"
)

// EncryptionOptions configures an encrypt or decrypt operation
type EncryptionOptions map[string]string

// Decrypt decrypts a message
func (*Crypto) Decrypt(
	ctx *context.Context,
	recipient x509.PrivateKey,
	ciphertextEncoded interface{},
	format string,
	options EncryptionOptions,
) interface{} {
	ciphertext, function, label :=
		prepareDecrypt(ctx, &recipient, ciphertextEncoded, options)
	plaintext, err :=
		executeDecrypt(&recipient, ciphertext, function, label, options)
	if err != nil {
		throw(ctx, err)
	}
	encoded, err := encodeBinary(plaintext, format)
	if err != nil {
		throw(ctx, err)
	}
	return encoded
}

// Decrypt decrypts a message and interprets it as a UTF-8 string
func (*Crypto) DecryptString(
	ctx *context.Context,
	recipient x509.PrivateKey,
	ciphertextEncoded interface{},
	options EncryptionOptions,
) string {
	ciphertext, function, label :=
		prepareDecrypt(ctx, &recipient, ciphertextEncoded, options)
	plaintext, err :=
		executeDecrypt(&recipient, ciphertext, function, label, options)
	if err != nil {
		throw(ctx, err)
	}
	message, err := decodeString(plaintext)
	if err != nil {
		throw(ctx, err)
	}
	return message
}

// Encrypt encrypts a message for a recipient
func (*Crypto) Encrypt(
	ctx *context.Context,
	recipient x509.PublicKey,
	plaintextEncoded interface{},
	format string,
	options EncryptionOptions,
) interface{} {
	plaintext, function, label :=
		prepareEncrypt(ctx, &recipient, plaintextEncoded, options)
	ciphertext, err :=
		executeEncrypt(&recipient, plaintext, function, label, options)
	if err != nil {
		throw(ctx, err)
	}
	encoded, err := encodeBinary(ciphertext, format)
	if err != nil {
		throw(ctx, err)
	}
	return encoded
}

// Encrypt encrypts a string message for a recipient
func (*Crypto) EncryptString(
	ctx *context.Context,
	recipient x509.PublicKey,
	message string,
	format string,
	options EncryptionOptions,
) interface{} {
	plaintext := []byte(message)
	function, label := prepareEncryptString(ctx, &recipient, options)
	ciphertext, err :=
		executeEncrypt(&recipient, plaintext, function, label, options)
	if err != nil {
		throw(ctx, err)
	}
	encoded, err := encodeBinary(ciphertext, format)
	if err != nil {
		throw(ctx, err)
	}
	return encoded
}

func prepareDecrypt(
	ctx *context.Context,
	recipient *x509.PrivateKey,
	ciphertextEncoded interface{},
	options EncryptionOptions,
) ([]byte, *hash.Hash, []byte) {
	err := validatePrivateKey(recipient)
	if err != nil {
		throw(ctx, err)
	}
	ciphertext, err := decodeCiphertext(ciphertextEncoded)
	if err != nil {
		throw(ctx, err)
	}
	function, err := makeEncryptionFunction(ctx, options["hash"])
	if err != nil {
		throw(ctx, err)
	}
	label := decodeLabel(options["label"])
	return ciphertext, function, label
}

func executeDecrypt(
	recipient *x509.PrivateKey,
	ciphertext []byte,
	function *hash.Hash,
	label []byte,
	options EncryptionOptions,
) ([]byte, error) {
	var plaintext []byte
	var err error
	switch recipient.Algorithm {
	case "RSA":
		key := recipient.Key.(*rsa.PrivateKey)
		plaintext, err = decryptRSA(key, ciphertext, function, label, options)
	default:
		err = errors.New("invalid private key")
	}
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func decryptRSA(
	recipient *rsa.PrivateKey,
	ciphertext []byte,
	function *hash.Hash,
	label []byte,
	options EncryptionOptions,
) ([]byte, error) {
	switch options["type"] {
	case "":
		return decryptPKCS(recipient, ciphertext)
	case "oaep":
		return decryptOAEP(recipient, ciphertext, function, label)
	default:
		err := errors.New("unsupported type: " + options["type"])
		return nil, err
	}
}

func decryptPKCS(
	recipient *rsa.PrivateKey,
	ciphertext []byte,
) ([]byte, error) {
	plaintext, err := rsa.DecryptPKCS1v15(rand.Reader, recipient, ciphertext)
	if err != nil {
		err = errors.Wrap(err, "failed to decrypt")
		return nil, err
	}
	return plaintext, nil
}

func decryptOAEP(
	recipient *rsa.PrivateKey,
	ciphertext []byte,
	function *hash.Hash,
	label []byte,
) ([]byte, error) {
	plaintext, err := rsa.DecryptOAEP(
		*function,
		rand.Reader,
		recipient,
		ciphertext,
		label,
	)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func prepareEncrypt(
	ctx *context.Context,
	recipient *x509.PublicKey,
	plaintextEncoded interface{},
	options EncryptionOptions,
) ([]byte, *hash.Hash, []byte) {
	err := validatePublicKey(recipient)
	if err != nil {
		throw(ctx, err)
	}
	plaintext, err := decodePlaintext(plaintextEncoded)
	if err != nil {
		throw(ctx, err)
	}
	function, err := makeEncryptionFunction(ctx, options["hash"])
	if err != nil {
		throw(ctx, err)
	}
	label := decodeLabel(options["label"])
	return plaintext, function, label
}

func prepareEncryptString(
	ctx *context.Context,
	recipient *x509.PublicKey,
	options EncryptionOptions,
) (*hash.Hash, []byte) {
	err := validatePublicKey(recipient)
	if err != nil {
		throw(ctx, err)
	}
	function, err := makeEncryptionFunction(ctx, options["hash"])
	if err != nil {
		throw(ctx, err)
	}
	label := decodeLabel(options["label"])
	return function, label
}

func executeEncrypt(
	recipient *x509.PublicKey,
	plaintext []byte,
	function *hash.Hash,
	label []byte,
	options EncryptionOptions,
) ([]byte, error) {
	var ciphertext []byte
	var err error
	switch recipient.Algorithm {
	case "RSA":
		key := recipient.Key.(*rsa.PublicKey)
		ciphertext, err = encryptRSA(key, plaintext, function, label, options)
	default:
		err = errors.New("invalid public key")
	}
	if err != nil {
		return nil, err
	}
	return ciphertext, nil
}

func encryptRSA(
	recipient *rsa.PublicKey,
	plaintext []byte,
	function *hash.Hash,
	label []byte,
	options EncryptionOptions,
) ([]byte, error) {
	switch options["type"] {
	case "":
		return encryptPKCS(recipient, plaintext)
	case "oaep":
		return encryptOAEP(recipient, plaintext, function, label)
	default:
		err := errors.New("unsupported type: " + options["type"])
		return nil, err
	}
}

func encryptPKCS(
	recipient *rsa.PublicKey,
	plaintext []byte,
) ([]byte, error) {
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, recipient, plaintext)
	if err != nil {
		err = errors.Wrap(err, "failed to encrypt")
		return nil, err
	}
	return ciphertext, err
}

func encryptOAEP(
	recipient *rsa.PublicKey,
	plaintext []byte,
	function *hash.Hash,
	label []byte,
) ([]byte, error) {
	ciphertext, err := rsa.EncryptOAEP(
		*function,
		rand.Reader,
		recipient,
		plaintext,
		label,
	)
	if err != nil {
		return nil, err
	}
	return ciphertext, nil
}

func decodeCiphertext(encoded interface{}) ([]byte, error) {
	decoded, err := decodeBinaryDetect(encoded)
	if err != nil {
		err = errors.Wrap(err, "could not decode ciphertext")
		return nil, err
	}
	return decoded, nil
}

func makeEncryptionFunction(
	ctx *context.Context,
	encoded string,
) (*hash.Hash, error) {
	if encoded == "" {
		encoded = "sha256"
	}
	err := unsupportedFunction(encoded)
	if err != nil {
		return nil, err
	}
	return makeFunction(ctx, encoded), nil
}

func decodeLabel(encoded string) []byte {
	return []byte(encoded)
}
