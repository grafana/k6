package goja

import (
	"math"
	"math/bits"
	"reflect"

	"github.com/dop251/goja/unistring"
)

type objectGoSliceReflect struct {
	objectGoArrayReflect
}

func (o *objectGoSliceReflect) init() {
	o.objectGoArrayReflect._init()
	o.lengthProp.writable = true
	o.putIdx = o._putIdx
}

func (o *objectGoSliceReflect) _putIdx(idx int, v Value, throw bool) bool {
	if idx >= o.value.Len() {
		o.grow(idx + 1)
	}
	return o.objectGoArrayReflect._putIdx(idx, v, throw)
}

func (o *objectGoSliceReflect) grow(size int) {
	oldcap := o.value.Cap()
	if oldcap < size {
		n := reflect.MakeSlice(o.value.Type(), size, growCap(size, o.value.Len(), oldcap))
		reflect.Copy(n, o.value)
		o.value.Set(n)
	} else {
		tail := o.value.Slice(o.value.Len(), size)
		zero := reflect.Zero(o.value.Type().Elem())
		for i := 0; i < tail.Len(); i++ {
			tail.Index(i).Set(zero)
		}
		o.value.SetLen(size)
	}
	o.updateLen()
}

func (o *objectGoSliceReflect) shrink(size int) {
	tail := o.value.Slice(size, o.value.Len())
	zero := reflect.Zero(o.value.Type().Elem())
	for i := 0; i < tail.Len(); i++ {
		tail.Index(i).Set(zero)
	}
	o.value.SetLen(size)
	o.updateLen()
}

func (o *objectGoSliceReflect) putLength(v uint32, throw bool) bool {
	if bits.UintSize == 32 && v > math.MaxInt32 {
		panic(rangeError("Integer value overflows 32-bit int"))
	}
	newLen := int(v)
	curLen := o.value.Len()
	if newLen > curLen {
		o.grow(newLen)
	} else if newLen < curLen {
		o.shrink(newLen)
	}
	return true
}

func (o *objectGoSliceReflect) setOwnStr(name unistring.String, val Value, throw bool) bool {
	if name == "length" {
		return o.putLength(o.val.runtime.toLengthUint32(val), throw)
	}
	return o.objectGoArrayReflect.setOwnStr(name, val, throw)
}

func (o *objectGoSliceReflect) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if name == "length" {
		return o.val.runtime.defineArrayLength(&o.lengthProp, descr, o.putLength, throw)
	}
	return o.objectGoArrayReflect.defineOwnPropertyStr(name, descr, throw)
}

func (o *objectGoSliceReflect) equal(other objectImpl) bool {
	if other, ok := other.(*objectGoSliceReflect); ok {
		return o.value.Interface() == other.value.Interface()
	}
	return false
}
