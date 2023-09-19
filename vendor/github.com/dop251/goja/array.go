package goja

import (
	"fmt"
	"math"
	"math/bits"
	"reflect"
	"strconv"

	"github.com/dop251/goja/unistring"
)

type arrayIterObject struct {
	baseObject
	obj     *Object
	nextIdx int64
	kind    iterationKind
}

func (ai *arrayIterObject) next() Value {
	if ai.obj == nil {
		return ai.val.runtime.createIterResultObject(_undefined, true)
	}
	if ta, ok := ai.obj.self.(*typedArrayObject); ok {
		ta.viewedArrayBuf.ensureNotDetached(true)
	}
	l := toLength(ai.obj.self.getStr("length", nil))
	index := ai.nextIdx
	if index >= l {
		ai.obj = nil
		return ai.val.runtime.createIterResultObject(_undefined, true)
	}
	ai.nextIdx++
	idxVal := valueInt(index)
	if ai.kind == iterationKindKey {
		return ai.val.runtime.createIterResultObject(idxVal, false)
	}
	elementValue := nilSafe(ai.obj.self.getIdx(idxVal, nil))
	var result Value
	if ai.kind == iterationKindValue {
		result = elementValue
	} else {
		result = ai.val.runtime.newArrayValues([]Value{idxVal, elementValue})
	}
	return ai.val.runtime.createIterResultObject(result, false)
}

func (r *Runtime) createArrayIterator(iterObj *Object, kind iterationKind) Value {
	o := &Object{runtime: r}

	ai := &arrayIterObject{
		obj:  iterObj,
		kind: kind,
	}
	ai.class = classObject
	ai.val = o
	ai.extensible = true
	o.self = ai
	ai.prototype = r.getArrayIteratorPrototype()
	ai.init()

	return o
}

type arrayObject struct {
	baseObject
	values         []Value
	length         uint32
	objCount       int
	propValueCount int
	lengthProp     valueProperty
}

func (a *arrayObject) init() {
	a.baseObject.init()
	a.lengthProp.writable = true

	a._put("length", &a.lengthProp)
}

func (a *arrayObject) _setLengthInt(l uint32, throw bool) bool {
	ret := true
	if l <= a.length {
		if a.propValueCount > 0 {
			// Slow path
			for i := len(a.values) - 1; i >= int(l); i-- {
				if prop, ok := a.values[i].(*valueProperty); ok {
					if !prop.configurable {
						l = uint32(i) + 1
						ret = false
						break
					}
					a.propValueCount--
				}
			}
		}
	}
	if l <= uint32(len(a.values)) {
		if l >= 16 && l < uint32(cap(a.values))>>2 {
			ar := make([]Value, l)
			copy(ar, a.values)
			a.values = ar
		} else {
			ar := a.values[l:len(a.values)]
			for i := range ar {
				ar[i] = nil
			}
			a.values = a.values[:l]
		}
	}
	a.length = l
	if !ret {
		a.val.runtime.typeErrorResult(throw, "Cannot redefine property: length")
	}
	return ret
}

func (a *arrayObject) setLengthInt(l uint32, throw bool) bool {
	if l == a.length {
		return true
	}
	if !a.lengthProp.writable {
		a.val.runtime.typeErrorResult(throw, "length is not writable")
		return false
	}
	return a._setLengthInt(l, throw)
}

func (a *arrayObject) setLength(v uint32, throw bool) bool {
	if !a.lengthProp.writable {
		a.val.runtime.typeErrorResult(throw, "length is not writable")
		return false
	}
	return a._setLengthInt(v, throw)
}

func (a *arrayObject) getIdx(idx valueInt, receiver Value) Value {
	prop := a.getOwnPropIdx(idx)
	if prop == nil {
		if a.prototype != nil {
			if receiver == nil {
				return a.prototype.self.getIdx(idx, a.val)
			}
			return a.prototype.self.getIdx(idx, receiver)
		}
	}
	if prop, ok := prop.(*valueProperty); ok {
		if receiver == nil {
			return prop.get(a.val)
		}
		return prop.get(receiver)
	}
	return prop
}

