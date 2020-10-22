/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package modules

import (
	"fmt"
	"sync"
)

//nolint:gochecknoglobals
var (
	modules = make(map[string]interface{})
	mx      sync.RWMutex
)

// Get returns the module registered with name.
func Get(name string) interface{} {
	mx.RLock()
	defer mx.RUnlock()
	return modules[name]
}

// Register the given mod as a JavaScript module, available
// for import from JS scripts by name.
// This function panics if a module with the same name is already registered.
func Register(name string, mod interface{}) {
	mx.Lock()
	defer mx.Unlock()

	if _, ok := modules[name]; ok {
		panic(fmt.Sprintf("module already registered: %s", name))
	}
	modules[name] = mod
}
