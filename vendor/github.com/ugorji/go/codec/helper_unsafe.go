// +build !safe
// +build !appengine
// +build go1.7

// Copyright (c) 2012-2020 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import (
	"reflect"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"
)

// This file has unsafe variants of some helper methods.
// MARKER: See helper_not_unsafe.go for the usage information.

// For reflect.Value code, we decided to do the following:
//    - if we know the kind, we can elide conditional checks for
//      - SetXXX (Int, Uint, String, Bool, etc)
//      - SetLen
//
// We can also optimize
//      - IsNil

const safeMode = false

// helperUnsafeCopyMapEntry says that we should copy the pointer in the map
// to another value during mapRange/iteration and mapGet calls.
//
// The only callers of mapRange/iteration is encode. Here, we just walk through the values
// and encode them

// The only caller of mapGet is decode. Here, it does a Get if the underlying value is a pointer,
// and decodes into that.

// For both users, we are very careful NOT to modify or keep the pointers around.
// Consequently, it is ok for take advantage of the performance that the map is not modified
// during an iteration and we can just "peek" at the internal value" in the map and use it.
const helperUnsafeCopyMapEntry = false

// MARKER: keep in sync with GO_ROOT/src/reflect/value.go
const (
	unsafeFlagStickyRO = 1 << 5
	unsafeFlagEmbedRO  = 1 << 6
	unsafeFlagIndir    = 1 << 7
	unsafeFlagAddr     = 1 << 8
	unsafeFlagKindMask = (1 << 5) - 1 // 5 bits for 27 kinds (up to 31)
	// unsafeTypeKindDirectIface = 1 << 5
)

type unsafeString struct {
	Data unsafe.Pointer
	Len  int
}

type unsafeSlice struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

type unsafeIntf struct {
	typ  unsafe.Pointer
	word unsafe.Pointer
}

type unsafeReflectValue struct {
	typ  unsafe.Pointer
	ptr  unsafe.Pointer
	flag uintptr
}

var unsafeZeroSlice = unsafeSlice{unsafe.Pointer(&unsafeZeroArr[0]), 0, 0}

func stringView(v []byte) string {
	return *(*string)(unsafe.Pointer(&v))
}

func bytesView(v string) (b []byte) {
	sx := (*unsafeString)(unsafe.Pointer(&v))
	bx := (*unsafeSlice)(unsafe.Pointer(&b))
	bx.Data, bx.Len, bx.Cap = sx.Data, sx.Len, sx.Len
	return
}

func isNil(v interface{}) (rv reflect.Value, isnil bool) {
	var ui = (*unsafeIntf)(unsafe.Pointer(&v))
	if ui.word == nil {
		isnil = true
		return
	}
	rv = rv4i(v) // reflect.value is cheap and inline'able
	tk := rv.Kind()
	isnil = (tk == reflect.Interface || tk == reflect.Slice) && *(*unsafe.Pointer)(ui.word) == nil
	return
}

// return the pointer for a reference (map/chan/func/pointer/unsafe.Pointer).
// true references (map, func, chan, ptr - NOT slice) may be double-referenced? as flagIndir
func rvRefPtr(v *unsafeReflectValue) unsafe.Pointer {
	if v.flag&unsafeFlagIndir != 0 {
		return *(*unsafe.Pointer)(v.ptr)
	}
	return v.ptr
}

func rv2ptr(urv *unsafeReflectValue) unsafe.Pointer {
	if refBitset.isset(byte(urv.flag&unsafeFlagKindMask)) && urv.flag&unsafeFlagIndir != 0 {
		return *(*unsafe.Pointer)(urv.ptr)
	}
	return urv.ptr
}

// func rvAddr(rv reflect.Value) uintptr {
// 	return uintptr((*unsafeReflectValue)(unsafe.Pointer(&rv)).ptr)
// }

func eq4i(i0, i1 interface{}) bool {
	v0 := (*unsafeIntf)(unsafe.Pointer(&i0))
	v1 := (*unsafeIntf)(unsafe.Pointer(&i1))
	return v0.typ == v1.typ && v0.word == v1.word
}

