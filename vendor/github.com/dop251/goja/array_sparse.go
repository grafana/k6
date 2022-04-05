package goja

import (
	"fmt"
	"math"
	"math/bits"
	"reflect"
	"sort"
	"strconv"

	"github.com/dop251/goja/unistring"
)

type sparseArrayItem struct {
	idx   uint32
	value Value
}

type sparseArrayObject struct {
	baseObject
	items          []sparseArrayItem
	length         uint32
	propValueCount int
	lengthProp     valueProperty
}

func (a *sparseArrayObject) findIdx(idx uint32) int {
	return sort.Search(len(a.items), func(i int) bool {
		return a.items[i].idx >= idx
	})
}

func (a *sparseArrayObject) _setLengthInt(l uint32, throw bool) bool {
	ret := true
	if l <= a.length {
		if a.propValueCount > 0 {
			// Slow path
			for i := len(a.items) - 1; i >= 0; i-- {
				item := a.items[i]
				if item.idx <= l {
					break
				}
				if prop, ok := item.value.(*valueProperty); ok {
					if !prop.configurable {
						l = item.idx + 1
						ret = false
						break
					}
					a.propValueCount--
				}
			}
		}
	}

	idx := a.findIdx(l)

	aa := a.items[idx:]
	for i := range aa {
		aa[i].value = nil
	}
	a.items = a.items[:idx]
	a.length = l
	if !ret {
		a.val.runtime.typeErrorResult(throw, "Cannot redefine property: length")
	}
	return ret
}

func (a *sparseArrayObject) setLengthInt(l uint32, throw bool) bool {
	if l == a.length {
		return true
	}
	if !a.lengthProp.writable {
		a.val.runtime.typeErrorResult(throw, "length is not writable")
		return false
	}
	return a._setLengthInt(l, throw)
}

func (a *sparseArrayObject) setLength(v uint32, throw bool) bool {
	if !a.lengthProp.writable {
		a.val.runtime.typeErrorResult(throw, "length is not writable")
		return false
	}
	return a._setLengthInt(v, throw)
}

func (a *sparseArrayObject) _getIdx(idx uint32) Value {
	i := a.findIdx(idx)
	if i < len(a.items) && a.items[i].idx == idx {
		return a.items[i].value
	}

	return nil
}

func (a *sparseArrayObject) getStr(name unistring.String, receiver Value) Value {
	return a.getStrWithOwnProp(a.getOwnPropStr(name), name, receiver)
}

func (a *sparseArrayObject) getIdx(idx valueInt, receiver Value) Value {
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

func (a *sparseArrayObject) getLengthProp() *valueProperty {
	a.lengthProp.value = intToValue(int64(a.length))
	return &a.lengthProp
}

func (a *sparseArrayObject) getOwnPropStr(name unistring.String) Value {
	if idx := strToArrayIdx(name); idx != math.MaxUint32 {
		return a._getIdx(idx)
	}
	if name == "length" {
		return a.getLengthProp()
	}
	return a.baseObject.getOwnPropStr(name)
}

func (a *sparseArrayObject) getOwnPropIdx(idx valueInt) Value {
	if idx := toIdx(idx); idx != math.MaxUint32 {
		return a._getIdx(idx)
	}
	return a.baseObject.getOwnPropStr(idx.string())
}

func (a *sparseArrayObject) add(idx uint32, val Value) {
	i := a.findIdx(idx)
	a.items = append(a.items, sparseArrayItem{})
	copy(a.items[i+1:], a.items[i:])
	a.items[i] = sparseArrayItem{
		idx:   idx,
		value: val,
	}
}

func (a *sparseArrayObject) _setOwnIdx(idx uint32, val Value, throw bool) bool {
	var prop Value
	i := a.findIdx(idx)
	if i < len(a.items) && a.items[i].idx == idx {
		prop = a.items[i].value
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
		}

		if idx >= a.length {
			if !a.setLengthInt(idx+1, throw) {
				return false
			}
		}

		if a.expand(idx) {
			a.items = append(a.items, sparseArrayItem{})
			copy(a.items[i+1:], a.items[i:])
			a.items[i] = sparseArrayItem{
				idx:   idx,
				value: val,
			}
		} else {
			ar := a.val.self.(*arrayObject)
			ar.values[idx] = val
			ar.objCount++
			return true
		}
	} else {
		if prop, ok := prop.(*valueProperty); ok {
			if !prop.isWritable() {
				a.val.runtime.typeErrorResult(throw)
				return false
			}
			prop.set(a.val, val)
		} else {
			a.items[i].value = val
		}
	}
	return true
}

