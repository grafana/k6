// +build !notfastpath

// Copyright (c) 2012-2020 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

// Code generated from fast-path.go.tmpl - DO NOT EDIT.

package codec

// Fast path functions try to create a fast path encode or decode implementation
// for common maps and slices.
//
// We define the functions and register them in this single file
// so as not to pollute the encode.go and decode.go, and create a dependency in there.
// This file can be omitted without causing a build failure.
//
// The advantage of fast paths is:
//	  - Many calls bypass reflection altogether
//
// Currently support
//	  - slice of all builtin types (numeric, bool, string, []byte)
//    - maps of builtin types to builtin or interface{} type, EXCEPT FOR
//      keys of type uintptr, int8/16/32, uint16/32, float32/64, bool, interface{}
//      AND values of type type int8/16/32, uint16/32
// This should provide adequate "typical" implementations.
//
// Note that fast track decode functions must handle values for which an address cannot be obtained.
// For example:
//	 m2 := map[string]int{}
//	 p2 := []interface{}{m2}
//	 // decoding into p2 will bomb if fast track functions do not treat like unaddressable.
//

import (
	"reflect"
	"sort"
)

const fastpathEnabled = true

type fastpathT struct{}

var fastpathTV fastpathT

type fastpathE struct {
	rt    reflect.Type
	encfn func(*Encoder, *codecFnInfo, reflect.Value)
	decfn func(*Decoder, *codecFnInfo, reflect.Value)
}

type fastpathA [54]fastpathE
type fastpathARtid [54]uintptr

var fastpathAv fastpathA
var fastpathAvRtid fastpathARtid

type fastpathAslice struct{}

func (fastpathAslice) Len() int { return 54 }
func (fastpathAslice) Less(i, j int) bool {
	return fastpathAvRtid[uint(i)] < fastpathAvRtid[uint(j)]
}
func (fastpathAslice) Swap(i, j int) {
	fastpathAvRtid[uint(i)], fastpathAvRtid[uint(j)] = fastpathAvRtid[uint(j)], fastpathAvRtid[uint(i)]
	fastpathAv[uint(i)], fastpathAv[uint(j)] = fastpathAv[uint(j)], fastpathAv[uint(i)]
}

func fastpathAvIndex(rtid uintptr) int {
	// use binary search to grab the index (adapted from sort/search.go)
	// Note: we use goto (instead of for loop) so this can be inlined.
	// h, i, j := 0, 0, 54
	var h, i uint
	var j uint = 54
LOOP:
	if i < j {
		h = (i + j) >> 1 // avoid overflow when computing h // h = i + (j-i)/2
		if fastpathAvRtid[h] < rtid {
			i = h + 1
		} else {
			j = h
		}
		goto LOOP
	}
	if i < 54 && fastpathAvRtid[i] == rtid {
		return int(i)
	}
	return -1
}

// due to possible initialization loop error, make fastpath in an init()
func init() {
	var i uint = 0
	fn := func(v interface{},
		fe func(*Encoder, *codecFnInfo, reflect.Value),
		fd func(*Decoder, *codecFnInfo, reflect.Value)) {
		xrt := reflect.TypeOf(v)
		xptr := rt2id(xrt)
		fastpathAvRtid[i] = xptr
		fastpathAv[i] = fastpathE{xrt, fe, fd}
		i++
	}

	fn([]interface{}(nil), (*Encoder).fastpathEncSliceIntfR, (*Decoder).fastpathDecSliceIntfR)
	fn([]string(nil), (*Encoder).fastpathEncSliceStringR, (*Decoder).fastpathDecSliceStringR)
	fn([][]byte(nil), (*Encoder).fastpathEncSliceBytesR, (*Decoder).fastpathDecSliceBytesR)
	fn([]float64(nil), (*Encoder).fastpathEncSliceFloat64R, (*Decoder).fastpathDecSliceFloat64R)
	fn([]uint64(nil), (*Encoder).fastpathEncSliceUint64R, (*Decoder).fastpathDecSliceUint64R)
	fn([]int(nil), (*Encoder).fastpathEncSliceIntR, (*Decoder).fastpathDecSliceIntR)
	fn([]int32(nil), (*Encoder).fastpathEncSliceInt32R, (*Decoder).fastpathDecSliceInt32R)
	fn([]int64(nil), (*Encoder).fastpathEncSliceInt64R, (*Decoder).fastpathDecSliceInt64R)
	fn([]bool(nil), (*Encoder).fastpathEncSliceBoolR, (*Decoder).fastpathDecSliceBoolR)

	fn(map[string]interface{}(nil), (*Encoder).fastpathEncMapStringIntfR, (*Decoder).fastpathDecMapStringIntfR)
	fn(map[string]string(nil), (*Encoder).fastpathEncMapStringStringR, (*Decoder).fastpathDecMapStringStringR)
	fn(map[string][]byte(nil), (*Encoder).fastpathEncMapStringBytesR, (*Decoder).fastpathDecMapStringBytesR)
	fn(map[string]uint8(nil), (*Encoder).fastpathEncMapStringUint8R, (*Decoder).fastpathDecMapStringUint8R)
	fn(map[string]uint64(nil), (*Encoder).fastpathEncMapStringUint64R, (*Decoder).fastpathDecMapStringUint64R)
	fn(map[string]int(nil), (*Encoder).fastpathEncMapStringIntR, (*Decoder).fastpathDecMapStringIntR)
	fn(map[string]int64(nil), (*Encoder).fastpathEncMapStringInt64R, (*Decoder).fastpathDecMapStringInt64R)
	fn(map[string]float64(nil), (*Encoder).fastpathEncMapStringFloat64R, (*Decoder).fastpathDecMapStringFloat64R)
	fn(map[string]bool(nil), (*Encoder).fastpathEncMapStringBoolR, (*Decoder).fastpathDecMapStringBoolR)
	fn(map[uint8]interface{}(nil), (*Encoder).fastpathEncMapUint8IntfR, (*Decoder).fastpathDecMapUint8IntfR)
	fn(map[uint8]string(nil), (*Encoder).fastpathEncMapUint8StringR, (*Decoder).fastpathDecMapUint8StringR)
	fn(map[uint8][]byte(nil), (*Encoder).fastpathEncMapUint8BytesR, (*Decoder).fastpathDecMapUint8BytesR)
	fn(map[uint8]uint8(nil), (*Encoder).fastpathEncMapUint8Uint8R, (*Decoder).fastpathDecMapUint8Uint8R)
	fn(map[uint8]uint64(nil), (*Encoder).fastpathEncMapUint8Uint64R, (*Decoder).fastpathDecMapUint8Uint64R)
	fn(map[uint8]int(nil), (*Encoder).fastpathEncMapUint8IntR, (*Decoder).fastpathDecMapUint8IntR)
	fn(map[uint8]int64(nil), (*Encoder).fastpathEncMapUint8Int64R, (*Decoder).fastpathDecMapUint8Int64R)
	fn(map[uint8]float64(nil), (*Encoder).fastpathEncMapUint8Float64R, (*Decoder).fastpathDecMapUint8Float64R)
	fn(map[uint8]bool(nil), (*Encoder).fastpathEncMapUint8BoolR, (*Decoder).fastpathDecMapUint8BoolR)
	fn(map[uint64]interface{}(nil), (*Encoder).fastpathEncMapUint64IntfR, (*Decoder).fastpathDecMapUint64IntfR)
	fn(map[uint64]string(nil), (*Encoder).fastpathEncMapUint64StringR, (*Decoder).fastpathDecMapUint64StringR)
	fn(map[uint64][]byte(nil), (*Encoder).fastpathEncMapUint64BytesR, (*Decoder).fastpathDecMapUint64BytesR)
	fn(map[uint64]uint8(nil), (*Encoder).fastpathEncMapUint64Uint8R, (*Decoder).fastpathDecMapUint64Uint8R)
	fn(map[uint64]uint64(nil), (*Encoder).fastpathEncMapUint64Uint64R, (*Decoder).fastpathDecMapUint64Uint64R)
	fn(map[uint64]int(nil), (*Encoder).fastpathEncMapUint64IntR, (*Decoder).fastpathDecMapUint64IntR)
	fn(map[uint64]int64(nil), (*Encoder).fastpathEncMapUint64Int64R, (*Decoder).fastpathDecMapUint64Int64R)
	fn(map[uint64]float64(nil), (*Encoder).fastpathEncMapUint64Float64R, (*Decoder).fastpathDecMapUint64Float64R)
	fn(map[uint64]bool(nil), (*Encoder).fastpathEncMapUint64BoolR, (*Decoder).fastpathDecMapUint64BoolR)
	fn(map[int]interface{}(nil), (*Encoder).fastpathEncMapIntIntfR, (*Decoder).fastpathDecMapIntIntfR)
	fn(map[int]string(nil), (*Encoder).fastpathEncMapIntStringR, (*Decoder).fastpathDecMapIntStringR)
	fn(map[int][]byte(nil), (*Encoder).fastpathEncMapIntBytesR, (*Decoder).fastpathDecMapIntBytesR)
	fn(map[int]uint8(nil), (*Encoder).fastpathEncMapIntUint8R, (*Decoder).fastpathDecMapIntUint8R)
	fn(map[int]uint64(nil), (*Encoder).fastpathEncMapIntUint64R, (*Decoder).fastpathDecMapIntUint64R)
	fn(map[int]int(nil), (*Encoder).fastpathEncMapIntIntR, (*Decoder).fastpathDecMapIntIntR)
	fn(map[int]int64(nil), (*Encoder).fastpathEncMapIntInt64R, (*Decoder).fastpathDecMapIntInt64R)
	fn(map[int]float64(nil), (*Encoder).fastpathEncMapIntFloat64R, (*Decoder).fastpathDecMapIntFloat64R)
	fn(map[int]bool(nil), (*Encoder).fastpathEncMapIntBoolR, (*Decoder).fastpathDecMapIntBoolR)
	fn(map[int64]interface{}(nil), (*Encoder).fastpathEncMapInt64IntfR, (*Decoder).fastpathDecMapInt64IntfR)
	fn(map[int64]string(nil), (*Encoder).fastpathEncMapInt64StringR, (*Decoder).fastpathDecMapInt64StringR)
	fn(map[int64][]byte(nil), (*Encoder).fastpathEncMapInt64BytesR, (*Decoder).fastpathDecMapInt64BytesR)
	fn(map[int64]uint8(nil), (*Encoder).fastpathEncMapInt64Uint8R, (*Decoder).fastpathDecMapInt64Uint8R)
	fn(map[int64]uint64(nil), (*Encoder).fastpathEncMapInt64Uint64R, (*Decoder).fastpathDecMapInt64Uint64R)
	fn(map[int64]int(nil), (*Encoder).fastpathEncMapInt64IntR, (*Decoder).fastpathDecMapInt64IntR)
	fn(map[int64]int64(nil), (*Encoder).fastpathEncMapInt64Int64R, (*Decoder).fastpathDecMapInt64Int64R)
	fn(map[int64]float64(nil), (*Encoder).fastpathEncMapInt64Float64R, (*Decoder).fastpathDecMapInt64Float64R)
	fn(map[int64]bool(nil), (*Encoder).fastpathEncMapInt64BoolR, (*Decoder).fastpathDecMapInt64BoolR)

	sort.Sort(fastpathAslice{})
}

// -- encode