func rv4i(i interface{}) (rv reflect.Value) {
	// Unfortunately, we cannot get the "kind" of the interface directly here.
	// We need the 'rtype', whose structure changes in different go versions.
	// Finally, it's not clear that there is benefit to reimplementing it,
	// as the "escapes(i)" is not clearly expensive since we want i to exist on the heap.

	return reflect.ValueOf(i)
}

func rv2i(rv reflect.Value) interface{} {
	// We tap into implememtation details from
	// the source go stdlib reflect/value.go, and trims the implementation.
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*interface{})(unsafe.Pointer(&unsafeIntf{typ: urv.typ, word: rv2ptr(urv)}))
}

func rvIsNil(rv reflect.Value) bool {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	if urv.flag&unsafeFlagIndir != 0 {
		return *(*unsafe.Pointer)(urv.ptr) == nil
	}
	return urv.ptr == nil
}

func rvSetSliceLen(rv reflect.Value, length int) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	(*unsafeString)(urv.ptr).Len = length
}

func rvZeroAddrK(t reflect.Type, k reflect.Kind) (rv reflect.Value) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	urv.flag = uintptr(k) | unsafeFlagIndir | unsafeFlagAddr
	urv.typ = ((*unsafeIntf)(unsafe.Pointer(&t))).word
	urv.ptr = unsafe_New(urv.typ)
	return
}

func rvZeroAddr(t reflect.Type) reflect.Value {
	return rvZeroAddrK(t, t.Kind())
}

func rvZeroK(t reflect.Type, k reflect.Kind) (rv reflect.Value) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	urv.typ = ((*unsafeIntf)(unsafe.Pointer(&t))).word
	if refBitset.isset(byte(k)) {
		urv.flag = uintptr(k)
	} else if (k == reflect.Struct || k == reflect.Array) && t.Size() > uintptr(len(unsafeZeroArr)) {
		urv.flag = uintptr(k) | unsafeFlagIndir | unsafeFlagAddr
		urv.ptr = unsafe_New(urv.typ)
	} else {
		urv.flag = uintptr(k) | unsafeFlagIndir
		urv.ptr = unsafe.Pointer(&unsafeZeroArr[0])
	}
	return
}

func rvZero(t reflect.Type) reflect.Value {
	return rvZeroK(t, t.Kind())
}

func rvConvert(v reflect.Value, t reflect.Type) (rv reflect.Value) {
	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*urv = *uv
	urv.typ = ((*unsafeIntf)(unsafe.Pointer(&t))).word
	return
}

func rt2id(rt reflect.Type) uintptr {
	return uintptr(((*unsafeIntf)(unsafe.Pointer(&rt))).word)
}

func i2rtid(i interface{}) uintptr {
	return uintptr(((*unsafeIntf)(unsafe.Pointer(&i))).typ)
}

// --------------------------

func unsafeCmpZero(ptr unsafe.Pointer, size int) bool {
	var s1, s2 string
	s1u := (*unsafeString)(unsafe.Pointer(&s1))
	s2u := (*unsafeString)(unsafe.Pointer(&s2))
	s1u.Data, s1u.Len, s2u.Len = ptr, size, size
	if size <= len(unsafeZeroArr) {
		s2u.Data = unsafe.Pointer(&unsafeZeroArr[0])
	} else {
		arr := make([]byte, size)
		s2u.Data = unsafe.Pointer(&arr[0])
	}
	return s1 == s2 // memcmp
}

