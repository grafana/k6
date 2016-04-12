package js

import (
	"errors"
	"fmt"
	"github.com/robertkrimen/otto"
	"io/ioutil"
	"net/http"
	"time"
)

type JSError string

func (e JSError) Error() { return e }

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

func jsHTTPGetFactory(vm *otto.Otto, impl func(url string) (*http.Response, error)) func(otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		url, err := call.Argument(0).ToString()
		if err != nil {
			panic(JSError(fmt.Sprintf("Couldn't call function: %s", err)))
		}

		res, err := impl(url)
		if err != nil {
			panic(JSError(fmt.Sprintf("HTTP GET impl error: %s", err)))
		}
		defer res.Body.Close()

		obj, err := vm.Object("new Object()")
		if err != nil {
			panic(JSError(fmt.Sprintf("Couldn't create an Object(): %s", err)))
		}
		body, _ := ioutil.ReadAll(res.Body)
		obj.Set("body", string(body))
		obj.Set("statusCode", res.StatusCode)
		obj.Set("header", res.Header)

		return obj.Value()
	}
}
