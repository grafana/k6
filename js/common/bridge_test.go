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
	"strconv"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
)

type bridgeTestFieldsType struct {
	Exported       string
	ExportedTag    string `js:"renamed"`
	ExportedHidden string `js:"-"`
	unexported     string
	unexportedTag  string `js:"unexported"`
}

type bridgeTestMethodsType struct{}

func (bridgeTestMethodsType) ExportedFn() {}

//lint:ignore U1000 needed for the actual test to check that it won't be seen
func (bridgeTestMethodsType) unexportedFn() {}

func (*bridgeTestMethodsType) ExportedPtrFn() {}

//lint:ignore U1000 needed for the actual test to check that it won't be seen
func (*bridgeTestMethodsType) unexportedPtrFn() {}

type bridgeTestOddFieldsType struct {
	TwoWords string
	URL      string
}

type bridgeTestErrorType struct{}

func (bridgeTestErrorType) Error() error { return errors.New("error") }

type bridgeTestJSValueType struct{}

func (bridgeTestJSValueType) Func(arg goja.Value) goja.Value { return arg }

type bridgeTestJSValueErrorType struct{}

func (bridgeTestJSValueErrorType) Func(arg goja.Value) (goja.Value, error) {
	if goja.IsUndefined(arg) {
		return goja.Undefined(), errors.New("missing argument")
	}
	return arg, nil
}

type bridgeTestJSValueContextType struct{}

func (bridgeTestJSValueContextType) Func(ctx context.Context, arg goja.Value) goja.Value {
	return arg
}

type bridgeTestJSValueContextErrorType struct{}

func (bridgeTestJSValueContextErrorType) Func(ctx context.Context, arg goja.Value) (goja.Value, error) {
	if goja.IsUndefined(arg) {
		return goja.Undefined(), errors.New("missing argument")
	}
	return arg, nil
}

type bridgeTestNativeFunctionType struct{}

func (bridgeTestNativeFunctionType) Func(call goja.FunctionCall) goja.Value {
	return call.Argument(0)
}

type bridgeTestNativeFunctionErrorType struct{}

func (bridgeTestNativeFunctionErrorType) Func(call goja.FunctionCall) (goja.Value, error) {
	arg := call.Argument(0)
	if goja.IsUndefined(arg) {
		return goja.Undefined(), errors.New("missing argument")
	}
	return arg, nil
}

type bridgeTestNativeFunctionContextType struct{}

func (bridgeTestNativeFunctionContextType) Func(ctx context.Context, call goja.FunctionCall) goja.Value {
	return call.Argument(0)
}

type bridgeTestNativeFunctionContextErrorType struct{}

func (bridgeTestNativeFunctionContextErrorType) Func(ctx context.Context, call goja.FunctionCall) (goja.Value, error) {
	arg := call.Argument(0)
	if goja.IsUndefined(arg) {
		return goja.Undefined(), errors.New("missing argument")
	}
	return arg, nil
}

type bridgeTestAddType struct{}

func (bridgeTestAddType) Add(a, b int) int { return a + b }

type bridgeTestAddWithErrorType struct{}

func (bridgeTestAddWithErrorType) AddWithError(a, b int) (int, error) {
	res := a + b
	if res < 0 {
		return 0, errors.New("answer is negative")
	}
	return res, nil
}

type bridgeTestContextType struct{}

func (bridgeTestContextType) Context(ctx context.Context) {}

type bridgeTestContextAddType struct{}

func (bridgeTestContextAddType) ContextAdd(ctx context.Context, a, b int) int {
	return a + b
}

type bridgeTestContextAddWithErrorType struct{}

func (bridgeTestContextAddWithErrorType) ContextAddWithError(ctx context.Context, a, b int) (int, error) {
	res := a + b
	if res < 0 {
		return 0, errors.New("answer is negative")
	}
	return res, nil
}

type bridgeTestContextInjectType struct {
	ctx context.Context
}

func (t *bridgeTestContextInjectType) ContextInject(ctx context.Context) { t.ctx = ctx }

type bridgeTestContextInjectPtrType struct {
	ctxPtr *context.Context
}

func (t *bridgeTestContextInjectPtrType) ContextInjectPtr(ctxPtr *context.Context) { t.ctxPtr = ctxPtr }

type bridgeTestSumType struct{}

func (bridgeTestSumType) Sum(nums ...int) int {
	sum := 0
	for v := range nums {
		sum += v
	}
	return sum
}

