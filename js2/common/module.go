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
	"reflect"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
)

type Module struct {
	Context context.Context
	Impl    interface{}
}

func (m *Module) Export(rt *goja.Runtime) goja.Value {
	return m.Proxy(rt, m.Impl)
}

func (m *Module) Proxy(rt *goja.Runtime, v interface{}) goja.Value {
	ctxT := reflect.TypeOf((*context.Context)(nil)).Elem()
	errorT := reflect.TypeOf((*error)(nil)).Elem()

	exports := rt.NewObject()
	mapper := FieldNameMapper{}

	val := reflect.ValueOf(v)
	typ := val.Type()
	for i := 0; i < typ.NumMethod(); i++ {
		i := i
		methT := typ.Method(i)
		name := mapper.MethodName(typ, methT)
		if name == "" {
			continue
		}
		meth := val.Method(i)

		in := make([]reflect.Type, methT.Type.NumIn())
		for i := 0; i < len(in); i++ {
			in[i] = methT.Type.In(i)
		}
		out := make([]reflect.Type, methT.Type.NumOut())
		for i := 0; i < len(out); i++ {
			out[i] = methT.Type.Out(i)
		}

		// Skip over the first input arg; it'll be the bound object.
		in = in[1:]

		// If the first argument is a context.Context, inject the given context.
		// The function will error if called outside of a valid context.
		if len(in) > 0 && in[0].Implements(ctxT) {
			in = in[1:]
			meth = m.injectContext(in, out, methT, meth, rt)
		}

		// If the last return value is an error, turn it into a JS throw.
		if len(out) > 0 && out[len(out)-1] == errorT {
			out = out[:len(out)-1]
			meth = m.injectErrorHandler(in, out, methT, meth, rt)
		}

		_ = exports.Set(name, meth.Interface())
	}

	elem := val
	elemTyp := typ
	if typ.Kind() == reflect.Ptr {
		elem = val.Elem()
		elemTyp = elem.Type()
	}
	for i := 0; i < elemTyp.NumField(); i++ {
		f := elemTyp.Field(i)
		k := mapper.FieldName(elemTyp, f)
		if k == "" {
			continue
		}
		_ = exports.Set(k, elem.Field(i).Interface())
	}

	return rt.ToValue(exports)
}

func (m *Module) injectContext(in, out []reflect.Type, methT reflect.Method, meth reflect.Value, rt *goja.Runtime) reflect.Value {
	return reflect.MakeFunc(
		reflect.FuncOf(in, out, methT.Type.IsVariadic()),
		func(args []reflect.Value) []reflect.Value {
			if m.Context == nil {
				panic(rt.NewGoError(errors.Errorf("%s needs a valid VU context", methT.Name)))
			}

			select {
			case <-m.Context.Done():
				panic(rt.NewGoError(errors.Errorf("test has ended")))
			default:
			}

			ctx := reflect.ValueOf(m.Context)
			return meth.Call(append([]reflect.Value{ctx}, args...))
		},
	)
}

func (m *Module) injectErrorHandler(in, out []reflect.Type, methT reflect.Method, meth reflect.Value, rt *goja.Runtime) reflect.Value {
	return reflect.MakeFunc(
		reflect.FuncOf(in, out, methT.Type.IsVariadic()),
		func(args []reflect.Value) []reflect.Value {
			ret := meth.Call(args)
			err := ret[len(ret)-1]
			if !err.IsNil() {
				panic(rt.NewGoError(err.Interface().(error)))
			}
			return ret[:len(ret)-1]
		},
	)
}
