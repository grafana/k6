package streams

import (
	"fmt"
	"reflect"

	"github.com/dop251/goja"
	"github.com/liuxd6825/k6server/js/common"
	"github.com/liuxd6825/k6server/js/modules"
)

// newResolvedPromise instantiates a new resolved promise.
func newResolvedPromise(vu modules.VU, with goja.Value) *goja.Promise {
	promise, resolve, _ := vu.Runtime().NewPromise()
	resolve(with)
	return promise
}

// newRejectedPromise instantiates a new rejected promise.
func newRejectedPromise(vu modules.VU, with any) *goja.Promise {
	promise, _, reject := vu.Runtime().NewPromise()
	reject(with)
	return promise
}

// promiseThen facilitates instantiating a new promise and defining callbacks for to be executed
// on fulfillment as well as rejection, directly from Go.
func promiseThen(
	rt *goja.Runtime,
	promise *goja.Promise,
	onFulfilled, onRejected func(goja.Value),
) (*goja.Promise, error) {
	val, err := rt.RunString(
		`(function(promise, onFulfilled, onRejected) { return promise.then(onFulfilled, onRejected) })`)
	if err != nil {
		return nil, newError(RuntimeError, "unable to initialize promiseThen internal helper function")
	}

	cal, ok := goja.AssertFunction(val)
	if !ok {
		return nil, newError(RuntimeError, "the internal promiseThen helper is not a function")
	}

	if onRejected == nil {
		val, err = cal(goja.Undefined(), rt.ToValue(promise), rt.ToValue(onFulfilled))
	} else {
		val, err = cal(goja.Undefined(), rt.ToValue(promise), rt.ToValue(onFulfilled), rt.ToValue(onRejected))
	}

	if err != nil {
		return nil, err
	}

	newPromise, ok := val.Export().(*goja.Promise)
	if !ok {
		return nil, newError(RuntimeError, "unable to cast the internal promiseThen helper's return value to a promise")
	}

	return newPromise, nil
}

// isNumber returns true if the given goja value holds a number
func isNumber(value goja.Value) bool {
	_, isFloat := value.Export().(float64)
	_, isInt := value.Export().(int64)

	return isFloat || isInt
}

// isNonNegativeNumber implements the [IsNonNegativeNumber] algorithm.
//
// [IsNonNegativeNumber]: https://streams.spec.whatwg.org/#is-non-negative-number
func isNonNegativeNumber(value goja.Value) bool {
	if common.IsNullish(value) {
		return false
	}

	if !isNumber(value) {
		return false
	}

	if value.ToFloat() < 0 || value.ToInteger() < 0 {
		return false
	}

	return true
}

// setReadOnlyPropertyOf sets a read-only property on the given [goja.Object].
func setReadOnlyPropertyOf(obj *goja.Object, objName, propName string, propValue goja.Value) error {
	err := obj.DefineDataProperty(propName,
		propValue,
		goja.FLAG_FALSE,
		goja.FLAG_FALSE,
		goja.FLAG_TRUE,
	)
	if err != nil {
		return fmt.Errorf("unable to define %s read-only property on %s object; reason: %w", propName, objName, err)
	}

	return nil
}

// isObject determines whether the given [goja.Value] is a [goja.Object] or not.
func isObject(val goja.Value) bool {
	return val != nil && val.ExportType() != nil && val.ExportType().Kind() == reflect.Map
}
