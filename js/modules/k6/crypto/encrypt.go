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
	ciphertext := prepareDecrypt(ctx, ciphertextEncoded)
	plaintext, err := executeDecrypt(recipient, ciphertext, format, options)
	if err != nil {
		throw(ctx, err)
	}
	return plaintext
}

func prepareDecrypt(
	ctx *context.Context,
	ciphertextEncoded interface{},
) []byte {
	ciphertext, err := decodeCiphertext(ciphertextEncoded)
	if err != nil {
		throw(ctx, err)
	}
	return ciphertext
}

func executeDecrypt(
	recipient x509.PrivateKey,
	ciphertext []byte,
	format string,
	options EncryptionOptions,
) (interface{}, error) {
	var plaintext []byte
	var err error
	switch recipient.Type {
	case "RSA":
		plaintext, err = decryptRSA(recipient.RSA, ciphertext, options)
	default:
		err = errors.New("invalid private key")
	}
	if err != nil {
		return nil, err
	}
	encoded, err := encodeBinary(plaintext, format)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func decryptRSA(
	recipient *rsa.PrivateKey,
	ciphertext []byte,
	options EncryptionOptions,
) ([]byte, error) {
	switch options["type"] {
	case "":
		return decryptPKCS(recipient, ciphertext)
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

func decodeCiphertext(encoded interface{}) ([]byte, error) {
	decoded, err := decodeBinaryDetect(encoded)
	if err != nil {
		err = errors.Wrap(err, "could not decode ciphertext")
		return nil, err
	}
	return decoded, nil
}
