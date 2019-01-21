/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package common

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"

	"github.com/dop251/goja"
)

// NewRandSource is copied from goja's source code:
// https://github.com/dop251/goja/blob/master/goja/main.go#L44
// The returned RandSource is NOT safe for concurrent use:
// https://golang.org/pkg/math/rand/#NewSource
func NewRandSource() goja.RandSource {
	var seed int64
	if err := binary.Read(crand.Reader, binary.LittleEndian, &seed); err != nil {
		panic(fmt.Errorf("could not read random bytes: %v", err))
	}
	return rand.New(rand.NewSource(seed)).Float64
}
