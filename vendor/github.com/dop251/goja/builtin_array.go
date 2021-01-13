package goja

import (
	"math"
	"sort"
)

func (r *Runtime) newArray(prototype *Object) (a *arrayObject) {
	v := &Object{runtime: r}

	a = &arrayObject{}
	a.class = classArray
	a.val = v
	a.extensible = true
	v.self = a
	a.prototype = prototype
	a.init()
	return
}

func (r *Runtime) newArrayObject() *arrayObject {
	return r.newArray(r.global.ArrayPrototype)
}

func setArrayValues(a *arrayObject, values []Value) *arrayObject {
	a.values = values
	a.length = uint32(len(values))
	a.objCount = len(values)
	return a
}

func setArrayLength(a *arrayObject, l int64) *arrayObject {
	a.setOwnStr("length", intToValue(l), true)
	return a
}

func arraySpeciesCreate(obj *Object, size int64) *Object {
	if isArray(obj) {
		v := obj.self.getStr("constructor", nil)
		if constructObj, ok := v.(*Object); ok {
			v = constructObj.self.getSym(SymSpecies, nil)
			if v == _null {
				v = nil
			}
		}

		if v != nil && v != _undefined {
			constructObj, _ := v.(*Object)
			if constructObj != nil {
				if constructor := constructObj.self.assertConstructor(); constructor != nil {
					return constructor([]Value{intToValue(size)}, constructObj)
				}
			}
			panic(obj.runtime.NewTypeError("Species is not a constructor"))
		}
	}
	return obj.runtime.newArrayLength(size)
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func relToIdx(rel, l int64) int64 {
	if rel >= 0 {
		return min(rel, l)
	}
	return max(l+rel, 0)
}

func (r *Runtime) newArrayValues(values []Value) *Object {
	return setArrayValues(r.newArrayObject(), values).val
}

func (r *Runtime) newArrayLength(l int64) *Object {
	return setArrayLength(r.newArrayObject(), l).val
}

func (r *Runtime) builtin_newArray(args []Value, proto *Object) *Object {
	l := len(args)
	if l == 1 {
		if al, ok := args[0].(valueInt); ok {
			return setArrayLength(r.newArray(proto), int64(al)).val
		} else if f, ok := args[0].(valueFloat); ok {
			al := int64(f)
			if float64(al) == float64(f) {
				return r.newArrayLength(al)
			} else {
				panic(r.newError(r.global.RangeError, "Invalid array length"))
			}
		}
		return setArrayValues(r.newArray(proto), []Value{args[0]}).val
	} else {
		argsCopy := make([]Value, l)
		copy(argsCopy, args)
		return setArrayValues(r.newArray(proto), argsCopy).val
	}
}

func (r *Runtime) generic_push(obj *Object, call FunctionCall) Value {
	l := toLength(obj.self.getStr("length", nil))
	nl := l + int64(len(call.Arguments))
	if nl >= maxInt {
		r.typeErrorResult(true, "Invalid array length")
		panic("unreachable")
	}
	for i, arg := range call.Arguments {
		obj.self.setOwnIdx(valueInt(l+int64(i)), arg, true)
	}
	n := valueInt(nl)
	obj.self.setOwnStr("length", n, true)
	return n
}

func (r *Runtime) arrayproto_push(call FunctionCall) Value {
	obj := call.This.ToObject(r)
	return r.generic_push(obj, call)
}

func (r *Runtime) arrayproto_pop_generic(obj *Object) Value {
	l := toLength(obj.self.getStr("length", nil))
	if l == 0 {
		obj.self.setOwnStr("length", intToValue(0), true)
		return _undefined
	}
	idx := valueInt(l - 1)
	val := obj.self.getIdx(idx, nil)
	obj.self.deleteIdx(idx, true)
	obj.self.setOwnStr("length", idx, true)
	return val
}

func (r *Runtime) arrayproto_pop(call FunctionCall) Value {
	obj := call.This.ToObject(r)
	if a, ok := obj.self.(*arrayObject); ok {
		l := a.length
		if l > 0 {
			var val Value
			l--
			if l < uint32(len(a.values)) {
				val = a.values[l]
			}
			if val == nil {
				// optimisation bail-out
				return r.arrayproto_pop_generic(obj)
			}
			if _, ok := val.(*valueProperty); ok {
				// optimisation bail-out
				return r.arrayproto_pop_generic(obj)
			}
			//a._setLengthInt(l, false)
			a.values[l] = nil
			a.values = a.values[:l]
			a.length = l
			return val
		}
		return _undefined
	} else {
		return r.arrayproto_pop_generic(obj)
	}
}

func (r *Runtime) arrayproto_join(call FunctionCall) Value {
	o := call.This.ToObject(r)
	l := int(toLength(o.self.getStr("length", nil)))
	var sep valueString
	if s := call.Argument(0); s != _undefined {
		sep = s.toString()
	} else {
		sep = asciiString(",")
	}
	if l == 0 {
		return stringEmpty
	}

	var buf valueStringBuilder

	element0 := o.self.getIdx(valueInt(0), nil)
	if element0 != nil && element0 != _undefined && element0 != _null {
		buf.WriteString(element0.toString())
	}

	for i := 1; i < l; i++ {
		buf.WriteString(sep)
		element := o.self.getIdx(valueInt(int64(i)), nil)
		if element != nil && element != _undefined && element != _null {
			buf.WriteString(element.toString())
		}
	}

	return buf.String()
}

func (r *Runtime) arrayproto_toString(call FunctionCall) Value {
	array := call.This.ToObject(r)
	f := array.self.getStr("join", nil)
	if fObj, ok := f.(*Object); ok {
		if fcall, ok := fObj.self.assertCallable(); ok {
			return fcall(FunctionCall{
				This: array,
			})
		}
	}
	return r.objectproto_toString(FunctionCall{
		This: array,
	})
}

func (r *Runtime) writeItemLocaleString(item Value, buf *valueStringBuilder) {
	if item != nil && item != _undefined && item != _null {
		if f, ok := r.getVStr(item, "toLocaleString").(*Object); ok {
			if c, ok := f.self.assertCallable(); ok {
				strVal := c(FunctionCall{
					This: item,
				})
				buf.WriteString(strVal.toString())
				return
			}
		}
		r.typeErrorResult(true, "Property 'toLocaleString' of object %s is not a function", item)
	}
}

func (r *Runtime) arrayproto_toLocaleString(call FunctionCall) Value {
	array := call.This.ToObject(r)
	var buf valueStringBuilder
	if a := r.checkStdArrayObj(array); a != nil {
		for i, item := range a.values {
			if i > 0 {
				buf.WriteRune(',')
			}
			r.writeItemLocaleString(item, &buf)
		}
	} else {
		length := toLength(array.self.getStr("length", nil))
		for i := int64(0); i < length; i++ {
			if i > 0 {
				buf.WriteRune(',')
			}
			item := array.self.getIdx(valueInt(i), nil)
			r.writeItemLocaleString(item, &buf)
		}
	}

	return buf.String()
}

func isConcatSpreadable(obj *Object) bool {
	spreadable := obj.self.getSym(SymIsConcatSpreadable, nil)
	if spreadable != nil && spreadable != _undefined {
		return spreadable.ToBoolean()
	}
	return isArray(obj)
}

func (r *Runtime) arrayproto_concat_append(a *Object, item Value) {
	aLength := toLength(a.self.getStr("length", nil))
	if obj, ok := item.(*Object); ok && isConcatSpreadable(obj) {
		length := toLength(obj.self.getStr("length", nil))
		if aLength+length >= maxInt {
			panic(r.NewTypeError("Invalid array length"))
		}
		for i := int64(0); i < length; i++ {
			v := obj.self.getIdx(valueInt(i), nil)
			if v != nil {
				createDataPropertyOrThrow(a, intToValue(aLength), v)
			}
			aLength++
		}
	} else {
		createDataPropertyOrThrow(a, intToValue(aLength), item)
		aLength++
	}
	a.self.setOwnStr("length", intToValue(aLength), true)
}

func (r *Runtime) arrayproto_concat(call FunctionCall) Value {
	obj := call.This.ToObject(r)
	a := arraySpeciesCreate(obj, 0)
	r.arrayproto_concat_append(a, call.This.ToObject(r))
	for _, item := range call.Arguments {
		r.arrayproto_concat_append(a, item)
	}
	return a
}

func (r *Runtime) arrayproto_slice(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	start := relToIdx(call.Argument(0).ToInteger(), length)
	var end int64
	if endArg := call.Argument(1); endArg != _undefined {
		end = endArg.ToInteger()
	} else {
		end = length
	}
	end = relToIdx(end, length)

	count := end - start
	if count < 0 {
		count = 0
	}

	a := arraySpeciesCreate(o, count)
	if src := r.checkStdArrayObj(o); src != nil {
		if dst, ok := a.self.(*arrayObject); ok {
			values := make([]Value, count)
			copy(values, src.values[start:])
			setArrayValues(dst, values)
			return a
		}
	}

	n := int64(0)
	for start < end {
		p := o.self.getIdx(valueInt(start), nil)
		if p != nil {
			createDataPropertyOrThrow(a, valueInt(n), p)
		}
		start++
		n++
	}
	return a
}

func (r *Runtime) arrayproto_sort(call FunctionCall) Value {
	o := call.This.ToObject(r)

	var compareFn func(FunctionCall) Value
	arg := call.Argument(0)
	if arg != _undefined {
		if arg, ok := call.Argument(0).(*Object); ok {
			compareFn, _ = arg.self.assertCallable()
		}
		if compareFn == nil {
			panic(r.NewTypeError("The comparison function must be either a function or undefined"))
		}
	}

	ctx := arraySortCtx{
		obj:     o.self,
		compare: compareFn,
	}

	sort.Sort(&ctx)
	return o
}

func (r *Runtime) arrayproto_splice(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	actualStart := relToIdx(call.Argument(0).ToInteger(), length)
	var actualDeleteCount int64
	switch len(call.Arguments) {
	case 0:
	case 1:
		actualDeleteCount = length - actualStart
	default:
		actualDeleteCount = min(max(call.Argument(1).ToInteger(), 0), length-actualStart)
	}
	a := arraySpeciesCreate(o, actualDeleteCount)
	itemCount := max(int64(len(call.Arguments)-2), 0)
	newLength := length - actualDeleteCount + itemCount
	if src := r.checkStdArrayObj(o); src != nil {
		if dst, ok := a.self.(*arrayObject); ok {
			values := make([]Value, actualDeleteCount)
			copy(values, src.values[actualStart:])
			setArrayValues(dst, values)
		} else {
			for k := int64(0); k < actualDeleteCount; k++ {
				createDataPropertyOrThrow(a, intToValue(k), src.values[k+actualStart])
			}
		}
		var values []Value
		if itemCount < actualDeleteCount {
			values = src.values
			copy(values[actualStart+itemCount:], values[actualStart+actualDeleteCount:])
			tail := values[newLength:]
			for k := range tail {
				tail[k] = nil
			}
			values = values[:newLength]
		} else if itemCount > actualDeleteCount {
			if int64(cap(src.values)) >= newLength {
				values = src.values[:newLength]
				copy(values[actualStart+itemCount:], values[actualStart+actualDeleteCount:length])
			} else {
				values = make([]Value, newLength)
				copy(values, src.values[:actualStart])
				copy(values[actualStart+itemCount:], src.values[actualStart+actualDeleteCount:])
			}
		} else {
			values = src.values
		}
		if itemCount > 0 {
			copy(values[actualStart:], call.Arguments[2:])
		}
		src.values = values
		src.objCount = len(values)
	} else {
		for k := int64(0); k < actualDeleteCount; k++ {
			from := valueInt(k + actualStart)
			if o.self.hasPropertyIdx(from) {
				createDataPropertyOrThrow(a, valueInt(k), o.self.getIdx(from, nil))
			}
		}

		if itemCount < actualDeleteCount {
			for k := actualStart; k < length-actualDeleteCount; k++ {
				from := valueInt(k + actualDeleteCount)
				to := valueInt(k + itemCount)
				if o.self.hasPropertyIdx(from) {
					o.self.setOwnIdx(to, o.self.getIdx(from, nil), true)
				} else {
					o.self.deleteIdx(to, true)
				}
			}

			for k := length; k > length-actualDeleteCount+itemCount; k-- {
				o.self.deleteIdx(valueInt(k-1), true)
			}
		} else if itemCount > actualDeleteCount {
			for k := length - actualDeleteCount; k > actualStart; k-- {
				from := valueInt(k + actualDeleteCount - 1)
				to := valueInt(k + itemCount - 1)
				if o.self.hasPropertyIdx(from) {
					o.self.setOwnIdx(to, o.self.getIdx(from, nil), true)
				} else {
					o.self.deleteIdx(to, true)
				}
			}
		}

		if itemCount > 0 {
			for i, item := range call.Arguments[2:] {
				o.self.setOwnIdx(valueInt(actualStart+int64(i)), item, true)
			}
		}
	}

	o.self.setOwnStr("length", intToValue(newLength), true)

	return a
}

func (r *Runtime) arrayproto_unshift(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	argCount := int64(len(call.Arguments))
	newLen := intToValue(length + argCount)
	newSize := length + argCount
	if arr := r.checkStdArrayObj(o); arr != nil && newSize < math.MaxUint32 {
		if int64(cap(arr.values)) >= newSize {
			arr.values = arr.values[:newSize]
			copy(arr.values[argCount:], arr.values[:length])
		} else {
			values := make([]Value, newSize)
			copy(values[argCount:], arr.values)
			arr.values = values
		}
		copy(arr.values, call.Arguments)
		arr.objCount = int(arr.length)
	} else {
		for k := length - 1; k >= 0; k-- {
			from := valueInt(k)
			to := valueInt(k + argCount)
			if o.self.hasPropertyIdx(from) {
				o.self.setOwnIdx(to, o.self.getIdx(from, nil), true)
			} else {
				o.self.deleteIdx(to, true)
			}
		}

		for k, arg := range call.Arguments {
			o.self.setOwnIdx(valueInt(int64(k)), arg, true)
		}
	}

	o.self.setOwnStr("length", newLen, true)
	return newLen
}

func (r *Runtime) arrayproto_indexOf(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	if length == 0 {
		return intToValue(-1)
	}

	n := call.Argument(1).ToInteger()
	if n >= length {
		return intToValue(-1)
	}

	if n < 0 {
		n = max(length+n, 0)
	}

	searchElement := call.Argument(0)

	if arr := r.checkStdArrayObj(o); arr != nil {
		for i, val := range arr.values[n:] {
			if searchElement.StrictEquals(val) {
				return intToValue(n + int64(i))
			}
		}
		return intToValue(-1)
	}

	for ; n < length; n++ {
		idx := valueInt(n)
		if o.self.hasPropertyIdx(idx) {
			if val := o.self.getIdx(idx, nil); val != nil {
				if searchElement.StrictEquals(val) {
					return idx
				}
			}
		}
	}

	return intToValue(-1)
}

func (r *Runtime) arrayproto_includes(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	if length == 0 {
		return valueFalse
	}

	n := call.Argument(1).ToInteger()
	if n >= length {
		return valueFalse
	}

	if n < 0 {
		n = max(length+n, 0)
	}

	searchElement := call.Argument(0)
	if searchElement == _negativeZero {
		searchElement = _positiveZero
	}

	if arr := r.checkStdArrayObj(o); arr != nil {
		for _, val := range arr.values[n:] {
			if searchElement.SameAs(val) {
				return valueTrue
			}
		}
		return valueFalse
	}

	for ; n < length; n++ {
		idx := valueInt(n)
		val := nilSafe(o.self.getIdx(idx, nil))
		if searchElement.SameAs(val) {
			return valueTrue
		}
	}

	return valueFalse
}

func (r *Runtime) arrayproto_lastIndexOf(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	if length == 0 {
		return intToValue(-1)
	}

	var fromIndex int64

	if len(call.Arguments) < 2 {
		fromIndex = length - 1
	} else {
		fromIndex = call.Argument(1).ToInteger()
		if fromIndex >= 0 {
			fromIndex = min(fromIndex, length-1)
		} else {
			fromIndex += length
		}
	}

	searchElement := call.Argument(0)

	if arr := r.checkStdArrayObj(o); arr != nil {
		vals := arr.values
		for k := fromIndex; k >= 0; k-- {
			if v := vals[k]; v != nil && searchElement.StrictEquals(v) {
				return intToValue(k)
			}
		}
		return intToValue(-1)
	}

	for k := fromIndex; k >= 0; k-- {
		idx := valueInt(k)
		if o.self.hasPropertyIdx(idx) {
			if val := o.self.getIdx(idx, nil); val != nil {
				if searchElement.StrictEquals(val) {
					return idx
				}
			}
		}
	}

	return intToValue(-1)
}

func (r *Runtime) arrayproto_every(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	callbackFn := r.toCallable(call.Argument(0))
	fc := FunctionCall{
		This:      call.Argument(1),
		Arguments: []Value{nil, nil, o},
	}
	for k := int64(0); k < length; k++ {
		idx := valueInt(k)
		if val := o.self.getIdx(idx, nil); val != nil {
			fc.Arguments[0] = val
			fc.Arguments[1] = idx
			if !callbackFn(fc).ToBoolean() {
				return valueFalse
			}
		}
	}
	return valueTrue
}

func (r *Runtime) arrayproto_some(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	callbackFn := r.toCallable(call.Argument(0))
	fc := FunctionCall{
		This:      call.Argument(1),
		Arguments: []Value{nil, nil, o},
	}
	for k := int64(0); k < length; k++ {
		idx := valueInt(k)
		if val := o.self.getIdx(idx, nil); val != nil {
			fc.Arguments[0] = val
			fc.Arguments[1] = idx
			if callbackFn(fc).ToBoolean() {
				return valueTrue
			}
		}
	}
	return valueFalse
}

func (r *Runtime) arrayproto_forEach(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	callbackFn := r.toCallable(call.Argument(0))
	fc := FunctionCall{
		This:      call.Argument(1),
		Arguments: []Value{nil, nil, o},
	}
	for k := int64(0); k < length; k++ {
		idx := valueInt(k)
		if val := o.self.getIdx(idx, nil); val != nil {
			fc.Arguments[0] = val
			fc.Arguments[1] = idx
			callbackFn(fc)
		}
	}
	return _undefined
}

func (r *Runtime) arrayproto_map(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	callbackFn := r.toCallable(call.Argument(0))
	fc := FunctionCall{
		This:      call.Argument(1),
		Arguments: []Value{nil, nil, o},
	}
	a := arraySpeciesCreate(o, length)
	if _, stdSrc := o.self.(*arrayObject); stdSrc {
		if arr, ok := a.self.(*arrayObject); ok {
			values := make([]Value, length)
			for k := int64(0); k < length; k++ {
				idx := valueInt(k)
				if val := o.self.getIdx(idx, nil); val != nil {
					fc.Arguments[0] = val
					fc.Arguments[1] = idx
					values[k] = callbackFn(fc)
				}
			}
			setArrayValues(arr, values)
			return a
		}
	}
	for k := int64(0); k < length; k++ {
		idx := valueInt(k)
		if val := o.self.getIdx(idx, nil); val != nil {
			fc.Arguments[0] = val
			fc.Arguments[1] = idx
			createDataPropertyOrThrow(a, idx, callbackFn(fc))
		}
	}
	return a
}

func (r *Runtime) arrayproto_filter(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	callbackFn := call.Argument(0).ToObject(r)
	if callbackFn, ok := callbackFn.self.assertCallable(); ok {
		a := arraySpeciesCreate(o, 0)
		fc := FunctionCall{
			This:      call.Argument(1),
			Arguments: []Value{nil, nil, o},
		}
		if _, stdSrc := o.self.(*arrayObject); stdSrc {
			if arr := r.checkStdArrayObj(a); arr != nil {
				var values []Value
				for k := int64(0); k < length; k++ {
					idx := valueInt(k)
					if val := o.self.getIdx(idx, nil); val != nil {
						fc.Arguments[0] = val
						fc.Arguments[1] = idx
						if callbackFn(fc).ToBoolean() {
							values = append(values, val)
						}
					}
				}
				setArrayValues(arr, values)
				return a
			}
		}

		to := int64(0)
		for k := int64(0); k < length; k++ {
			idx := valueInt(k)
			if val := o.self.getIdx(idx, nil); val != nil {
				fc.Arguments[0] = val
				fc.Arguments[1] = idx
				if callbackFn(fc).ToBoolean() {
					createDataPropertyOrThrow(a, intToValue(to), val)
					to++
				}
			}
		}
		return a
	} else {
		r.typeErrorResult(true, "%s is not a function", call.Argument(0))
	}
	panic("unreachable")
}

func (r *Runtime) arrayproto_reduce(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	callbackFn := call.Argument(0).ToObject(r)
	if callbackFn, ok := callbackFn.self.assertCallable(); ok {
		fc := FunctionCall{
			This:      _undefined,
			Arguments: []Value{nil, nil, nil, o},
		}

		var k int64

		if len(call.Arguments) >= 2 {
			fc.Arguments[0] = call.Argument(1)
		} else {
			for ; k < length; k++ {
				idx := valueInt(k)
				if val := o.self.getIdx(idx, nil); val != nil {
					fc.Arguments[0] = val
					break
				}
			}
			if fc.Arguments[0] == nil {
				r.typeErrorResult(true, "No initial value")
				panic("unreachable")
			}
			k++
		}

		for ; k < length; k++ {
			idx := valueInt(k)
			if val := o.self.getIdx(idx, nil); val != nil {
				fc.Arguments[1] = val
				fc.Arguments[2] = idx
				fc.Arguments[0] = callbackFn(fc)
			}
		}
		return fc.Arguments[0]
	} else {
		r.typeErrorResult(true, "%s is not a function", call.Argument(0))
	}
	panic("unreachable")
}

func (r *Runtime) arrayproto_reduceRight(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	callbackFn := call.Argument(0).ToObject(r)
	if callbackFn, ok := callbackFn.self.assertCallable(); ok {
		fc := FunctionCall{
			This:      _undefined,
			Arguments: []Value{nil, nil, nil, o},
		}

		k := length - 1

		if len(call.Arguments) >= 2 {
			fc.Arguments[0] = call.Argument(1)
		} else {
			for ; k >= 0; k-- {
				idx := valueInt(k)
				if val := o.self.getIdx(idx, nil); val != nil {
					fc.Arguments[0] = val
					break
				}
			}
			if fc.Arguments[0] == nil {
				r.typeErrorResult(true, "No initial value")
				panic("unreachable")
			}
			k--
		}

		for ; k >= 0; k-- {
			idx := valueInt(k)
			if val := o.self.getIdx(idx, nil); val != nil {
				fc.Arguments[1] = val
				fc.Arguments[2] = idx
				fc.Arguments[0] = callbackFn(fc)
			}
		}
		return fc.Arguments[0]
	} else {
		r.typeErrorResult(true, "%s is not a function", call.Argument(0))
	}
	panic("unreachable")
}

func arrayproto_reverse_generic_step(o *Object, lower, upper int64) {
	lowerP := valueInt(lower)
	upperP := valueInt(upper)
	lowerValue := o.self.getIdx(lowerP, nil)
	upperValue := o.self.getIdx(upperP, nil)
	if lowerValue != nil && upperValue != nil {
		o.self.setOwnIdx(lowerP, upperValue, true)
		o.self.setOwnIdx(upperP, lowerValue, true)
	} else if lowerValue == nil && upperValue != nil {
		o.self.setOwnIdx(lowerP, upperValue, true)
		o.self.deleteIdx(upperP, true)
	} else if lowerValue != nil && upperValue == nil {
		o.self.deleteIdx(lowerP, true)
		o.self.setOwnIdx(upperP, lowerValue, true)
	}
}

func (r *Runtime) arrayproto_reverse_generic(o *Object, start int64) {
	l := toLength(o.self.getStr("length", nil))
	middle := l / 2
	for lower := start; lower != middle; lower++ {
		arrayproto_reverse_generic_step(o, lower, l-lower-1)
	}
}

func (r *Runtime) arrayproto_reverse(call FunctionCall) Value {
	o := call.This.ToObject(r)
	if a := r.checkStdArrayObj(o); a != nil {
		l := len(a.values)
		middle := l / 2
		for lower := 0; lower != middle; lower++ {
			upper := l - lower - 1
			a.values[lower], a.values[upper] = a.values[upper], a.values[lower]
		}
		//TODO: go arrays
	} else {
		r.arrayproto_reverse_generic(o, 0)
	}
	return o
}

func (r *Runtime) arrayproto_shift(call FunctionCall) Value {
	o := call.This.ToObject(r)
	length := toLength(o.self.getStr("length", nil))
	if length == 0 {
		o.self.setOwnStr("length", intToValue(0), true)
		return _undefined
	}
	first := o.self.getIdx(valueInt(0), nil)
	for i := int64(1); i < length; i++ {
		v := o.self.getIdx(valueInt(i), nil)
		if v != nil {
			o.self.setOwnIdx(valueInt(i-1), v, true)
		} else {
			o.self.deleteIdx(valueInt(i-1), true)
		}
	}

	lv := valueInt(length - 1)
	o.self.deleteIdx(lv, true)
	o.self.setOwnStr("length", lv, true)

	return first
}

func (r *Runtime) arrayproto_values(call FunctionCall) Value {
	return r.createArrayIterator(call.This.ToObject(r), iterationKindValue)
}

func (r *Runtime) arrayproto_keys(call FunctionCall) Value {
	return r.createArrayIterator(call.This.ToObject(r), iterationKindKey)
}

func (r *Runtime) arrayproto_copyWithin(call FunctionCall) Value {
	o := call.This.ToObject(r)
	l := toLength(o.self.getStr("length", nil))
	var relEnd, dir int64
	to := relToIdx(call.Argument(0).ToInteger(), l)
	from := relToIdx(call.Argument(1).ToInteger(), l)
	if end := call.Argument(2); end != _undefined {
		relEnd = end.ToInteger()
	} else {
		relEnd = l
	}
	final := relToIdx(relEnd, l)
	count := min(final-from, l-to)
	if arr := r.checkStdArrayObj(o); arr != nil {
		if count > 0 {
			copy(arr.values[to:to+count], arr.values[from:from+count])
		}
		return o
	}
	if from < to && to < from+count {
		dir = -1
		from = from + count - 1
		to = to + count - 1
	} else {
		dir = 1
	}
	for count > 0 {
		if o.self.hasPropertyIdx(valueInt(from)) {
			o.self.setOwnIdx(valueInt(to), o.self.getIdx(valueInt(from), nil), true)
		} else {
			o.self.deleteIdx(valueInt(to), true)
		}
		from += dir
		to += dir
		count--
	}

	return o
}

func (r *Runtime) arrayproto_entries(call FunctionCall) Value {
	return r.createArrayIterator(call.This.ToObject(r), iterationKindKeyValue)
}

func (r *Runtime) arrayproto_fill(call FunctionCall) Value {
	o := call.This.ToObject(r)
	l := toLength(o.self.getStr("length", nil))
	k := relToIdx(call.Argument(1).ToInteger(), l)
	var relEnd int64
	if endArg := call.Argument(2); endArg != _undefined {
		relEnd = endArg.ToInteger()
	} else {
		relEnd = l
	}
	final := relToIdx(relEnd, l)
	value := call.Argument(0)
	if arr := r.checkStdArrayObj(o); arr != nil {
		for ; k < final; k++ {
			arr.values[k] = value
		}
	} else {
		for ; k < final; k++ {
			o.self.setOwnIdx(valueInt(k), value, true)
		}
	}
	return o
}

func (r *Runtime) arrayproto_find(call FunctionCall) Value {
	o := call.This.ToObject(r)
	l := toLength(o.self.getStr("length", nil))
	predicate := r.toCallable(call.Argument(0))
	fc := FunctionCall{
		This:      call.Argument(1),
		Arguments: []Value{nil, nil, o},
	}
	for k := int64(0); k < l; k++ {
		idx := valueInt(k)
		kValue := o.self.getIdx(idx, nil)
		fc.Arguments[0], fc.Arguments[1] = kValue, idx
		if predicate(fc).ToBoolean() {
			return kValue
		}
	}

	return _undefined
}

func (r *Runtime) arrayproto_findIndex(call FunctionCall) Value {
	o := call.This.ToObject(r)
	l := toLength(o.self.getStr("length", nil))
	predicate := r.toCallable(call.Argument(0))
	fc := FunctionCall{
		This:      call.Argument(1),
		Arguments: []Value{nil, nil, o},
	}
	for k := int64(0); k < l; k++ {
		idx := valueInt(k)
		kValue := o.self.getIdx(idx, nil)
		fc.Arguments[0], fc.Arguments[1] = kValue, idx
		if predicate(fc).ToBoolean() {
			return idx
		}
	}

	return intToValue(-1)
}

func (r *Runtime) arrayproto_flat(call FunctionCall) Value {
	o := call.This.ToObject(r)
	l := toLength(o.self.getStr("length", nil))
	depthNum := int64(1)
	if len(call.Arguments) > 0 {
		depthNum = call.Argument(0).ToInteger()
	}
	a := arraySpeciesCreate(o, 0)
	r.flattenIntoArray(a, o, l, 0, depthNum, nil, nil)
	return a
}

func (r *Runtime) flattenIntoArray(target, source *Object, sourceLen, start, depth int64, mapperFunction func(FunctionCall) Value, thisArg Value) int64 {
	targetIndex, sourceIndex := start, int64(0)
	for sourceIndex < sourceLen {
		p := intToValue(sourceIndex)
		if source.hasProperty(p.toString()) {
			element := source.get(p, source)
			if mapperFunction != nil {
				element = mapperFunction(FunctionCall{
					This:      thisArg,
					Arguments: []Value{element, p, source},
				})
			}
			var elementArray *Object
			if depth > 0 {
				if elementObj, ok := element.(*Object); ok && isArray(elementObj) {
					elementArray = elementObj
				}
			}
			if elementArray != nil {
				elementLen := toLength(elementArray.self.getStr("length", nil))
				targetIndex = r.flattenIntoArray(target, elementArray, elementLen, targetIndex, depth-1, nil, nil)
			} else {
				if targetIndex >= maxInt-1 {
					panic(r.NewTypeError("Invalid array length"))
				}
				createDataPropertyOrThrow(target, intToValue(targetIndex), element)
				targetIndex++
			}
		}
		sourceIndex++
	}
	return targetIndex
}

func (r *Runtime) arrayproto_flatMap(call FunctionCall) Value {
	o := call.This.ToObject(r)
	l := toLength(o.self.getStr("length", nil))
	callbackFn := r.toCallable(call.Argument(0))
	thisArg := Undefined()
	if len(call.Arguments) > 1 {
		thisArg = call.Argument(1)
	}
	a := arraySpeciesCreate(o, 0)
	r.flattenIntoArray(a, o, l, 0, 1, callbackFn, thisArg)
	return a
}

func (r *Runtime) checkStdArrayObj(obj *Object) *arrayObject {
	if arr, ok := obj.self.(*arrayObject); ok &&
		arr.propValueCount == 0 &&
		arr.length == uint32(len(arr.values)) &&
		uint32(arr.objCount) == arr.length {

		return arr
	}

	return nil
}

func (r *Runtime) checkStdArray(v Value) *arrayObject {
	if obj, ok := v.(*Object); ok {
		return r.checkStdArrayObj(obj)
	}

	return nil
}

func (r *Runtime) checkStdArrayIter(v Value) *arrayObject {
	if arr := r.checkStdArray(v); arr != nil &&
		arr.getSym(SymIterator, nil) == r.global.arrayValues {

		return arr
	}

	return nil
}

func (r *Runtime) array_from(call FunctionCall) Value {
	var mapFn func(FunctionCall) Value
	if mapFnArg := call.Argument(1); mapFnArg != _undefined {
		if mapFnObj, ok := mapFnArg.(*Object); ok {
			if fn, ok := mapFnObj.self.assertCallable(); ok {
				mapFn = fn
			}
		}
		if mapFn == nil {
			panic(r.NewTypeError("%s is not a function", mapFnArg))
		}
	}
	t := call.Argument(2)
	items := call.Argument(0)
	if mapFn == nil && call.This == r.global.Array { // mapFn may mutate the array
		if arr := r.checkStdArrayIter(items); arr != nil {
			items := make([]Value, len(arr.values))
			copy(items, arr.values)
			return r.newArrayValues(items)
		}
	}

	var ctor func(args []Value, newTarget *Object) *Object
	if call.This != r.global.Array {
		if o, ok := call.This.(*Object); ok {
			if c := o.self.assertConstructor(); c != nil {
				ctor = c
			}
		}
	}
	var arr *Object
	if usingIterator := toMethod(r.getV(items, SymIterator)); usingIterator != nil {
		if ctor != nil {
			arr = ctor([]Value{}, nil)
		} else {
			arr = r.newArrayValues(nil)
		}
		iter := r.getIterator(items, usingIterator)
		if mapFn == nil {
			if a := r.checkStdArrayObj(arr); a != nil {
				var values []Value
				r.iterate(iter, func(val Value) {
					values = append(values, val)
				})
				setArrayValues(a, values)
				return arr
			}
		}
		k := int64(0)
		r.iterate(iter, func(val Value) {
			if mapFn != nil {
				val = mapFn(FunctionCall{This: t, Arguments: []Value{val, intToValue(k)}})
			}
			createDataPropertyOrThrow(arr, intToValue(k), val)
			k++
		})
		arr.self.setOwnStr("length", intToValue(k), true)
	} else {
		arrayLike := items.ToObject(r)
		l := toLength(arrayLike.self.getStr("length", nil))
		if ctor != nil {
			arr = ctor([]Value{intToValue(l)}, nil)
		} else {
			arr = r.newArrayValues(nil)
		}
		if mapFn == nil {
			if a := r.checkStdArrayObj(arr); a != nil {
				values := make([]Value, l)
				for k := int64(0); k < l; k++ {
					values[k] = nilSafe(arrayLike.self.getIdx(valueInt(k), nil))
				}
				setArrayValues(a, values)
				return arr
			}
		}
		for k := int64(0); k < l; k++ {
			idx := valueInt(k)
			item := arrayLike.self.getIdx(idx, nil)
			if mapFn != nil {
				item = mapFn(FunctionCall{This: t, Arguments: []Value{item, idx}})
			} else {
				item = nilSafe(item)
			}
			createDataPropertyOrThrow(arr, idx, item)
		}
		arr.self.setOwnStr("length", intToValue(l), true)
	}

	return arr
}

func (r *Runtime) array_isArray(call FunctionCall) Value {
	if o, ok := call.Argument(0).(*Object); ok {
		if isArray(o) {
			return valueTrue
		}
	}
	return valueFalse
}

func (r *Runtime) array_of(call FunctionCall) Value {
	var ctor func(args []Value, newTarget *Object) *Object
	if call.This != r.global.Array {
		if o, ok := call.This.(*Object); ok {
			if c := o.self.assertConstructor(); c != nil {
				ctor = c
			}
		}
	}
	if ctor == nil {
		values := make([]Value, len(call.Arguments))
		copy(values, call.Arguments)
		return r.newArrayValues(values)
	}
	l := intToValue(int64(len(call.Arguments)))
	arr := ctor([]Value{l}, nil)
	for i, val := range call.Arguments {
		createDataPropertyOrThrow(arr, intToValue(int64(i)), val)
	}
	arr.self.setOwnStr("length", l, true)
	return arr
}

func (r *Runtime) arrayIterProto_next(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	if iter, ok := thisObj.self.(*arrayIterObject); ok {
		return iter.next()
	}
	panic(r.NewTypeError("Method Array Iterator.prototype.next called on incompatible receiver %s", thisObj.String()))
}

func (r *Runtime) createArrayProto(val *Object) objectImpl {
	o := &arrayObject{
		baseObject: baseObject{
			class:      classArray,
			val:        val,
			extensible: true,
			prototype:  r.global.ObjectPrototype,
		},
	}
	o.init()

	o._putProp("constructor", r.global.Array, true, false, true)
	o._putProp("concat", r.newNativeFunc(r.arrayproto_concat, nil, "concat", nil, 1), true, false, true)
	o._putProp("copyWithin", r.newNativeFunc(r.arrayproto_copyWithin, nil, "copyWithin", nil, 2), true, false, true)
	o._putProp("entries", r.newNativeFunc(r.arrayproto_entries, nil, "entries", nil, 0), true, false, true)
	o._putProp("every", r.newNativeFunc(r.arrayproto_every, nil, "every", nil, 1), true, false, true)
	o._putProp("fill", r.newNativeFunc(r.arrayproto_fill, nil, "fill", nil, 1), true, false, true)
	o._putProp("filter", r.newNativeFunc(r.arrayproto_filter, nil, "filter", nil, 1), true, false, true)
	o._putProp("find", r.newNativeFunc(r.arrayproto_find, nil, "find", nil, 1), true, false, true)
	o._putProp("findIndex", r.newNativeFunc(r.arrayproto_findIndex, nil, "findIndex", nil, 1), true, false, true)
	o._putProp("flat", r.newNativeFunc(r.arrayproto_flat, nil, "flat", nil, 0), true, false, true)
	o._putProp("flatMap", r.newNativeFunc(r.arrayproto_flatMap, nil, "flatMap", nil, 1), true, false, true)
	o._putProp("forEach", r.newNativeFunc(r.arrayproto_forEach, nil, "forEach", nil, 1), true, false, true)
	o._putProp("includes", r.newNativeFunc(r.arrayproto_includes, nil, "includes", nil, 1), true, false, true)
	o._putProp("indexOf", r.newNativeFunc(r.arrayproto_indexOf, nil, "indexOf", nil, 1), true, false, true)
	o._putProp("join", r.newNativeFunc(r.arrayproto_join, nil, "join", nil, 1), true, false, true)
	o._putProp("keys", r.newNativeFunc(r.arrayproto_keys, nil, "keys", nil, 0), true, false, true)
	o._putProp("lastIndexOf", r.newNativeFunc(r.arrayproto_lastIndexOf, nil, "lastIndexOf", nil, 1), true, false, true)
	o._putProp("map", r.newNativeFunc(r.arrayproto_map, nil, "map", nil, 1), true, false, true)
	o._putProp("pop", r.newNativeFunc(r.arrayproto_pop, nil, "pop", nil, 0), true, false, true)
	o._putProp("push", r.newNativeFunc(r.arrayproto_push, nil, "push", nil, 1), true, false, true)
	o._putProp("reduce", r.newNativeFunc(r.arrayproto_reduce, nil, "reduce", nil, 1), true, false, true)
	o._putProp("reduceRight", r.newNativeFunc(r.arrayproto_reduceRight, nil, "reduceRight", nil, 1), true, false, true)
	o._putProp("reverse", r.newNativeFunc(r.arrayproto_reverse, nil, "reverse", nil, 0), true, false, true)
	o._putProp("shift", r.newNativeFunc(r.arrayproto_shift, nil, "shift", nil, 0), true, false, true)
	o._putProp("slice", r.newNativeFunc(r.arrayproto_slice, nil, "slice", nil, 2), true, false, true)
	o._putProp("some", r.newNativeFunc(r.arrayproto_some, nil, "some", nil, 1), true, false, true)
	o._putProp("sort", r.newNativeFunc(r.arrayproto_sort, nil, "sort", nil, 1), true, false, true)
	o._putProp("splice", r.newNativeFunc(r.arrayproto_splice, nil, "splice", nil, 2), true, false, true)
	o._putProp("toLocaleString", r.newNativeFunc(r.arrayproto_toLocaleString, nil, "toLocaleString", nil, 0), true, false, true)
	o._putProp("toString", r.global.arrayToString, true, false, true)
	o._putProp("unshift", r.newNativeFunc(r.arrayproto_unshift, nil, "unshift", nil, 1), true, false, true)
	valuesFunc := r.newNativeFunc(r.arrayproto_values, nil, "values", nil, 0)
	r.global.arrayValues = valuesFunc
	o._putProp("values", valuesFunc, true, false, true)

	o._putSym(SymIterator, valueProp(valuesFunc, true, false, true))

	bl := r.newBaseObject(nil, classObject)
	bl.setOwnStr("copyWithin", valueTrue, true)
	bl.setOwnStr("entries", valueTrue, true)
	bl.setOwnStr("fill", valueTrue, true)
	bl.setOwnStr("find", valueTrue, true)
	bl.setOwnStr("findIndex", valueTrue, true)
	bl.setOwnStr("flat", valueTrue, true)
	bl.setOwnStr("flatMap", valueTrue, true)
	bl.setOwnStr("includes", valueTrue, true)
	bl.setOwnStr("keys", valueTrue, true)
	bl.setOwnStr("values", valueTrue, true)
	o._putSym(SymUnscopables, valueProp(bl.val, false, false, true))

	return o
}

func (r *Runtime) createArray(val *Object) objectImpl {
	o := r.newNativeFuncConstructObj(val, r.builtin_newArray, "Array", r.global.ArrayPrototype, 1)
	o._putProp("from", r.newNativeFunc(r.array_from, nil, "from", nil, 1), true, false, true)
	o._putProp("isArray", r.newNativeFunc(r.array_isArray, nil, "isArray", nil, 1), true, false, true)
	o._putProp("of", r.newNativeFunc(r.array_of, nil, "of", nil, 0), true, false, true)
	o._putSym(SymSpecies, &valueProperty{
		getterFunc:   r.newNativeFunc(r.returnThis, nil, "get [Symbol.species]", nil, 0),
		accessor:     true,
		configurable: true,
	})

	return o
}

func (r *Runtime) createArrayIterProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.global.IteratorPrototype, classObject)

	o._putProp("next", r.newNativeFunc(r.arrayIterProto_next, nil, "next", nil, 0), true, false, true)
	o._putSym(SymToStringTag, valueProp(asciiString(classArrayIterator), false, false, true))

	return o
}

