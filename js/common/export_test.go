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
	"testing"

	"github.com/dop251/goja"
	jslib "github.com/loadimpact/k6/js/lib"
	"github.com/stretchr/testify/assert"
)

func TestExportBytes(t *testing.T) {
	nativeTests := []struct {
		name string
		arg  interface{}
	}{
		{
			name: "native call - []byte",
			arg:  []byte{1, 2, 3, 4, 5},
		},
		{
			name: "native call - string",
			arg:  "test",
		},
	}
	for _, tt := range nativeTests {
		t.Run(tt.name, func(t *testing.T) {
			rt := goja.New()
			val := rt.ToValue(tt.arg)
			if got := ExportBytes(rt, val); !reflect.DeepEqual(got, tt.arg) {
				t.Errorf("ExportBytes() = %v, want %v", got, tt.arg)
			}
		})
	}

	jsTests := []struct {
		name    string
		jsCode  string
		jsVarName string
		want    interface{}
	}{
		{
			name:    "empty Uint8Array",
			jsCode:  `var buf = new Uint8Array(0);`,
			jsVarName: "buf",
			want:    []byte{},
		},
		{
			name:    "uninitialized Uint8Array",
			jsCode:  `var buf = new Uint8Array(3);`,
			jsVarName: "buf",
			want:    []byte{0,0,0},
		},
		{
			name:    "initialized Uint8Array",
			jsCode:  `var buf = new Uint8Array(3); buf[0]=0; buf[1]=1; buf[2]=2;`,
			jsVarName: "buf",
			want:    []byte{0,1,2},
		},
		{
			name:    "js string",
			jsCode:  `var buf = "abc";`,
			jsVarName: "buf",
			want:    "abc",
		},
		{
			name:    "js object",
			jsCode:  `var buf = new Object();`,
			jsVarName: "buf",
			want:    make(map[string]interface{}),
		},
	}

	for _, tt := range jsTests {
		t.Run(tt.name, func(t *testing.T) {
			rt := goja.New()
			// need core js for Uint8Array
			_, err := rt.RunProgram(jslib.GetCoreJS())
			assert.NoError(t, err)
			_, err = rt.RunString(tt.jsCode)
			assert.NoError(t, err)
			jsVal := rt.Get(tt.jsVarName)
			got := ExportBytes(rt, jsVal)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExportBytes() = %v, want %v", got, tt.want)
			}
		})

	}
}

type example struct {}
func (*example) ByteSlice(ctx context.Context, size int) (goja.Value, error) {
	b := make([]byte, size)
	return GetRuntime(ctx).ToValue(b), nil
}

// example results from benchmark:
// BenchmarkExportBytes/string_baseline_Export-8                            34433214           33.7 ns/op
// BenchmarkExportBytes/string(1)-8                                         36619372           35.4 ns/op
// BenchmarkExportBytes/Uint8Array(1)-8                                        97584        11225 ns/op
// BenchmarkExportBytes/Uint8Array(1000)-8                                       225      5303649 ns/op
// BenchmarkExportBytes/Uint8Array(1000000)-8                                      2    578171708 ns/op
// BenchmarkExportBytes/[]byte(1)-8                                          5297571          228 ns/op
// BenchmarkExportBytes/[]byte(1000)-8                                       5516696          255 ns/op
// BenchmarkExportBytes/largeByteSliceAllocatedViaByteSlice-8                5299710          249 ns/op
// BenchmarkExportBytes/allocate_Uint8Array(1)-8                               40939        31815 ns/op
// BenchmarkExportBytes/allocate_Uint8Array(1000)-8                              205      5887581 ns/op
// BenchmarkExportBytes/call_example.byteSlice(1000)-8                        213500         5635 ns/op
// BenchmarkExportBytes/forEach_Uint8Array(1000)_to_Uint8Array(1000)-8           117     10801693 ns/op
// BenchmarkExportBytes/forEach_Uint8Array(1000)_to_[]byte(1000)-8               220      5712987 ns/op
// BenchmarkExportBytes/[]byte(1000)_set_bytes-8                                1910       568346 ns/op
// BenchmarkExportBytes/example.byteSlice(1000)_set_bytes-8                     1934       568199 ns/op

