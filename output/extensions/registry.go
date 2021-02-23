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

package extensions

import (
	"fmt"
	"sync"

	"github.com/loadimpact/k6/output"
)

//nolint:gochecknoglobals
var (
	modules = make(map[string]func(output.Params) (output.Output, error))
	mx      sync.RWMutex
)

// GetAll returns all registered extensions.
func GetAll() map[string]func(output.Params) (output.Output, error) {
	mx.RLock()
	defer mx.RUnlock()
	res := make(map[string]func(output.Params) (output.Output, error), len(modules))
	for k, v := range modules {
		res[k] = v
	}
	return res
}

// Get returns the output module constructor with the specified name.
func Get(name string) func(output.Params) (output.Output, error) {
	mx.RLock()
	defer mx.RUnlock()
	return modules[name]
}

// Register the given output module constructor. This function panics if a
// module with the same name is already registered.
func Register(name string, mod func(output.Params) (output.Output, error)) {
	mx.Lock()
	defer mx.Unlock()

	if _, ok := modules[name]; ok {
		panic(fmt.Sprintf("output module already registered: %s", name))
	}
	modules[name] = mod
}