func (a *sparseArrayObject) setOwnStr(name unistring.String, val Value, throw bool) bool {
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

func (a *sparseArrayObject) setOwnIdx(idx valueInt, val Value, throw bool) bool {
	if idx := toIdx(idx); idx != math.MaxUint32 {
		return a._setOwnIdx(idx, val, throw)
	}

	return a.baseObject.setOwnStr(idx.string(), val, throw)
}

func (a *sparseArrayObject) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return a._setForeignStr(name, a.getOwnPropStr(name), val, receiver, throw)
}

func (a *sparseArrayObject) setForeignIdx(name valueInt, val, receiver Value, throw bool) (bool, bool) {
	return a._setForeignIdx(name, a.getOwnPropIdx(name), val, receiver, throw)
}

type sparseArrayPropIter struct {
	a   *sparseArrayObject
	idx int
}

func (i *sparseArrayPropIter) next() (propIterItem, iterNextFunc) {
	for i.idx < len(i.a.items) {
		name := asciiString(strconv.Itoa(int(i.a.items[i.idx].idx)))
		prop := i.a.items[i.idx].value
		i.idx++
		if prop != nil {
			return propIterItem{name: name, value: prop}, i.next
		}
	}

	return i.a.baseObject.iterateStringKeys()()
}

func (a *sparseArrayObject) iterateStringKeys() iterNextFunc {
	return (&sparseArrayPropIter{
		a: a,
	}).next
}

func (a *sparseArrayObject) stringKeys(all bool, accum []Value) []Value {
	if all {
		for _, item := range a.items {
			accum = append(accum, asciiString(strconv.FormatUint(uint64(item.idx), 10)))
		}
	} else {
		for _, item := range a.items {
			if prop, ok := item.value.(*valueProperty); ok && !prop.enumerable {
				continue
			}
			accum = append(accum, asciiString(strconv.FormatUint(uint64(item.idx), 10)))
		}
	}

	return a.baseObject.stringKeys(all, accum)
}

func (a *sparseArrayObject) setValues(values []Value, objCount int) {
	a.items = make([]sparseArrayItem, 0, objCount)
	for i, val := range values {
		if val != nil {
			a.items = append(a.items, sparseArrayItem{
				idx:   uint32(i),
				value: val,
			})
		}
	}
}

func (a *sparseArrayObject) hasOwnPropertyStr(name unistring.String) bool {
	if idx := strToArrayIdx(name); idx != math.MaxUint32 {
		i := a.findIdx(idx)
		return i < len(a.items) && a.items[i].idx == idx
	} else {
		return a.baseObject.hasOwnPropertyStr(name)
	}
}

func (a *sparseArrayObject) hasOwnPropertyIdx(idx valueInt) bool {
	if idx := toIdx(idx); idx != math.MaxUint32 {
		i := a.findIdx(idx)
		return i < len(a.items) && a.items[i].idx == idx
	}

	return a.baseObject.hasOwnPropertyStr(idx.string())
}

func (a *sparseArrayObject) expand(idx uint32) bool {
	if l := len(a.items); l >= 1024 {
		if ii := a.items[l-1].idx; ii > idx {
			idx = ii
		}
		if (bits.UintSize == 64 || idx < math.MaxInt32) && int(idx)>>3 < l {
			//log.Println("Switching sparse->standard")
			ar := &arrayObject{
				baseObject:     a.baseObject,
				length:         a.length,
				propValueCount: a.propValueCount,
			}
			ar.setValuesFromSparse(a.items, int(idx))
			ar.val.self = ar
			ar.lengthProp.writable = a.lengthProp.writable
			a._put("length", &ar.lengthProp)
			return false
		}
	}
	return true
}

func (a *sparseArrayObject) _defineIdxProperty(idx uint32, desc PropertyDescriptor, throw bool) bool {
	var existing Value
	i := a.findIdx(idx)
	if i < len(a.items) && a.items[i].idx == idx {
		existing = a.items[i].value
	}
	prop, ok := a.baseObject._defineOwnProperty(unistring.String(strconv.FormatUint(uint64(idx), 10)), existing, desc, throw)
	if ok {
		if idx >= a.length {
			if !a.setLengthInt(idx+1, throw) {
				return false
			}
		}
		if i >= len(a.items) || a.items[i].idx != idx {
			if a.expand(idx) {
				a.items = append(a.items, sparseArrayItem{})
				copy(a.items[i+1:], a.items[i:])
				a.items[i] = sparseArrayItem{
					idx:   idx,
					value: prop,
				}
				if idx >= a.length {
					a.length = idx + 1
				}
			} else {
				a.val.self.(*arrayObject).values[idx] = prop
			}
		} else {
			a.items[i].value = prop
		}
		if _, ok := prop.(*valueProperty); ok {
			a.propValueCount++
		}
	}
	return ok
}