func isEmptyValue(v reflect.Value, tinfos *TypeInfos, recursive bool) bool {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	if urv.flag == 0 {
		return true
	}
	if recursive {
		return isEmptyValueFallbackRecur(urv, v, tinfos)
	}
	// t := rvPtrToType(urv.typ)
	// // it is empty if it is a zero value OR it points to a zero value
	// if urv.flag&unsafeFlagIndir == 0 { // this is a pointer
	// 	if urv.ptr == nil {
	// 		return true
	// 	}
	// 	return unsafeCmpZero(*(*unsafe.Pointer)(urv.ptr), int(t.Elem().Size()))
	// }
	// return unsafeCmpZero(urv.ptr, int(t.Size()))
	// return unsafeCmpZero(urv.ptr, int(rvPtrToType(urv.typ).Size()))

	return unsafeCmpZero(urv.ptr, int(rvType(v).Size()))
}

func isEmptyValueFallbackRecur(urv *unsafeReflectValue, v reflect.Value, tinfos *TypeInfos) bool {
	const recursive = true

	switch v.Kind() {
	case reflect.Invalid:
		return true
	case reflect.String:
		return (*unsafeString)(urv.ptr).Len == 0
	case reflect.Slice:
		return (*unsafeSlice)(urv.ptr).Len == 0
	case reflect.Bool:
		return !*(*bool)(urv.ptr)
	case reflect.Int:
		return *(*int)(urv.ptr) == 0
	case reflect.Int8:
		return *(*int8)(urv.ptr) == 0
	case reflect.Int16:
		return *(*int16)(urv.ptr) == 0
	case reflect.Int32:
		return *(*int32)(urv.ptr) == 0
	case reflect.Int64:
		return *(*int64)(urv.ptr) == 0
	case reflect.Uint:
		return *(*uint)(urv.ptr) == 0
	case reflect.Uint8:
		return *(*uint8)(urv.ptr) == 0
	case reflect.Uint16:
		return *(*uint16)(urv.ptr) == 0
	case reflect.Uint32:
		return *(*uint32)(urv.ptr) == 0
	case reflect.Uint64:
		return *(*uint64)(urv.ptr) == 0
	case reflect.Uintptr:
		return *(*uintptr)(urv.ptr) == 0
	case reflect.Float32:
		return *(*float32)(urv.ptr) == 0
	case reflect.Float64:
		return *(*float64)(urv.ptr) == 0
	case reflect.Complex64:
		return unsafeCmpZero(urv.ptr, 8)
	case reflect.Complex128:
		return unsafeCmpZero(urv.ptr, 16)
	case reflect.Struct:
		return isEmptyStruct(v, tinfos, recursive)
	case reflect.Interface, reflect.Ptr:
		// isnil := urv.ptr == nil // (not sufficient, as a pointer value encodes the type)
		isnil := urv.ptr == nil || *(*unsafe.Pointer)(urv.ptr) == nil
		if recursive && !isnil {
			return isEmptyValue(v.Elem(), tinfos, recursive)
		}
		return isnil
	case reflect.UnsafePointer:
		return urv.ptr == nil || *(*unsafe.Pointer)(urv.ptr) == nil
	case reflect.Chan:
		return urv.ptr == nil || chanlen(rvRefPtr(urv)) == 0
	case reflect.Map:
		return urv.ptr == nil || maplen(rvRefPtr(urv)) == 0
	case reflect.Array:
		return rvLenArray(v) == 0
	}
	return false
}

// --------------------------

// atomicXXX is expected to be 2 words (for symmetry with atomic.Value)
//
// Note that we do not atomically load/store length and data pointer separately,
// as this could lead to some races. Instead, we atomically load/store cappedSlice.
//
// Note: with atomic.(Load|Store)Pointer, we MUST work with an unsafe.Pointer directly.

// ----------------------
type atomicTypeInfoSlice struct {
	v unsafe.Pointer // *[]rtid2ti
	// _ uint64         // padding (atomicXXX expected to be 2 words)
}

func (x *atomicTypeInfoSlice) load() (s []rtid2ti) {
	x2 := atomic.LoadPointer(&x.v)
	if x2 != nil {
		s = *(*[]rtid2ti)(x2)
	}
	return
}

func (x *atomicTypeInfoSlice) store(p []rtid2ti) {
	atomic.StorePointer(&x.v, unsafe.Pointer(&p))
}

