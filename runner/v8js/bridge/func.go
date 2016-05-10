package bridge

import (
	"errors"
	"fmt"
	"reflect"
)

type Func struct {
	Func      reflect.Value
	In, Out   []Type
	IsVaradic bool
	VarArg    Type
}

func (f *Func) Call(args []interface{}) error {
	rArgs := make([]reflect.Value, 0, len(args))
	for i, v := range args {
		t := Type{}
		if i >= len(f.In) {
			if f.IsVaradic {
				t = f.VarArg
			} else {
				break
			}
		} else {
			t = f.In[i]
		}

		if err := t.Cast(&v); err != nil {
			return err
		}
		rArgs = append(rArgs, reflect.ValueOf(v))
	}
	f.Func.Call(rArgs)
	return nil
}

func (f *Func) JS(mod, name string) string {
	return fmt.Sprintf(`function() { __internal__._invoke('%s', '%s', Array.prototype.slice.call(arguments)); }`, mod, name)
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
		if !fnT.IsVariadic() || i != fnT.NumIn()-1 {
			fn.In = append(fn.In, BridgeType(fnT.In(i)))
		} else {
			fn.IsVaradic = true
			fn.VarArg = BridgeType(fnT.In(i).Elem())
		}
	}
	for i := 0; i < fnT.NumOut(); i++ {
		fn.Out = append(fn.Out, BridgeType(fnT.Out(i)))
	}

	return fn
}