func (a *sparseArrayObject) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if idx := strToArrayIdx(name); idx != math.MaxUint32 {
		return a._defineIdxProperty(idx, descr, throw)
	}
	if name == "length" {
		return a.val.runtime.defineArrayLength(a.getLengthProp(), descr, a.setLength, throw)
	}
	return a.baseObject.defineOwnPropertyStr(name, descr, throw)
}

func (a *sparseArrayObject) defineOwnPropertyIdx(idx valueInt, descr PropertyDescriptor, throw bool) bool {
	if idx := toIdx(idx); idx != math.MaxUint32 {
		return a._defineIdxProperty(idx, descr, throw)
	}
	return a.baseObject.defineOwnPropertyStr(idx.string(), descr, throw)
}

func (a *sparseArrayObject) _deleteIdxProp(idx uint32, throw bool) bool {
	i := a.findIdx(idx)
	if i < len(a.items) && a.items[i].idx == idx {
		if p, ok := a.items[i].value.(*valueProperty); ok {
			if !p.configurable {
				a.val.runtime.typeErrorResult(throw, "Cannot delete property '%d' of %s", idx, a.val.toString())
				return false
			}
			a.propValueCount--
		}
		copy(a.items[i:], a.items[i+1:])
		a.items[len(a.items)-1].value = nil
		a.items = a.items[:len(a.items)-1]
	}
	return true
}

func (a *sparseArrayObject) deleteStr(name unistring.String, throw bool) bool {
	if idx := strToArrayIdx(name); idx != math.MaxUint32 {
		return a._deleteIdxProp(idx, throw)
	}
	return a.baseObject.deleteStr(name, throw)
}

func (a *sparseArrayObject) deleteIdx(idx valueInt, throw bool) bool {
	if idx := toIdx(idx); idx != math.MaxUint32 {
		return a._deleteIdxProp(idx, throw)
	}
	return a.baseObject.deleteStr(idx.string(), throw)
}

func (a *sparseArrayObject) sortLen() int64 {
	if len(a.items) > 0 {
		return int64(a.items[len(a.items)-1].idx) + 1
	}

	return 0
}

func (a *sparseArrayObject) export(ctx *objectExportCtx) interface{} {
	if v, exists := ctx.get(a.val); exists {
		return v
	}
	arr := make([]interface{}, a.length)
	ctx.put(a.val, arr)
	var prevIdx uint32
	for _, item := range a.items {
		idx := item.idx
		for i := prevIdx; i < idx; i++ {
			if a.prototype != nil {
				if v := a.prototype.self.getIdx(valueInt(i), nil); v != nil {
					arr[i] = exportValue(v, ctx)
				}
			}
		}
		v := item.value
		if v != nil {
			if prop, ok := v.(*valueProperty); ok {
				v = prop.get(a.val)
			}
			arr[idx] = exportValue(v, ctx)
		}
		prevIdx = idx + 1
	}
	for i := prevIdx; i < a.length; i++ {
		if a.prototype != nil {
			if v := a.prototype.self.getIdx(valueInt(i), nil); v != nil {
				arr[i] = exportValue(v, ctx)
			}
		}
	}
	return arr
}

func (a *sparseArrayObject) exportType() reflect.Type {
	return reflectTypeArray
}

func (a *sparseArrayObject) exportToArrayOrSlice(dst reflect.Value, typ reflect.Type, ctx *objectExportCtx) error {
	r := a.val.runtime
	if iter := a.getSym(SymIterator, nil); iter == r.global.arrayValues || iter == nil {
		l := toIntStrict(int64(a.length))
		if dst.Len() != l {
			if typ.Kind() == reflect.Array {
				return fmt.Errorf("cannot convert an Array into an array, lengths mismatch (have %d, need %d)", l, dst.Len())
			} else {
				dst.Set(reflect.MakeSlice(typ, l, l))
			}
		}
		ctx.putTyped(a.val, typ, dst.Interface())
		for _, item := range a.items {
			val := item.value
			if p, ok := val.(*valueProperty); ok {
				val = p.get(a.val)
			}
			idx := toIntStrict(int64(item.idx))
			if idx >= l {
				break
			}
			err := r.toReflectValue(val, dst.Index(idx), ctx)
			if err != nil {
				return fmt.Errorf("could not convert array element %v to %v at %d: %w", item.value, typ, idx, err)
			}
		}
		return nil
	}
	return a.baseObject.exportToArrayOrSlice(dst, typ, ctx)
}