// MARKER: in safe mode, atomicXXX are atomic.Value, which contains an interface{}.
// This is 2 words.
// consider padding atomicXXX here with a uintptr, so they fit into 2 words also.

// --------------------------
type atomicRtidFnSlice struct {
	v unsafe.Pointer // *[]codecRtidFn
}

func (x *atomicRtidFnSlice) load() (s []codecRtidFn) {
	x2 := atomic.LoadPointer(&x.v)
	if x2 != nil {
		s = *(*[]codecRtidFn)(x2)
	}
	return
}

func (x *atomicRtidFnSlice) store(p []codecRtidFn) {
	atomic.StorePointer(&x.v, unsafe.Pointer(&p))
}

// --------------------------
type atomicClsErr struct {
	v unsafe.Pointer // *clsErr
}

func (x *atomicClsErr) load() (e clsErr) {
	x2 := (*clsErr)(atomic.LoadPointer(&x.v))
	if x2 != nil {
		e = *x2
	}
	return
}

func (x *atomicClsErr) store(p clsErr) {
	atomic.StorePointer(&x.v, unsafe.Pointer(&p))
}

// --------------------------

// to create a reflect.Value for each member field of fauxUnion,
// we first create a global fauxUnion, and create reflect.Value
// for them all.
// This way, we have the flags and type in the reflect.Value.
// Then, when a reflect.Value is called, we just copy it,
// update the ptr to the fauxUnion's, and return it.

type unsafeDecNakedWrapper struct {
	fauxUnion
	ru, ri, rf, rl, rs, rb, rt reflect.Value // mapping to the primitives above
}

func (n *unsafeDecNakedWrapper) init() {
	n.ru = rv4i(&n.u).Elem()
	n.ri = rv4i(&n.i).Elem()
	n.rf = rv4i(&n.f).Elem()
	n.rl = rv4i(&n.l).Elem()
	n.rs = rv4i(&n.s).Elem()
	n.rt = rv4i(&n.t).Elem()
	n.rb = rv4i(&n.b).Elem()
	// n.rr[] = rv4i(&n.)
}

var defUnsafeDecNakedWrapper unsafeDecNakedWrapper

func init() {
	defUnsafeDecNakedWrapper.init()
}

func (n *fauxUnion) ru() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.ru
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.u)
	return
}
func (n *fauxUnion) ri() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.ri
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.i)
	return
}
func (n *fauxUnion) rf() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.rf
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.f)
	return
}
func (n *fauxUnion) rl() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.rl
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.l)
	return
}
func (n *fauxUnion) rs() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.rs
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.s)
	return
}
func (n *fauxUnion) rt() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.rt
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.t)
	return
}
func (n *fauxUnion) rb() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.rb
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.b)
	return
}

// --------------------------
func rvSetBytes(rv reflect.Value, v []byte) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*[]byte)(urv.ptr) = v
}

func rvSetString(rv reflect.Value, v string) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*string)(urv.ptr) = v
}

func rvSetBool(rv reflect.Value, v bool) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*bool)(urv.ptr) = v
}

func rvSetTime(rv reflect.Value, v time.Time) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*time.Time)(urv.ptr) = v
}

func rvSetFloat32(rv reflect.Value, v float32) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*float32)(urv.ptr) = v
}

func rvSetFloat64(rv reflect.Value, v float64) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*float64)(urv.ptr) = v
}

func rvSetInt(rv reflect.Value, v int) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*int)(urv.ptr) = v
}

func rvSetInt8(rv reflect.Value, v int8) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*int8)(urv.ptr) = v
}

func rvSetInt16(rv reflect.Value, v int16) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*int16)(urv.ptr) = v
}

func rvSetInt32(rv reflect.Value, v int32) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*int32)(urv.ptr) = v
}

func rvSetInt64(rv reflect.Value, v int64) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*int64)(urv.ptr) = v
}

func rvSetUint(rv reflect.Value, v uint) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uint)(urv.ptr) = v
}

