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
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules/k6/http"
)

const extPrefix string = "k6/x/"

//nolint:gochecknoglobals
var (
	modules = make(map[string]interface{})
	mx      sync.RWMutex
)

// Register the given mod as an external JavaScript module that can be imported
// by name. The name must be unique across all registered modules and must be
// prefixed with "k6/x/", otherwise this function will panic.
func Register(name string, mod interface{}) {
	if !strings.HasPrefix(name, extPrefix) {
		panic(fmt.Errorf("external module names must be prefixed with '%s', tried to register: %s", extPrefix, name))
	}

	mx.Lock()
	defer mx.Unlock()

	if _, ok := modules[name]; ok {
		panic(fmt.Sprintf("module already registered: %s", name))
	}
	modules[name] = mod
}

// HasModuleInstancePerVU should be implemented by all native Golang modules that
// would require per-VU state. k6 will call their NewModuleInstancePerVU() methods
// every time a VU imports the module and use its result as the returned object.
type HasModuleInstancePerVU interface {
	NewModuleInstancePerVU() interface{}
}

// IsModuleV2 ... TODO better name
type IsModuleV2 interface {
	NewModuleInstance(InstanceCore) Instance
}

// checks that modules implement HasModuleInstancePerVU
// this is done here as otherwise there will be a loop if the module imports this package
var _ HasModuleInstancePerVU = http.New()

// GetJSModules returns a map of all registered js modules
func GetJSModules() map[string]interface{} {
	mx.Lock()
	defer mx.Unlock()
	result := make(map[string]interface{}, len(modules))

	for name, module := range modules {
		result[name] = module
	}

	return result
}

// Instance is what a module needs to return
type Instance interface {
	InstanceCore
	GetExports() Exports
}

func getInterfaceMethods() []string {
	var t Instance
	T := reflect.TypeOf(&t).Elem()
	result := make([]string, T.NumMethod())

	for i := range result {
		result[i] = T.Method(i).Name
	}

	return result
}

// InstanceCore is something that will be provided to modules and they need to embed it in ModuleInstance
type InstanceCore interface {
	// we can add other methods here
	// sealing field will help probably with pointing users that they just need to embed this in the
	GetContext() context.Context
}

// Exports is representation of ESM exports of a module
type Exports struct {
	// Default is what will be the `default` export of a module
	Default interface{}
	// Named is the named exports of a module
	Named map[string]interface{}
}

// GenerateExports generates an Exports from a module akin to how common.Bind does now.
// it also skips anything that is expected will not want to be exported such as methods and fields coming from
// interfaces defined in this package.
func GenerateExports(v interface{}) Exports {
	exports := make(map[string]interface{})
	val := reflect.ValueOf(v)
	typ := val.Type()
	badNames := getInterfaceMethods()
outer:
	for i := 0; i < typ.NumMethod(); i++ {
		meth := typ.Method(i)
		for _, badname := range badNames {
			if meth.Name == badname {
				continue outer
			}
		}
		name := common.MethodName(typ, meth)

		fn := val.Method(i)
		exports[name] = fn.Interface()
	}

	// If v is a pointer, we need to indirect it to access its fields.
	if typ.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = val.Type()
	}
	var mic InstanceCore // TODO move this out
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type == reflect.TypeOf(&mic).Elem() {
			continue
		}
		name := common.FieldName(typ, field)
		if name != "" {
			exports[name] = val.Field(i).Interface()
		}
	}
	return Exports{Default: exports, Named: exports}
}
