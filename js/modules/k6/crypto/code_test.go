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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeBinaryKnown(t *testing.T) {
	t.Run("Unsupported", func(t *testing.T) {
		_, err := decodeBinaryKnown([]byte{1, 2, 3}, "nucleaonic")
		assert.EqualError(t, err, "unsupported binary encoding: nucleaonic")
	})

	t.Run("ByteArrayValid", func(t *testing.T) {
		value := []interface{}{
			interface{}(int64(1)),
			interface{}(int64(2)),
			interface{}(int64(3)),
		}
		result, err := decodeBinaryKnown(value, "binary")
		assert.NoError(t, err)
		assert.Equal(t, []byte{1, 2, 3}, result)
	})

	t.Run("ByteArrayInvalid", func(t *testing.T) {
		_, err := decodeBinaryKnown("010203", "binary")
		assert.EqualError(t, err, "not a byte array")
	})

	t.Run("HexValid", func(t *testing.T) {
		result, err := decodeBinaryKnown("010203", "hex")
		assert.NoError(t, err)
		assert.Equal(t, []byte{1, 2, 3}, result)
	})

	t.Run("HexInvalid", func(t *testing.T) {
		_, err := decodeBinaryKnown([]byte{1, 2, 3}, "hex")
		assert.EqualError(t, err, "not a hex string")
	})

	t.Run("Base64Valid", func(t *testing.T) {
		result, err := decodeBinaryKnown("AQID", "base64")
		assert.NoError(t, err)
		assert.Equal(t, []byte{1, 2, 3}, result)
	})

	t.Run("Base64Invalid", func(t *testing.T) {
		_, err := decodeBinaryKnown([]byte{1, 2, 3}, "base64")
		assert.EqualError(t, err, "not a base64 string")
	})

	t.Run("StringValid", func(t *testing.T) {
		result, err := decodeBinaryKnown("msg", "string")
		assert.NoError(t, err)
		assert.Equal(t, []byte{109, 115, 103}, result)
	})

	t.Run("StringInvalid", func(t *testing.T) {
		_, err := decodeBinaryKnown([]byte{1, 2, 3}, "string")
		assert.EqualError(t, err, "not a string")
	})
}

func TestDecodeBinaryDetect(t *testing.T) {
	t.Run("Unrecognized", func(t *testing.T) {
		_, err := decodeBinaryDetect("bad-binary")
		assert.EqualError(t, err, "unrecognized binary encoding")
	})

	t.Run("ByteArray", func(t *testing.T) {
		value := []interface{}{
			interface{}(int64(1)),
			interface{}(int64(2)),
			interface{}(int64(3)),
		}
		result, err := decodeBinaryDetect(value)
		assert.NoError(t, err)
		assert.Equal(t, []byte{1, 2, 3}, result)
	})

	t.Run("Hex", func(t *testing.T) {
		result, err := decodeBinaryDetect("010203")
		assert.NoError(t, err)
		assert.Equal(t, []byte{1, 2, 3}, result)
	})

	t.Run("Base64", func(t *testing.T) {
		result, err := decodeBinaryDetect("AQID")
		assert.NoError(t, err)
		assert.Equal(t, []byte{1, 2, 3}, result)
	})
}

func TestEncodeBinary(t *testing.T) {
	t.Run("Unsupported", func(t *testing.T) {
		_, err := encodeBinary([]byte{1, 2, 3}, "nucleonic")
		assert.EqualError(t, err, "unsupported binary encoding: nucleonic")
	})

	t.Run("Default", func(t *testing.T) {
		result, err := encodeBinary([]byte{1, 2, 3}, "")
		assert.NoError(t, err)
		assert.Equal(t, []byte{1, 2, 3}, result)
	})

	t.Run("ByteArray", func(t *testing.T) {
		result, err := encodeBinary([]byte{1, 2, 3}, "binary")
		assert.NoError(t, err)
		assert.Equal(t, []byte{1, 2, 3}, result)
	})

	t.Run("Hex", func(t *testing.T) {
		result, err := encodeBinary([]byte{1, 2, 3}, "hex")
		assert.NoError(t, err)
		assert.Equal(t, "010203", result)
	})

	t.Run("Base64", func(t *testing.T) {
		result, err := encodeBinary([]byte{1, 2, 3}, "base64")
		assert.NoError(t, err)
		assert.Equal(t, "AQID", result)
	})
}
