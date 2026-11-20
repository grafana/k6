package sobek

import (
	"math"

	"github.com/grafana/sobek/unistring"
)

type iteratorHelperState uint8

const (
	iteratorHelperSuspendedStart iteratorHelperState = iota
	iteratorHelperSuspendedYield
	iteratorHelperExecuting
	iteratorHelperCompleted
)

// iteratorHelperObject is the object returned by the lazy Iterator.prototype methods (map, filter, take, drop
// and flatMap). It behaves like a generator: step produces the next value, or reports that the helper is done.
type iteratorHelperObject struct {
	baseObject
	underlying *iteratorRecord
	step       func() (value Value, ok bool)
	// abort, if set, replaces the default closing of the underlying iterator in return().
	abort func()
	state iteratorHelperState
}

func (h *iteratorHelperObject) nextResult(_ Value) (Value, bool) {
	r := h.val.runtime
	switch h.state {
	case iteratorHelperExecuting:
		panic(r.NewTypeError("Iterator Helper is already running"))
	case iteratorHelperCompleted:
		return _undefined, false
	}
	h.state = iteratorHelperExecuting
	defer func() {
		if h.state == iteratorHelperExecuting {
			h.state = iteratorHelperCompleted
		}
	}()
	value, ok := h.step()
	if !ok {
		h.state = iteratorHelperCompleted
		return _undefined, false
	}
	h.state = iteratorHelperSuspendedYield
	return value, true
}

func (h *iteratorHelperObject) next() Value {
	value, valid := h.nextResult(nil)
	return h.val.runtime.createIterResultObject(value, !valid)
}

func (h *iteratorHelperObject) _return() Value {
	r := h.val.runtime
	switch h.state {
	case iteratorHelperExecuting:
		panic(r.NewTypeError("Iterator Helper is already running"))
	case iteratorHelperCompleted:
		return r.createIterResultObject(_undefined, true)
	case iteratorHelperSuspendedStart:
		h.state = iteratorHelperCompleted
		if h.abort != nil {
			h.abort()
		} else {
			h.underlying.returnIter()
		}
		return r.createIterResultObject(_undefined, true)
	}
	h.state = iteratorHelperExecuting
	defer func() {
		h.state = iteratorHelperCompleted
	}()
	if h.abort != nil {
		h.abort()
	} else {
		h.underlying.returnIter()
	}
	return r.createIterResultObject(_undefined, true)
}

// wrapForValidIteratorObject is the object returned by Iterator.from() for iterators that do not inherit from
// Iterator.prototype.
type wrapForValidIteratorObject struct {
	baseObject
	iterated *iteratorRecord
}

func (r *Runtime) newIteratorHelper(underlying *iteratorRecord, step func() (Value, bool)) *iteratorHelperObject {
	h := &iteratorHelperObject{
		underlying: underlying,
		step:       step,
	}
	o := &Object{runtime: r}
	h.class = classObject
	h.val = o
	h.extensible = true
	o.self = h
	h.prototype = r.getIteratorHelperPrototype()
	h.init()
	return h
}

func (r *Runtime) getIteratorDirect(obj *Object) *iteratorRecord {
	var next func(FunctionCall) Value
	var nextRes func(Value) (Value, bool)
	if nextObj, ok := obj.self.getStr("next", nil).(*Object); ok {
		if call, ok := nextObj.self.assertCallable(); ok {
			next = call
			if nf, ok := nextObj.self.(*iteratorNextFunction); ok {
				nextRes = nf.getNextResult(obj)
			}
		}
	}
	return &iteratorRecord{
		iterator: obj,
		next:     next,
		nextRes:  nextRes,
	}
}

func (r *Runtime) getIteratorFlattenable(obj Value, iterateStrings bool) *iteratorRecord {
	if _, ok := obj.(*Object); !ok {
		if _, isString := obj.(String); !isString || !iterateStrings {
			panic(r.NewTypeError("%s is not an object", obj.String()))
		}
	}
	var iter = obj
	if method := toMethod(r.getV(obj, SymIterator)); method != nil {
		iter = method(FunctionCall{This: obj})
	}
	return r.getIteratorDirect(r.toObject(iter))
}

