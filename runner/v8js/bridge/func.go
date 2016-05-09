package bridge

import (
	"errors"
	"reflect"
)

type Func struct {
	Func    reflect.Value
	In, Out []Type
}

// Creates a bridged function.
// Panics if raw is not a function; this is a blatant programming error.
func BridgeFunc(raw interface{}) Func {
	fn := Func{Func: reflect.ValueOf(raw)}
	fnT := fn.Func.Type()

	// We can only bridge functions
	if fn.Func.Kind() != reflect.Func {
		panic(errors.New("That's not a function >_>"))
	}

	for i := 0; i < fnT.NumIn(); i++ {
		fn.In = append(fn.In, BridgeType(fnT.In(i)))
	}
	for i := 0; i < fnT.NumOut(); i++ {
		fn.Out = append(fn.Out, BridgeType(fnT.Out(i)))
	}

	return fn
}
