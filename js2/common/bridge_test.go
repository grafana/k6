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

type bridgeTestType struct {
	Exported      string
	ExportedTag   string `js:"renamed"`
	unexported    string
	unexportedTag string `js:"unexported"`

	TwoWords string
	URL      string

	Counter int
}

func (bridgeTestType) ExportedFn()   {}
func (bridgeTestType) unexportedFn() {}

func (*bridgeTestType) ExportedPtrFn()   {}
func (*bridgeTestType) unexportedPtrFn() {}

func (bridgeTestType) Error() error { return errors.New("error") }

func (bridgeTestType) Add(a, b int) int { return a + b }

func (bridgeTestType) AddWithError(a, b int) (int, error) {
	res := a + b
	if res < 0 {
		return 0, errors.New("answer is negative")
	}
	return res, nil
}

func (bridgeTestType) Context(ctx context.Context) {}

func (bridgeTestType) ContextAdd(ctx context.Context, a, b int) int {
	return a + b
}

func (bridgeTestType) ContextAddWithError(ctx context.Context, a, b int) (int, error) {
	res := a + b
	if res < 0 {
		return 0, errors.New("answer is negative")
	}
	return res, nil
}

func (m *bridgeTestType) Count() int {
	m.Counter++
	return m.Counter
}

func (bridgeTestType) Sum(nums ...int) int {
	sum := 0
	for v := range nums {
		sum += v
	}
	return sum
}

func (m bridgeTestType) SumWithContext(ctx context.Context, nums ...int) int {
	return m.Sum(nums...)
}

func (m bridgeTestType) SumWithError(nums ...int) (int, error) {
	sum := m.Sum(nums...)
	if sum < 0 {
		return 0, errors.New("answer is negative")
	}
	return sum, nil
}

func (m bridgeTestType) SumWithContextAndError(ctx context.Context, nums ...int) (int, error) {
	return m.SumWithError(nums...)
}

func TestFieldNameMapper(t *testing.T) {
	typ := reflect.TypeOf(bridgeTestType{})
	t.Run("Fields", func(t *testing.T) {
		names := map[string]string{
			"Exported":      "exported",
			"ExportedTag":   "renamed",
			"unexported":    "",
			"unexportedTag": "",
			"TwoWords":      "two_words",
			"URL":           "url",
		}
		for name, result := range names {
			t.Run(name, func(t *testing.T) {
				f, ok := typ.FieldByName(name)
				if assert.True(t, ok) {
					assert.Equal(t, result, (FieldNameMapper{}).FieldName(typ, f))
				}
			})
		}
	})
	t.Run("Methods", func(t *testing.T) {
		t.Run("ExportedFn", func(t *testing.T) {
			m, ok := typ.MethodByName("ExportedFn")
			if assert.True(t, ok) {
				assert.Equal(t, "exportedFn", (FieldNameMapper{}).MethodName(typ, m))
			}
		})
		t.Run("unexportedFn", func(t *testing.T) {
			_, ok := typ.MethodByName("unexportedFn")
			assert.False(t, ok)
		})
	})
}

func TestBindToGlobal(t *testing.T) {
	testdata := map[string]struct {
		Obj  interface{}
		Keys []string
		Not  []string
	}{
		"Value": {
			bridgeTestType{},
			[]string{"exported", "renamed", "exportedFn"},
			[]string{"exportedPtrFn"},
		},
		"Pointer": {
			&bridgeTestType{},
			[]string{"exported", "renamed", "exportedFn", "exportedPtrFn"},
			[]string{},
		},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			rt := goja.New()
			unbind := BindToGlobal(rt, data.Obj)
			for _, k := range data.Keys {
				t.Run(k, func(t *testing.T) {
					v := rt.Get(k)
					if assert.NotNil(t, v) {
						assert.False(t, goja.IsUndefined(v), "value is undefined")
					}
				})
			}
			for _, k := range data.Not {
				t.Run(k, func(t *testing.T) {
					assert.Nil(t, rt.Get(k), "unexpected member bridged")
				})
			}

			t.Run("Unbind", func(t *testing.T) {
				unbind()
				for _, k := range data.Keys {
					t.Run(k, func(t *testing.T) {
						v := rt.Get(k)
						assert.True(t, goja.IsUndefined(v), "value is not undefined")
					})
				}
			})
		})
	}
}