// stepValue implements IteratorStepValue. It returns false if the iterator is exhausted.
func (ir *iteratorRecord) stepValue() (Value, bool) {
	r := ir.iterator.runtime
	if ir.next == nil {
		panic(r.NewTypeError("iterator.next is missing or not a function"))
	}
	if ir.nextRes != nil {
		return ir.nextRes(_undefined)
	}
	res := r.toObject(ir.next(FunctionCall{This: ir.iterator}))
	if iteratorComplete(res) {
		return nil, false
	}
	return iteratorValue(res), true
}

// stepDone implements IteratorStep. Unlike stepValue it does not read the value of the result.
func (ir *iteratorRecord) stepDone() bool {
	r := ir.iterator.runtime
	if ir.next == nil {
		panic(r.NewTypeError("iterator.next is missing or not a function"))
	}
	if ir.nextRes != nil {
		_, ok := ir.nextRes(_undefined)
		return !ok
	}
	return iteratorComplete(r.toObject(ir.next(FunctionCall{This: ir.iterator})))
}

// closeIterators implements IteratorCloseAll. The iterators are closed in reverse order, nil entries are skipped.
// If ex is not nil, any errors thrown while closing are ignored and ex is re-thrown, otherwise the first error
// (if any) is thrown after all the iterators have been closed.
func closeIterators(iters []*iteratorRecord, ex interface{}) {
	for i := len(iters) - 1; i >= 0; i-- {
		if iter := iters[i]; iter != nil {
			if ex != nil {
				_ = tryFunc(iter.returnIter)
			} else {
				ex = tryFunc(iter.returnIter)
			}
		}
	}
	if ex != nil {
		panic(ex)
	}
}

// closeOnThrow runs f and, if it throws, closes the iterator (ignoring any error from return()) before rethrowing.
func (ir *iteratorRecord) closeOnThrow(f func()) {
	if ex := tryFunc(f); ex != nil {
		_ = tryFunc(ir.returnIter)
		panic(ex)
	}
}

func (ir *iteratorRecord) callOrClose(f func(FunctionCall) Value, args ...Value) (ret Value) {
	ir.closeOnThrow(func() {
		ret = f(FunctionCall{This: _undefined, Arguments: args})
	})
	return
}

func (r *Runtime) thisIteratorObject(call FunctionCall, method string) *Object {
	if obj, ok := call.This.(*Object); ok {
		return obj
	}
	panic(r.NewTypeError("Method Iterator.prototype.%s called on incompatible receiver %s", method, call.This.String()))
}

// iteratorWithCallback implements the common prologue of the helpers taking a callback: the receiver is closed
// if the argument is not callable.
func (r *Runtime) iteratorWithCallback(call FunctionCall, method string) (*iteratorRecord, func(FunctionCall) Value) {
	thisObj := r.thisIteratorObject(call, method)
	arg := call.Argument(0)
	if obj, ok := arg.(*Object); ok {
		if callback, ok := obj.self.assertCallable(); ok {
			return r.getIteratorDirect(thisObj), callback
		}
	}
	(&iteratorRecord{iterator: thisObj}).closeOnThrow(func() {
		panic(r.NewTypeError("%s is not a function", arg.String()))
	})
	return nil, nil
}

// iteratorLimit validates the argument of take() and drop(), closing the receiver on failure.
func (r *Runtime) iteratorLimit(thisObj *Object, arg Value) float64 {
	iterated := &iteratorRecord{iterator: thisObj}
	var limit float64
	iterated.closeOnThrow(func() {
		limit = arg.ToNumber().ToFloat()
		if math.IsNaN(limit) {
			panic(r.newError(r.getRangeError(), "Iterator limit must be a number"))
		}
		limit = math.Trunc(limit)
		if limit < 0 {
			panic(r.newError(r.getRangeError(), "Iterator limit must not be negative"))
		}
	})
	return limit
}

func (r *Runtime) iteratorProto_map(call FunctionCall) Value {
	iterated, mapper := r.iteratorWithCallback(call, "map")
	var counter int64
	return r.newIteratorHelper(iterated, func() (Value, bool) {
		value, ok := iterated.stepValue()
		if !ok {
			return nil, false
		}
		mapped := iterated.callOrClose(mapper, value, intToValue(counter))
		counter++
		return mapped, true
	}).val
}

