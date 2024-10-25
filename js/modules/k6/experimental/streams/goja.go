package streams

import (
	"fmt"
	"reflect"

	"github.com/grafana/sobek"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

// newResolvedPromise instantiates a new resolved promise.
func newResolvedPromise(vu modules.VU, with sobek.Value) *sobek.Promise {
	promise, resolve, _ := vu.Runtime().NewPromise()
	err := resolve(with)
	if err != nil { // TODO(@mstoykov): likely better to actually call Promise.resolve directly
		panic(err)
	}

	return promise
}

// newRejectedPromise instantiates a new rejected promise.
func newRejectedPromise(vu modules.VU, with any) *sobek.Promise {
	promise, _, reject := vu.Runtime().NewPromise()
	err := reject(with)
	if err != nil {
		panic(err)
	}
	return promise
}

// promiseThen facilitates instantiating a new promise and defining callbacks for to be executed
// on fulfillment as well as rejection, directly from Go.
func promiseThen(
	rt *sobek.Runtime,
	promise *sobek.Promise,
	onFulfilled, onRejected func(sobek.Value),
) (*sobek.Promise, error) {
	val, err := rt.RunString(
		`(function(promise, onFulfilled, onRejected) { return promise.then(onFulfilled, onRejected) })`)
	if err != nil {
		return nil, newError(RuntimeError, "unable to initialize promiseThen internal helper function")
	}

	cal, ok := sobek.AssertFunction(val)
	if !ok {
		return nil, newError(RuntimeError, "the internal promiseThen helper is not a function")
	}

	if onRejected == nil {
		val, err = cal(sobek.Undefined(), rt.ToValue(promise), rt.ToValue(onFulfilled))
	} else {
		val, err = cal(sobek.Undefined(), rt.ToValue(promise), rt.ToValue(onFulfilled), rt.ToValue(onRejected))
	}

	if err != nil {
		return nil, err
	}

	newPromise, ok := val.Export().(*sobek.Promise)
	if !ok {
		return nil, newError(RuntimeError, "unable to cast the internal promiseThen helper's return value to a promise")
	}

	return newPromise, nil
}

// isNumber returns true if the given sobek.Value holds a number
func isNumber(value sobek.Value) bool {
	_, isFloat := value.Export().(float64)
	_, isInt := value.Export().(int64)

	return isFloat || isInt
}

// isNonNegativeNumber implements the [IsNonNegativeNumber] algorithm.
//
// [IsNonNegativeNumber]: https://streams.spec.whatwg.org/#is-non-negative-number
func isNonNegativeNumber(value sobek.Value) bool {
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

// setReadOnlyPropertyOf sets a read-only property on the given [sobek.Object].
func setReadOnlyPropertyOf(obj *sobek.Object, objName, propName string, propValue sobek.Value) error {
	err := obj.DefineDataProperty(propName,
		propValue,
		sobek.FLAG_FALSE,
		sobek.FLAG_FALSE,
		sobek.FLAG_TRUE,
	)
	if err != nil {
		return fmt.Errorf("unable to define %s read-only property on %s object; reason: %w", propName, objName, err)
	}

	return nil
}

// isObject determines whether the given [sobek.Value] is a [sobek.Object] or not.
func isObject(val sobek.Value) bool {
	return val != nil && val.ExportType() != nil && val.ExportType().Kind() == reflect.Map
}
