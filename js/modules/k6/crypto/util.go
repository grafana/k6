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

	"github.com/loadimpact/k6/js/common"
	"github.com/pkg/errors"
)

func throw(ctx *context.Context, err error) {
	common.Throw(common.GetRuntime(*ctx), err)
}

func decodeFunction(encoded string) (gocrypto.Hash, error) {
	switch encoded {
	case "md4":
		return gocrypto.MD4, nil
	case "md5":
		return gocrypto.MD5, nil
	case "sha1":
		return gocrypto.SHA1, nil
	case "sha224":
		return gocrypto.SHA224, nil
	case "sha256":
		return gocrypto.SHA256, nil
	case "sha384":
		return gocrypto.SHA384, nil
	case "sha512":
		return gocrypto.SHA512, nil
	case "md5sha1":
		return gocrypto.MD5SHA1, nil
	case "ripemd160":
		return gocrypto.RIPEMD160, nil
	case "sha3_224":
		return gocrypto.SHA3_224, nil
	case "sha3_256":
		return gocrypto.SHA3_256, nil
	case "sha3_384":
		return gocrypto.SHA3_384, nil
	case "sha3_512":
		return gocrypto.SHA3_512, nil
	case "sha512_224":
		return gocrypto.SHA512_224, nil
	case "sha512_256":
		return gocrypto.SHA512_256, nil
	case "blake2s_256":
		return gocrypto.BLAKE2s_256, nil
	case "blake2b_256":
		return gocrypto.BLAKE2b_256, nil
	case "blake2b_384":
		return gocrypto.BLAKE2b_384, nil
	case "blake2b_512":
		return gocrypto.BLAKE2b_512, nil
	default:
		err := errors.New("unsupported hash function: " + encoded)
		return 0, err
	}
}

// Remove cases to enable as functions are implemented
func unsupportedFunction(function string) error {
	switch function {
	case "sha224":
		fallthrough
	case "md5sha1":
		fallthrough
	case "sha3_224":
		fallthrough
	case "sha3_256":
		fallthrough
	case "sha3_384":
		fallthrough
	case "sha3_512":
		fallthrough
	case "blake2s_256":
		fallthrough
	case "blake2b_256":
		fallthrough
	case "blake2b_384":
		fallthrough
	case "blake2b_512":
		err := errors.New("unsupported hash function: " + function)
		return err
	default:
		return nil
	}
}