func (r *Runtime) iteratorProto_filter(call FunctionCall) Value {
	iterated, predicate := r.iteratorWithCallback(call, "filter")
	var counter int64
	return r.newIteratorHelper(iterated, func() (Value, bool) {
		for {
			value, ok := iterated.stepValue()
			if !ok {
				return nil, false
			}
			selected := iterated.callOrClose(predicate, value, intToValue(counter))
			counter++
			if selected.ToBoolean() {
				return value, true
			}
		}
	}).val
}

func (r *Runtime) iteratorProto_take(call FunctionCall) Value {
	thisObj := r.thisIteratorObject(call, "take")
	remaining := r.iteratorLimit(thisObj, call.Argument(0))
	iterated := r.getIteratorDirect(thisObj)
	return r.newIteratorHelper(iterated, func() (Value, bool) {
		if remaining == 0 {
			iterated.returnIter()
			return nil, false
		}
		if !math.IsInf(remaining, 1) {
			remaining--
		}
		return iterated.stepValue()
	}).val
}

func (r *Runtime) iteratorProto_drop(call FunctionCall) Value {
	thisObj := r.thisIteratorObject(call, "drop")
	remaining := r.iteratorLimit(thisObj, call.Argument(0))
	iterated := r.getIteratorDirect(thisObj)
	return r.newIteratorHelper(iterated, func() (Value, bool) {
		for remaining > 0 {
			if !math.IsInf(remaining, 1) {
				remaining--
			}
			if _, valid := iterated.stepValue(); !valid {
				return nil, false
			}
		}
		return iterated.stepValue()
	}).val
}

func (r *Runtime) iteratorProto_flatMap(call FunctionCall) Value {
	iterated, mapper := r.iteratorWithCallback(call, "flatMap")
	var counter int64
	var inner *iteratorRecord
	h := r.newIteratorHelper(iterated, func() (Value, bool) {
		for {
			if inner == nil {
				value, ok := iterated.stepValue()
				if !ok {
					return nil, false
				}
				mapped := iterated.callOrClose(mapper, value, intToValue(counter))
				counter++
				iterated.closeOnThrow(func() {
					inner = r.getIteratorFlattenable(mapped, false)
				})
			}
			var value Value
			var ok bool
			iterated.closeOnThrow(func() {
				value, ok = inner.stepValue()
			})
			if ok {
				return value, true
			}
			inner = nil
		}
	})
	h.abort = func() {
		if inner != nil {
			iterated.closeOnThrow(inner.returnIter)
		}
		iterated.returnIter()
	}
	return h.val
}

func (r *Runtime) iteratorProto_reduce(call FunctionCall) Value {
	iterated, reducer := r.iteratorWithCallback(call, "reduce")
	var accumulator Value
	var counter int64
	if len(call.Arguments) > 1 {
		accumulator = call.Arguments[1]
	} else {
		var ok bool
		accumulator, ok = iterated.stepValue()
		if !ok {
			panic(r.NewTypeError("Reduce of empty iterator with no initial value"))
		}
		counter = 1
	}
	for {
		value, ok := iterated.stepValue()
		if !ok {
			return accumulator
		}
		accumulator = iterated.callOrClose(reducer, accumulator, value, intToValue(counter))
		counter++
	}
}

func (r *Runtime) iteratorProto_toArray(call FunctionCall) Value {
	iterated := r.getIteratorDirect(r.thisIteratorObject(call, "toArray"))
	var items []Value
	for {
		value, ok := iterated.stepValue()
		if !ok {
			return r.newArrayValues(items)
		}
		items = append(items, value)
	}
}

func (r *Runtime) iteratorProto_forEach(call FunctionCall) Value {
	iterated, fn := r.iteratorWithCallback(call, "forEach")
	for counter := int64(0); ; counter++ {
		value, ok := iterated.stepValue()
		if !ok {
			return _undefined
		}
		iterated.callOrClose(fn, value, intToValue(counter))
	}
}

func (r *Runtime) iteratorProto_some(call FunctionCall) Value {
	iterated, predicate := r.iteratorWithCallback(call, "some")
	for counter := int64(0); ; counter++ {
		value, ok := iterated.stepValue()
		if !ok {
			return valueFalse
		}
		if iterated.callOrClose(predicate, value, intToValue(counter)).ToBoolean() {
			iterated.returnIter()
			return valueTrue
		}
	}
}