func rvSetUintptr(rv reflect.Value, v uintptr) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uintptr)(urv.ptr) = v
}

func rvSetUint8(rv reflect.Value, v uint8) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uint8)(urv.ptr) = v
}

func rvSetUint16(rv reflect.Value, v uint16) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uint16)(urv.ptr) = v
}

func rvSetUint32(rv reflect.Value, v uint32) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uint32)(urv.ptr) = v
}

func rvSetUint64(rv reflect.Value, v uint64) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uint64)(urv.ptr) = v
}

// ----------------

// rvSetDirect is rv.Set for all kinds except reflect.Interface
func rvSetDirect(rv reflect.Value, v reflect.Value) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	if uv.flag&unsafeFlagIndir == 0 {
		*(*unsafe.Pointer)(urv.ptr) = uv.ptr
	} else if uv.ptr == unsafe.Pointer(&unsafeZeroArr[0]) {
		typedmemclr(urv.typ, urv.ptr)
	} else {
		typedmemmove(urv.typ, urv.ptr, uv.ptr)
	}
}

func rvSetDirectZero(rv reflect.Value) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	typedmemclr(urv.typ, urv.ptr)
}

// rvSlice returns a slice of the slice of lenth
func rvSlice(rv reflect.Value, length int) (v reflect.Value) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	*uv = *urv
	var x []unsafe.Pointer
	uv.ptr = unsafe.Pointer(&x)
	*(*unsafeSlice)(uv.ptr) = *(*unsafeSlice)(urv.ptr)
	(*unsafeSlice)(uv.ptr).Len = length
	return
}

// ------------

func rvSliceIndex(rv reflect.Value, i int, ti *typeInfo) (v reflect.Value) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	uv.ptr = unsafe.Pointer(uintptr(((*unsafeSlice)(urv.ptr)).Data) + uintptr(int(ti.elemsize)*i))
	uv.typ = ((*unsafeIntf)(unsafe.Pointer(&ti.elem))).word
	uv.flag = uintptr(ti.elemkind) | unsafeFlagIndir | unsafeFlagAddr
	return
}

func rvSliceZeroCap(t reflect.Type) (v reflect.Value) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	urv.typ = ((*unsafeIntf)(unsafe.Pointer(&t))).word
	urv.flag = uintptr(reflect.Slice) | unsafeFlagIndir
	urv.ptr = unsafe.Pointer(&unsafeZeroSlice)
	return
}

func rvLenSlice(rv reflect.Value) int {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return (*unsafeSlice)(urv.ptr).Len
}

func rvCapSlice(rv reflect.Value) int {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return (*unsafeSlice)(urv.ptr).Cap
}

func rvGetArrayBytesRO(rv reflect.Value, scratch []byte) (bs []byte) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	bx := (*unsafeSlice)(unsafe.Pointer(&bs))
	bx.Data = urv.ptr
	bx.Len = rvLenArray(rv)
	bx.Cap = bx.Len
	return
}

func rvGetArray4Slice(rv reflect.Value) (v reflect.Value) {
	// It is possible that this slice is based off an array with a larger
	// len that we want (where array len == slice cap).
	// However, it is ok to create an array type that is a subset of the full
	// e.g. full slice is based off a *[16]byte, but we can create a *[4]byte
	// off of it. That is ok.
	//
	// Consequently, we use rvLenSlice, not rvCapSlice.

	t := reflectArrayOf(rvLenSlice(rv), rvType(rv).Elem())
	// v = rvZeroAddrK(t, reflect.Array)

	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	uv.flag = uintptr(reflect.Array) | unsafeFlagIndir | unsafeFlagAddr
	uv.typ = ((*unsafeIntf)(unsafe.Pointer(&t))).word

	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	uv.ptr = *(*unsafe.Pointer)(urv.ptr) // slice rv has a ptr to the slice.

	return
}

