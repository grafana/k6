package js

import (
	"github.com/robertkrimen/otto"
	"time"
)

func jsSleepFactory(impl func(time.Duration)) func(otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		seconds, err := call.Argument(0).ToFloat()
		if err != nil {
			seconds = 0.0
		}
		impl(time.Duration(seconds * float64(time.Second)))
		return otto.UndefinedValue()
	}
}

func jsLogFactory(impl func(string)) func(otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		text, err := call.Argument(0).ToString()
		if err != nil {
			text = "[ERROR]"
		}
		impl(text)
		return otto.UndefinedValue()
	}
}