func (a *arrayObject) getOwnPropStr(name unistring.String) Value {
	if len(a.values) > 0 {
		if i := strToArrayIdx(name); i != math.MaxUint32 {
			if i < uint32(len(a.values)) {
				return a.values[i]
			}
		}
	}
	if name == "length" {
		return a.getLengthProp()
	}
	return a.baseObject.getOwnPropStr(name)
}

func (a *arrayObject) getOwnPropIdx(idx valueInt) Value {
	if i := toIdx(idx); i != math.MaxUint32 {
		if i < uint32(len(a.values)) {
			return a.values[i]
		}
		return nil
	}

	return a.baseObject.getOwnPropStr(idx.string())
}

func (a *arrayObject) sortLen() int {
	return len(a.values)
}

func (a *arrayObject) sortGet(i int) Value {
	v := a.values[i]
	if p, ok := v.(*valueProperty); ok {
		v = p.get(a.val)
	}
	return v
}

func (a *arrayObject) swap(i int, j int) {
	a.values[i], a.values[j] = a.values[j], a.values[i]
}

func (a *arrayObject) getStr(name unistring.String, receiver Value) Value {
	return a.getStrWithOwnProp(a.getOwnPropStr(name), name, receiver)
}

func (a *arrayObject) getLengthProp() *valueProperty {
	a.lengthProp.value = intToValue(int64(a.length))
	return &a.lengthProp
}

func (a *arrayObject) setOwnIdx(idx valueInt, val Value, throw bool) bool {
	if i := toIdx(idx); i != math.MaxUint32 {
		return a._setOwnIdx(i, val, throw)
	} else {
		return a.baseObject.setOwnStr(idx.string(), val, throw)
	}
}

func (a *arrayObject) _setOwnIdx(idx uint32, val Value, throw bool) bool {
	var prop Value
	if idx < uint32(len(a.values)) {
		prop = a.values[idx]
	}

	if prop == nil {
		if proto := a.prototype; proto != nil {
			// we know it's foreign because prototype loops are not allowed
			if res, ok := proto.self.setForeignIdx(valueInt(idx), val, a.val, throw); ok {
				return res
			}
		}
		// new property
		if !a.extensible {
			a.val.runtime.typeErrorResult(throw, "Cannot add property %d, object is not extensible", idx)
			return false
		} else {
			if idx >= a.length {
				if !a.setLengthInt(idx+1, throw) {
					return false
				}
			}
			if idx >= uint32(len(a.values)) {
				if !a.expand(idx) {
					a.val.self.(*sparseArrayObject).add(idx, val)
					return true
				}
			}
			a.objCount++
		}
	} else {
		if prop, ok := prop.(*valueProperty); ok {
			if !prop.isWritable() {
				a.val.runtime.typeErrorResult(throw)
				return false
			}
			prop.set(a.val, val)
			return true
		}
	}
	a.values[idx] = val
	return true
}

func (a *arrayObject) setOwnStr(name unistring.String, val Value, throw bool) bool {
	if idx := strToArrayIdx(name); idx != math.MaxUint32 {
		return a._setOwnIdx(idx, val, throw)
	} else {
		if name == "length" {
			return a.setLength(a.val.runtime.toLengthUint32(val), throw)
		} else {
			return a.baseObject.setOwnStr(name, val, throw)
		}
	}
}

func (a *arrayObject) setForeignIdx(idx valueInt, val, receiver Value, throw bool) (bool, bool) {
	return a._setForeignIdx(idx, a.getOwnPropIdx(idx), val, receiver, throw)
}

func (a *arrayObject) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return a._setForeignStr(name, a.getOwnPropStr(name), val, receiver, throw)
}

type arrayPropIter struct {
	a     *arrayObject
	limit int
	idx   int
}

func (i *arrayPropIter) next() (propIterItem, iterNextFunc) {
	for i.idx < len(i.a.values) && i.idx < i.limit {
		name := asciiString(strconv.Itoa(i.idx))
		prop := i.a.values[i.idx]
		i.idx++
		if prop != nil {
			return propIterItem{name: name, value: prop}, i.next
		}
	}

	return i.a.baseObject.iterateStringKeys()()
}

func (a *arrayObject) iterateStringKeys() iterNextFunc {
	return (&arrayPropIter{
		a:     a,
		limit: len(a.values),
	}).next
}