func rvGetSlice4Array(rv reflect.Value, tslice reflect.Type) (v reflect.Value) {
	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))

	var x []unsafe.Pointer

	uv.ptr = unsafe.Pointer(&x)
	uv.typ = ((*unsafeIntf)(unsafe.Pointer(&tslice))).word
	uv.flag = unsafeFlagIndir | uintptr(reflect.Slice)

	s := (*unsafeSlice)(uv.ptr)
	s.Data = urv.ptr
	s.Len = rvLenArray(rv)
	s.Cap = s.Len
	return
}

func rvCopySlice(dest, src reflect.Value) {
	t := rvType(dest).Elem()
	urv := (*unsafeReflectValue)(unsafe.Pointer(&dest))
	destPtr := urv.ptr
	urv = (*unsafeReflectValue)(unsafe.Pointer(&src))
	typedslicecopy((*unsafeIntf)(unsafe.Pointer(&t)).word,
		*(*unsafeSlice)(destPtr), *(*unsafeSlice)(urv.ptr))
}

// ------------

func rvGetBool(rv reflect.Value) bool {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*bool)(v.ptr)
}

func rvGetBytes(rv reflect.Value) []byte {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*[]byte)(v.ptr)
}

func rvGetTime(rv reflect.Value) time.Time {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*time.Time)(v.ptr)
}

func rvGetString(rv reflect.Value) string {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*string)(v.ptr)
}

func rvGetFloat64(rv reflect.Value) float64 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*float64)(v.ptr)
}

func rvGetFloat32(rv reflect.Value) float32 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*float32)(v.ptr)
}

func rvGetInt(rv reflect.Value) int {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*int)(v.ptr)
}

func rvGetInt8(rv reflect.Value) int8 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*int8)(v.ptr)
}

func rvGetInt16(rv reflect.Value) int16 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*int16)(v.ptr)
}

func rvGetInt32(rv reflect.Value) int32 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*int32)(v.ptr)
}

func rvGetInt64(rv reflect.Value) int64 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*int64)(v.ptr)
}

func rvGetUint(rv reflect.Value) uint {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uint)(v.ptr)
}

func rvGetUint8(rv reflect.Value) uint8 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uint8)(v.ptr)
}

func rvGetUint16(rv reflect.Value) uint16 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uint16)(v.ptr)
}

func rvGetUint32(rv reflect.Value) uint32 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uint32)(v.ptr)
}

func rvGetUint64(rv reflect.Value) uint64 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uint64)(v.ptr)
}

func rvGetUintptr(rv reflect.Value) uintptr {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uintptr)(v.ptr)
}

func rvLenMap(rv reflect.Value) int {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return maplen(rvRefPtr(v))
}

func rvLenArray(rv reflect.Value) int {
	return rv.Len()
}

// ------------ map range and map indexing ----------

// regular calls to map via reflection: MapKeys, MapIndex, MapRange/MapIter etc
// will always allocate for each map key or value.
//
// It is more performant to provide a value that the map entry is set into,
// and that elides the allocation.

// unsafeMapHashIter
//
// go 1.4+ has runtime/hashmap.go or runtime/map.go which has a
// hIter struct with the first 2 values being key and value
// of the current iteration.
//
// This *hIter is passed to mapiterinit, mapiternext, mapiterkey, mapiterelem.
// We bypass the reflect wrapper functions and just use the *hIter directly.
//
// Though *hIter has many fields, we only care about the first 2.
type unsafeMapHashIter struct {
	key, value unsafe.Pointer
	// other fields are ignored
}

type mapIter struct {
	unsafeMapIter
}

type unsafeMapIter struct {
	it             *unsafeMapHashIter
	mtyp, mptr     unsafe.Pointer
	k, v           reflect.Value
	kisref, visref bool
	mapvalues      bool
	done           bool
	started        bool
}