func BenchmarkExportBytes(b *testing.B) {
	rt := goja.New()
	rt.SetFieldNameMapper(FieldNameMapper{})
	ctx := context.Background()
	ctx = WithRuntime(ctx, rt)
	rt.Set("example", Bind(rt, &example{}, &ctx))
	_, err := rt.RunProgram(jslib.GetCoreJS())
	assert.NoError(b, err)
	_, err = rt.RunString(`
		var str = "1";
		var smallArray = new Uint8Array(1);
		var largeArray = new Uint8Array(1000);
		var largeArray2 = new Uint8Array(largeArray.byteLength);
		var mbArray = new Uint8Array(100000);
		var largeByteSliceAllocatedViaByteSlice = example.byteSlice(1000);
	`)
	assert.NoError(b, err)
	rt.Set("smallByteSlice", make([]byte, 1))
	rt.Set("largeByteSlice", make([]byte, 1000))
	smallArray := rt.Get("smallArray")
	str := rt.Get("str")
	largeArray := rt.Get("largeArray")
	mbArray := rt.Get("mbArray")
	largeByteSliceAllocatedViaByteSlice := rt.Get("largeByteSliceAllocatedViaByteSlice")
	smallByteSlice := rt.Get("smallByteSlice")
	largeByteSlice := rt.Get("largeByteSlice")
	b.Run("string baseline Export", func(b *testing.B) {
		for i:= 0; i<b.N; i++ {
			_ = str.Export()
		}
	})
	b.Run("string(1)", func(b *testing.B) {
		for i:= 0; i<b.N; i++ {
			_ = ExportBytes(rt, str)
		}
	})
	b.Run("Uint8Array(1)", func(b *testing.B) {
		for i:= 0; i<b.N; i++ {
			_ = ExportBytes(rt, smallArray)
		}
	})
	b.Run("Uint8Array(1000)", func(b *testing.B) {
		for i:= 0; i<b.N; i++ {
			_ = ExportBytes(rt, largeArray)
		}
	})
	b.Run("Uint8Array(1000000)", func(b *testing.B) {
		for i:= 0; i<b.N; i++ {
			_ = ExportBytes(rt, mbArray)
		}
	})
	b.Run("[]byte(1)", func(b *testing.B) {
		for i:= 0; i<b.N; i++ {
			_ = ExportBytes(rt, smallByteSlice)
		}
	})
	b.Run("[]byte(1000)", func(b *testing.B) {
		for i:= 0; i<b.N; i++ {
			_ = ExportBytes(rt, largeByteSlice)
		}
	})
	b.Run("largeByteSliceAllocatedViaByteSlice", func(b *testing.B) {
		for i:= 0; i<b.N; i++ {
			_ = ExportBytes(rt, largeByteSliceAllocatedViaByteSlice)
		}
	})
	b.Run("allocate Uint8Array(1)", func(b *testing.B) {
		jsSrc := `
			var asdf = new Uint8Array(1);
			`
		pgm, err := goja.Compile("testing", jsSrc, true)
		assert.NoError(b, err)
		b.ResetTimer()
		for i:= 0; i<b.N; i++ {
			_, err = rt.RunProgram(pgm)
			assert.NoError(b, err)
		}
	})
	b.Run("allocate Uint8Array(1000)", func(b *testing.B) {
		jsSrc := `
			var asdf = new Uint8Array(1000);
			`
		pgm, err := goja.Compile("testing", jsSrc, true)
		assert.NoError(b, err)
		b.ResetTimer()
		for i:= 0; i<b.N; i++ {
			_, err = rt.RunProgram(pgm)
			assert.NoError(b, err)
		}
	})
	b.Run("call example.byteSlice(1000)", func(b *testing.B) {
		jsSrc := `
			var asdf = example.byteSlice(1000);
			`
		pgm, err := goja.Compile("testing", jsSrc, true)
		assert.NoError(b, err)
		b.ResetTimer()
		for i:= 0; i<b.N; i++ {
			_, err = rt.RunProgram(pgm)
			assert.NoError(b, err)
		}
	})
	b.Run("forEach Uint8Array(1000) to Uint8Array(1000)", func(b *testing.B) {
		jsSrc := `
			largeArray.forEach(function(x,i) { largeArray2[i] = x; });
			`
		pgm, err := goja.Compile("testing", jsSrc, true)
		assert.NoError(b, err)
		b.ResetTimer()
		for i:= 0; i<b.N; i++ {
			_, err = rt.RunProgram(pgm)
			assert.NoError(b, err)
		}
	})
	b.Run("forEach Uint8Array(1000) to []byte(1000)", func(b *testing.B) {
		jsSrc := `
			largeArray.forEach(function(x,i) { largeByteSlice[i] = x; });
			`
		pgm, err := goja.Compile("testing", jsSrc, true)
		assert.NoError(b, err)
		b.ResetTimer()
		for i:= 0; i<b.N; i++ {
			_, err = rt.RunProgram(pgm)
			assert.NoError(b, err)
		}
	})
	b.Run("[]byte(1000) set bytes", func(b *testing.B) {
		jsSrc := `
			for (var x=0; x<1000; x++) { largeByteSlice[x] = x; }
		`
		pgm, err := goja.Compile("testing", jsSrc, true)
		assert.NoError(b, err)
		b.ResetTimer()
		for i:= 0; i<b.N; i++ {
			_, err = rt.RunProgram(pgm)
			assert.NoError(b, err)
		}
	})
	b.Run("example.byteSlice(1000) set bytes", func(b *testing.B) {
		jsSrc := `
			for (var x=0; x<1000; x++) { largeByteSliceAllocatedViaByteSlice[x] = x; }
		`
		pgm, err := goja.Compile("testing", jsSrc, true)
		assert.NoError(b, err)
		b.ResetTimer()
		for i:= 0; i<b.N; i++ {
			_, err = rt.RunProgram(pgm)
			assert.NoError(b, err)
		}
	})
}
