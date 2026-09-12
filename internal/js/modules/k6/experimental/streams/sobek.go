package streams

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

// newWebIDLConstructor wraps a native Sobek constructor so it follows Web IDL call semantics.
// Sobek otherwise treats a native constructor call without `new` exactly like construction and
// does not expose a NewTarget that lets the native function distinguish the two cases.
func newWebIDLConstructor(rt *sobek.Runtime, name string, constructor any) (sobek.Value, error) {
	factory, err := rt.RunString(`
(function(constructor, name) {
  const wrapper = function(...args) {
    if (new.target === undefined) {
      throw new TypeError(name + " constructor must be called with new");
    }
    return Reflect.construct(constructor, args, new.target);
  };
  Object.defineProperty(wrapper, "name", { value: name, configurable: true });
  return wrapper;
})`)
	if err != nil {
		return nil, err
	}

	call, ok := sobek.AssertFunction(factory)
	if !ok {
		return nil, newError(RuntimeError, "Web IDL constructor wrapper factory is not a function")
	}

	return call(sobek.Undefined(), rt.ToValue(constructor), rt.ToValue(name))
}

// promiseWrapper holds a [sobek.Promise] together with its resolve and reject functions,
// so that it can be settled later from Go code.
type promiseWrapper struct {
	promise          *sobek.Promise
	resolve          func(any) error
	reject           func(any) error
	settlementQueued bool
}

// newPromiseWrapper creates a new pending [promiseWrapper].
func newPromiseWrapper(rt *sobek.Runtime) *promiseWrapper {
	p, resolve, reject := rt.NewPromise()
	return &promiseWrapper{promise: p, resolve: resolve, reject: reject}
}

func (pw *promiseWrapper) isPending() bool {
	return !pw.settlementQueued && pw.promise.State() == sobek.PromiseStatePending
}

func (pw *promiseWrapper) queueSettlement() {
	pw.settlementQueued = true
}

// resolveWith resolves the wrapped promise with the given value.
func (pw *promiseWrapper) resolveWith(value any) {
	pw.queueSettlement()
	if err := pw.resolve(value); err != nil {
		panic(err) // TODO(@mstoykov): propagate as error instead
	}
}

// rejectWith rejects the wrapped promise with the given reason, unwrapping [jsError] values.
func (pw *promiseWrapper) rejectWith(reason any) {
	pw.queueSettlement()
	if jsErr, ok := reason.(*jsError); ok {
		reason = jsErr.Err()
	}
	if err := pw.reject(reason); err != nil {
		panic(err) // TODO(@mstoykov): propagate as error instead
	}
}

// newResolvedPromiseWrapper creates a new [promiseWrapper] resolved with the given value.
func newResolvedPromiseWrapper(rt *sobek.Runtime, value any) *promiseWrapper {
	pw := newPromiseWrapper(rt)
	pw.resolveWith(value)
	return pw
}

// newRejectedPromiseWrapper creates a new [promiseWrapper] rejected with the given reason.
func newRejectedPromiseWrapper(rt *sobek.Runtime, reason any) *promiseWrapper {
	pw := newPromiseWrapper(rt)
	pw.rejectWith(reason)
	return pw
}

// newResolvedPromise instantiates a new resolved promise.
func newResolvedPromise(vu modules.VU, with sobek.Value) *sobek.Promise {
	return newResolvedPromiseWrapper(vu.Runtime(), with).promise
}

// newRejectedPromise instantiates a new rejected promise.
func newRejectedPromise(vu modules.VU, with any) *sobek.Promise {
	return newRejectedPromiseWrapper(vu.Runtime(), with).promise
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

// promiseThenReturn is like [promiseThen], but its callbacks return a [sobek.Value] which the
// resulting promise adopts (i.e. if a callback returns a promise, the resulting promise follows
// it). This is needed to implement the specification's "reacting to a promise ... returns X"
// steps, where X may itself be a promise.
func promiseThenReturn(
	rt *sobek.Runtime,
	promise *sobek.Promise,
	onFulfilled, onRejected func(sobek.Value) sobek.Value,
) (*sobek.Promise, error) {
	val, err := rt.RunString(
		`(function(promise, onFulfilled, onRejected) { return promise.then(onFulfilled, onRejected) })`)
	if err != nil {
		return nil, newError(RuntimeError, "unable to initialize promiseThenReturn internal helper function")
	}

	cal, ok := sobek.AssertFunction(val)
	if !ok {
		return nil, newError(RuntimeError, "the internal promiseThenReturn helper is not a function")
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
		return nil, newError(RuntimeError, "unable to cast the internal promiseThenReturn helper's return value to a promise")
	}

	return newPromise, nil
}

// markPromiseHandled marks the given promise as handled to prevent unhandled rejection
// tracking. See https://github.com/dop251/goja/issues/565.
func markPromiseHandled(rt *sobek.Runtime, p *sobek.Promise) {
	doNothing := func(sobek.Value) {}
	if _, err := promiseThen(rt, p, doNothing, doNothing); err != nil {
		common.Throw(rt, newError(RuntimeError, err.Error()))
	}
}

// throwableValue converts an internal error value into a value suitable for rejecting a
// promise with or throwing, unwrapping [jsError] instances.
func throwableValue(err any) any {
	if jsErr, ok := err.(*jsError); ok {
		return jsErr.Err()
	}
	return err
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

// setDefaultPrototypePropertyOf sets a property with the default Web IDL prototype method
// descriptors: writable, configurable, and enumerable.
func setDefaultPrototypePropertyOf(obj *sobek.Object, propName string, propValue sobek.Value) error {
	err := obj.DefineDataProperty(propName,
		propValue,
		sobek.FLAG_TRUE,
		sobek.FLAG_TRUE,
		sobek.FLAG_TRUE,
	)
	if err != nil {
		return fmt.Errorf("unable to define %s property; reason: %w", propName, err)
	}

	return nil
}

func hasOwnProperty(obj *sobek.Object, propName string) bool {
	return slices.Contains(obj.GetOwnPropertyNames(), propName)
}

// isObject determines whether the given [sobek.Value] is a [sobek.Object] or not.
func isObject(val sobek.Value) bool {
	return val != nil && val.ExportType() != nil && val.ExportType().Kind() == reflect.Map
}
