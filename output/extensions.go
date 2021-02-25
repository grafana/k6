/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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

package output

import (
	"fmt"
	"sync"
)

//nolint:gochecknoglobals
var (
	extensions = make(map[string]func(Params) (Output, error))
	mx         sync.RWMutex
)

// GetExtensions returns all registered extensions.
func GetExtensions() map[string]func(Params) (Output, error) {
	mx.RLock()
	defer mx.RUnlock()
	res := make(map[string]func(Params) (Output, error), len(extensions))
	for k, v := range extensions {
		res[k] = v
	}
	return res
}

// RegisterExtension registers the given output extension constructor. This
// function panics if a module with the same name is already registered.
func RegisterExtension(name string, mod func(Params) (Output, error)) {
	mx.Lock()
	defer mx.Unlock()

	if _, ok := extensions[name]; ok {
		panic(fmt.Sprintf("output extension already registered: %s", name))
	}
	extensions[name] = mod
}
