package goja

import "github.com/dop251/goja/unistring"

type argumentsObject struct {
	baseObject
	length int
}

type mappedProperty struct {
	valueProperty
	v *Value
}

func (a *argumentsObject) getStr(name unistring.String, receiver Value) Value {
	return a.getStrWithOwnProp(a.getOwnPropStr(name), name, receiver)
}

func (a *argumentsObject) getOwnPropStr(name unistring.String) Value {
	if mapped, ok := a.values[name].(*mappedProperty); ok {
		return *mapped.v
	}

	return a.baseObject.getOwnPropStr(name)
}

func (a *argumentsObject) init() {
	a.baseObject.init()
	a._putProp("length", intToValue(int64(a.length)), true, false, true)
}

func (a *argumentsObject) setOwnStr(name unistring.String, val Value, throw bool) bool {
	if prop, ok := a.values[name].(*mappedProperty); ok {
		if !prop.writable {
			a.val.runtime.typeErrorResult(throw, "Property is not writable: %s", name)
			return false
		}
		*prop.v = val
		return true
	}
	return a.baseObject.setOwnStr(name, val, throw)
}

func (a *argumentsObject) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return a._setForeignStr(name, a.getOwnPropStr(name), val, receiver, throw)
}

/*func (a *argumentsObject) putStr(name string, val Value, throw bool) {
	if prop, ok := a.values[name].(*mappedProperty); ok {
		if !prop.writable {
			a.val.runtime.typeErrorResult(throw, "Property is not writable: %s", name)
			return
		}
		*prop.v = val
		return
	}
	a.baseObject.putStr(name, val, throw)
}*/

func (a *argumentsObject) deleteStr(name unistring.String, throw bool) bool {
	if prop, ok := a.values[name].(*mappedProperty); ok {
		if !a.checkDeleteProp(name, &prop.valueProperty, throw) {
			return false
		}
		a._delete(name)
		return true
	}

	return a.baseObject.deleteStr(name, throw)
}

type argumentsPropIter struct {
	wrapped iterNextFunc
}

func (i *argumentsPropIter) next() (propIterItem, iterNextFunc) {
	var item propIterItem
	item, i.wrapped = i.wrapped()
	if i.wrapped == nil {
		return propIterItem{}, nil
	}
	if prop, ok := item.value.(*mappedProperty); ok {
		item.value = *prop.v
	}
	return item, i.next
}

func (a *argumentsObject) enumerateUnfiltered() iterNextFunc {
	return a.recursiveIter((&argumentsPropIter{
		wrapped: a.ownIter(),
	}).next)
}

func (a *argumentsObject) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if mapped, ok := a.values[name].(*mappedProperty); ok {
		existing := &valueProperty{
			configurable: mapped.configurable,
			writable:     true,
			enumerable:   mapped.enumerable,
			value:        mapped.get(a.val),
		}

		val, ok := a.baseObject._defineOwnProperty(name, existing, descr, throw)
		if !ok {
			return false
		}

		if prop, ok := val.(*valueProperty); ok {
			if !prop.accessor {
				*mapped.v = prop.value
			}
			if prop.accessor || !prop.writable {
				a._put(name, prop)
				return true
			}
			mapped.configurable = prop.configurable
			mapped.enumerable = prop.enumerable
		} else {
			*mapped.v = val
			mapped.configurable = true
			mapped.enumerable = true
		}

		return true
	}

	return a.baseObject.defineOwnPropertyStr(name, descr, throw)
}

func (a *argumentsObject) export() interface{} {
	arr := make([]interface{}, a.length)
	for i := range arr {
		v := a.getIdx(valueInt(int64(i)), nil)
		if v != nil {
			arr[i] = v.Export()
		}
	}
	return arr
}
