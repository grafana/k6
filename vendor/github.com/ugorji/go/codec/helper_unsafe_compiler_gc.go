// +build !safe
// +build !appengine
// +build go1.7
// +build gc

// Copyright (c) 2012-2020 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import (
	"reflect"
	"unsafe"
)

func rvType(rv reflect.Value) reflect.Type {
	return rvPtrToType(((*unsafeReflectValue)(unsafe.Pointer(&rv))).typ) // rv.Type()
}

//go:linkname unsafeZeroArr runtime.zeroVal
var unsafeZeroArr [1024]byte

//go:linkname rvPtrToType reflect.toType
//go:noescape
func rvPtrToType(typ unsafe.Pointer) reflect.Type