type bridgeTestSumWithContextType struct{}

func (bridgeTestSumWithContextType) SumWithContext(ctx context.Context, nums ...int) int {
	sum := 0
	for v := range nums {
		sum += v
	}
	return sum
}

type bridgeTestSumWithErrorType struct{}

func (bridgeTestSumWithErrorType) SumWithError(nums ...int) (int, error) {
	sum := 0
	for v := range nums {
		sum += v
	}
	if sum < 0 {
		return 0, errors.New("answer is negative")
	}
	return sum, nil
}

type bridgeTestSumWithContextAndErrorType struct{}

func (m bridgeTestSumWithContextAndErrorType) SumWithContextAndError(ctx context.Context, nums ...int) (int, error) {
	sum := 0
	for v := range nums {
		sum += v
	}
	if sum < 0 {
		return 0, errors.New("answer is negative")
	}
	return sum, nil
}

type bridgeTestCounterType struct {
	Counter int
}

func (m *bridgeTestCounterType) Count() int {
	m.Counter++
	return m.Counter
}

type bridgeTestConstructorType struct{}

type bridgeTestConstructorSpawnedType struct{}

func (bridgeTestConstructorType) XConstructor() bridgeTestConstructorSpawnedType {
	return bridgeTestConstructorSpawnedType{}
}

func TestFieldNameMapper(t *testing.T) {
	testdata := []struct {
		Typ     reflect.Type
		Fields  map[string]string
		Methods map[string]string
	}{
		{reflect.TypeOf(bridgeTestFieldsType{}), map[string]string{
			"Exported":       "exported",
			"ExportedTag":    "renamed",
			"ExportedHidden": "",
			"unexported":     "",
			"unexportedTag":  "",
		}, nil},
		{reflect.TypeOf(bridgeTestMethodsType{}), nil, map[string]string{
			"ExportedFn":   "exportedFn",
			"unexportedFn": "",
		}},
		{reflect.TypeOf(bridgeTestOddFieldsType{}), map[string]string{
			"TwoWords": "two_words",
			"URL":      "url",
		}, nil},
		{reflect.TypeOf(bridgeTestConstructorType{}), nil, map[string]string{
			"XConstructor": "Constructor",
		}},
	}
	for _, data := range testdata {
		for field, name := range data.Fields {
			t.Run(field, func(t *testing.T) {
				f, ok := data.Typ.FieldByName(field)
				if assert.True(t, ok, "no such field") {
					assert.Equal(t, name, (FieldNameMapper{}).FieldName(data.Typ, f))
				}
			})
		}
		for meth, name := range data.Methods {
			t.Run(meth, func(t *testing.T) {
				m, ok := data.Typ.MethodByName(meth)
				if name != "" {
					if assert.True(t, ok, "no such method") {
						assert.Equal(t, name, (FieldNameMapper{}).MethodName(data.Typ, m))
					}
				} else {
					assert.False(t, ok, "exported by accident")
				}
			})
		}
	}
}

func TestBindToGlobal(t *testing.T) {
	rt := goja.New()
	unbind := BindToGlobal(rt, map[string]interface{}{"a": 1})
	assert.Equal(t, int64(1), rt.Get("a").Export())
	unbind()
	assert.Nil(t, rt.Get("a").Export())
}