func (r *Runtime) iteratorProto_every(call FunctionCall) Value {
	iterated, predicate := r.iteratorWithCallback(call, "every")
	for counter := int64(0); ; counter++ {
		value, ok := iterated.stepValue()
		if !ok {
			return valueTrue
		}
		if !iterated.callOrClose(predicate, value, intToValue(counter)).ToBoolean() {
			iterated.returnIter()
			return valueFalse
		}
	}
}

func (r *Runtime) iteratorProto_find(call FunctionCall) Value {
	iterated, predicate := r.iteratorWithCallback(call, "find")
	for counter := int64(0); ; counter++ {
		value, ok := iterated.stepValue()
		if !ok {
			return _undefined
		}
		if iterated.callOrClose(predicate, value, intToValue(counter)).ToBoolean() {
			iterated.returnIter()
			return value
		}
	}
}

func (r *Runtime) iteratorHelperProto_next(call FunctionCall) Value {
	if obj, ok := call.This.(*Object); ok {
		if h, ok := obj.self.(*iteratorHelperObject); ok {
			return h.next()
		}
	}
	panic(r.NewTypeError("Method Iterator Helper.prototype.next called on incompatible receiver"))
}

func (r *Runtime) iteratorHelperProto_return(call FunctionCall) Value {
	if obj, ok := call.This.(*Object); ok {
		if h, ok := obj.self.(*iteratorHelperObject); ok {
			return h._return()
		}
	}
	panic(r.NewTypeError("Method Iterator Helper.prototype.return called on incompatible receiver"))
}

func (r *Runtime) wrapForValidIteratorProto_next(call FunctionCall) Value {
	if obj, ok := call.This.(*Object); ok {
		if w, ok := obj.self.(*wrapForValidIteratorObject); ok {
			if w.iterated.next == nil {
				panic(r.NewTypeError("iterator.next is missing or not a function"))
			}
			return w.iterated.next(FunctionCall{This: w.iterated.iterator})
		}
	}
	panic(r.NewTypeError("Method %%WrapForValidIteratorPrototype%%.next called on incompatible receiver"))
}

func (r *Runtime) wrapForValidIteratorProto_return(call FunctionCall) Value {
	if obj, ok := call.This.(*Object); ok {
		if w, ok := obj.self.(*wrapForValidIteratorObject); ok {
			iter := w.iterated.iterator
			if method := toMethod(iter.self.getStr("return", nil)); method != nil {
				return method(FunctionCall{This: iter})
			}
			return r.createIterResultObject(_undefined, true)
		}
	}
	panic(r.NewTypeError("Method %%WrapForValidIteratorPrototype%%.return called on incompatible receiver"))
}

func (r *Runtime) iterator_from(call FunctionCall) Value {
	iterated := r.getIteratorFlattenable(call.Argument(0), true)
	if hasInstance(r.getIteratorCtor(), iterated.iterator) {
		return iterated.iterator
	}
	w := &wrapForValidIteratorObject{iterated: iterated}
	o := &Object{runtime: r}
	w.class = classObject
	w.val = o
	w.extensible = true
	o.self = w
	w.prototype = r.getWrapForValidIteratorPrototype()
	w.init()
	return o
}

func (r *Runtime) iterator_concat(call FunctionCall) Value {
	type iterable struct {
		obj        *Object
		openMethod func(FunctionCall) Value
	}
	iterables := make([]iterable, len(call.Arguments))
	for i, item := range call.Arguments {
		obj, ok := item.(*Object)
		if !ok {
			panic(r.NewTypeError("%s is not an object", item.String()))
		}
		method := toMethod(obj.self.getSym(SymIterator, nil))
		if method == nil {
			panic(r.NewTypeError("object is not iterable"))
		}
		iterables[i] = iterable{obj: obj, openMethod: method}
	}
	var inner *iteratorRecord
	// Iterator.concat() has no underlying iterator of its own, so the helper is given an empty record, closing
	// which is a no-op.
	h := r.newIteratorHelper(&iteratorRecord{}, func() (Value, bool) {
		for {
			if inner == nil {
				if len(iterables) == 0 {
					return nil, false
				}
				item := iterables[0]
				iterables[0] = iterable{}
				iterables = iterables[1:]
				iter, ok := item.openMethod(FunctionCall{This: item.obj}).(*Object)
				if !ok {
					panic(r.NewTypeError("Result of the Symbol.iterator method is not an object"))
				}
				inner = r.getIteratorDirect(iter)
			}
			if value, ok := inner.stepValue(); ok {
				return value, true
			}
			inner = nil
		}
	})
	h.abort = func() {
		if inner != nil {
			inner.returnIter()
		}
	}
	return h.val
}