func (r *Runtime) initArray() {
	r.global.arrayToString = r.newNativeFunc(r.arrayproto_toString, nil, "toString", nil, 0)

	r.global.ArrayIteratorPrototype = r.newLazyObject(r.createArrayIterProto)
	//r.global.ArrayPrototype = r.newArray(r.global.ObjectPrototype).val
	//o := r.global.ArrayPrototype.self
	r.global.ArrayPrototype = r.newLazyObject(r.createArrayProto)

	//r.global.Array = r.newNativeFuncConstruct(r.builtin_newArray, "Array", r.global.ArrayPrototype, 1)
	//o = r.global.Array.self
	//o._putProp("isArray", r.newNativeFunc(r.array_isArray, nil, "isArray", nil, 1), true, false, true)
	r.global.Array = r.newLazyObject(r.createArray)

	r.addToGlobal("Array", r.global.Array)
}

type sortable interface {
	sortLen() int64
	sortGet(int64) Value
	swap(int64, int64)
}

type arraySortCtx struct {
	obj     sortable
	compare func(FunctionCall) Value
}

func (a *arraySortCtx) sortCompare(x, y Value) int {
	if x == nil && y == nil {
		return 0
	}

	if x == nil {
		return 1
	}

	if y == nil {
		return -1
	}

	if x == _undefined && y == _undefined {
		return 0
	}

	if x == _undefined {
		return 1
	}

	if y == _undefined {
		return -1
	}

	if a.compare != nil {
		f := a.compare(FunctionCall{
			This:      _undefined,
			Arguments: []Value{x, y},
		}).ToFloat()
		if f > 0 {
			return 1
		}
		if f < 0 {
			return -1
		}
		if math.Signbit(f) {
			return -1
		}
		return 0
	}
	return x.toString().compareTo(y.toString())
}

// sort.Interface

func (a *arraySortCtx) Len() int {
	return int(a.obj.sortLen())
}

func (a *arraySortCtx) Less(j, k int) bool {
	return a.sortCompare(a.obj.sortGet(int64(j)), a.obj.sortGet(int64(k))) < 0
}

func (a *arraySortCtx) Swap(j, k int) {
	a.obj.swap(int64(j), int64(k))
}