func (t *unsafeMapIter) Next() (r bool) {
	if t == nil || t.done {
		return
	}
	if t.started {
		mapiternext((unsafe.Pointer)(t.it))
	} else {
		t.started = true
	}

	t.done = t.it.key == nil
	if t.done {
		return
	}

	k := (*unsafeReflectValue)(unsafe.Pointer(&t.k))
	if helperUnsafeCopyMapEntry {
		unsafeMapSet(k.typ, k.ptr, t.it.key, t.kisref)
	} else {
		k.ptr = t.it.key
	}

	if t.mapvalues {
		v := (*unsafeReflectValue)(unsafe.Pointer(&t.v))
		if helperUnsafeCopyMapEntry {
			unsafeMapSet(v.typ, v.ptr, t.it.value, t.visref)
		} else {
			v.ptr = t.it.value
		}
	}
	return true
}

func (t *unsafeMapIter) Key() (r reflect.Value) {
	return t.k
}

func (t *unsafeMapIter) Value() (r reflect.Value) {
	return t.v
}

func (t *unsafeMapIter) Done() {
}

// unsafeMapSet does equivalent of: p = p2
func unsafeMapSet(ptyp, p, p2 unsafe.Pointer, isref bool) {
	if isref {
		*(*unsafe.Pointer)(p) = *(*unsafe.Pointer)(p2) // p2
	} else {
		typedmemmove(ptyp, p, p2) // *(*unsafe.Pointer)(p2)) // p2)
	}
}

// unsafeMapKVPtr returns the pointer if flagIndir, else it returns a pointer to the pointer.
// It is needed as maps always keep a reference to the underlying value.
func unsafeMapKVPtr(urv *unsafeReflectValue) unsafe.Pointer {
	if urv.flag&unsafeFlagIndir == 0 {
		return unsafe.Pointer(&urv.ptr)
	}
	return urv.ptr
}

func mapRange(t *mapIter, m, k, v reflect.Value, mapvalues bool) {
	if rvIsNil(m) {
		t.done = true
		return
	}
	t.done = false
	t.started = false
	t.mapvalues = mapvalues

	var urv *unsafeReflectValue

	urv = (*unsafeReflectValue)(unsafe.Pointer(&m))
	t.mtyp = urv.typ
	t.mptr = rvRefPtr(urv)

	t.it = (*unsafeMapHashIter)(mapiterinit(t.mtyp, t.mptr))

	t.k = k
	t.kisref = refBitset.isset(byte(k.Kind()))

	if mapvalues {
		t.v = v
		t.visref = refBitset.isset(byte(v.Kind()))
	} else {
		t.v = reflect.Value{}
	}
}

func mapGet(m, k, v reflect.Value) (vv reflect.Value) {
	var urv = (*unsafeReflectValue)(unsafe.Pointer(&k))
	var kptr = unsafeMapKVPtr(urv)

	urv = (*unsafeReflectValue)(unsafe.Pointer(&m))

	vvptr := mapaccess(urv.typ, rvRefPtr(urv), kptr)
	if vvptr == nil {
		return
	}
	// vvptr = *(*unsafe.Pointer)(vvptr)

	urv = (*unsafeReflectValue)(unsafe.Pointer(&v))
	if helperUnsafeCopyMapEntry {
		unsafeMapSet(urv.typ, urv.ptr, vvptr, refBitset.isset(byte(v.Kind())))
	} else {
		urv.ptr = vvptr
	}
	return v
}

func mapSet(m, k, v reflect.Value) {
	var urv = (*unsafeReflectValue)(unsafe.Pointer(&k))
	var kptr = unsafeMapKVPtr(urv)
	urv = (*unsafeReflectValue)(unsafe.Pointer(&v))
	var vptr = unsafeMapKVPtr(urv)
	urv = (*unsafeReflectValue)(unsafe.Pointer(&m))
	mapassign(urv.typ, rvRefPtr(urv), kptr, vptr)
}

// func mapDelete(m, k reflect.Value) {
// 	var urv = (*unsafeReflectValue)(unsafe.Pointer(&k))
// 	var kptr = unsafeMapKVPtr(urv)
// 	urv = (*unsafeReflectValue)(unsafe.Pointer(&m))
// 	mapdelete(urv.typ, rv2ptr(urv), kptr)
// }