type iteratorZipMode uint8

const (
	iteratorZipShortest iteratorZipMode = iota
	iteratorZipLongest
	iteratorZipStrict
)

// getIteratorZipOptions reads the options argument of Iterator.zip() and Iterator.zipKeyed(). The padding option
// is only read in the "longest" mode.
func (r *Runtime) getIteratorZipOptions(arg Value) (mode iteratorZipMode, padding *Object) {
	if arg == _undefined {
		return
	}
	options, ok := arg.(*Object)
	if !ok {
		panic(r.NewTypeError("Options must be an object"))
	}
	m := options.self.getStr("mode", nil)
	if m != nil && m != _undefined {
		s, ok := m.(String)
		if !ok {
			panic(r.NewTypeError("Invalid zip mode"))
		}
		switch s.String() {
		case "shortest":
		case "longest":
			mode = iteratorZipLongest
		case "strict":
			mode = iteratorZipStrict
		default:
			panic(r.NewTypeError("Invalid zip mode: %s", s.String()))
		}
	}
	if mode == iteratorZipLongest {
		if p := options.self.getStr("padding", nil); p != nil && p != _undefined {
			if padding, ok = p.(*Object); !ok {
				panic(r.NewTypeError("Padding must be an object"))
			}
		}
	}
	return
}

// iteratorZip implements IteratorZip.
func (r *Runtime) iteratorZip(iters []*iteratorRecord, mode iteratorZipMode, padding []Value, finishResults func([]Value) Value) Value {
	// iters doubles as the list of open iterators: an iterator is set to nil once it is exhausted or has thrown.
	h := r.newIteratorHelper(&iteratorRecord{}, func() (Value, bool) {
		if len(iters) == 0 {
			return nil, false
		}
		results := make([]Value, len(iters))
		for i, iter := range iters {
			if iter == nil {
				results[i] = padding[i]
				continue
			}
			var value Value
			var ok bool
			if ex := tryFunc(func() {
				value, ok = iter.stepValue()
			}); ex != nil {
				iters[i] = nil
				closeIterators(iters, ex)
			}
			if !ok {
				iters[i] = nil
				switch mode {
				case iteratorZipShortest:
					closeIterators(iters, nil)
					return nil, false
				case iteratorZipStrict:
					if i != 0 {
						closeIterators(iters, r.NewTypeError("Iterators passed to zip() in strict mode have different lengths"))
					}
					for k := 1; k < len(iters); k++ {
						var done bool
						if ex := tryFunc(func() {
							done = iters[k].stepDone()
						}); ex != nil {
							iters[k] = nil
							closeIterators(iters, ex)
						}
						if !done {
							closeIterators(iters, r.NewTypeError("Iterators passed to zip() in strict mode have different lengths"))
						}
						iters[k] = nil
					}
					return nil, false
				}
				open := false
				for _, iter := range iters {
					if iter != nil {
						open = true
						break
					}
				}
				if !open {
					return nil, false
				}
				value = padding[i]
			}
			results[i] = value
		}
		return finishResults(results), true
	})
	h.abort = func() {
		closeIterators(iters, nil)
	}
	return h.val
}