func TestBind(t *testing.T) {
	template := bridgeTestType{
		Exported:      "a",
		ExportedTag:   "b",
		unexported:    "c",
		unexportedTag: "d",
	}
	testdata := map[string]func() interface{}{
		"Value":   func() interface{} { return template },
		"Pointer": func() interface{} { tmp := template; return &tmp },
	}
	for vtype, vfn := range testdata {
		t.Run(vtype, func(t *testing.T) {
			rt := goja.New()
			v := vfn()
			ctx := new(context.Context)
			rt.Set("obj", Bind(rt, v, ctx))

			t.Run("unexportedFn", func(t *testing.T) {
				_, err := RunString(rt, `obj.unexportedFn()`)
				assert.EqualError(t, err, "TypeError: Object has no member 'unexportedFn'")
			})
			t.Run("ExportedFn", func(t *testing.T) {
				_, err := RunString(rt, `obj.exportedFn()`)
				assert.NoError(t, err)
			})
			t.Run("unexportedPtrFn", func(t *testing.T) {
				_, err := RunString(rt, `obj.unexportedPtrFn()`)
				assert.EqualError(t, err, "TypeError: Object has no member 'unexportedPtrFn'")
			})
			t.Run("ExportedPtrFn", func(t *testing.T) {
				_, err := RunString(rt, `obj.exportedPtrFn()`)
				if vtype == "Pointer" {
					assert.NoError(t, err)
				} else {
					assert.EqualError(t, err, "TypeError: Object has no member 'exportedPtrFn'")
				}
			})
			t.Run("Error", func(t *testing.T) {
				_, err := RunString(rt, `obj.error()`)
				assert.EqualError(t, err, "GoError: error")
			})
			t.Run("Add", func(t *testing.T) {
				v, err := RunString(rt, `obj.add(1, 2)`)
				if assert.NoError(t, err) {
					assert.Equal(t, int64(3), v.Export())
				}
			})
			t.Run("AddWithError", func(t *testing.T) {
				v, err := RunString(rt, `obj.addWithError(1, 2)`)
				if assert.NoError(t, err) {
					assert.Equal(t, int64(3), v.Export())
				}

				t.Run("Negative", func(t *testing.T) {
					_, err := RunString(rt, `obj.addWithError(0, -1)`)
					assert.EqualError(t, err, "GoError: answer is negative")
				})
			})
			t.Run("Context", func(t *testing.T) {
				_, err := RunString(rt, `obj.context()`)
				assert.EqualError(t, err, "GoError: Context needs a valid VU context")

				t.Run("Valid", func(t *testing.T) {
					*ctx = context.Background()
					defer func() { *ctx = nil }()

					_, err := RunString(rt, `obj.context()`)
					assert.NoError(t, err)
				})

				t.Run("Expired", func(t *testing.T) {
					ctx_, cancel := context.WithCancel(context.Background())
					cancel()
					*ctx = ctx_
					defer func() { *ctx = nil }()

					_, err := RunString(rt, `obj.context()`)
					assert.EqualError(t, err, "GoError: test has ended")
				})
			})
			t.Run("ContextAdd", func(t *testing.T) {
				_, err := RunString(rt, `obj.contextAdd(1, 2)`)
				assert.EqualError(t, err, "GoError: ContextAdd needs a valid VU context")

				t.Run("Valid", func(t *testing.T) {
					*ctx = context.Background()
					defer func() { *ctx = nil }()

					v, err := RunString(rt, `obj.contextAdd(1, 2)`)
					if assert.NoError(t, err) {
						assert.Equal(t, int64(3), v.Export())
					}
				})

				t.Run("Expired", func(t *testing.T) {
					ctx_, cancel := context.WithCancel(context.Background())
					cancel()
					*ctx = ctx_
					defer func() { *ctx = nil }()

					_, err := RunString(rt, `obj.contextAdd(1, 2)`)
					assert.EqualError(t, err, "GoError: test has ended")
				})
			})
			t.Run("ContextAddWithError", func(t *testing.T) {
				_, err := RunString(rt, `obj.contextAddWithError(1, 2)`)
				assert.EqualError(t, err, "GoError: ContextAddWithError needs a valid VU context")

				t.Run("Valid", func(t *testing.T) {
					*ctx = context.Background()
					defer func() { *ctx = nil }()

					v, err := RunString(rt, `obj.contextAddWithError(1, 2)`)
					if assert.NoError(t, err) {
						assert.Equal(t, int64(3), v.Export())
					}

					t.Run("Negative", func(t *testing.T) {
						_, err := RunString(rt, `obj.contextAddWithError(0, -1)`)
						assert.EqualError(t, err, "GoError: answer is negative")
					})
				})
				t.Run("Expired", func(t *testing.T) {
					ctx_, cancel := context.WithCancel(context.Background())
					cancel()
					*ctx = ctx_
					defer func() { *ctx = nil }()

					_, err := RunString(rt, `obj.contextAddWithError(1, 2)`)
					assert.EqualError(t, err, "GoError: test has ended")
				})
			})
			if impl, ok := v.(*bridgeTestType); ok {
				t.Run("Count", func(t *testing.T) {
					for i := 0; i < 10; i++ {
						t.Run(strconv.Itoa(i), func(t *testing.T) {
							v, err := RunString(rt, `obj.count()`)
							if assert.NoError(t, err) {
								assert.Equal(t, int64(i+1), v.Export())
								assert.Equal(t, i+1, impl.Counter)
							}
						})
					}
				})
			} else {
				t.Run("Count", func(t *testing.T) {
					_, err := RunString(rt, `obj.count()`)
					assert.EqualError(t, err, "TypeError: Object has no member 'count'")
				})
			}
			for name, fname := range map[string]string{
				"Sum":                    "sum",
				"SumWithContext":         "sumWithContext",
				"SumWithError":           "sumWithError",
				"SumWithContextAndError": "sumWithContextAndError",
			} {
				*ctx = context.Background()
				defer func() { *ctx = nil }()

				t.Run(name, func(t *testing.T) {
					sum := 0
					args := []string{}
					for i := 0; i < 10; i++ {
						args = append(args, strconv.Itoa(i))
						sum += i
						t.Run(strconv.Itoa(i), func(t *testing.T) {
							code := fmt.Sprintf(`obj.%s(%s)`, fname, strings.Join(args, ", "))
							v, err := RunString(rt, code)
							if assert.NoError(t, err) {
								assert.Equal(t, int64(sum), v.Export())
							}
						})
					}
				})
			}

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

			t.Run("Counter", func(t *testing.T) {
				v, err := RunString(rt, `obj.counter`)
				if assert.NoError(t, err) {
					assert.Equal(t, int64(0), v.Export())
				}
			})
		})
	}
}