// -- -- fast path type switch
func fastpathEncodeTypeSwitch(iv interface{}, e *Encoder) bool {
	switch v := iv.(type) {
	case []interface{}:
		fastpathTV.EncSliceIntfV(v, e)
	case *[]interface{}:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncSliceIntfV(*v, e)
		}
	case []string:
		fastpathTV.EncSliceStringV(v, e)
	case *[]string:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncSliceStringV(*v, e)
		}
	case [][]byte:
		fastpathTV.EncSliceBytesV(v, e)
	case *[][]byte:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncSliceBytesV(*v, e)
		}
	case []float64:
		fastpathTV.EncSliceFloat64V(v, e)
	case *[]float64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncSliceFloat64V(*v, e)
		}
	case []uint64:
		fastpathTV.EncSliceUint64V(v, e)
	case *[]uint64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncSliceUint64V(*v, e)
		}
	case []int:
		fastpathTV.EncSliceIntV(v, e)
	case *[]int:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncSliceIntV(*v, e)
		}
	case []int32:
		fastpathTV.EncSliceInt32V(v, e)
	case *[]int32:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncSliceInt32V(*v, e)
		}
	case []int64:
		fastpathTV.EncSliceInt64V(v, e)
	case *[]int64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncSliceInt64V(*v, e)
		}
	case []bool:
		fastpathTV.EncSliceBoolV(v, e)
	case *[]bool:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncSliceBoolV(*v, e)
		}
	case map[string]interface{}:
		fastpathTV.EncMapStringIntfV(v, e)
	case *map[string]interface{}:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapStringIntfV(*v, e)
		}
	case map[string]string:
		fastpathTV.EncMapStringStringV(v, e)
	case *map[string]string:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapStringStringV(*v, e)
		}
	case map[string][]byte:
		fastpathTV.EncMapStringBytesV(v, e)
	case *map[string][]byte:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapStringBytesV(*v, e)
		}
	case map[string]uint8:
		fastpathTV.EncMapStringUint8V(v, e)
	case *map[string]uint8:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapStringUint8V(*v, e)
		}
	case map[string]uint64:
		fastpathTV.EncMapStringUint64V(v, e)
	case *map[string]uint64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapStringUint64V(*v, e)
		}
	case map[string]int:
		fastpathTV.EncMapStringIntV(v, e)
	case *map[string]int:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapStringIntV(*v, e)
		}
	case map[string]int64:
		fastpathTV.EncMapStringInt64V(v, e)
	case *map[string]int64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapStringInt64V(*v, e)
		}
	case map[string]float64:
		fastpathTV.EncMapStringFloat64V(v, e)
	case *map[string]float64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapStringFloat64V(*v, e)
		}
	case map[string]bool:
		fastpathTV.EncMapStringBoolV(v, e)
	case *map[string]bool:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapStringBoolV(*v, e)
		}
	case map[uint8]interface{}:
		fastpathTV.EncMapUint8IntfV(v, e)
	case *map[uint8]interface{}:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint8IntfV(*v, e)
		}
	case map[uint8]string:
		fastpathTV.EncMapUint8StringV(v, e)
	case *map[uint8]string:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint8StringV(*v, e)
		}
	case map[uint8][]byte:
		fastpathTV.EncMapUint8BytesV(v, e)
	case *map[uint8][]byte:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint8BytesV(*v, e)
		}
	case map[uint8]uint8:
		fastpathTV.EncMapUint8Uint8V(v, e)
	case *map[uint8]uint8:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint8Uint8V(*v, e)
		}
	case map[uint8]uint64:
		fastpathTV.EncMapUint8Uint64V(v, e)
	case *map[uint8]uint64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint8Uint64V(*v, e)
		}
	case map[uint8]int:
		fastpathTV.EncMapUint8IntV(v, e)
	case *map[uint8]int:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint8IntV(*v, e)
		}
	case map[uint8]int64:
		fastpathTV.EncMapUint8Int64V(v, e)
	case *map[uint8]int64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint8Int64V(*v, e)
		}
	case map[uint8]float64:
		fastpathTV.EncMapUint8Float64V(v, e)
	case *map[uint8]float64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint8Float64V(*v, e)
		}
	case map[uint8]bool:
		fastpathTV.EncMapUint8BoolV(v, e)
	case *map[uint8]bool:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint8BoolV(*v, e)
		}
	case map[uint64]interface{}:
		fastpathTV.EncMapUint64IntfV(v, e)
	case *map[uint64]interface{}:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint64IntfV(*v, e)
		}
	case map[uint64]string:
		fastpathTV.EncMapUint64StringV(v, e)
	case *map[uint64]string:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint64StringV(*v, e)
		}
	case map[uint64][]byte:
		fastpathTV.EncMapUint64BytesV(v, e)
	case *map[uint64][]byte:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint64BytesV(*v, e)
		}
	case map[uint64]uint8:
		fastpathTV.EncMapUint64Uint8V(v, e)
	case *map[uint64]uint8:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint64Uint8V(*v, e)
		}
	case map[uint64]uint64:
		fastpathTV.EncMapUint64Uint64V(v, e)
	case *map[uint64]uint64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint64Uint64V(*v, e)
		}
	case map[uint64]int:
		fastpathTV.EncMapUint64IntV(v, e)
	case *map[uint64]int:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint64IntV(*v, e)
		}
	case map[uint64]int64:
		fastpathTV.EncMapUint64Int64V(v, e)
	case *map[uint64]int64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint64Int64V(*v, e)
		}
	case map[uint64]float64:
		fastpathTV.EncMapUint64Float64V(v, e)
	case *map[uint64]float64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint64Float64V(*v, e)
		}
	case map[uint64]bool:
		fastpathTV.EncMapUint64BoolV(v, e)
	case *map[uint64]bool:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapUint64BoolV(*v, e)
		}
	case map[int]interface{}:
		fastpathTV.EncMapIntIntfV(v, e)
	case *map[int]interface{}:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapIntIntfV(*v, e)
		}
	case map[int]string:
		fastpathTV.EncMapIntStringV(v, e)
	case *map[int]string:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapIntStringV(*v, e)
		}
	case map[int][]byte:
		fastpathTV.EncMapIntBytesV(v, e)
	case *map[int][]byte:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapIntBytesV(*v, e)
		}
	case map[int]uint8:
		fastpathTV.EncMapIntUint8V(v, e)
	case *map[int]uint8:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapIntUint8V(*v, e)
		}
	case map[int]uint64:
		fastpathTV.EncMapIntUint64V(v, e)
	case *map[int]uint64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapIntUint64V(*v, e)
		}
	case map[int]int:
		fastpathTV.EncMapIntIntV(v, e)
	case *map[int]int:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapIntIntV(*v, e)
		}
	case map[int]int64:
		fastpathTV.EncMapIntInt64V(v, e)
	case *map[int]int64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapIntInt64V(*v, e)
		}
	case map[int]float64:
		fastpathTV.EncMapIntFloat64V(v, e)
	case *map[int]float64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapIntFloat64V(*v, e)
		}
	case map[int]bool:
		fastpathTV.EncMapIntBoolV(v, e)
	case *map[int]bool:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapIntBoolV(*v, e)
		}
	case map[int64]interface{}:
		fastpathTV.EncMapInt64IntfV(v, e)
	case *map[int64]interface{}:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapInt64IntfV(*v, e)
		}
	case map[int64]string:
		fastpathTV.EncMapInt64StringV(v, e)
	case *map[int64]string:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapInt64StringV(*v, e)
		}
	case map[int64][]byte:
		fastpathTV.EncMapInt64BytesV(v, e)
	case *map[int64][]byte:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapInt64BytesV(*v, e)
		}
	case map[int64]uint8:
		fastpathTV.EncMapInt64Uint8V(v, e)
	case *map[int64]uint8:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapInt64Uint8V(*v, e)
		}
	case map[int64]uint64:
		fastpathTV.EncMapInt64Uint64V(v, e)
	case *map[int64]uint64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapInt64Uint64V(*v, e)
		}
	case map[int64]int:
		fastpathTV.EncMapInt64IntV(v, e)
	case *map[int64]int:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapInt64IntV(*v, e)
		}
	case map[int64]int64:
		fastpathTV.EncMapInt64Int64V(v, e)
	case *map[int64]int64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapInt64Int64V(*v, e)
		}
	case map[int64]float64:
		fastpathTV.EncMapInt64Float64V(v, e)
	case *map[int64]float64:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapInt64Float64V(*v, e)
		}
	case map[int64]bool:
		fastpathTV.EncMapInt64BoolV(v, e)
	case *map[int64]bool:
		if *v == nil {
			e.e.EncodeNil()
		} else {
			fastpathTV.EncMapInt64BoolV(*v, e)
		}
	default:
		_ = v // workaround https://github.com/golang/go/issues/12927 seen in go1.4
		return false
	}
	return true
}