func (r *Runtime) iterator_zip(call FunctionCall) Value {
	iterables, ok := call.Argument(0).(*Object)
	if !ok {
		panic(r.NewTypeError("%s is not an object", call.Argument(0).String()))
	}
	mode, paddingOption := r.getIteratorZipOptions(call.Argument(1))
	var iters []*iteratorRecord
	inputIter := r.getIterator(iterables, nil)
	for {
		var next Value
		if ex := tryFunc(func() {
			next, ok = inputIter.stepValue()
		}); ex != nil {
			closeIterators(iters, ex)
		}
		if !ok {
			break
		}
		var iter *iteratorRecord
		if ex := tryFunc(func() {
			iter = r.getIteratorFlattenable(next, false)
		}); ex != nil {
			closeIterators(append([]*iteratorRecord{inputIter}, iters...), ex)
		}
		iters = append(iters, iter)
	}
	var padding []Value
	if mode == iteratorZipLongest {
		padding = make([]Value, len(iters))
		for i := range padding {
			padding[i] = _undefined
		}
		if paddingOption != nil {
			if ex := tryFunc(func() {
				paddingIter := r.getIterator(paddingOption, nil)
				for i := range padding {
					value, ok := paddingIter.stepValue()
					if !ok {
						return
					}
					padding[i] = value
				}
				paddingIter.returnIter()
			}); ex != nil {
				closeIterators(iters, ex)
			}
		}
	}
	return r.iteratorZip(iters, mode, padding, func(results []Value) Value {
		return r.newArrayValues(results)
	})
}

func (r *Runtime) iterator_zipKeyed(call FunctionCall) Value {
	iterables, ok := call.Argument(0).(*Object)
	if !ok {
		panic(r.NewTypeError("%s is not an object", call.Argument(0).String()))
	}
	mode, paddingOption := r.getIteratorZipOptions(call.Argument(1))
	var iters []*iteratorRecord
	var keys []Value
	if ex := tryFunc(func() {
		for item, next := iterateEnumerableProperties(iterables)(); next != nil; item, next = next() {
			if item.value == _undefined {
				continue
			}
			iters = append(iters, r.getIteratorFlattenable(item.value, false))
			keys = append(keys, item.name)
		}
	}); ex != nil {
		closeIterators(iters, ex)
	}
	var padding []Value
	if mode == iteratorZipLongest {
		padding = make([]Value, len(iters))
		if paddingOption != nil {
			if ex := tryFunc(func() {
				for i, key := range keys {
					padding[i] = nilSafe(paddingOption.get(key, nil))
				}
			}); ex != nil {
				closeIterators(iters, ex)
			}
		} else {
			for i := range padding {
				padding[i] = _undefined
			}
		}
	}
	return r.iteratorZip(iters, mode, padding, func(results []Value) Value {
		obj := r.newBaseObject(nil, classObject).val
		for i, key := range keys {
			createDataPropertyOrThrow(obj, key, results[i])
		}
		return obj
	})
}

func (r *Runtime) builtin_newIterator(_ []Value, newTarget *Object) *Object {
	if newTarget == nil || newTarget == r.global.Iterator {
		panic(r.NewTypeError("Abstract class Iterator not directly constructable"))
	}
	proto := r.getPrototypeFromCtor(newTarget, r.global.Iterator, r.getIteratorPrototype())
	return r.newBaseObject(proto, classObject).val
}

// setterThatIgnoresPrototypeProperties implements SetterThatIgnoresPrototypeProperties. It is used by the
// accessors on Iterator.prototype, which are accessors only for web compatibility.
func (r *Runtime) setterThatIgnoresPrototypeProperties(this Value, home *Object, homeName string, p Value, v Value) {
	obj, ok := this.(*Object)
	if !ok {
		panic(r.NewTypeError("%s setter called on incompatible receiver %s", homeName, this.String()))
	}
	if obj == home {
		panic(r.NewTypeError("Cannot assign to a read-only property of %s", homeName))
	}
	if obj.getOwnProp(p) == nil {
		createDataPropertyOrThrow(obj, p, v)
	} else {
		obj.set(p, v, obj, true)
	}
}

func (r *Runtime) iteratorProtoAccessor(key Value, get func() Value, name string) Value {
	return &valueProperty{
		accessor: true,
		getterFunc: r.newNativeFunc(func(FunctionCall) Value {
			return get()
		}, unistring.String("get "+name), 0),
		setterFunc: r.newNativeFunc(func(call FunctionCall) Value {
			r.setterThatIgnoresPrototypeProperties(call.This, r.getIteratorPrototype(), "Iterator.prototype", key, call.Argument(0))
			return _undefined
		}, unistring.String("set "+name), 1),
		configurable: true,
	}
}

