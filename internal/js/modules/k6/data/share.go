package data

import (
	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
)

// TODO fix it not working really well with setupData or just make it more broken
// TODO fix it working with console.log
type sharedArray struct {
	arr []string
}

type wrappedSharedArray struct {
	sharedArray

	rt       *sobek.Runtime
	freeze   sobek.Callable
	isFrozen sobek.Callable
	parse    sobek.Callable
}

func (s sharedArray) wrap(rt *sobek.Runtime) sobek.Value {
	freeze, _ := sobek.AssertFunction(rt.GlobalObject().Get("Object").ToObject(rt).Get("freeze"))
	isFrozen, _ := sobek.AssertFunction(rt.GlobalObject().Get("Object").ToObject(rt).Get("isFrozen"))
	parse, _ := sobek.AssertFunction(rt.GlobalObject().Get("JSON").ToObject(rt).Get("parse"))
	return rt.NewDynamicArray(wrappedSharedArray{
		sharedArray: s,
		rt:          rt,
		freeze:      freeze,
		isFrozen:    isFrozen,
		parse:       parse,
	})
}

func (s wrappedSharedArray) Set(_ int, _ sobek.Value) bool {
	panic(s.rt.NewTypeError("SharedArray is immutable")) // this is specifically a type error
}

func (s wrappedSharedArray) SetLen(_ int) bool {
	panic(s.rt.NewTypeError("SharedArray is immutable")) // this is specifically a type error
}

func (s wrappedSharedArray) Get(index int) sobek.Value {
	if index < 0 || index >= len(s.arr) {
		return sobek.Undefined()
	}
	val, err := s.parse(sobek.Undefined(), s.rt.ToValue(s.arr[index]))
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

func (s wrappedSharedArray) deepFreeze(rt *sobek.Runtime, val sobek.Value) error {
	if val != nil && sobek.IsNull(val) {
		return nil
	}

	_, err := s.freeze(sobek.Undefined(), val)
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
			frozen, err := s.isFrozen(sobek.Undefined(), prop)
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
