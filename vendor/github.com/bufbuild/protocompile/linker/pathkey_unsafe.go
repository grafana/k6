// Copyright 2020-2024 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !appengine && !gopherjs && !purego
// +build !appengine,!gopherjs,!purego

// NB: other environments where unsafe is inappropriate should use "purego" build tag
// https://github.com/golang/go/issues/23172

package linker

import (
	"reflect"
	"unsafe"

	"google.golang.org/protobuf/reflect/protoreflect"
)

var pathElementType = reflect.TypeOf(protoreflect.SourcePath{}).Elem()

func pathKey(p protoreflect.SourcePath) interface{} {
	if p == nil {
		// Reflection code below doesn't work with nil slices
		return [0]int32{}
	}
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(reflect.ValueOf(&p).Pointer()))
	array := reflect.NewAt(reflect.ArrayOf(hdr.Len, pathElementType), unsafe.Pointer(hdr.Data))
	return array.Elem().Interface()
}
