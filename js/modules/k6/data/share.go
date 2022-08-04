package data

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
)

// TODO fix it not working really well with setupData or just make it more broken
// TODO fix it working with console.log
type sharedArray struct {
	arr []string
}

type wrappedSharedArray struct {
	sharedArray

	rt       *goja.Runtime
	freeze   goja.Callable
	isFrozen goja.Callable
	parse    goja.Callable
}

func (s sharedArray) wrap(rt *goja.Runtime) goja.Value {
	freeze, _ := goja.AssertFunction(rt.GlobalObject().Get("Object").ToObject(rt).Get("freeze"))
	isFrozen, _ := goja.AssertFunction(rt.GlobalObject().Get("Object").ToObject(rt).Get("isFrozen"))
	parse, _ := goja.AssertFunction(rt.GlobalObject().Get("JSON").ToObject(rt).Get("parse"))
	return rt.NewDynamicArray(wrappedSharedArray{
		sharedArray: s,
		rt:          rt,
		freeze:      freeze,
		isFrozen:    isFrozen,
		parse:       parse,
	})
}

func (s wrappedSharedArray) Set(index int, val goja.Value) bool {
	panic(s.rt.NewTypeError("SharedArray is immutable")) // this is specifically a type error
}

func (s wrappedSharedArray) SetLen(len int) bool {
	panic(s.rt.NewTypeError("SharedArray is immutable")) // this is specifically a type error
}

func (s wrappedSharedArray) Get(index int) goja.Value {
	if index < 0 || index >= len(s.arr) {
		return goja.Undefined()
	}
	val, err := s.parse(goja.Undefined(), s.rt.ToValue(s.arr[index]))
	if err != nil {
		common.Throw(s.rt, err)
	}
	err = s.deepFreeze(s.rt, val)
	if err != nil {
		common.Throw(s.rt, err)
	}

	return val
}

func (s wrappedSharedArray) Len() int {
	return len(s.arr)
}

func (s wrappedSharedArray) deepFreeze(rt *goja.Runtime, val goja.Value) error {
	if val != nil && goja.IsNull(val) {
		return nil
	}

	_, err := s.freeze(goja.Undefined(), val)
	if err != nil {
		return err
	}

	o := val.ToObject(rt)
	if o == nil {
		return nil
	}
	for _, key := range o.Keys() {
		prop := o.Get(key)
		if prop != nil {
			// isFrozen returns true for all non objects so it we don't need to check that
			frozen, err := s.isFrozen(goja.Undefined(), prop)
			if err != nil {
				return err
			}
			if !frozen.ToBoolean() { // prevent cycles
				if err = s.deepFreeze(rt, prop); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
