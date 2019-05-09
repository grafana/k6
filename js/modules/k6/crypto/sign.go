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
	"crypto/sha256"
	"fmt"

	"github.com/pkg/errors"
)

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
