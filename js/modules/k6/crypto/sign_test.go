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
	gocrypto "crypto"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Expected struct {
	SHA256 []byte
}

const message = "They know, get out now!"
var expected = Expected{
	SHA256: bytes(
		"cec66fa2e0ad6286b01c5d975631664f54ad80e0ab46907769823e0c33264e8a",
	),
}

func bytes (encoded string) []byte {
	decoded, _ := hex.DecodeString(encoded)
	return decoded
}

func TestHashMessage(t *testing.T) {
	if testing.Short() {
		return
	}

	t.Run("Unsupported", func(t *testing.T) {
		_, err := hashMessage(0, message)
		assert.EqualError(t, err, "unsupported hash function: 0")
	})

	t.Run("SHA256", func(t *testing.T) {
		digest, err := hashMessage(gocrypto.SHA256, message)
		assert.NoError(t, err)
		assert.Equal(t, expected.SHA256, digest)
	})
}

func TestDecodeSignature(t *testing.T) {
	t.Run("BadFormat", func(t *testing.T) {
		_, err := decodeSignature("bad-signature")
		assert.EqualError(t, err, "unrecognized signature encoding")
	})

	t.Run("Base64", func(t *testing.T) {
		signature, err := decodeSignature("AQIDBA==")
		assert.NoError(t, err)
		assert.Equal(t, bytes("01020304"), signature)
	})

	t.Run("Hex", func(t *testing.T) {
		signature, err := decodeSignature("01020304")
		assert.NoError(t, err)
		assert.Equal(t, bytes("01020304"), signature)
	})
}