func (a *arrayObject) stringKeys(all bool, accum []Value) []Value {
	for i, prop := range a.values {
		name := strconv.Itoa(i)
		if prop != nil {
			if !all {
				if prop, ok := prop.(*valueProperty); ok && !prop.enumerable {
					continue
				}
			}
			accum = append(accum, asciiString(name))
		}
	}
	return a.baseObject.stringKeys(all, accum)
}

func (a *arrayObject) hasOwnPropertyStr(name unistring.String) bool {
	if idx := strToArrayIdx(name); idx != math.MaxUint32 {
		return idx < uint32(len(a.values)) && a.values[idx] != nil
	} else {
		return a.baseObject.hasOwnPropertyStr(name)
	}
}

func (a *arrayObject) hasOwnPropertyIdx(idx valueInt) bool {
	if idx := toIdx(idx); idx != math.MaxUint32 {
		return idx < uint32(len(a.values)) && a.values[idx] != nil
	}
	return a.baseObject.hasOwnPropertyStr(idx.string())
}

func (a *arrayObject) hasPropertyIdx(idx valueInt) bool {
	if a.hasOwnPropertyIdx(idx) {
		return true
	}

	if a.prototype != nil {
		return a.prototype.self.hasPropertyIdx(idx)
	}

	return false
}

func (a *arrayObject) expand(idx uint32) bool {
	targetLen := idx + 1
	if targetLen > uint32(len(a.values)) {
		if targetLen < uint32(cap(a.values)) {
			a.values = a.values[:targetLen]
		} else {
			if idx > 4096 && (a.objCount == 0 || idx/uint32(a.objCount) > 10) {
				//log.Println("Switching standard->sparse")
				sa := &sparseArrayObject{
					baseObject:     a.baseObject,
					length:         a.length,
					propValueCount: a.propValueCount,
				}
				sa.setValues(a.values, a.objCount+1)
				sa.val.self = sa
				sa.lengthProp.writable = a.lengthProp.writable
				sa._put("length", &sa.lengthProp)
				return false
			} else {
				if bits.UintSize == 32 {
					if targetLen >= math.MaxInt32 {
						panic(a.val.runtime.NewTypeError("Array index overflows int"))
					}
				}
				tl := int(targetLen)
				newValues := make([]Value, tl, growCap(tl, len(a.values), cap(a.values)))
				copy(newValues, a.values)
				a.values = newValues
			}
		}
	}
	return true
}

func (r *Runtime) defineArrayLength(prop *valueProperty, descr PropertyDescriptor, setter func(uint32, bool) bool, throw bool) bool {
	var newLen uint32
	ret := true
	if descr.Value != nil {
		newLen = r.toLengthUint32(descr.Value)
	}

	if descr.Configurable == FLAG_TRUE || descr.Enumerable == FLAG_TRUE || descr.Getter != nil || descr.Setter != nil {
		ret = false
		goto Reject
	}

	if descr.Value != nil {
		oldLen := uint32(prop.value.ToInteger())
		if oldLen != newLen {
			ret = setter(newLen, false)
		}
	} else {
		ret = true
	}

	if descr.Writable != FLAG_NOT_SET {
		w := descr.Writable.Bool()
		if prop.writable {
			prop.writable = w
		} else {
			if w {
				ret = false
				goto Reject
			}
		}
	}

Reject:
	if !ret {
		r.typeErrorResult(throw, "Cannot redefine property: length")
	}

	return ret
}

func (a *arrayObject) _defineIdxProperty(idx uint32, desc PropertyDescriptor, throw bool) bool {
	var existing Value
	if idx < uint32(len(a.values)) {
		existing = a.values[idx]
	}
	prop, ok := a.baseObject._defineOwnProperty(unistring.String(strconv.FormatUint(uint64(idx), 10)), existing, desc, throw)
	if ok {
		if idx >= a.length {
			if !a.setLengthInt(idx+1, throw) {
				return false
			}
		}
		if a.expand(idx) {
			a.values[idx] = prop
			a.objCount++
			if _, ok := prop.(*valueProperty); ok {
				a.propValueCount++
			}
		} else {
			a.val.self.(*sparseArrayObject).add(idx, prop)
		}
	}
	return ok
}

