package js

import (
	"github.com/robertkrimen/otto"
)

func Check(val, arg0 otto.Value) (bool, error) {
	switch {
	case val.IsFunction():
		val, err := val.Call(otto.UndefinedValue(), arg0)
		if err != nil {
			return false, err
		}
		return Check(val, arg0)
	case val.IsBoolean():
		b, err := val.ToBoolean()
		if err != nil {
			return false, err
		}
		return b, nil
	case val.IsNumber():
		f, err := val.ToFloat()
		if err != nil {
			return false, err
		}
		return f != 0, nil
	case val.IsString():
		s, err := val.ToString()
		if err != nil {
			return false, err
		}
		return s != "", nil
	default:
		return false, nil
	}
}

func throw(vm *otto.Otto, v interface{}) {
	if err, ok := v.(error); ok {
		panic(vm.MakeCustomError("Error", err.Error()))
	}
	panic(v)
}
