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
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/dop251/goja"
	"github.com/serenize/snaker"
)

var (
	ctxPtrT = reflect.TypeOf((*context.Context)(nil))
	ctxT    = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorT  = reflect.TypeOf((*error)(nil)).Elem()

	constructWrap = MustCompile(
		"__constructor__",
		`(function(impl) { return function() { return impl.apply(this, arguments); } })`,
		true,
	)
)

// Returns the JS name for an exported struct field. The name is snake_cased, with respect for
// certain common initialisms (URL, ID, HTTP, etc).
func FieldName(t reflect.Type, f reflect.StructField) string {
	// PkgPath is non-empty for unexported fields.
	if f.PkgPath != "" {
		return ""
	}

	// Allow a `js:"name"` tag to override the default name.
	if tag := f.Tag.Get("js"); tag != "" {
		// Matching encoding/json, `js:"-"` hides a field.
		if tag == "-" {
			return ""
		}
		return tag
	}

	// Default to lowercasing the first character of the field name.
	return snaker.CamelToSnake(f.Name)
}

// Returns the JS name for an exported method. The first letter of the method's name is
// lowercased, otherwise it is unaltered.
func MethodName(t reflect.Type, m reflect.Method) string {
	// PkgPath is non-empty for unexported methods.
	if m.PkgPath != "" {
		return ""
	}

	// A field with a name beginning with an X is a constructor, and just gets the prefix stripped.
	// Note: They also get some special treatment from Bridge(), see further down.
	if m.Name[0] == 'X' {
		return m.Name[1:]
	}

	// Lowercase the first character of the method name.
	return strings.ToLower(m.Name[0:1]) + m.Name[1:]
}

// FieldNameMapper for goja.Runtime.SetFieldNameMapper()
type FieldNameMapper struct{}

func (FieldNameMapper) FieldName(t reflect.Type, f reflect.StructField) string { return FieldName(t, f) }

func (FieldNameMapper) MethodName(t reflect.Type, m reflect.Method) string { return MethodName(t, m) }

// Binds an object's members to the global scope. Returns a function that un-binds them.
// Note that this will panic if passed something that isn't a struct; please don't do that.
func BindToGlobal(rt *goja.Runtime, data map[string]interface{}) func() {
	keys := make([]string, len(data))
	i := 0
	for k, v := range data {
		rt.Set(k, v)
		keys[i] = k
		i++
	}

	return func() {
		for _, k := range keys {
			rt.Set(k, goja.Undefined())
		}
	}
}

func Bind(rt *goja.Runtime, v interface{}, ctxPtr *context.Context) map[string]interface{} {
	exports := make(map[string]interface{})

	val := reflect.ValueOf(v)
	typ := val.Type()
	for i := 0; i < typ.NumMethod(); i++ {
		meth := typ.Method(i)
		name := MethodName(typ, meth)
		if name == "" {
			continue
		}
		fn := val.Method(i)

		// Figure out if we want to do any wrapping of it.
		fnT := fn.Type()
		numIn := fnT.NumIn()
		numOut := fnT.NumOut()
		hasError := (numOut > 1 && fnT.Out(1) == errorT)
		wantsContext := false
		wantsContextPtr := false
		if numIn > 0 {
			in0 := fnT.In(0)
			switch in0 {
			case ctxT:
				wantsContext = true
			case ctxPtrT:
				wantsContextPtr = true
			}
		}
		if hasError || wantsContext || wantsContextPtr {
			// Varadic functions are called a bit differently.
			varadic := fnT.IsVariadic()

			// Collect input types, but skip the context (if any).
			var in []reflect.Type
			if numIn > 0 {
				inOffset := 0
				if wantsContext || wantsContextPtr {
					inOffset = 1
				}
				in = make([]reflect.Type, numIn-inOffset)
				for i := inOffset; i < numIn; i++ {
					in[i-inOffset] = fnT.In(i)
				}
			}

			// Collect the output type (if any). JS functions can only return a single value, but
			// allow returning an error, which will be thrown as a JS exception.
			var out []reflect.Type
			if numOut != 0 {
				out = []reflect.Type{fnT.Out(0)}
			}

			wrappedFn := fn
			fn = reflect.MakeFunc(
				reflect.FuncOf(in, out, varadic),
				func(args []reflect.Value) []reflect.Value {
					if wantsContext {
						if ctxPtr == nil || *ctxPtr == nil {
							Throw(rt, errors.New(fmt.Sprintf("%s needs a valid VU context", meth.Name)))
						}
						args = append([]reflect.Value{reflect.ValueOf(*ctxPtr)}, args...)
					} else if wantsContextPtr {
						args = append([]reflect.Value{reflect.ValueOf(ctxPtr)}, args...)
					}

					var res []reflect.Value
					if varadic {
						res = wrappedFn.CallSlice(args)
					} else {
						res = wrappedFn.Call(args)
					}

					if hasError {
						if !res[1].IsNil() {
							Throw(rt, res[1].Interface().(error))
						}
						res = res[:1]
					}

					return res
				},
			)
		}

		// X-Prefixed methods are assumed to be constructors; use a closure to wrap them in a
		// pure-JS function to allow them to be `new`d. (This is an awful hack...)
		if meth.Name[0] == 'X' {
			wrapperV, _ := rt.RunProgram(constructWrap)
			wrapper, _ := goja.AssertFunction(wrapperV)
			v, _ := wrapper(goja.Undefined(), rt.ToValue(fn.Interface()))
			exports[name] = v
		} else {
			exports[name] = fn.Interface()
		}
	}

	// If v is a pointer, we need to indirect it to access fields.
	if typ.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = val.Type()
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := FieldName(typ, field)
		if name != "" {
			exports[name] = val.Field(i).Interface()
		}
	}

	return exports
}