// return an addressable reflect value that can be used in mapRange and mapGet operations.
//
// all calls to mapGet or mapRange will call here to get an addressable reflect.Value.
func mapAddrLoopvarRV(t reflect.Type, k reflect.Kind) (r reflect.Value) {
	return rvZeroAddrK(t, k)
}

// ---------- ENCODER optimized ---------------

func (e *Encoder) jsondriver() *jsonEncDriver {
	return (*jsonEncDriver)((*unsafeIntf)(unsafe.Pointer(&e.e)).word)
}

// ---------- DECODER optimized ---------------

func (d *Decoder) checkBreak() bool {
	// MARKER: jsonDecDriver.CheckBreak() costs over 80, and this isn't inlined.
	// Consequently, there's no benefit in incurring the cost of this
	// wrapping function checkBreak.
	//
	// It is faster to just call the interface method directly.

	// if d.js {
	// 	return d.jsondriver().CheckBreak()
	// }
	// if d.cbor {
	// 	return d.cbordriver().CheckBreak()
	// }
	return d.d.CheckBreak()
}

func (d *Decoder) jsondriver() *jsonDecDriver {
	return (*jsonDecDriver)((*unsafeIntf)(unsafe.Pointer(&d.d)).word)
}

// ---------- structFieldInfo optimized ---------------

func (n *structFieldInfoPathNode) rvField(v reflect.Value) (rv reflect.Value) {
	// we already know this is exported, and maybe embedded (based on what si says)
	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	// clear flagEmbedRO if necessary, and inherit permission bits from v
	urv.flag = uv.flag&(unsafeFlagStickyRO|unsafeFlagIndir|unsafeFlagAddr) | uintptr(n.kind)
	urv.typ = ((*unsafeIntf)(unsafe.Pointer(&n.typ))).word
	urv.ptr = unsafe.Pointer(uintptr(uv.ptr) + uintptr(n.offset))
	return
}

// ---------- go linknames (LINKED to runtime/reflect) ---------------

// MARKER: always check that these linknames match subsequent versions of go

//go:linkname maplen reflect.maplen
//go:noescape
func maplen(typ unsafe.Pointer) int

//go:linkname chanlen reflect.chanlen
//go:noescape
func chanlen(typ unsafe.Pointer) int

//go:linkname mapiterinit reflect.mapiterinit
//go:noescape
func mapiterinit(typ unsafe.Pointer, it unsafe.Pointer) (key unsafe.Pointer)

//go:linkname mapiternext reflect.mapiternext
//go:noescape
func mapiternext(it unsafe.Pointer) (key unsafe.Pointer)

//go:linkname mapaccess reflect.mapaccess
//go:noescape
func mapaccess(typ unsafe.Pointer, m unsafe.Pointer, key unsafe.Pointer) (val unsafe.Pointer)

//go:linkname mapassign reflect.mapassign
//go:noescape
func mapassign(typ unsafe.Pointer, m unsafe.Pointer, key, val unsafe.Pointer)

//go:linkname mapdelete reflect.mapdelete
//go:noescape
func mapdelete(typ unsafe.Pointer, m unsafe.Pointer, key unsafe.Pointer)

//go:linkname typedmemmove runtime.typedmemmove
//go:noescape
func typedmemmove(typ unsafe.Pointer, dst, src unsafe.Pointer)

//go:linkname typedmemclr runtime.typedmemclr
//go:noescape
func typedmemclr(typ unsafe.Pointer, dst unsafe.Pointer)

//go:linkname typedslicecopy reflect.typedslicecopy
//go:noescape
func typedslicecopy(elemType unsafe.Pointer, dst, src unsafeSlice) int

// //go:linkname memmove reflect.memmove
// //go:noescape
// func memmove(dst, src unsafe.Pointer, n int)

//go:linkname unsafe_New reflect.unsafe_New
//go:noescape
func unsafe_New(typ unsafe.Pointer) unsafe.Pointer

var _ = runtime.MemProfileRate
