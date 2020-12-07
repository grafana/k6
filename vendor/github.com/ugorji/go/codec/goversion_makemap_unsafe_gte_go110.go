// +build go1.10
// +build !safe
// +build !appengine

// Copyright (c) 2012-2020 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import (
	"reflect"
	"unsafe"
)

func makeMapReflect(typ reflect.Type, size int) (rv reflect.Value) {
	t := (*unsafeIntf)(unsafe.Pointer(&typ)).word
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	urv.typ = t
	urv.flag = uintptr(reflect.Map)
	urv.ptr = makemap(t, size, nil)
	return
}

//go:linkname makemap runtime.makemap
//go:noescape
func makemap(typ unsafe.Pointer, size int, h unsafe.Pointer) unsafe.Pointer
