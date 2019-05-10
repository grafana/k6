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

func TestDecodeBinaryDetect(t *testing.T) {
	t.Run("BadFormat", func(t *testing.T) {
		_, err := decodeBinaryDetect("bad-signature")
		assert.EqualError(t, err, "unrecognized binary encoding")
	})

	t.Run("Base64", func(t *testing.T) {
		signature, err := decodeBinaryDetect("AQIDBA==")
		assert.NoError(t, err)
		assert.Equal(t, bytes("01020304"), signature)
	})

	t.Run("Hex", func(t *testing.T) {
		signature, err := decodeBinaryDetect("01020304")
		assert.NoError(t, err)
		assert.Equal(t, bytes("01020304"), signature)
	})
}
