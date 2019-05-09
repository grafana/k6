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
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/modules/k6/crypto/x509"
	"github.com/pkg/errors"
)

// SigningOptions communicates options for a signing operation
type SigningOptions struct{
	Type string
}

// Verify checks for a valid message signature
func (Crypto) Verify(
	ctx context.Context,
	signer x509.PublicKey,
	functionEncoded string,
	message string,
	signatureEncoded string,
	options SigningOptions,
) bool {
	function, digest, signature :=
		prepareVerify(ctx, functionEncoded, message, signatureEncoded)
	return executeVerify(ctx, &signer, function, digest, signature, &options)
}

func prepareVerify(
	ctx context.Context,
	functionEncoded string,
	message string,
	signatureEncoded string,
) (gocrypto.Hash, []byte, []byte) {
	function, err := decodeFunction(functionEncoded)
	if err != nil {
		throw(ctx, err)
	}
	digest, err := hashMessage(function, message)
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
	ctx context.Context,
	signer *x509.PublicKey,
	function gocrypto.Hash,
	digest []byte,
	signature []byte,
	options *SigningOptions,
) bool {
	switch signer.Type {
	case "RSA":
		return verifyRSA(ctx, signer.RSA, function, digest, signature, options)
	default:
		err := errors.New("unsupported cryptosystem")
		throw(ctx, err)
		return false
	}
}

func verifyRSA(
	ctx context.Context,
	signer *rsa.PublicKey,
	function gocrypto.Hash,
	digest []byte,
	signature []byte,
	options *SigningOptions,
) bool {
	switch options.Type {
	case "":
		return verifyPKCS(signer, function, digest, signature)
	default:
		err := errors.New("unsupported type: " + options.Type)
		throw(ctx, err)
		return false
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

func decodeFunction(encoded string) (gocrypto.Hash, error) {
	switch encoded {
	case "MD4":
		return gocrypto.MD4, nil
	case "MD5":
		return gocrypto.MD5, nil
	case "SHA1":
		return gocrypto.SHA1, nil
	case "SHA224":
		return gocrypto.SHA224, nil
	case "SHA256":
		return gocrypto.SHA256, nil
	case "SHA512":
		return gocrypto.SHA512, nil
	case "MD5SHA1":
		return gocrypto.MD5SHA1, nil
	case "RIPEMD160":
		return gocrypto.RIPEMD160, nil
	case "SHA3_224":
		return gocrypto.SHA3_224, nil
	case "SHA3_256":
		return gocrypto.SHA3_256, nil
	case "SHA3_384":
		return gocrypto.SHA3_384, nil
	case "SHA3_512":
		return gocrypto.SHA3_512, nil
	case "SHA512_224":
		return gocrypto.SHA512_224, nil
	case "SHA512_256":
		return gocrypto.SHA512_256, nil
	case "BLAKE2s_256":
		return gocrypto.BLAKE2s_256, nil
	case "BLAKE2b_256":
		return gocrypto.BLAKE2b_256, nil
	case "BLAKE2b_384":
		return gocrypto.BLAKE2b_384, nil
	case "BLAKE2b_512":
		return gocrypto.BLAKE2b_512, nil
	default:
		err := errors.New("unsupported hash function: " + encoded)
		return 0, err
	}
}

func hashMessage(function gocrypto.Hash, message string) ([]byte, error) {
	bytes := []byte(message)
	switch function {
	case gocrypto.SHA256:
		result := sha256.Sum256(bytes)
		return result[:], nil
	default:
		msg := fmt.Sprintf("unsupported hash function: %d", function)
		err := errors.New(msg)
		return nil, err
	}
}

func decodeSignature(encoded string) ([]byte, error) {
	decoded, err := hex.DecodeString(encoded)
	if err == nil {
		return decoded, nil
	}
	decoded, err = base64.StdEncoding.DecodeString(encoded)
	if err == nil {
		return decoded, nil
	}
	err = errors.New("unrecognized signature encoding")
	return nil, err
}

func throw(ctx context.Context, err error) {
	common.Throw(common.GetRuntime(ctx), err)
}