func (r *Runtime) addIteratorHelpers(o *baseObject) {
	o._putProp("map", r.newNativeFunc(r.iteratorProto_map, "map", 1), true, false, true)
	o._putProp("filter", r.newNativeFunc(r.iteratorProto_filter, "filter", 1), true, false, true)
	o._putProp("take", r.newNativeFunc(r.iteratorProto_take, "take", 1), true, false, true)
	o._putProp("drop", r.newNativeFunc(r.iteratorProto_drop, "drop", 1), true, false, true)
	o._putProp("flatMap", r.newNativeFunc(r.iteratorProto_flatMap, "flatMap", 1), true, false, true)
	o._putProp("reduce", r.newNativeFunc(r.iteratorProto_reduce, "reduce", 1), true, false, true)
	o._putProp("toArray", r.newNativeFunc(r.iteratorProto_toArray, "toArray", 0), true, false, true)
	o._putProp("forEach", r.newNativeFunc(r.iteratorProto_forEach, "forEach", 1), true, false, true)
	o._putProp("some", r.newNativeFunc(r.iteratorProto_some, "some", 1), true, false, true)
	o._putProp("every", r.newNativeFunc(r.iteratorProto_every, "every", 1), true, false, true)
	o._putProp("find", r.newNativeFunc(r.iteratorProto_find, "find", 1), true, false, true)

	o._put("constructor", r.iteratorProtoAccessor(asciiString("constructor"), func() Value {
		return r.getIteratorCtor()
	}, "constructor"))
	o._putSym(SymToStringTag, r.iteratorProtoAccessor(SymToStringTag, func() Value {
		return asciiString(classIterator)
	}, "[Symbol.toStringTag]"))
}

func (r *Runtime) createIteratorHelperProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.getIteratorPrototype(), classObject)

	o._putProp("next", r.newIteratorNextFunc(r.iteratorHelperProto_next, 0, func(iterator *Object) func(Value) (Value, bool) {
		if i, ok := iterator.self.(*iteratorHelperObject); ok {
			return i.nextResult
		}
		return nil
	}), true, false, true)
	o._putProp("return", r.newNativeFunc(r.iteratorHelperProto_return, "return", 0), true, false, true)
	o._putSym(SymToStringTag, valueProp(asciiString(classIteratorHelper), false, false, true))

	return o
}

func (r *Runtime) getIteratorHelperPrototype() *Object {
	var o *Object
	if o = r.global.IteratorHelperPrototype; o == nil {
		o = &Object{runtime: r}
		r.global.IteratorHelperPrototype = o
		o.self = r.createIteratorHelperProto(o)
	}
	return o
}

func (r *Runtime) createWrapForValidIteratorProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.getIteratorPrototype(), classObject)

	o._putProp("next", r.newIteratorNextFunc(r.wrapForValidIteratorProto_next, 0, func(iterator *Object) func(Value) (Value, bool) {
		if i, ok := iterator.self.(*wrapForValidIteratorObject); ok {
			return i.iterated.nextRes
		}
		return nil
	}), true, false, true)
	o._putProp("return", r.newNativeFunc(r.wrapForValidIteratorProto_return, "return", 0), true, false, true)

	return o
}

func (r *Runtime) getWrapForValidIteratorPrototype() *Object {
	var o *Object
	if o = r.global.WrapForValidIteratorPrototype; o == nil {
		o = &Object{runtime: r}
		r.global.WrapForValidIteratorPrototype = o
		o.self = r.createWrapForValidIteratorProto(o)
	}
	return o
}

func (r *Runtime) createIteratorCtor(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newIterator, r.getIteratorPrototype(), "Iterator", 0)
	o._putProp("concat", r.newNativeFunc(r.iterator_concat, "concat", 0), true, false, true)
	o._putProp("from", r.newNativeFunc(r.iterator_from, "from", 1), true, false, true)
	o._putProp("zip", r.newNativeFunc(r.iterator_zip, "zip", 1), true, false, true)
	o._putProp("zipKeyed", r.newNativeFunc(r.iterator_zipKeyed, "zipKeyed", 1), true, false, true)

	return o
}

func (r *Runtime) getIteratorCtor() *Object {
	ret := r.global.Iterator
	if ret == nil {
		ret = &Object{runtime: r}
		r.global.Iterator = ret
		ret.self = r.createIteratorCtor(ret)
	}
	return ret
}