func (a *arrayObject) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if idx := strToArrayIdx(name); idx != math.MaxUint32 {
		return a._defineIdxProperty(idx, descr, throw)
	}
	if name == "length" {
		return a.val.runtime.defineArrayLength(a.getLengthProp(), descr, a.setLength, throw)
	}
	return a.baseObject.defineOwnPropertyStr(name, descr, throw)
}

func (a *arrayObject) defineOwnPropertyIdx(idx valueInt, descr PropertyDescriptor, throw bool) bool {
	if idx := toIdx(idx); idx != math.MaxUint32 {
		return a._defineIdxProperty(idx, descr, throw)
	}
	return a.baseObject.defineOwnPropertyStr(idx.string(), descr, throw)
}

func (a *arrayObject) _deleteIdxProp(idx uint32, throw bool) bool {
	if idx < uint32(len(a.values)) {
		if v := a.values[idx]; v != nil {
			if p, ok := v.(*valueProperty); ok {
				if !p.configurable {
					a.val.runtime.typeErrorResult(throw, "Cannot delete property '%d' of %s", idx, a.val.toString())
					return false
				}
				a.propValueCount--
			}
			a.values[idx] = nil
			a.objCount--
		}
	}
	return true
}

func (a *arrayObject) deleteStr(name unistring.String, throw bool) bool {
	if idx := strToArrayIdx(name); idx != math.MaxUint32 {
		return a._deleteIdxProp(idx, throw)
	}
	return a.baseObject.deleteStr(name, throw)
}

func (a *arrayObject) deleteIdx(idx valueInt, throw bool) bool {
	if idx := toIdx(idx); idx != math.MaxUint32 {
		return a._deleteIdxProp(idx, throw)
	}
	return a.baseObject.deleteStr(idx.string(), throw)
}

func (a *arrayObject) export(ctx *objectExportCtx) interface{} {
	if v, exists := ctx.get(a.val); exists {
		return v
	}
	arr := make([]interface{}, a.length)
	ctx.put(a.val, arr)
	if a.propValueCount == 0 && a.length == uint32(len(a.values)) && uint32(a.objCount) == a.length {
		for i, v := range a.values {
			if v != nil {
				arr[i] = exportValue(v, ctx)
			}
		}
	} else {
		for i := uint32(0); i < a.length; i++ {
			v := a.getIdx(valueInt(i), nil)
			if v != nil {
				arr[i] = exportValue(v, ctx)
			}
		}
	}
	return arr
}

func (a *arrayObject) exportType() reflect.Type {
	return reflectTypeArray
}

func (a *arrayObject) exportToArrayOrSlice(dst reflect.Value, typ reflect.Type, ctx *objectExportCtx) error {
	r := a.val.runtime
	if iter := a.getSym(SymIterator, nil); iter == r.getArrayValues() || iter == nil {
		l := toIntStrict(int64(a.length))
		if typ.Kind() == reflect.Array {
			if dst.Len() != l {
				return fmt.Errorf("cannot convert an Array into an array, lengths mismatch (have %d, need %d)", l, dst.Len())
			}
		} else {
			dst.Set(reflect.MakeSlice(typ, l, l))
		}
		ctx.putTyped(a.val, typ, dst.Interface())
		for i := 0; i < l; i++ {
			if i >= len(a.values) {
				break
			}
			val := a.values[i]
			if p, ok := val.(*valueProperty); ok {
				val = p.get(a.val)
			}
			err := r.toReflectValue(val, dst.Index(i), ctx)
			if err != nil {
				return fmt.Errorf("could not convert array element %v to %v at %d: %w", val, typ, i, err)
			}
		}
		return nil
	}
	return a.baseObject.exportToArrayOrSlice(dst, typ, ctx)
}

func (a *arrayObject) setValuesFromSparse(items []sparseArrayItem, newMaxIdx int) {
	a.values = make([]Value, newMaxIdx+1)
	for _, item := range items {
		a.values[item.idx] = item.value
	}
	a.objCount = len(items)
}

func toIdx(v valueInt) uint32 {
	if v >= 0 && v < math.MaxUint32 {
		return uint32(v)
	}
	return math.MaxUint32
}