// -- -- fast path functions
func (e *Encoder) fastpathEncSliceIntfR(f *codecFnInfo, rv reflect.Value) {
	if f.ti.mbs {
		fastpathTV.EncAsMapSliceIntfV(rv2i(rv).([]interface{}), e)
	} else {
		fastpathTV.EncSliceIntfV(rv2i(rv).([]interface{}), e)
	}
}
func (fastpathT) EncSliceIntfV(v []interface{}, e *Encoder) {
	e.arrayStart(len(v))
	for j := range v {
		e.arrayElem()
		e.encode(v[j])
	}
	e.arrayEnd()
}
func (fastpathT) EncAsMapSliceIntfV(v []interface{}, e *Encoder) {
	e.haltOnMbsOddLen(len(v))
	e.mapStart(len(v) >> 1) // e.mapStart(len(v) / 2)
	for j := range v {
		if j&1 == 0 { // if j%2 == 0 {
			e.mapElemKey()
		} else {
			e.mapElemValue()
		}
		e.encode(v[j])
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncSliceStringR(f *codecFnInfo, rv reflect.Value) {
	if f.ti.mbs {
		fastpathTV.EncAsMapSliceStringV(rv2i(rv).([]string), e)
	} else {
		fastpathTV.EncSliceStringV(rv2i(rv).([]string), e)
	}
}
func (fastpathT) EncSliceStringV(v []string, e *Encoder) {
	e.arrayStart(len(v))
	for j := range v {
		e.arrayElem()
		e.e.EncodeString(v[j])
	}
	e.arrayEnd()
}
func (fastpathT) EncAsMapSliceStringV(v []string, e *Encoder) {
	e.haltOnMbsOddLen(len(v))
	e.mapStart(len(v) >> 1) // e.mapStart(len(v) / 2)
	for j := range v {
		if j&1 == 0 { // if j%2 == 0 {
			e.mapElemKey()
		} else {
			e.mapElemValue()
		}
		e.e.EncodeString(v[j])
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncSliceBytesR(f *codecFnInfo, rv reflect.Value) {
	if f.ti.mbs {
		fastpathTV.EncAsMapSliceBytesV(rv2i(rv).([][]byte), e)
	} else {
		fastpathTV.EncSliceBytesV(rv2i(rv).([][]byte), e)
	}
}
func (fastpathT) EncSliceBytesV(v [][]byte, e *Encoder) {
	e.arrayStart(len(v))
	for j := range v {
		e.arrayElem()
		e.e.EncodeStringBytesRaw(v[j])
	}
	e.arrayEnd()
}
func (fastpathT) EncAsMapSliceBytesV(v [][]byte, e *Encoder) {
	e.haltOnMbsOddLen(len(v))
	e.mapStart(len(v) >> 1) // e.mapStart(len(v) / 2)
	for j := range v {
		if j&1 == 0 { // if j%2 == 0 {
			e.mapElemKey()
		} else {
			e.mapElemValue()
		}
		e.e.EncodeStringBytesRaw(v[j])
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncSliceFloat64R(f *codecFnInfo, rv reflect.Value) {
	if f.ti.mbs {
		fastpathTV.EncAsMapSliceFloat64V(rv2i(rv).([]float64), e)
	} else {
		fastpathTV.EncSliceFloat64V(rv2i(rv).([]float64), e)
	}
}
func (fastpathT) EncSliceFloat64V(v []float64, e *Encoder) {
	e.arrayStart(len(v))
	for j := range v {
		e.arrayElem()
		e.e.EncodeFloat64(v[j])
	}
	e.arrayEnd()
}
func (fastpathT) EncAsMapSliceFloat64V(v []float64, e *Encoder) {
	e.haltOnMbsOddLen(len(v))
	e.mapStart(len(v) >> 1) // e.mapStart(len(v) / 2)
	for j := range v {
		if j&1 == 0 { // if j%2 == 0 {
			e.mapElemKey()
		} else {
			e.mapElemValue()
		}
		e.e.EncodeFloat64(v[j])
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncSliceUint64R(f *codecFnInfo, rv reflect.Value) {
	if f.ti.mbs {
		fastpathTV.EncAsMapSliceUint64V(rv2i(rv).([]uint64), e)
	} else {
		fastpathTV.EncSliceUint64V(rv2i(rv).([]uint64), e)
	}
}
func (fastpathT) EncSliceUint64V(v []uint64, e *Encoder) {
	e.arrayStart(len(v))
	for j := range v {
		e.arrayElem()
		e.e.EncodeUint(v[j])
	}
	e.arrayEnd()
}
func (fastpathT) EncAsMapSliceUint64V(v []uint64, e *Encoder) {
	e.haltOnMbsOddLen(len(v))
	e.mapStart(len(v) >> 1) // e.mapStart(len(v) / 2)
	for j := range v {
		if j&1 == 0 { // if j%2 == 0 {
			e.mapElemKey()
		} else {
			e.mapElemValue()
		}
		e.e.EncodeUint(v[j])
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncSliceIntR(f *codecFnInfo, rv reflect.Value) {
	if f.ti.mbs {
		fastpathTV.EncAsMapSliceIntV(rv2i(rv).([]int), e)
	} else {
		fastpathTV.EncSliceIntV(rv2i(rv).([]int), e)
	}
}
func (fastpathT) EncSliceIntV(v []int, e *Encoder) {
	e.arrayStart(len(v))
	for j := range v {
		e.arrayElem()
		e.e.EncodeInt(int64(v[j]))
	}
	e.arrayEnd()
}
func (fastpathT) EncAsMapSliceIntV(v []int, e *Encoder) {
	e.haltOnMbsOddLen(len(v))
	e.mapStart(len(v) >> 1) // e.mapStart(len(v) / 2)
	for j := range v {
		if j&1 == 0 { // if j%2 == 0 {
			e.mapElemKey()
		} else {
			e.mapElemValue()
		}
		e.e.EncodeInt(int64(v[j]))
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncSliceInt32R(f *codecFnInfo, rv reflect.Value) {
	if f.ti.mbs {
		fastpathTV.EncAsMapSliceInt32V(rv2i(rv).([]int32), e)
	} else {
		fastpathTV.EncSliceInt32V(rv2i(rv).([]int32), e)
	}
}
func (fastpathT) EncSliceInt32V(v []int32, e *Encoder) {
	e.arrayStart(len(v))
	for j := range v {
		e.arrayElem()
		e.e.EncodeInt(int64(v[j]))
	}
	e.arrayEnd()
}
func (fastpathT) EncAsMapSliceInt32V(v []int32, e *Encoder) {
	e.haltOnMbsOddLen(len(v))
	e.mapStart(len(v) >> 1) // e.mapStart(len(v) / 2)
	for j := range v {
		if j&1 == 0 { // if j%2 == 0 {
			e.mapElemKey()
		} else {
			e.mapElemValue()
		}
		e.e.EncodeInt(int64(v[j]))
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncSliceInt64R(f *codecFnInfo, rv reflect.Value) {
	if f.ti.mbs {
		fastpathTV.EncAsMapSliceInt64V(rv2i(rv).([]int64), e)
	} else {
		fastpathTV.EncSliceInt64V(rv2i(rv).([]int64), e)
	}
}
func (fastpathT) EncSliceInt64V(v []int64, e *Encoder) {
	e.arrayStart(len(v))
	for j := range v {
		e.arrayElem()
		e.e.EncodeInt(v[j])
	}
	e.arrayEnd()
}
func (fastpathT) EncAsMapSliceInt64V(v []int64, e *Encoder) {
	e.haltOnMbsOddLen(len(v))
	e.mapStart(len(v) >> 1) // e.mapStart(len(v) / 2)
	for j := range v {
		if j&1 == 0 { // if j%2 == 0 {
			e.mapElemKey()
		} else {
			e.mapElemValue()
		}
		e.e.EncodeInt(v[j])
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncSliceBoolR(f *codecFnInfo, rv reflect.Value) {
	if f.ti.mbs {
		fastpathTV.EncAsMapSliceBoolV(rv2i(rv).([]bool), e)
	} else {
		fastpathTV.EncSliceBoolV(rv2i(rv).([]bool), e)
	}
}
func (fastpathT) EncSliceBoolV(v []bool, e *Encoder) {
	e.arrayStart(len(v))
	for j := range v {
		e.arrayElem()
		e.e.EncodeBool(v[j])
	}
	e.arrayEnd()
}
func (fastpathT) EncAsMapSliceBoolV(v []bool, e *Encoder) {
	e.haltOnMbsOddLen(len(v))
	e.mapStart(len(v) >> 1) // e.mapStart(len(v) / 2)
	for j := range v {
		if j&1 == 0 { // if j%2 == 0 {
			e.mapElemKey()
		} else {
			e.mapElemValue()
		}
		e.e.EncodeBool(v[j])
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapStringIntfR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapStringIntfV(rv2i(rv).(map[string]interface{}), e)
}
func (fastpathT) EncMapStringIntfV(v map[string]interface{}, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]string, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(stringSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.encode(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.encode(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapStringStringR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapStringStringV(rv2i(rv).(map[string]string), e)
}
func (fastpathT) EncMapStringStringV(v map[string]string, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]string, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(stringSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeString(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeString(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapStringBytesR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapStringBytesV(rv2i(rv).(map[string][]byte), e)
}
func (fastpathT) EncMapStringBytesV(v map[string][]byte, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]string, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(stringSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeStringBytesRaw(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeStringBytesRaw(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapStringUint8R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapStringUint8V(rv2i(rv).(map[string]uint8), e)
}
func (fastpathT) EncMapStringUint8V(v map[string]uint8, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]string, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(stringSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeUint(uint64(v[k2]))
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeUint(uint64(v2))
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapStringUint64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapStringUint64V(rv2i(rv).(map[string]uint64), e)
}
func (fastpathT) EncMapStringUint64V(v map[string]uint64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]string, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(stringSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeUint(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeUint(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapStringIntR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapStringIntV(rv2i(rv).(map[string]int), e)
}
func (fastpathT) EncMapStringIntV(v map[string]int, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]string, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(stringSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeInt(int64(v[k2]))
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeInt(int64(v2))
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapStringInt64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapStringInt64V(rv2i(rv).(map[string]int64), e)
}
func (fastpathT) EncMapStringInt64V(v map[string]int64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]string, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(stringSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeInt(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeInt(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapStringFloat64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapStringFloat64V(rv2i(rv).(map[string]float64), e)
}
func (fastpathT) EncMapStringFloat64V(v map[string]float64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]string, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(stringSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeFloat64(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeFloat64(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapStringBoolR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapStringBoolV(rv2i(rv).(map[string]bool), e)
}
func (fastpathT) EncMapStringBoolV(v map[string]bool, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]string, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(stringSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeBool(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeString(k2)
			e.mapElemValue()
			e.e.EncodeBool(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint8IntfR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint8IntfV(rv2i(rv).(map[uint8]interface{}), e)
}
func (fastpathT) EncMapUint8IntfV(v map[uint8]interface{}, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint8, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint8Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.encode(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.encode(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint8StringR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint8StringV(rv2i(rv).(map[uint8]string), e)
}
func (fastpathT) EncMapUint8StringV(v map[uint8]string, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint8, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint8Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeString(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeString(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint8BytesR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint8BytesV(rv2i(rv).(map[uint8][]byte), e)
}
func (fastpathT) EncMapUint8BytesV(v map[uint8][]byte, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint8, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint8Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeStringBytesRaw(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeStringBytesRaw(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint8Uint8R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint8Uint8V(rv2i(rv).(map[uint8]uint8), e)
}
func (fastpathT) EncMapUint8Uint8V(v map[uint8]uint8, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint8, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint8Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeUint(uint64(v[k2]))
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeUint(uint64(v2))
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint8Uint64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint8Uint64V(rv2i(rv).(map[uint8]uint64), e)
}
func (fastpathT) EncMapUint8Uint64V(v map[uint8]uint64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint8, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint8Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeUint(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeUint(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint8IntR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint8IntV(rv2i(rv).(map[uint8]int), e)
}
func (fastpathT) EncMapUint8IntV(v map[uint8]int, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint8, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint8Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeInt(int64(v[k2]))
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeInt(int64(v2))
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint8Int64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint8Int64V(rv2i(rv).(map[uint8]int64), e)
}
func (fastpathT) EncMapUint8Int64V(v map[uint8]int64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint8, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint8Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeInt(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeInt(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint8Float64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint8Float64V(rv2i(rv).(map[uint8]float64), e)
}
func (fastpathT) EncMapUint8Float64V(v map[uint8]float64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint8, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint8Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeFloat64(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeFloat64(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint8BoolR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint8BoolV(rv2i(rv).(map[uint8]bool), e)
}
func (fastpathT) EncMapUint8BoolV(v map[uint8]bool, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint8, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint8Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeBool(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(uint64(k2))
			e.mapElemValue()
			e.e.EncodeBool(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint64IntfR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint64IntfV(rv2i(rv).(map[uint64]interface{}), e)
}
func (fastpathT) EncMapUint64IntfV(v map[uint64]interface{}, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.encode(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.encode(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint64StringR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint64StringV(rv2i(rv).(map[uint64]string), e)
}
func (fastpathT) EncMapUint64StringV(v map[uint64]string, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeString(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeString(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint64BytesR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint64BytesV(rv2i(rv).(map[uint64][]byte), e)
}
func (fastpathT) EncMapUint64BytesV(v map[uint64][]byte, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeStringBytesRaw(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeStringBytesRaw(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint64Uint8R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint64Uint8V(rv2i(rv).(map[uint64]uint8), e)
}
func (fastpathT) EncMapUint64Uint8V(v map[uint64]uint8, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeUint(uint64(v[k2]))
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeUint(uint64(v2))
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint64Uint64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint64Uint64V(rv2i(rv).(map[uint64]uint64), e)
}
func (fastpathT) EncMapUint64Uint64V(v map[uint64]uint64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeUint(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeUint(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint64IntR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint64IntV(rv2i(rv).(map[uint64]int), e)
}
func (fastpathT) EncMapUint64IntV(v map[uint64]int, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeInt(int64(v[k2]))
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeInt(int64(v2))
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint64Int64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint64Int64V(rv2i(rv).(map[uint64]int64), e)
}
func (fastpathT) EncMapUint64Int64V(v map[uint64]int64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeInt(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeInt(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint64Float64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint64Float64V(rv2i(rv).(map[uint64]float64), e)
}
func (fastpathT) EncMapUint64Float64V(v map[uint64]float64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeFloat64(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeFloat64(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapUint64BoolR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapUint64BoolV(rv2i(rv).(map[uint64]bool), e)
}
func (fastpathT) EncMapUint64BoolV(v map[uint64]bool, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]uint64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(uint64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeBool(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeUint(k2)
			e.mapElemValue()
			e.e.EncodeBool(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapIntIntfR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapIntIntfV(rv2i(rv).(map[int]interface{}), e)
}
func (fastpathT) EncMapIntIntfV(v map[int]interface{}, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(intSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.encode(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.encode(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapIntStringR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapIntStringV(rv2i(rv).(map[int]string), e)
}
func (fastpathT) EncMapIntStringV(v map[int]string, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(intSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeString(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeString(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapIntBytesR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapIntBytesV(rv2i(rv).(map[int][]byte), e)
}
func (fastpathT) EncMapIntBytesV(v map[int][]byte, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(intSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeStringBytesRaw(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeStringBytesRaw(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapIntUint8R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapIntUint8V(rv2i(rv).(map[int]uint8), e)
}
func (fastpathT) EncMapIntUint8V(v map[int]uint8, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(intSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeUint(uint64(v[k2]))
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeUint(uint64(v2))
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapIntUint64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapIntUint64V(rv2i(rv).(map[int]uint64), e)
}
func (fastpathT) EncMapIntUint64V(v map[int]uint64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(intSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeUint(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeUint(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapIntIntR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapIntIntV(rv2i(rv).(map[int]int), e)
}
func (fastpathT) EncMapIntIntV(v map[int]int, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(intSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeInt(int64(v[k2]))
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeInt(int64(v2))
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapIntInt64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapIntInt64V(rv2i(rv).(map[int]int64), e)
}
func (fastpathT) EncMapIntInt64V(v map[int]int64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(intSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeInt(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeInt(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapIntFloat64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapIntFloat64V(rv2i(rv).(map[int]float64), e)
}
func (fastpathT) EncMapIntFloat64V(v map[int]float64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(intSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeFloat64(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeFloat64(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapIntBoolR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapIntBoolV(rv2i(rv).(map[int]bool), e)
}
func (fastpathT) EncMapIntBoolV(v map[int]bool, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(intSlice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeBool(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(int64(k2))
			e.mapElemValue()
			e.e.EncodeBool(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapInt64IntfR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapInt64IntfV(rv2i(rv).(map[int64]interface{}), e)
}
func (fastpathT) EncMapInt64IntfV(v map[int64]interface{}, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(int64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.encode(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.encode(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapInt64StringR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapInt64StringV(rv2i(rv).(map[int64]string), e)
}
func (fastpathT) EncMapInt64StringV(v map[int64]string, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(int64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeString(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeString(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapInt64BytesR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapInt64BytesV(rv2i(rv).(map[int64][]byte), e)
}
func (fastpathT) EncMapInt64BytesV(v map[int64][]byte, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(int64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeStringBytesRaw(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeStringBytesRaw(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapInt64Uint8R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapInt64Uint8V(rv2i(rv).(map[int64]uint8), e)
}
func (fastpathT) EncMapInt64Uint8V(v map[int64]uint8, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(int64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeUint(uint64(v[k2]))
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeUint(uint64(v2))
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapInt64Uint64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapInt64Uint64V(rv2i(rv).(map[int64]uint64), e)
}
func (fastpathT) EncMapInt64Uint64V(v map[int64]uint64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(int64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeUint(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeUint(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapInt64IntR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapInt64IntV(rv2i(rv).(map[int64]int), e)
}
func (fastpathT) EncMapInt64IntV(v map[int64]int, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(int64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeInt(int64(v[k2]))
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeInt(int64(v2))
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapInt64Int64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapInt64Int64V(rv2i(rv).(map[int64]int64), e)
}
func (fastpathT) EncMapInt64Int64V(v map[int64]int64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(int64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeInt(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeInt(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapInt64Float64R(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapInt64Float64V(rv2i(rv).(map[int64]float64), e)
}
func (fastpathT) EncMapInt64Float64V(v map[int64]float64, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(int64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeFloat64(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeFloat64(v2)
		}
	}
	e.mapEnd()
}
func (e *Encoder) fastpathEncMapInt64BoolR(f *codecFnInfo, rv reflect.Value) {
	fastpathTV.EncMapInt64BoolV(rv2i(rv).(map[int64]bool), e)
}
func (fastpathT) EncMapInt64BoolV(v map[int64]bool, e *Encoder) {
	e.mapStart(len(v))
	if e.h.Canonical {
		v2 := make([]int64, len(v))
		var i uint
		for k := range v {
			v2[i] = k
			i++
		}
		sort.Sort(int64Slice(v2))
		for _, k2 := range v2 {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeBool(v[k2])
		}
	} else {
		for k2, v2 := range v {
			e.mapElemKey()
			e.e.EncodeInt(k2)
			e.mapElemValue()
			e.e.EncodeBool(v2)
		}
	}
	e.mapEnd()
}

// -- decode

// -- -- fast path type switch
func fastpathDecodeTypeSwitch(iv interface{}, d *Decoder) bool {
	var changed bool
	var containerLen int
	switch v := iv.(type) {
	case []interface{}:
		fastpathTV.DecSliceIntfN(v, d)
	case *[]interface{}:
		var v2 []interface{}
		if v2, changed = fastpathTV.DecSliceIntfY(*v, d); changed {
			*v = v2
		}
	case []string:
		fastpathTV.DecSliceStringN(v, d)
	case *[]string:
		var v2 []string
		if v2, changed = fastpathTV.DecSliceStringY(*v, d); changed {
			*v = v2
		}
	case [][]byte:
		fastpathTV.DecSliceBytesN(v, d)
	case *[][]byte:
		var v2 [][]byte
		if v2, changed = fastpathTV.DecSliceBytesY(*v, d); changed {
			*v = v2
		}
	case []float64:
		fastpathTV.DecSliceFloat64N(v, d)
	case *[]float64:
		var v2 []float64
		if v2, changed = fastpathTV.DecSliceFloat64Y(*v, d); changed {
			*v = v2
		}
	case []uint64:
		fastpathTV.DecSliceUint64N(v, d)
	case *[]uint64:
		var v2 []uint64
		if v2, changed = fastpathTV.DecSliceUint64Y(*v, d); changed {
			*v = v2
		}
	case []int:
		fastpathTV.DecSliceIntN(v, d)
	case *[]int:
		var v2 []int
		if v2, changed = fastpathTV.DecSliceIntY(*v, d); changed {
			*v = v2
		}
	case []int32:
		fastpathTV.DecSliceInt32N(v, d)
	case *[]int32:
		var v2 []int32
		if v2, changed = fastpathTV.DecSliceInt32Y(*v, d); changed {
			*v = v2
		}
	case []int64:
		fastpathTV.DecSliceInt64N(v, d)
	case *[]int64:
		var v2 []int64
		if v2, changed = fastpathTV.DecSliceInt64Y(*v, d); changed {
			*v = v2
		}
	case []bool:
		fastpathTV.DecSliceBoolN(v, d)
	case *[]bool:
		var v2 []bool
		if v2, changed = fastpathTV.DecSliceBoolY(*v, d); changed {
			*v = v2
		}
	case map[string]interface{}:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapStringIntfL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[string]interface{}:
		fastpathTV.DecMapStringIntfX(v, d)
	case map[string]string:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapStringStringL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[string]string:
		fastpathTV.DecMapStringStringX(v, d)
	case map[string][]byte:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapStringBytesL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[string][]byte:
		fastpathTV.DecMapStringBytesX(v, d)
	case map[string]uint8:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapStringUint8L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[string]uint8:
		fastpathTV.DecMapStringUint8X(v, d)
	case map[string]uint64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapStringUint64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[string]uint64:
		fastpathTV.DecMapStringUint64X(v, d)
	case map[string]int:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapStringIntL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[string]int:
		fastpathTV.DecMapStringIntX(v, d)
	case map[string]int64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapStringInt64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[string]int64:
		fastpathTV.DecMapStringInt64X(v, d)
	case map[string]float64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapStringFloat64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[string]float64:
		fastpathTV.DecMapStringFloat64X(v, d)
	case map[string]bool:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapStringBoolL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[string]bool:
		fastpathTV.DecMapStringBoolX(v, d)
	case map[uint8]interface{}:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint8IntfL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint8]interface{}:
		fastpathTV.DecMapUint8IntfX(v, d)
	case map[uint8]string:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint8StringL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint8]string:
		fastpathTV.DecMapUint8StringX(v, d)
	case map[uint8][]byte:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint8BytesL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint8][]byte:
		fastpathTV.DecMapUint8BytesX(v, d)
	case map[uint8]uint8:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint8Uint8L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint8]uint8:
		fastpathTV.DecMapUint8Uint8X(v, d)
	case map[uint8]uint64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint8Uint64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint8]uint64:
		fastpathTV.DecMapUint8Uint64X(v, d)
	case map[uint8]int:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint8IntL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint8]int:
		fastpathTV.DecMapUint8IntX(v, d)
	case map[uint8]int64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint8Int64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint8]int64:
		fastpathTV.DecMapUint8Int64X(v, d)
	case map[uint8]float64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint8Float64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint8]float64:
		fastpathTV.DecMapUint8Float64X(v, d)
	case map[uint8]bool:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint8BoolL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint8]bool:
		fastpathTV.DecMapUint8BoolX(v, d)
	case map[uint64]interface{}:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint64IntfL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint64]interface{}:
		fastpathTV.DecMapUint64IntfX(v, d)
	case map[uint64]string:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint64StringL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint64]string:
		fastpathTV.DecMapUint64StringX(v, d)
	case map[uint64][]byte:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint64BytesL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint64][]byte:
		fastpathTV.DecMapUint64BytesX(v, d)
	case map[uint64]uint8:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint64Uint8L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint64]uint8:
		fastpathTV.DecMapUint64Uint8X(v, d)
	case map[uint64]uint64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint64Uint64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint64]uint64:
		fastpathTV.DecMapUint64Uint64X(v, d)
	case map[uint64]int:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint64IntL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint64]int:
		fastpathTV.DecMapUint64IntX(v, d)
	case map[uint64]int64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint64Int64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint64]int64:
		fastpathTV.DecMapUint64Int64X(v, d)
	case map[uint64]float64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint64Float64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint64]float64:
		fastpathTV.DecMapUint64Float64X(v, d)
	case map[uint64]bool:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapUint64BoolL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[uint64]bool:
		fastpathTV.DecMapUint64BoolX(v, d)
	case map[int]interface{}:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapIntIntfL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int]interface{}:
		fastpathTV.DecMapIntIntfX(v, d)
	case map[int]string:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapIntStringL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int]string:
		fastpathTV.DecMapIntStringX(v, d)
	case map[int][]byte:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapIntBytesL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int][]byte:
		fastpathTV.DecMapIntBytesX(v, d)
	case map[int]uint8:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapIntUint8L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int]uint8:
		fastpathTV.DecMapIntUint8X(v, d)
	case map[int]uint64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapIntUint64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int]uint64:
		fastpathTV.DecMapIntUint64X(v, d)
	case map[int]int:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapIntIntL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int]int:
		fastpathTV.DecMapIntIntX(v, d)
	case map[int]int64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapIntInt64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int]int64:
		fastpathTV.DecMapIntInt64X(v, d)
	case map[int]float64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapIntFloat64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int]float64:
		fastpathTV.DecMapIntFloat64X(v, d)
	case map[int]bool:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapIntBoolL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int]bool:
		fastpathTV.DecMapIntBoolX(v, d)
	case map[int64]interface{}:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapInt64IntfL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int64]interface{}:
		fastpathTV.DecMapInt64IntfX(v, d)
	case map[int64]string:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapInt64StringL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int64]string:
		fastpathTV.DecMapInt64StringX(v, d)
	case map[int64][]byte:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapInt64BytesL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int64][]byte:
		fastpathTV.DecMapInt64BytesX(v, d)
	case map[int64]uint8:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapInt64Uint8L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int64]uint8:
		fastpathTV.DecMapInt64Uint8X(v, d)
	case map[int64]uint64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapInt64Uint64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int64]uint64:
		fastpathTV.DecMapInt64Uint64X(v, d)
	case map[int64]int:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapInt64IntL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int64]int:
		fastpathTV.DecMapInt64IntX(v, d)
	case map[int64]int64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapInt64Int64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int64]int64:
		fastpathTV.DecMapInt64Int64X(v, d)
	case map[int64]float64:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapInt64Float64L(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int64]float64:
		fastpathTV.DecMapInt64Float64X(v, d)
	case map[int64]bool:
		containerLen = d.mapStart()
		if containerLen != containerLenNil {
			if containerLen != 0 {
				fastpathTV.DecMapInt64BoolL(v, containerLen, d)
			}
			d.mapEnd()
		}
	case *map[int64]bool:
		fastpathTV.DecMapInt64BoolX(v, d)
	default:
		_ = v // workaround https://github.com/golang/go/issues/12927 seen in go1.4
		return false
	}
	return true
}

func fastpathDecodeSetZeroTypeSwitch(iv interface{}) bool {
	switch v := iv.(type) {
	case *[]interface{}:
		*v = nil
	case *[]string:
		*v = nil
	case *[][]byte:
		*v = nil
	case *[]float64:
		*v = nil
	case *[]uint64:
		*v = nil
	case *[]int:
		*v = nil
	case *[]int32:
		*v = nil
	case *[]int64:
		*v = nil
	case *[]bool:
		*v = nil

	case *map[string]interface{}:
		*v = nil
	case *map[string]string:
		*v = nil
	case *map[string][]byte:
		*v = nil
	case *map[string]uint8:
		*v = nil
	case *map[string]uint64:
		*v = nil
	case *map[string]int:
		*v = nil
	case *map[string]int64:
		*v = nil
	case *map[string]float64:
		*v = nil
	case *map[string]bool:
		*v = nil
	case *map[uint8]interface{}:
		*v = nil
	case *map[uint8]string:
		*v = nil
	case *map[uint8][]byte:
		*v = nil
	case *map[uint8]uint8:
		*v = nil
	case *map[uint8]uint64:
		*v = nil
	case *map[uint8]int:
		*v = nil
	case *map[uint8]int64:
		*v = nil
	case *map[uint8]float64:
		*v = nil
	case *map[uint8]bool:
		*v = nil
	case *map[uint64]interface{}:
		*v = nil
	case *map[uint64]string:
		*v = nil
	case *map[uint64][]byte:
		*v = nil
	case *map[uint64]uint8:
		*v = nil
	case *map[uint64]uint64:
		*v = nil
	case *map[uint64]int:
		*v = nil
	case *map[uint64]int64:
		*v = nil
	case *map[uint64]float64:
		*v = nil
	case *map[uint64]bool:
		*v = nil
	case *map[int]interface{}:
		*v = nil
	case *map[int]string:
		*v = nil
	case *map[int][]byte:
		*v = nil
	case *map[int]uint8:
		*v = nil
	case *map[int]uint64:
		*v = nil
	case *map[int]int:
		*v = nil
	case *map[int]int64:
		*v = nil
	case *map[int]float64:
		*v = nil
	case *map[int]bool:
		*v = nil
	case *map[int64]interface{}:
		*v = nil
	case *map[int64]string:
		*v = nil
	case *map[int64][]byte:
		*v = nil
	case *map[int64]uint8:
		*v = nil
	case *map[int64]uint64:
		*v = nil
	case *map[int64]int:
		*v = nil
	case *map[int64]int64:
		*v = nil
	case *map[int64]float64:
		*v = nil
	case *map[int64]bool:
		*v = nil

	default:
		_ = v // workaround https://github.com/golang/go/issues/12927 seen in go1.4
		return false
	}
	return true
}

// -- -- fast path functions

func (d *Decoder) fastpathDecSliceIntfR(f *codecFnInfo, rv reflect.Value) {
	if f.seq != seqTypeArray && rv.Kind() == reflect.Ptr {
		vp := rv2i(rv).(*[]interface{})
		if v, changed := fastpathTV.DecSliceIntfY(*vp, d); changed {
			*vp = v
		}
	} else {
		fastpathTV.DecSliceIntfN(rv2i(rv).([]interface{}), d)
	}
}
func (f fastpathT) DecSliceIntfX(vp *[]interface{}, d *Decoder) {
	if v, changed := f.DecSliceIntfY(*vp, d); changed {
		*vp = v
	}
}
func (fastpathT) DecSliceIntfY(v []interface{}, d *Decoder) (_ []interface{}, changed bool) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		if v == nil {
			return
		}
		return nil, true
	}
	if containerLenS == 0 {
		if v == nil {
			v = []interface{}{}
		} else if len(v) != 0 {
			v = v[:0]
		}
		slh.End()
		return v, true
	}
	hasLen := containerLenS > 0
	var xlen int
	if hasLen {
		if containerLenS > cap(v) {
			xlen = decInferLen(containerLenS, d.h.MaxInitLen, 16)
			if xlen <= cap(v) {
				v = v[:uint(xlen)]
			} else {
				v = make([]interface{}, uint(xlen))
			}
			changed = true
		} else if containerLenS != len(v) {
			v = v[:containerLenS]
			changed = true
		}
	}
	var j int
	for j = 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j == 0 && len(v) == 0 { // means hasLen == false
			xlen = decDefSliceCap
			v = make([]interface{}, uint(xlen))
			changed = true
		}
		if j >= len(v) {
			v = append(v, nil)
			changed = true
		}
		slh.ElemContainerState(j)
		d.decode(&v[uint(j)])
	}
	if j < len(v) {
		v = v[:uint(j)]
		changed = true
	} else if j == 0 && v == nil {
		v = []interface{}{}
		changed = true
	}
	slh.End()
	return v, changed
}
func (fastpathT) DecSliceIntfN(v []interface{}, d *Decoder) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		return
	}
	if containerLenS == 0 {
		slh.End()
		return
	}
	hasLen := containerLenS > 0
	for j := 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j >= len(v) {
			decArrayCannotExpand(slh, hasLen, len(v), j, containerLenS)
			return
		}
		slh.ElemContainerState(j)
		d.decode(&v[uint(j)])
	}
	slh.End()
}

func (d *Decoder) fastpathDecSliceStringR(f *codecFnInfo, rv reflect.Value) {
	if f.seq != seqTypeArray && rv.Kind() == reflect.Ptr {
		vp := rv2i(rv).(*[]string)
		if v, changed := fastpathTV.DecSliceStringY(*vp, d); changed {
			*vp = v
		}
	} else {
		fastpathTV.DecSliceStringN(rv2i(rv).([]string), d)
	}
}
func (f fastpathT) DecSliceStringX(vp *[]string, d *Decoder) {
	if v, changed := f.DecSliceStringY(*vp, d); changed {
		*vp = v
	}
}
func (fastpathT) DecSliceStringY(v []string, d *Decoder) (_ []string, changed bool) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		if v == nil {
			return
		}
		return nil, true
	}
	if containerLenS == 0 {
		if v == nil {
			v = []string{}
		} else if len(v) != 0 {
			v = v[:0]
		}
		slh.End()
		return v, true
	}
	hasLen := containerLenS > 0
	var xlen int
	if hasLen {
		if containerLenS > cap(v) {
			xlen = decInferLen(containerLenS, d.h.MaxInitLen, 16)
			if xlen <= cap(v) {
				v = v[:uint(xlen)]
			} else {
				v = make([]string, uint(xlen))
			}
			changed = true
		} else if containerLenS != len(v) {
			v = v[:containerLenS]
			changed = true
		}
	}
	var j int
	for j = 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j == 0 && len(v) == 0 { // means hasLen == false
			xlen = decDefSliceCap
			v = make([]string, uint(xlen))
			changed = true
		}
		if j >= len(v) {
			v = append(v, "")
			changed = true
		}
		slh.ElemContainerState(j)
		v[uint(j)] = string(d.d.DecodeStringAsBytes())
	}
	if j < len(v) {
		v = v[:uint(j)]
		changed = true
	} else if j == 0 && v == nil {
		v = []string{}
		changed = true
	}
	slh.End()
	return v, changed
}
func (fastpathT) DecSliceStringN(v []string, d *Decoder) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		return
	}
	if containerLenS == 0 {
		slh.End()
		return
	}
	hasLen := containerLenS > 0
	for j := 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j >= len(v) {
			decArrayCannotExpand(slh, hasLen, len(v), j, containerLenS)
			return
		}
		slh.ElemContainerState(j)
		v[uint(j)] = string(d.d.DecodeStringAsBytes())
	}
	slh.End()
}

func (d *Decoder) fastpathDecSliceBytesR(f *codecFnInfo, rv reflect.Value) {
	if f.seq != seqTypeArray && rv.Kind() == reflect.Ptr {
		vp := rv2i(rv).(*[][]byte)
		if v, changed := fastpathTV.DecSliceBytesY(*vp, d); changed {
			*vp = v
		}
	} else {
		fastpathTV.DecSliceBytesN(rv2i(rv).([][]byte), d)
	}
}
func (f fastpathT) DecSliceBytesX(vp *[][]byte, d *Decoder) {
	if v, changed := f.DecSliceBytesY(*vp, d); changed {
		*vp = v
	}
}
func (fastpathT) DecSliceBytesY(v [][]byte, d *Decoder) (_ [][]byte, changed bool) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		if v == nil {
			return
		}
		return nil, true
	}
	if containerLenS == 0 {
		if v == nil {
			v = [][]byte{}
		} else if len(v) != 0 {
			v = v[:0]
		}
		slh.End()
		return v, true
	}
	hasLen := containerLenS > 0
	var xlen int
	if hasLen {
		if containerLenS > cap(v) {
			xlen = decInferLen(containerLenS, d.h.MaxInitLen, 24)
			if xlen <= cap(v) {
				v = v[:uint(xlen)]
			} else {
				v = make([][]byte, uint(xlen))
			}
			changed = true
		} else if containerLenS != len(v) {
			v = v[:containerLenS]
			changed = true
		}
	}
	var j int
	for j = 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j == 0 && len(v) == 0 { // means hasLen == false
			xlen = decDefSliceCap
			v = make([][]byte, uint(xlen))
			changed = true
		}
		if j >= len(v) {
			v = append(v, nil)
			changed = true
		}
		slh.ElemContainerState(j)
		v[uint(j)] = d.d.DecodeBytes(nil, false)
	}
	if j < len(v) {
		v = v[:uint(j)]
		changed = true
	} else if j == 0 && v == nil {
		v = [][]byte{}
		changed = true
	}
	slh.End()
	return v, changed
}
func (fastpathT) DecSliceBytesN(v [][]byte, d *Decoder) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		return
	}
	if containerLenS == 0 {
		slh.End()
		return
	}
	hasLen := containerLenS > 0
	for j := 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j >= len(v) {
			decArrayCannotExpand(slh, hasLen, len(v), j, containerLenS)
			return
		}
		slh.ElemContainerState(j)
		v[uint(j)] = d.d.DecodeBytes(nil, false)
	}
	slh.End()
}

func (d *Decoder) fastpathDecSliceFloat64R(f *codecFnInfo, rv reflect.Value) {
	if f.seq != seqTypeArray && rv.Kind() == reflect.Ptr {
		vp := rv2i(rv).(*[]float64)
		if v, changed := fastpathTV.DecSliceFloat64Y(*vp, d); changed {
			*vp = v
		}
	} else {
		fastpathTV.DecSliceFloat64N(rv2i(rv).([]float64), d)
	}
}
func (f fastpathT) DecSliceFloat64X(vp *[]float64, d *Decoder) {
	if v, changed := f.DecSliceFloat64Y(*vp, d); changed {
		*vp = v
	}
}
func (fastpathT) DecSliceFloat64Y(v []float64, d *Decoder) (_ []float64, changed bool) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		if v == nil {
			return
		}
		return nil, true
	}
	if containerLenS == 0 {
		if v == nil {
			v = []float64{}
		} else if len(v) != 0 {
			v = v[:0]
		}
		slh.End()
		return v, true
	}
	hasLen := containerLenS > 0
	var xlen int
	if hasLen {
		if containerLenS > cap(v) {
			xlen = decInferLen(containerLenS, d.h.MaxInitLen, 8)
			if xlen <= cap(v) {
				v = v[:uint(xlen)]
			} else {
				v = make([]float64, uint(xlen))
			}
			changed = true
		} else if containerLenS != len(v) {
			v = v[:containerLenS]
			changed = true
		}
	}
	var j int
	for j = 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j == 0 && len(v) == 0 { // means hasLen == false
			xlen = decDefSliceCap
			v = make([]float64, uint(xlen))
			changed = true
		}
		if j >= len(v) {
			v = append(v, 0)
			changed = true
		}
		slh.ElemContainerState(j)
		v[uint(j)] = d.d.DecodeFloat64()
	}
	if j < len(v) {
		v = v[:uint(j)]
		changed = true
	} else if j == 0 && v == nil {
		v = []float64{}
		changed = true
	}
	slh.End()
	return v, changed
}
func (fastpathT) DecSliceFloat64N(v []float64, d *Decoder) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		return
	}
	if containerLenS == 0 {
		slh.End()
		return
	}
	hasLen := containerLenS > 0
	for j := 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j >= len(v) {
			decArrayCannotExpand(slh, hasLen, len(v), j, containerLenS)
			return
		}
		slh.ElemContainerState(j)
		v[uint(j)] = d.d.DecodeFloat64()
	}
	slh.End()
}

func (d *Decoder) fastpathDecSliceUint64R(f *codecFnInfo, rv reflect.Value) {
	if f.seq != seqTypeArray && rv.Kind() == reflect.Ptr {
		vp := rv2i(rv).(*[]uint64)
		if v, changed := fastpathTV.DecSliceUint64Y(*vp, d); changed {
			*vp = v
		}
	} else {
		fastpathTV.DecSliceUint64N(rv2i(rv).([]uint64), d)
	}
}
func (f fastpathT) DecSliceUint64X(vp *[]uint64, d *Decoder) {
	if v, changed := f.DecSliceUint64Y(*vp, d); changed {
		*vp = v
	}
}
func (fastpathT) DecSliceUint64Y(v []uint64, d *Decoder) (_ []uint64, changed bool) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		if v == nil {
			return
		}
		return nil, true
	}
	if containerLenS == 0 {
		if v == nil {
			v = []uint64{}
		} else if len(v) != 0 {
			v = v[:0]
		}
		slh.End()
		return v, true
	}
	hasLen := containerLenS > 0
	var xlen int
	if hasLen {
		if containerLenS > cap(v) {
			xlen = decInferLen(containerLenS, d.h.MaxInitLen, 8)
			if xlen <= cap(v) {
				v = v[:uint(xlen)]
			} else {
				v = make([]uint64, uint(xlen))
			}
			changed = true
		} else if containerLenS != len(v) {
			v = v[:containerLenS]
			changed = true
		}
	}
	var j int
	for j = 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j == 0 && len(v) == 0 { // means hasLen == false
			xlen = decDefSliceCap
			v = make([]uint64, uint(xlen))
			changed = true
		}
		if j >= len(v) {
			v = append(v, 0)
			changed = true
		}
		slh.ElemContainerState(j)
		v[uint(j)] = d.d.DecodeUint64()
	}
	if j < len(v) {
		v = v[:uint(j)]
		changed = true
	} else if j == 0 && v == nil {
		v = []uint64{}
		changed = true
	}
	slh.End()
	return v, changed
}
func (fastpathT) DecSliceUint64N(v []uint64, d *Decoder) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		return
	}
	if containerLenS == 0 {
		slh.End()
		return
	}
	hasLen := containerLenS > 0
	for j := 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j >= len(v) {
			decArrayCannotExpand(slh, hasLen, len(v), j, containerLenS)
			return
		}
		slh.ElemContainerState(j)
		v[uint(j)] = d.d.DecodeUint64()
	}
	slh.End()
}

func (d *Decoder) fastpathDecSliceIntR(f *codecFnInfo, rv reflect.Value) {
	if f.seq != seqTypeArray && rv.Kind() == reflect.Ptr {
		vp := rv2i(rv).(*[]int)
		if v, changed := fastpathTV.DecSliceIntY(*vp, d); changed {
			*vp = v
		}
	} else {
		fastpathTV.DecSliceIntN(rv2i(rv).([]int), d)
	}
}
func (f fastpathT) DecSliceIntX(vp *[]int, d *Decoder) {
	if v, changed := f.DecSliceIntY(*vp, d); changed {
		*vp = v
	}
}
func (fastpathT) DecSliceIntY(v []int, d *Decoder) (_ []int, changed bool) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		if v == nil {
			return
		}
		return nil, true
	}
	if containerLenS == 0 {
		if v == nil {
			v = []int{}
		} else if len(v) != 0 {
			v = v[:0]
		}
		slh.End()
		return v, true
	}
	hasLen := containerLenS > 0
	var xlen int
	if hasLen {
		if containerLenS > cap(v) {
			xlen = decInferLen(containerLenS, d.h.MaxInitLen, 8)
			if xlen <= cap(v) {
				v = v[:uint(xlen)]
			} else {
				v = make([]int, uint(xlen))
			}
			changed = true
		} else if containerLenS != len(v) {
			v = v[:containerLenS]
			changed = true
		}
	}
	var j int
	for j = 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j == 0 && len(v) == 0 { // means hasLen == false
			xlen = decDefSliceCap
			v = make([]int, uint(xlen))
			changed = true
		}
		if j >= len(v) {
			v = append(v, 0)
			changed = true
		}
		slh.ElemContainerState(j)
		v[uint(j)] = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
	}
	if j < len(v) {
		v = v[:uint(j)]
		changed = true
	} else if j == 0 && v == nil {
		v = []int{}
		changed = true
	}
	slh.End()
	return v, changed
}
func (fastpathT) DecSliceIntN(v []int, d *Decoder) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		return
	}
	if containerLenS == 0 {
		slh.End()
		return
	}
	hasLen := containerLenS > 0
	for j := 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j >= len(v) {
			decArrayCannotExpand(slh, hasLen, len(v), j, containerLenS)
			return
		}
		slh.ElemContainerState(j)
		v[uint(j)] = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
	}
	slh.End()
}

func (d *Decoder) fastpathDecSliceInt32R(f *codecFnInfo, rv reflect.Value) {
	if f.seq != seqTypeArray && rv.Kind() == reflect.Ptr {
		vp := rv2i(rv).(*[]int32)
		if v, changed := fastpathTV.DecSliceInt32Y(*vp, d); changed {
			*vp = v
		}
	} else {
		fastpathTV.DecSliceInt32N(rv2i(rv).([]int32), d)
	}
}
func (f fastpathT) DecSliceInt32X(vp *[]int32, d *Decoder) {
	if v, changed := f.DecSliceInt32Y(*vp, d); changed {
		*vp = v
	}
}
func (fastpathT) DecSliceInt32Y(v []int32, d *Decoder) (_ []int32, changed bool) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		if v == nil {
			return
		}
		return nil, true
	}
	if containerLenS == 0 {
		if v == nil {
			v = []int32{}
		} else if len(v) != 0 {
			v = v[:0]
		}
		slh.End()
		return v, true
	}
	hasLen := containerLenS > 0
	var xlen int
	if hasLen {
		if containerLenS > cap(v) {
			xlen = decInferLen(containerLenS, d.h.MaxInitLen, 4)
			if xlen <= cap(v) {
				v = v[:uint(xlen)]
			} else {
				v = make([]int32, uint(xlen))
			}
			changed = true
		} else if containerLenS != len(v) {
			v = v[:containerLenS]
			changed = true
		}
	}
	var j int
	for j = 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j == 0 && len(v) == 0 { // means hasLen == false
			xlen = decDefSliceCap
			v = make([]int32, uint(xlen))
			changed = true
		}
		if j >= len(v) {
			v = append(v, 0)
			changed = true
		}
		slh.ElemContainerState(j)
		v[uint(j)] = int32(chkOvf.IntV(d.d.DecodeInt64(), 32))
	}
	if j < len(v) {
		v = v[:uint(j)]
		changed = true
	} else if j == 0 && v == nil {
		v = []int32{}
		changed = true
	}
	slh.End()
	return v, changed
}
func (fastpathT) DecSliceInt32N(v []int32, d *Decoder) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		return
	}
	if containerLenS == 0 {
		slh.End()
		return
	}
	hasLen := containerLenS > 0
	for j := 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j >= len(v) {
			decArrayCannotExpand(slh, hasLen, len(v), j, containerLenS)
			return
		}
		slh.ElemContainerState(j)
		v[uint(j)] = int32(chkOvf.IntV(d.d.DecodeInt64(), 32))
	}
	slh.End()
}

func (d *Decoder) fastpathDecSliceInt64R(f *codecFnInfo, rv reflect.Value) {
	if f.seq != seqTypeArray && rv.Kind() == reflect.Ptr {
		vp := rv2i(rv).(*[]int64)
		if v, changed := fastpathTV.DecSliceInt64Y(*vp, d); changed {
			*vp = v
		}
	} else {
		fastpathTV.DecSliceInt64N(rv2i(rv).([]int64), d)
	}
}
func (f fastpathT) DecSliceInt64X(vp *[]int64, d *Decoder) {
	if v, changed := f.DecSliceInt64Y(*vp, d); changed {
		*vp = v
	}
}
func (fastpathT) DecSliceInt64Y(v []int64, d *Decoder) (_ []int64, changed bool) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		if v == nil {
			return
		}
		return nil, true
	}
	if containerLenS == 0 {
		if v == nil {
			v = []int64{}
		} else if len(v) != 0 {
			v = v[:0]
		}
		slh.End()
		return v, true
	}
	hasLen := containerLenS > 0
	var xlen int
	if hasLen {
		if containerLenS > cap(v) {
			xlen = decInferLen(containerLenS, d.h.MaxInitLen, 8)
			if xlen <= cap(v) {
				v = v[:uint(xlen)]
			} else {
				v = make([]int64, uint(xlen))
			}
			changed = true
		} else if containerLenS != len(v) {
			v = v[:containerLenS]
			changed = true
		}
	}
	var j int
	for j = 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j == 0 && len(v) == 0 { // means hasLen == false
			xlen = decDefSliceCap
			v = make([]int64, uint(xlen))
			changed = true
		}
		if j >= len(v) {
			v = append(v, 0)
			changed = true
		}
		slh.ElemContainerState(j)
		v[uint(j)] = d.d.DecodeInt64()
	}
	if j < len(v) {
		v = v[:uint(j)]
		changed = true
	} else if j == 0 && v == nil {
		v = []int64{}
		changed = true
	}
	slh.End()
	return v, changed
}
func (fastpathT) DecSliceInt64N(v []int64, d *Decoder) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		return
	}
	if containerLenS == 0 {
		slh.End()
		return
	}
	hasLen := containerLenS > 0
	for j := 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j >= len(v) {
			decArrayCannotExpand(slh, hasLen, len(v), j, containerLenS)
			return
		}
		slh.ElemContainerState(j)
		v[uint(j)] = d.d.DecodeInt64()
	}
	slh.End()
}

func (d *Decoder) fastpathDecSliceBoolR(f *codecFnInfo, rv reflect.Value) {
	if f.seq != seqTypeArray && rv.Kind() == reflect.Ptr {
		vp := rv2i(rv).(*[]bool)
		if v, changed := fastpathTV.DecSliceBoolY(*vp, d); changed {
			*vp = v
		}
	} else {
		fastpathTV.DecSliceBoolN(rv2i(rv).([]bool), d)
	}
}
func (f fastpathT) DecSliceBoolX(vp *[]bool, d *Decoder) {
	if v, changed := f.DecSliceBoolY(*vp, d); changed {
		*vp = v
	}
}
func (fastpathT) DecSliceBoolY(v []bool, d *Decoder) (_ []bool, changed bool) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		if v == nil {
			return
		}
		return nil, true
	}
	if containerLenS == 0 {
		if v == nil {
			v = []bool{}
		} else if len(v) != 0 {
			v = v[:0]
		}
		slh.End()
		return v, true
	}
	hasLen := containerLenS > 0
	var xlen int
	if hasLen {
		if containerLenS > cap(v) {
			xlen = decInferLen(containerLenS, d.h.MaxInitLen, 1)
			if xlen <= cap(v) {
				v = v[:uint(xlen)]
			} else {
				v = make([]bool, uint(xlen))
			}
			changed = true
		} else if containerLenS != len(v) {
			v = v[:containerLenS]
			changed = true
		}
	}
	var j int
	for j = 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j == 0 && len(v) == 0 { // means hasLen == false
			xlen = decDefSliceCap
			v = make([]bool, uint(xlen))
			changed = true
		}
		if j >= len(v) {
			v = append(v, false)
			changed = true
		}
		slh.ElemContainerState(j)
		v[uint(j)] = d.d.DecodeBool()
	}
	if j < len(v) {
		v = v[:uint(j)]
		changed = true
	} else if j == 0 && v == nil {
		v = []bool{}
		changed = true
	}
	slh.End()
	return v, changed
}
func (fastpathT) DecSliceBoolN(v []bool, d *Decoder) {
	slh, containerLenS := d.decSliceHelperStart()
	if slh.IsNil {
		return
	}
	if containerLenS == 0 {
		slh.End()
		return
	}
	hasLen := containerLenS > 0
	for j := 0; (hasLen && j < containerLenS) || !(hasLen || d.checkBreak()); j++ {
		if j >= len(v) {
			decArrayCannotExpand(slh, hasLen, len(v), j, containerLenS)
			return
		}
		slh.ElemContainerState(j)
		v[uint(j)] = d.d.DecodeBool()
	}
	slh.End()
}
func (d *Decoder) fastpathDecMapStringIntfR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[string]interface{})
		if *vp == nil {
			*vp = make(map[string]interface{}, decInferLen(containerLen, d.h.MaxInitLen, 32))
		}
		if containerLen != 0 {
			fastpathTV.DecMapStringIntfL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapStringIntfL(rv2i(rv).(map[string]interface{}), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapStringIntfX(vp *map[string]interface{}, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[string]interface{}, decInferLen(containerLen, d.h.MaxInitLen, 32))
		}
		if containerLen != 0 {
			f.DecMapStringIntfL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapStringIntfL(v map[string]interface{}, containerLen int, d *Decoder) {
	mapGet := v != nil && !d.h.MapValueReset && !d.h.InterfaceReset
	var mk string
	var mv interface{}
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = string(d.d.DecodeStringAsBytes())
		d.mapElemValue()
		if mapGet {
			mv = v[mk]
		} else {
			mv = nil
		}
		d.decode(&mv)
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapStringStringR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[string]string)
		if *vp == nil {
			*vp = make(map[string]string, decInferLen(containerLen, d.h.MaxInitLen, 32))
		}
		if containerLen != 0 {
			fastpathTV.DecMapStringStringL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapStringStringL(rv2i(rv).(map[string]string), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapStringStringX(vp *map[string]string, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[string]string, decInferLen(containerLen, d.h.MaxInitLen, 32))
		}
		if containerLen != 0 {
			f.DecMapStringStringL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapStringStringL(v map[string]string, containerLen int, d *Decoder) {
	var mk string
	var mv string
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = string(d.d.DecodeStringAsBytes())
		d.mapElemValue()
		mv = string(d.d.DecodeStringAsBytes())
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapStringBytesR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[string][]byte)
		if *vp == nil {
			*vp = make(map[string][]byte, decInferLen(containerLen, d.h.MaxInitLen, 40))
		}
		if containerLen != 0 {
			fastpathTV.DecMapStringBytesL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapStringBytesL(rv2i(rv).(map[string][]byte), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapStringBytesX(vp *map[string][]byte, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[string][]byte, decInferLen(containerLen, d.h.MaxInitLen, 40))
		}
		if containerLen != 0 {
			f.DecMapStringBytesL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapStringBytesL(v map[string][]byte, containerLen int, d *Decoder) {
	mapGet := v != nil && !d.h.MapValueReset
	var mk string
	var mv []byte
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = string(d.d.DecodeStringAsBytes())
		d.mapElemValue()
		if mapGet {
			mv = v[mk]
		} else {
			mv = nil
		}
		mv = d.d.DecodeBytes(mv, false)
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapStringUint8R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[string]uint8)
		if *vp == nil {
			*vp = make(map[string]uint8, decInferLen(containerLen, d.h.MaxInitLen, 17))
		}
		if containerLen != 0 {
			fastpathTV.DecMapStringUint8L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapStringUint8L(rv2i(rv).(map[string]uint8), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapStringUint8X(vp *map[string]uint8, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[string]uint8, decInferLen(containerLen, d.h.MaxInitLen, 17))
		}
		if containerLen != 0 {
			f.DecMapStringUint8L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapStringUint8L(v map[string]uint8, containerLen int, d *Decoder) {
	var mk string
	var mv uint8
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = string(d.d.DecodeStringAsBytes())
		d.mapElemValue()
		mv = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapStringUint64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[string]uint64)
		if *vp == nil {
			*vp = make(map[string]uint64, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			fastpathTV.DecMapStringUint64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapStringUint64L(rv2i(rv).(map[string]uint64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapStringUint64X(vp *map[string]uint64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[string]uint64, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			f.DecMapStringUint64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapStringUint64L(v map[string]uint64, containerLen int, d *Decoder) {
	var mk string
	var mv uint64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = string(d.d.DecodeStringAsBytes())
		d.mapElemValue()
		mv = d.d.DecodeUint64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapStringIntR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[string]int)
		if *vp == nil {
			*vp = make(map[string]int, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			fastpathTV.DecMapStringIntL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapStringIntL(rv2i(rv).(map[string]int), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapStringIntX(vp *map[string]int, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[string]int, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			f.DecMapStringIntL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapStringIntL(v map[string]int, containerLen int, d *Decoder) {
	var mk string
	var mv int
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = string(d.d.DecodeStringAsBytes())
		d.mapElemValue()
		mv = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapStringInt64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[string]int64)
		if *vp == nil {
			*vp = make(map[string]int64, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			fastpathTV.DecMapStringInt64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapStringInt64L(rv2i(rv).(map[string]int64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapStringInt64X(vp *map[string]int64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[string]int64, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			f.DecMapStringInt64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapStringInt64L(v map[string]int64, containerLen int, d *Decoder) {
	var mk string
	var mv int64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = string(d.d.DecodeStringAsBytes())
		d.mapElemValue()
		mv = d.d.DecodeInt64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapStringFloat64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[string]float64)
		if *vp == nil {
			*vp = make(map[string]float64, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			fastpathTV.DecMapStringFloat64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapStringFloat64L(rv2i(rv).(map[string]float64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapStringFloat64X(vp *map[string]float64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[string]float64, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			f.DecMapStringFloat64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapStringFloat64L(v map[string]float64, containerLen int, d *Decoder) {
	var mk string
	var mv float64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = string(d.d.DecodeStringAsBytes())
		d.mapElemValue()
		mv = d.d.DecodeFloat64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapStringBoolR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[string]bool)
		if *vp == nil {
			*vp = make(map[string]bool, decInferLen(containerLen, d.h.MaxInitLen, 17))
		}
		if containerLen != 0 {
			fastpathTV.DecMapStringBoolL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapStringBoolL(rv2i(rv).(map[string]bool), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapStringBoolX(vp *map[string]bool, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[string]bool, decInferLen(containerLen, d.h.MaxInitLen, 17))
		}
		if containerLen != 0 {
			f.DecMapStringBoolL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapStringBoolL(v map[string]bool, containerLen int, d *Decoder) {
	var mk string
	var mv bool
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = string(d.d.DecodeStringAsBytes())
		d.mapElemValue()
		mv = d.d.DecodeBool()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint8IntfR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint8]interface{})
		if *vp == nil {
			*vp = make(map[uint8]interface{}, decInferLen(containerLen, d.h.MaxInitLen, 17))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint8IntfL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint8IntfL(rv2i(rv).(map[uint8]interface{}), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint8IntfX(vp *map[uint8]interface{}, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint8]interface{}, decInferLen(containerLen, d.h.MaxInitLen, 17))
		}
		if containerLen != 0 {
			f.DecMapUint8IntfL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint8IntfL(v map[uint8]interface{}, containerLen int, d *Decoder) {
	mapGet := v != nil && !d.h.MapValueReset && !d.h.InterfaceReset
	var mk uint8
	var mv interface{}
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		d.mapElemValue()
		if mapGet {
			mv = v[mk]
		} else {
			mv = nil
		}
		d.decode(&mv)
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint8StringR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint8]string)
		if *vp == nil {
			*vp = make(map[uint8]string, decInferLen(containerLen, d.h.MaxInitLen, 17))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint8StringL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint8StringL(rv2i(rv).(map[uint8]string), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint8StringX(vp *map[uint8]string, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint8]string, decInferLen(containerLen, d.h.MaxInitLen, 17))
		}
		if containerLen != 0 {
			f.DecMapUint8StringL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint8StringL(v map[uint8]string, containerLen int, d *Decoder) {
	var mk uint8
	var mv string
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		d.mapElemValue()
		mv = string(d.d.DecodeStringAsBytes())
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint8BytesR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint8][]byte)
		if *vp == nil {
			*vp = make(map[uint8][]byte, decInferLen(containerLen, d.h.MaxInitLen, 25))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint8BytesL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint8BytesL(rv2i(rv).(map[uint8][]byte), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint8BytesX(vp *map[uint8][]byte, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint8][]byte, decInferLen(containerLen, d.h.MaxInitLen, 25))
		}
		if containerLen != 0 {
			f.DecMapUint8BytesL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint8BytesL(v map[uint8][]byte, containerLen int, d *Decoder) {
	mapGet := v != nil && !d.h.MapValueReset
	var mk uint8
	var mv []byte
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		d.mapElemValue()
		if mapGet {
			mv = v[mk]
		} else {
			mv = nil
		}
		mv = d.d.DecodeBytes(mv, false)
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint8Uint8R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint8]uint8)
		if *vp == nil {
			*vp = make(map[uint8]uint8, decInferLen(containerLen, d.h.MaxInitLen, 2))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint8Uint8L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint8Uint8L(rv2i(rv).(map[uint8]uint8), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint8Uint8X(vp *map[uint8]uint8, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint8]uint8, decInferLen(containerLen, d.h.MaxInitLen, 2))
		}
		if containerLen != 0 {
			f.DecMapUint8Uint8L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint8Uint8L(v map[uint8]uint8, containerLen int, d *Decoder) {
	var mk uint8
	var mv uint8
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		d.mapElemValue()
		mv = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint8Uint64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint8]uint64)
		if *vp == nil {
			*vp = make(map[uint8]uint64, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint8Uint64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint8Uint64L(rv2i(rv).(map[uint8]uint64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint8Uint64X(vp *map[uint8]uint64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint8]uint64, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			f.DecMapUint8Uint64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint8Uint64L(v map[uint8]uint64, containerLen int, d *Decoder) {
	var mk uint8
	var mv uint64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		d.mapElemValue()
		mv = d.d.DecodeUint64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint8IntR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint8]int)
		if *vp == nil {
			*vp = make(map[uint8]int, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint8IntL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint8IntL(rv2i(rv).(map[uint8]int), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint8IntX(vp *map[uint8]int, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint8]int, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			f.DecMapUint8IntL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint8IntL(v map[uint8]int, containerLen int, d *Decoder) {
	var mk uint8
	var mv int
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		d.mapElemValue()
		mv = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint8Int64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint8]int64)
		if *vp == nil {
			*vp = make(map[uint8]int64, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint8Int64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint8Int64L(rv2i(rv).(map[uint8]int64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint8Int64X(vp *map[uint8]int64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint8]int64, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			f.DecMapUint8Int64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint8Int64L(v map[uint8]int64, containerLen int, d *Decoder) {
	var mk uint8
	var mv int64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		d.mapElemValue()
		mv = d.d.DecodeInt64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint8Float64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint8]float64)
		if *vp == nil {
			*vp = make(map[uint8]float64, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint8Float64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint8Float64L(rv2i(rv).(map[uint8]float64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint8Float64X(vp *map[uint8]float64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint8]float64, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			f.DecMapUint8Float64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint8Float64L(v map[uint8]float64, containerLen int, d *Decoder) {
	var mk uint8
	var mv float64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		d.mapElemValue()
		mv = d.d.DecodeFloat64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint8BoolR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint8]bool)
		if *vp == nil {
			*vp = make(map[uint8]bool, decInferLen(containerLen, d.h.MaxInitLen, 2))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint8BoolL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint8BoolL(rv2i(rv).(map[uint8]bool), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint8BoolX(vp *map[uint8]bool, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint8]bool, decInferLen(containerLen, d.h.MaxInitLen, 2))
		}
		if containerLen != 0 {
			f.DecMapUint8BoolL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint8BoolL(v map[uint8]bool, containerLen int, d *Decoder) {
	var mk uint8
	var mv bool
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		d.mapElemValue()
		mv = d.d.DecodeBool()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint64IntfR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint64]interface{})
		if *vp == nil {
			*vp = make(map[uint64]interface{}, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint64IntfL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint64IntfL(rv2i(rv).(map[uint64]interface{}), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint64IntfX(vp *map[uint64]interface{}, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint64]interface{}, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			f.DecMapUint64IntfL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint64IntfL(v map[uint64]interface{}, containerLen int, d *Decoder) {
	mapGet := v != nil && !d.h.MapValueReset && !d.h.InterfaceReset
	var mk uint64
	var mv interface{}
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeUint64()
		d.mapElemValue()
		if mapGet {
			mv = v[mk]
		} else {
			mv = nil
		}
		d.decode(&mv)
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint64StringR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint64]string)
		if *vp == nil {
			*vp = make(map[uint64]string, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint64StringL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint64StringL(rv2i(rv).(map[uint64]string), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint64StringX(vp *map[uint64]string, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint64]string, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			f.DecMapUint64StringL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint64StringL(v map[uint64]string, containerLen int, d *Decoder) {
	var mk uint64
	var mv string
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeUint64()
		d.mapElemValue()
		mv = string(d.d.DecodeStringAsBytes())
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint64BytesR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint64][]byte)
		if *vp == nil {
			*vp = make(map[uint64][]byte, decInferLen(containerLen, d.h.MaxInitLen, 32))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint64BytesL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint64BytesL(rv2i(rv).(map[uint64][]byte), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint64BytesX(vp *map[uint64][]byte, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint64][]byte, decInferLen(containerLen, d.h.MaxInitLen, 32))
		}
		if containerLen != 0 {
			f.DecMapUint64BytesL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint64BytesL(v map[uint64][]byte, containerLen int, d *Decoder) {
	mapGet := v != nil && !d.h.MapValueReset
	var mk uint64
	var mv []byte
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeUint64()
		d.mapElemValue()
		if mapGet {
			mv = v[mk]
		} else {
			mv = nil
		}
		mv = d.d.DecodeBytes(mv, false)
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint64Uint8R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint64]uint8)
		if *vp == nil {
			*vp = make(map[uint64]uint8, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint64Uint8L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint64Uint8L(rv2i(rv).(map[uint64]uint8), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint64Uint8X(vp *map[uint64]uint8, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint64]uint8, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			f.DecMapUint64Uint8L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint64Uint8L(v map[uint64]uint8, containerLen int, d *Decoder) {
	var mk uint64
	var mv uint8
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeUint64()
		d.mapElemValue()
		mv = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint64Uint64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint64]uint64)
		if *vp == nil {
			*vp = make(map[uint64]uint64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint64Uint64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint64Uint64L(rv2i(rv).(map[uint64]uint64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint64Uint64X(vp *map[uint64]uint64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint64]uint64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapUint64Uint64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint64Uint64L(v map[uint64]uint64, containerLen int, d *Decoder) {
	var mk uint64
	var mv uint64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeUint64()
		d.mapElemValue()
		mv = d.d.DecodeUint64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint64IntR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint64]int)
		if *vp == nil {
			*vp = make(map[uint64]int, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint64IntL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint64IntL(rv2i(rv).(map[uint64]int), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint64IntX(vp *map[uint64]int, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint64]int, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapUint64IntL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint64IntL(v map[uint64]int, containerLen int, d *Decoder) {
	var mk uint64
	var mv int
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeUint64()
		d.mapElemValue()
		mv = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint64Int64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint64]int64)
		if *vp == nil {
			*vp = make(map[uint64]int64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint64Int64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint64Int64L(rv2i(rv).(map[uint64]int64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint64Int64X(vp *map[uint64]int64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint64]int64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapUint64Int64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint64Int64L(v map[uint64]int64, containerLen int, d *Decoder) {
	var mk uint64
	var mv int64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeUint64()
		d.mapElemValue()
		mv = d.d.DecodeInt64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint64Float64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint64]float64)
		if *vp == nil {
			*vp = make(map[uint64]float64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint64Float64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint64Float64L(rv2i(rv).(map[uint64]float64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint64Float64X(vp *map[uint64]float64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint64]float64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapUint64Float64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint64Float64L(v map[uint64]float64, containerLen int, d *Decoder) {
	var mk uint64
	var mv float64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeUint64()
		d.mapElemValue()
		mv = d.d.DecodeFloat64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapUint64BoolR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[uint64]bool)
		if *vp == nil {
			*vp = make(map[uint64]bool, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			fastpathTV.DecMapUint64BoolL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapUint64BoolL(rv2i(rv).(map[uint64]bool), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapUint64BoolX(vp *map[uint64]bool, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[uint64]bool, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			f.DecMapUint64BoolL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapUint64BoolL(v map[uint64]bool, containerLen int, d *Decoder) {
	var mk uint64
	var mv bool
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeUint64()
		d.mapElemValue()
		mv = d.d.DecodeBool()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapIntIntfR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int]interface{})
		if *vp == nil {
			*vp = make(map[int]interface{}, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			fastpathTV.DecMapIntIntfL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapIntIntfL(rv2i(rv).(map[int]interface{}), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapIntIntfX(vp *map[int]interface{}, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int]interface{}, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			f.DecMapIntIntfL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapIntIntfL(v map[int]interface{}, containerLen int, d *Decoder) {
	mapGet := v != nil && !d.h.MapValueReset && !d.h.InterfaceReset
	var mk int
	var mv interface{}
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		d.mapElemValue()
		if mapGet {
			mv = v[mk]
		} else {
			mv = nil
		}
		d.decode(&mv)
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapIntStringR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int]string)
		if *vp == nil {
			*vp = make(map[int]string, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			fastpathTV.DecMapIntStringL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapIntStringL(rv2i(rv).(map[int]string), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapIntStringX(vp *map[int]string, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int]string, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			f.DecMapIntStringL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapIntStringL(v map[int]string, containerLen int, d *Decoder) {
	var mk int
	var mv string
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		d.mapElemValue()
		mv = string(d.d.DecodeStringAsBytes())
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapIntBytesR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int][]byte)
		if *vp == nil {
			*vp = make(map[int][]byte, decInferLen(containerLen, d.h.MaxInitLen, 32))
		}
		if containerLen != 0 {
			fastpathTV.DecMapIntBytesL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapIntBytesL(rv2i(rv).(map[int][]byte), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapIntBytesX(vp *map[int][]byte, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int][]byte, decInferLen(containerLen, d.h.MaxInitLen, 32))
		}
		if containerLen != 0 {
			f.DecMapIntBytesL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapIntBytesL(v map[int][]byte, containerLen int, d *Decoder) {
	mapGet := v != nil && !d.h.MapValueReset
	var mk int
	var mv []byte
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		d.mapElemValue()
		if mapGet {
			mv = v[mk]
		} else {
			mv = nil
		}
		mv = d.d.DecodeBytes(mv, false)
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapIntUint8R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int]uint8)
		if *vp == nil {
			*vp = make(map[int]uint8, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			fastpathTV.DecMapIntUint8L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapIntUint8L(rv2i(rv).(map[int]uint8), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapIntUint8X(vp *map[int]uint8, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int]uint8, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			f.DecMapIntUint8L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapIntUint8L(v map[int]uint8, containerLen int, d *Decoder) {
	var mk int
	var mv uint8
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		d.mapElemValue()
		mv = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapIntUint64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int]uint64)
		if *vp == nil {
			*vp = make(map[int]uint64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapIntUint64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapIntUint64L(rv2i(rv).(map[int]uint64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapIntUint64X(vp *map[int]uint64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int]uint64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapIntUint64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapIntUint64L(v map[int]uint64, containerLen int, d *Decoder) {
	var mk int
	var mv uint64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		d.mapElemValue()
		mv = d.d.DecodeUint64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapIntIntR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int]int)
		if *vp == nil {
			*vp = make(map[int]int, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapIntIntL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapIntIntL(rv2i(rv).(map[int]int), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapIntIntX(vp *map[int]int, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int]int, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapIntIntL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapIntIntL(v map[int]int, containerLen int, d *Decoder) {
	var mk int
	var mv int
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		d.mapElemValue()
		mv = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapIntInt64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int]int64)
		if *vp == nil {
			*vp = make(map[int]int64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapIntInt64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapIntInt64L(rv2i(rv).(map[int]int64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapIntInt64X(vp *map[int]int64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int]int64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapIntInt64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapIntInt64L(v map[int]int64, containerLen int, d *Decoder) {
	var mk int
	var mv int64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		d.mapElemValue()
		mv = d.d.DecodeInt64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapIntFloat64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int]float64)
		if *vp == nil {
			*vp = make(map[int]float64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapIntFloat64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapIntFloat64L(rv2i(rv).(map[int]float64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapIntFloat64X(vp *map[int]float64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int]float64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapIntFloat64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapIntFloat64L(v map[int]float64, containerLen int, d *Decoder) {
	var mk int
	var mv float64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		d.mapElemValue()
		mv = d.d.DecodeFloat64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapIntBoolR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int]bool)
		if *vp == nil {
			*vp = make(map[int]bool, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			fastpathTV.DecMapIntBoolL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapIntBoolL(rv2i(rv).(map[int]bool), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapIntBoolX(vp *map[int]bool, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int]bool, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			f.DecMapIntBoolL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapIntBoolL(v map[int]bool, containerLen int, d *Decoder) {
	var mk int
	var mv bool
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		d.mapElemValue()
		mv = d.d.DecodeBool()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapInt64IntfR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int64]interface{})
		if *vp == nil {
			*vp = make(map[int64]interface{}, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			fastpathTV.DecMapInt64IntfL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapInt64IntfL(rv2i(rv).(map[int64]interface{}), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapInt64IntfX(vp *map[int64]interface{}, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int64]interface{}, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			f.DecMapInt64IntfL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapInt64IntfL(v map[int64]interface{}, containerLen int, d *Decoder) {
	mapGet := v != nil && !d.h.MapValueReset && !d.h.InterfaceReset
	var mk int64
	var mv interface{}
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeInt64()
		d.mapElemValue()
		if mapGet {
			mv = v[mk]
		} else {
			mv = nil
		}
		d.decode(&mv)
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapInt64StringR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int64]string)
		if *vp == nil {
			*vp = make(map[int64]string, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			fastpathTV.DecMapInt64StringL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapInt64StringL(rv2i(rv).(map[int64]string), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapInt64StringX(vp *map[int64]string, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int64]string, decInferLen(containerLen, d.h.MaxInitLen, 24))
		}
		if containerLen != 0 {
			f.DecMapInt64StringL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapInt64StringL(v map[int64]string, containerLen int, d *Decoder) {
	var mk int64
	var mv string
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeInt64()
		d.mapElemValue()
		mv = string(d.d.DecodeStringAsBytes())
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapInt64BytesR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int64][]byte)
		if *vp == nil {
			*vp = make(map[int64][]byte, decInferLen(containerLen, d.h.MaxInitLen, 32))
		}
		if containerLen != 0 {
			fastpathTV.DecMapInt64BytesL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapInt64BytesL(rv2i(rv).(map[int64][]byte), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapInt64BytesX(vp *map[int64][]byte, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int64][]byte, decInferLen(containerLen, d.h.MaxInitLen, 32))
		}
		if containerLen != 0 {
			f.DecMapInt64BytesL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapInt64BytesL(v map[int64][]byte, containerLen int, d *Decoder) {
	mapGet := v != nil && !d.h.MapValueReset
	var mk int64
	var mv []byte
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeInt64()
		d.mapElemValue()
		if mapGet {
			mv = v[mk]
		} else {
			mv = nil
		}
		mv = d.d.DecodeBytes(mv, false)
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapInt64Uint8R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int64]uint8)
		if *vp == nil {
			*vp = make(map[int64]uint8, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			fastpathTV.DecMapInt64Uint8L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapInt64Uint8L(rv2i(rv).(map[int64]uint8), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapInt64Uint8X(vp *map[int64]uint8, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int64]uint8, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			f.DecMapInt64Uint8L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapInt64Uint8L(v map[int64]uint8, containerLen int, d *Decoder) {
	var mk int64
	var mv uint8
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeInt64()
		d.mapElemValue()
		mv = uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapInt64Uint64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int64]uint64)
		if *vp == nil {
			*vp = make(map[int64]uint64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapInt64Uint64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapInt64Uint64L(rv2i(rv).(map[int64]uint64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapInt64Uint64X(vp *map[int64]uint64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int64]uint64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapInt64Uint64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapInt64Uint64L(v map[int64]uint64, containerLen int, d *Decoder) {
	var mk int64
	var mv uint64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeInt64()
		d.mapElemValue()
		mv = d.d.DecodeUint64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapInt64IntR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int64]int)
		if *vp == nil {
			*vp = make(map[int64]int, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapInt64IntL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapInt64IntL(rv2i(rv).(map[int64]int), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapInt64IntX(vp *map[int64]int, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int64]int, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapInt64IntL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapInt64IntL(v map[int64]int, containerLen int, d *Decoder) {
	var mk int64
	var mv int
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeInt64()
		d.mapElemValue()
		mv = int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapInt64Int64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int64]int64)
		if *vp == nil {
			*vp = make(map[int64]int64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapInt64Int64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapInt64Int64L(rv2i(rv).(map[int64]int64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapInt64Int64X(vp *map[int64]int64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int64]int64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapInt64Int64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapInt64Int64L(v map[int64]int64, containerLen int, d *Decoder) {
	var mk int64
	var mv int64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeInt64()
		d.mapElemValue()
		mv = d.d.DecodeInt64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapInt64Float64R(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int64]float64)
		if *vp == nil {
			*vp = make(map[int64]float64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			fastpathTV.DecMapInt64Float64L(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapInt64Float64L(rv2i(rv).(map[int64]float64), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapInt64Float64X(vp *map[int64]float64, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int64]float64, decInferLen(containerLen, d.h.MaxInitLen, 16))
		}
		if containerLen != 0 {
			f.DecMapInt64Float64L(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapInt64Float64L(v map[int64]float64, containerLen int, d *Decoder) {
	var mk int64
	var mv float64
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeInt64()
		d.mapElemValue()
		mv = d.d.DecodeFloat64()
		if v != nil {
			v[mk] = mv
		}
	}
}
func (d *Decoder) fastpathDecMapInt64BoolR(f *codecFnInfo, rv reflect.Value) {
	containerLen := d.mapStart()
	if rv.Kind() == reflect.Ptr {
		vp, _ := rv2i(rv).(*map[int64]bool)
		if *vp == nil {
			*vp = make(map[int64]bool, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			fastpathTV.DecMapInt64BoolL(*vp, containerLen, d)
		}
	} else if containerLen != 0 {
		fastpathTV.DecMapInt64BoolL(rv2i(rv).(map[int64]bool), containerLen, d)
	}
	d.mapEnd()
}
func (f fastpathT) DecMapInt64BoolX(vp *map[int64]bool, d *Decoder) {
	containerLen := d.mapStart()
	if containerLen == containerLenNil {
		*vp = nil
	} else {
		if *vp == nil {
			*vp = make(map[int64]bool, decInferLen(containerLen, d.h.MaxInitLen, 9))
		}
		if containerLen != 0 {
			f.DecMapInt64BoolL(*vp, containerLen, d)
		}
		d.mapEnd()
	}
}
func (fastpathT) DecMapInt64BoolL(v map[int64]bool, containerLen int, d *Decoder) {
	var mk int64
	var mv bool
	hasLen := containerLen > 0
	for j := 0; (hasLen && j < containerLen) || !(hasLen || d.checkBreak()); j++ {
		d.mapElemKey()
		mk = d.d.DecodeInt64()
		d.mapElemValue()
		mv = d.d.DecodeBool()
		if v != nil {
			v[mk] = mv
		}
	}
}