func TestBind(t *testing.T) {
	ctxPtr := new(context.Context)
	testdata := []struct {
		Name string
		V    interface{}
		Fn   func(t *testing.T, obj interface{}, rt *goja.Runtime)
	}{
		{"Fields", bridgeTestFieldsType{"a", "b", "c", "d", "e"}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			t.Run("Exported", func(t *testing.T) {
				v, err := RunString(rt, `obj.exported`)
				if assert.NoError(t, err) {
					assert.Equal(t, "a", v.Export())
				}
			})
			t.Run("ExportedTag", func(t *testing.T) {
				v, err := RunString(rt, `obj.renamed`)
				if assert.NoError(t, err) {
					assert.Equal(t, "b", v.Export())
				}
			})
			t.Run("unexported", func(t *testing.T) {
				v, err := RunString(rt, `obj.unexported`)
				if assert.NoError(t, err) {
					assert.Equal(t, nil, v.Export())
				}
			})
			t.Run("unexportedTag", func(t *testing.T) {
				v, err := RunString(rt, `obj.unexportedTag`)
				if assert.NoError(t, err) {
					assert.Equal(t, nil, v.Export())
				}
			})
		}},
		{"Methods", bridgeTestMethodsType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			t.Run("unexportedFn", func(t *testing.T) {
				_, err := RunString(rt, `obj.unexportedFn()`)
				assert.EqualError(t, err, "TypeError: Object has no member 'unexportedFn' at <eval>:1:17(3)")
			})
			t.Run("ExportedFn", func(t *testing.T) {
				_, err := RunString(rt, `obj.exportedFn()`)
				assert.NoError(t, err)
			})
			t.Run("unexportedPtrFn", func(t *testing.T) {
				_, err := RunString(rt, `obj.unexportedPtrFn()`)
				assert.EqualError(t, err, "TypeError: Object has no member 'unexportedPtrFn' at <eval>:1:20(3)")
			})
			t.Run("ExportedPtrFn", func(t *testing.T) {
				_, err := RunString(rt, `obj.exportedPtrFn()`)
				switch obj.(type) {
				case *bridgeTestMethodsType:
					assert.NoError(t, err)
				case bridgeTestMethodsType:
					assert.EqualError(t, err, "TypeError: Object has no member 'exportedPtrFn' at <eval>:1:18(3)")
				default:
					assert.Fail(t, "INVALID TYPE")
				}
			})
		}},
		{"Error", bridgeTestErrorType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.error()`)
			assert.Contains(t, err.Error(), "error")
		}},
		{"JSValue", bridgeTestJSValueType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			v, err := RunString(rt, `obj.func(1234)`)
			if assert.NoError(t, err) {
				assert.Equal(t, int64(1234), v.Export())
			}
		}},
		{"JSValueError", bridgeTestJSValueErrorType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.func()`)
			assert.Contains(t, err.Error(), "missing argument")

			t.Run("Valid", func(t *testing.T) {
				v, err := RunString(rt, `obj.func(1234)`)
				if assert.NoError(t, err) {
					assert.Equal(t, int64(1234), v.Export())
				}
			})
		}},
		{"JSValueContext", bridgeTestJSValueContextType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.func()`)
			assert.Contains(t, err.Error(), "func() can only be called from within default()")

			t.Run("Context", func(t *testing.T) {
				*ctxPtr = context.Background()
				defer func() { *ctxPtr = nil }()

				v, err := RunString(rt, `obj.func(1234)`)
				if assert.NoError(t, err) {
					assert.Equal(t, int64(1234), v.Export())
				}
			})
		}},
		{"JSValueContextError", bridgeTestJSValueContextErrorType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.func()`)
			assert.Contains(t, err.Error(), "func() can only be called from within default()")

			t.Run("Context", func(t *testing.T) {
				*ctxPtr = context.Background()
				defer func() { *ctxPtr = nil }()

				_, err := RunString(rt, `obj.func()`)
				assert.Contains(t, err.Error(), "missing argument")

				t.Run("Valid", func(t *testing.T) {
					v, err := RunString(rt, `obj.func(1234)`)
					if assert.NoError(t, err) {
						assert.Equal(t, int64(1234), v.Export())
					}
				})
			})
		}},
		{"NativeFunction", bridgeTestNativeFunctionType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			v, err := RunString(rt, `obj.func(1234)`)
			if assert.NoError(t, err) {
				assert.Equal(t, int64(1234), v.Export())
			}
		}},
		{"NativeFunctionError", bridgeTestNativeFunctionErrorType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.func()`)
			assert.Contains(t, err.Error(), "missing argument")

			t.Run("Valid", func(t *testing.T) {
				v, err := RunString(rt, `obj.func(1234)`)
				if assert.NoError(t, err) {
					assert.Equal(t, int64(1234), v.Export())
				}
			})
		}},
		{"NativeFunctionContext", bridgeTestNativeFunctionContextType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.func()`)
			assert.Contains(t, err.Error(), "func() can only be called from within default()")

			t.Run("Context", func(t *testing.T) {
				*ctxPtr = context.Background()
				defer func() { *ctxPtr = nil }()

				v, err := RunString(rt, `obj.func(1234)`)
				if assert.NoError(t, err) {
					assert.Equal(t, int64(1234), v.Export())
				}
			})
		}},
		{"NativeFunctionContextError", bridgeTestNativeFunctionContextErrorType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.func()`)
			assert.Contains(t, err.Error(), "func() can only be called from within default()")

			t.Run("Context", func(t *testing.T) {
				*ctxPtr = context.Background()
				defer func() { *ctxPtr = nil }()

				_, err := RunString(rt, `obj.func()`)
				assert.Contains(t, err.Error(), "missing argument")

				t.Run("Valid", func(t *testing.T) {
					v, err := RunString(rt, `obj.func(1234)`)
					if assert.NoError(t, err) {
						assert.Equal(t, int64(1234), v.Export())
					}
				})
			})
		}},
		{"Add", bridgeTestAddType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			v, err := RunString(rt, `obj.add(1, 2)`)
			if assert.NoError(t, err) {
				assert.Equal(t, int64(3), v.Export())
			}
		}},
		{"AddWithError", bridgeTestAddWithErrorType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			v, err := RunString(rt, `obj.addWithError(1, 2)`)
			if assert.NoError(t, err) {
				assert.Equal(t, int64(3), v.Export())
			}

			t.Run("Negative", func(t *testing.T) {
				_, err := RunString(rt, `obj.addWithError(0, -1)`)
				assert.Contains(t, err.Error(), "answer is negative")
			})
		}},
		{"AddWithError", bridgeTestAddWithErrorType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			v, err := RunString(rt, `obj.addWithError(1, 2)`)
			if assert.NoError(t, err) {
				assert.Equal(t, int64(3), v.Export())
			}

			t.Run("Negative", func(t *testing.T) {
				_, err := RunString(rt, `obj.addWithError(0, -1)`)
				assert.Contains(t, err.Error(), "answer is negative")
			})
		}},
		{"Context", bridgeTestContextType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.context()`)
			assert.Contains(t, err.Error(), "context() can only be called from within default()")

			t.Run("Valid", func(t *testing.T) {
				*ctxPtr = context.Background()
				defer func() { *ctxPtr = nil }()

				_, err := RunString(rt, `obj.context()`)
				assert.NoError(t, err)
			})
		}},
		{"ContextAdd", bridgeTestContextAddType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.contextAdd(1, 2)`)
			assert.Contains(t, err.Error(), "contextAdd() can only be called from within default()")

			t.Run("Valid", func(t *testing.T) {
				*ctxPtr = context.Background()
				defer func() { *ctxPtr = nil }()

				v, err := RunString(rt, `obj.contextAdd(1, 2)`)
				if assert.NoError(t, err) {
					assert.Equal(t, int64(3), v.Export())
				}
			})
		}},
		{"ContextAddWithError", bridgeTestContextAddWithErrorType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.contextAddWithError(1, 2)`)
			assert.Contains(t, err.Error(), "contextAddWithError() can only be called from within default()")

			t.Run("Valid", func(t *testing.T) {
				*ctxPtr = context.Background()
				defer func() { *ctxPtr = nil }()

				v, err := RunString(rt, `obj.contextAddWithError(1, 2)`)
				if assert.NoError(t, err) {
					assert.Equal(t, int64(3), v.Export())
				}

				t.Run("Negative", func(t *testing.T) {
					_, err := RunString(rt, `obj.contextAddWithError(0, -1)`)
					assert.Contains(t, err.Error(), "answer is negative")
				})
			})
		}},
		{"ContextInject", bridgeTestContextInjectType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.contextInject()`)
			switch impl := obj.(type) {
			case bridgeTestContextInjectType:
				assert.EqualError(t, err, "TypeError: Object has no member 'contextInject' at <eval>:1:18(3)")
			case *bridgeTestContextInjectType:
				assert.Contains(t, err.Error(), "contextInject() can only be called from within default()")
				assert.Equal(t, nil, impl.ctx)

				t.Run("Valid", func(t *testing.T) {
					*ctxPtr = context.Background()
					defer func() { *ctxPtr = nil }()

					_, err := RunString(rt, `obj.contextInject()`)
					assert.NoError(t, err)
					assert.Equal(t, *ctxPtr, impl.ctx)
				})
			}
		}},
		{"ContextInjectPtr", bridgeTestContextInjectPtrType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.contextInjectPtr()`)
			switch impl := obj.(type) {
			case bridgeTestContextInjectPtrType:
				assert.EqualError(t, err, "TypeError: Object has no member 'contextInjectPtr' at <eval>:1:21(3)")
			case *bridgeTestContextInjectPtrType:
				assert.NoError(t, err)
				assert.Equal(t, ctxPtr, impl.ctxPtr)
			}
		}},
		{"Count", bridgeTestCounterType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			switch impl := obj.(type) {
			case *bridgeTestCounterType:
				for i := 0; i < 10; i++ {
					t.Run(strconv.Itoa(i), func(t *testing.T) {
						v, err := RunString(rt, `obj.count()`)
						if assert.NoError(t, err) {
							assert.Equal(t, int64(i+1), v.Export())
							assert.Equal(t, i+1, impl.Counter)
						}
					})
				}
			case bridgeTestCounterType:
				_, err := RunString(rt, `obj.count()`)
				assert.EqualError(t, err, "TypeError: Object has no member 'count' at <eval>:1:10(3)")
			default:
				assert.Fail(t, "UNKNOWN TYPE")
			}
		}},
		{"Sum", bridgeTestSumType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			sum := 0
			args := []string{}
			for i := 0; i < 10; i++ {
				args = append(args, strconv.Itoa(i))
				sum += i
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					code := fmt.Sprintf(`obj.sum(%s)`, strings.Join(args, ", "))
					v, err := RunString(rt, code)
					if assert.NoError(t, err) {
						assert.Equal(t, int64(sum), v.Export())
					}
				})
			}
		}},
		{"SumWithContext", bridgeTestSumWithContextType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.sumWithContext(1, 2)`)
			assert.Contains(t, err.Error(), "sumWithContext() can only be called from within default()")

			t.Run("Valid", func(t *testing.T) {
				*ctxPtr = context.Background()
				defer func() { *ctxPtr = nil }()

				sum := 0
				args := []string{}
				for i := 0; i < 10; i++ {
					args = append(args, strconv.Itoa(i))
					sum += i
					t.Run(strconv.Itoa(i), func(t *testing.T) {
						code := fmt.Sprintf(`obj.sumWithContext(%s)`, strings.Join(args, ", "))
						v, err := RunString(rt, code)
						if assert.NoError(t, err) {
							assert.Equal(t, int64(sum), v.Export())
						}
					})
				}
			})
		}},
		{"SumWithError", bridgeTestSumWithErrorType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			sum := 0
			args := []string{}
			for i := 0; i < 10; i++ {
				args = append(args, strconv.Itoa(i))
				sum += i
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					code := fmt.Sprintf(`obj.sumWithError(%s)`, strings.Join(args, ", "))
					v, err := RunString(rt, code)
					if assert.NoError(t, err) {
						assert.Equal(t, int64(sum), v.Export())
					}
				})
			}
		}},
		{"SumWithContextAndError", bridgeTestSumWithContextAndErrorType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			_, err := RunString(rt, `obj.sumWithContextAndError(1, 2)`)
			assert.Contains(t, err.Error(), "sumWithContextAndError() can only be called from within default()")

			t.Run("Valid", func(t *testing.T) {
				*ctxPtr = context.Background()
				defer func() { *ctxPtr = nil }()

				sum := 0
				args := []string{}
				for i := 0; i < 10; i++ {
					args = append(args, strconv.Itoa(i))
					sum += i
					t.Run(strconv.Itoa(i), func(t *testing.T) {
						code := fmt.Sprintf(`obj.sumWithContextAndError(%s)`, strings.Join(args, ", "))
						v, err := RunString(rt, code)
						if assert.NoError(t, err) {
							assert.Equal(t, int64(sum), v.Export())
						}
					})
				}
			})
		}},
		{"Constructor", bridgeTestConstructorType{}, func(t *testing.T, obj interface{}, rt *goja.Runtime) {
			v, err := RunString(rt, `new obj.Constructor()`)
			assert.NoError(t, err)
			assert.IsType(t, bridgeTestConstructorSpawnedType{}, v.Export())
		}},
	}

	vfns := map[string]func(interface{}) interface{}{
		"Value": func(v interface{}) interface{} { return v },
		"Pointer": func(v interface{}) interface{} {
			val := reflect.ValueOf(v)
			ptr := reflect.New(val.Type())
			ptr.Elem().Set(val)
			return ptr.Interface()
		},
	}

	for name, vfn := range vfns {
		t.Run(name, func(t *testing.T) {
			for _, data := range testdata {
				t.Run(data.Name, func(t *testing.T) {
					rt := goja.New()
					rt.SetFieldNameMapper(FieldNameMapper{})
					obj := vfn(data.V)
					rt.Set("obj", Bind(rt, obj, ctxPtr))
					data.Fn(t, obj, rt)
				})
			}
		})
	}
}

func BenchmarkProxy(b *testing.B) {
	types := []struct {
		Name, FnName string
		Value        interface{}
		Fn           func(b *testing.B, fn interface{})
	}{
		{"Fields", "", bridgeTestFieldsType{}, nil},
		{"Methods", "exportedFn", bridgeTestMethodsType{}, func(b *testing.B, fn interface{}) {
			f := fn.(func())
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f()
			}
		}},
		{"Error", "", bridgeTestErrorType{}, nil},
		{"Add", "add", bridgeTestAddType{}, func(b *testing.B, fn interface{}) {
			f := fn.(func(int, int) int)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f(1, 2)
			}
		}},
		{"AddError", "addWithError", bridgeTestAddWithErrorType{}, func(b *testing.B, fn interface{}) {
			b.Skip()
			f := fn.(func(int, int) int)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f(1, 2)
			}
		}},
		{"Context", "context", bridgeTestContextType{}, func(b *testing.B, fn interface{}) {
			b.Skip()
			f := fn.(func())
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f()
			}
		}},
		{"ContextAdd", "contextAdd", bridgeTestContextAddType{}, func(b *testing.B, fn interface{}) {
			b.Skip()
			f := fn.(func(int, int) int)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f(1, 2)
			}
		}},
		{"ContextAddError", "contextAddWithError", bridgeTestContextAddWithErrorType{}, func(b *testing.B, fn interface{}) {
			b.Skip()
			f := fn.(func(int, int) int)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f(1, 2)
			}
		}},
		{"Sum", "sum", bridgeTestSumType{}, func(b *testing.B, fn interface{}) {
			f := fn.(func(...int) int)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f(1, 2, 3)
			}
		}},
		{"SumContext", "sumWithContext", bridgeTestSumWithContextType{}, func(b *testing.B, fn interface{}) {
			b.Skip()
			f := fn.(func(...int) int)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f(1, 2, 3)
			}
		}},
		{"SumError", "sumWithError", bridgeTestSumWithErrorType{}, func(b *testing.B, fn interface{}) {
			b.Skip()
			f := fn.(func(...int) int)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f(1, 2, 3)
			}
		}},
		{"SumContextError", "sumWithContextAndError", bridgeTestSumWithContextAndErrorType{}, func(b *testing.B, fn interface{}) {
			b.Skip()
			f := fn.(func(...int) int)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f(1, 2, 3)
			}
		}},
		{"Constructor", "Constructor", bridgeTestConstructorType{}, func(b *testing.B, fn interface{}) {
			f, _ := goja.AssertFunction(fn.(goja.Value))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = f(goja.Undefined())
			}
		}},
	}
	vfns := []struct {
		Name string
		Fn   func(interface{}) interface{}
	}{
		{"Value", func(v interface{}) interface{} { return v }},
		{"Pointer", func(v interface{}) interface{} {
			val := reflect.ValueOf(v)
			ptr := reflect.New(val.Type())
			ptr.Elem().Set(val)
			return ptr.Interface()
		}},
	}

	for _, vfn := range vfns {
		b.Run(vfn.Name, func(b *testing.B) {
			for _, typ := range types {
				b.Run(typ.Name, func(b *testing.B) {
					v := vfn.Fn(typ.Value)

					b.Run("ToValue", func(b *testing.B) {
						rt := goja.New()
						rt.SetFieldNameMapper(FieldNameMapper{})
						b.ResetTimer()

						for i := 0; i < b.N; i++ {
							rt.ToValue(v)
						}
					})

					b.Run("Bridge", func(b *testing.B) {
						rt := goja.New()
						rt.SetFieldNameMapper(FieldNameMapper{})
						ctx := context.Background()
						b.ResetTimer()

						for i := 0; i < b.N; i++ {
							Bind(rt, v, &ctx)
						}
					})

					if typ.FnName != "" {
						b.Run("Call", func(b *testing.B) {
							rt := goja.New()
							rt.SetFieldNameMapper(FieldNameMapper{})
							ctx := context.Background()
							fn := Bind(rt, v, &ctx)[typ.FnName]
							typ.Fn(b, fn)
						})
					}
				})
			}
		})
	}
}
