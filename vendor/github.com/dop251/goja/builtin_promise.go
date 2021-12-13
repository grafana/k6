package goja

import (
	"github.com/dop251/goja/unistring"
	"reflect"
)

type PromiseState int
type PromiseRejectionOperation int

type promiseReactionType int

const (
	PromiseStatePending PromiseState = iota
	PromiseStateFulfilled
	PromiseStateRejected
)

const (
	PromiseRejectionReject PromiseRejectionOperation = iota
	PromiseRejectionHandle
)

const (
	promiseReactionFulfill promiseReactionType = iota
	promiseReactionReject
)

type PromiseRejectionTracker func(p *Promise, operation PromiseRejectionOperation)

type jobCallback struct {
	callback func(FunctionCall) Value
}

type promiseCapability struct {
	promise               *Object
	resolveObj, rejectObj *Object
}

type promiseReaction struct {
	capability *promiseCapability
	typ        promiseReactionType
	handler    *jobCallback
}

var typePromise = reflect.TypeOf((*Promise)(nil))

// Promise is a Go wrapper around ECMAScript Promise. Calling Runtime.ToValue() on it
// returns the underlying Object. Calling Export() on a Promise Object returns a Promise.
//
// Use Runtime.NewPromise() to create one. Calling Runtime.ToValue() on a zero object or nil returns null Value.
//
// WARNING: Instances of Promise are not goroutine-safe. See Runtime.NewPromise() for more details.
type Promise struct {
	baseObject
	state            PromiseState
	result           Value
	fulfillReactions []*promiseReaction
	rejectReactions  []*promiseReaction
	handled          bool
}

func (p *Promise) State() PromiseState {
	return p.state
}

func (p *Promise) Result() Value {
	return p.result
}

func (p *Promise) toValue(r *Runtime) Value {
	if p == nil || p.val == nil {
		return _null
	}
	promise := p.val
	if promise.runtime != r {
		panic(r.NewTypeError("Illegal runtime transition of a Promise"))
	}
	return promise
}

func (p *Promise) createResolvingFunctions() (resolve, reject *Object) {
	r := p.val.runtime
	alreadyResolved := false
	return p.val.runtime.newNativeFunc(func(call FunctionCall) Value {
			if alreadyResolved {
				return _undefined
			}
			alreadyResolved = true
			resolution := call.Argument(0)
			if resolution.SameAs(p.val) {
				return p.reject(r.NewTypeError("Promise self-resolution"))
			}
			if obj, ok := resolution.(*Object); ok {
				var thenAction Value
				ex := r.vm.try(func() {
					thenAction = obj.self.getStr("then", nil)
				})
				if ex != nil {
					return p.reject(ex.val)
				}
				if call, ok := assertCallable(thenAction); ok {
					job := r.newPromiseResolveThenableJob(p, resolution, &jobCallback{callback: call})
					r.enqueuePromiseJob(job)
					return _undefined
				}
			}
			return p.fulfill(resolution)
		}, nil, "", nil, 1),
		p.val.runtime.newNativeFunc(func(call FunctionCall) Value {
			if alreadyResolved {
				return _undefined
			}
			alreadyResolved = true
			reason := call.Argument(0)
			return p.reject(reason)
		}, nil, "", nil, 1)
}

func (p *Promise) reject(reason Value) Value {
	reactions := p.rejectReactions
	p.result = reason
	p.fulfillReactions, p.rejectReactions = nil, nil
	p.state = PromiseStateRejected
	r := p.val.runtime
	if !p.handled {
		r.trackPromiseRejection(p, PromiseRejectionReject)
	}
	r.triggerPromiseReactions(reactions, reason)
	return _undefined
}

func (p *Promise) fulfill(value Value) Value {
	reactions := p.fulfillReactions
	p.result = value
	p.fulfillReactions, p.rejectReactions = nil, nil
	p.state = PromiseStateFulfilled
	p.val.runtime.triggerPromiseReactions(reactions, value)
	return _undefined
}

func (p *Promise) exportType() reflect.Type {
	return typePromise
}

func (p *Promise) export(*objectExportCtx) interface{} {
	return p
}

func (r *Runtime) newPromiseResolveThenableJob(p *Promise, thenable Value, then *jobCallback) func() {
	return func() {
		resolve, reject := p.createResolvingFunctions()
		ex := r.vm.try(func() {
			r.callJobCallback(then, thenable, resolve, reject)
		})
		if ex != nil {
			if fn, ok := reject.self.assertCallable(); ok {
				fn(FunctionCall{Arguments: []Value{ex.val}})
			}
		}
	}
}

func (r *Runtime) enqueuePromiseJob(job func()) {
	r.jobQueue = append(r.jobQueue, job)
}

func (r *Runtime) triggerPromiseReactions(reactions []*promiseReaction, argument Value) {
	for _, reaction := range reactions {
		r.enqueuePromiseJob(r.newPromiseReactionJob(reaction, argument))
	}
}

func (r *Runtime) newPromiseReactionJob(reaction *promiseReaction, argument Value) func() {
	return func() {
		var handlerResult Value
		fulfill := false
		if reaction.handler == nil {
			handlerResult = argument
			if reaction.typ == promiseReactionFulfill {
				fulfill = true
			}
		} else {
			ex := r.vm.try(func() {
				handlerResult = r.callJobCallback(reaction.handler, _undefined, argument)
				fulfill = true
			})
			if ex != nil {
				handlerResult = ex.val
			}
		}
		if reaction.capability != nil {
			if fulfill {
				reaction.capability.resolve(handlerResult)
			} else {
				reaction.capability.reject(handlerResult)
			}
		}
	}
}

func (r *Runtime) newPromise(proto *Object) *Promise {
	o := &Object{runtime: r}

	po := &Promise{}
	po.class = classPromise
	po.val = o
	po.extensible = true
	o.self = po
	po.prototype = proto
	po.init()
	return po
}

func (r *Runtime) builtin_newPromise(args []Value, newTarget *Object) *Object {
	if newTarget == nil {
		panic(r.needNew("Promise"))
	}
	var arg0 Value
	if len(args) > 0 {
		arg0 = args[0]
	}
	executor := r.toCallable(arg0)

	proto := r.getPrototypeFromCtor(newTarget, r.global.Promise, r.global.PromisePrototype)
	po := r.newPromise(proto)

	resolve, reject := po.createResolvingFunctions()
	ex := r.vm.try(func() {
		executor(FunctionCall{Arguments: []Value{resolve, reject}})
	})
	if ex != nil {
		if fn, ok := reject.self.assertCallable(); ok {
			fn(FunctionCall{Arguments: []Value{ex.val}})
		}
	}
	return po.val
}

func (r *Runtime) promiseProto_then(call FunctionCall) Value {
	thisObj := r.toObject(call.This)
	if p, ok := thisObj.self.(*Promise); ok {
		c := r.speciesConstructorObj(thisObj, r.global.Promise)
		resultCapability := r.newPromiseCapability(c)
		return r.performPromiseThen(p, call.Argument(0), call.Argument(1), resultCapability)
	}
	panic(r.NewTypeError("Method Promise.prototype.then called on incompatible receiver %s", r.objectproto_toString(FunctionCall{This: thisObj})))
}

func (r *Runtime) newPromiseCapability(c *Object) *promiseCapability {
	pcap := new(promiseCapability)
	if c == r.global.Promise {
		p := r.newPromise(r.global.PromisePrototype)
		pcap.resolveObj, pcap.rejectObj = p.createResolvingFunctions()
		pcap.promise = p.val
	} else {
		var resolve, reject Value
		executor := r.newNativeFunc(func(call FunctionCall) Value {
			if resolve != nil {
				panic(r.NewTypeError("resolve is already set"))
			}
			if reject != nil {
				panic(r.NewTypeError("reject is already set"))
			}
			if arg := call.Argument(0); arg != _undefined {
				resolve = arg
			}
			if arg := call.Argument(1); arg != _undefined {
				reject = arg
			}
			return nil
		}, nil, "", nil, 2)
		pcap.promise = r.toConstructor(c)([]Value{executor}, c)
		pcap.resolveObj = r.toObject(resolve)
		r.toCallable(pcap.resolveObj) // make sure it's callable
		pcap.rejectObj = r.toObject(reject)
		r.toCallable(pcap.rejectObj)
	}
	return pcap
}

func (r *Runtime) performPromiseThen(p *Promise, onFulfilled, onRejected Value, resultCapability *promiseCapability) Value {
	var onFulfilledJobCallback, onRejectedJobCallback *jobCallback
	if f, ok := assertCallable(onFulfilled); ok {
		onFulfilledJobCallback = &jobCallback{callback: f}
	}
	if f, ok := assertCallable(onRejected); ok {
		onRejectedJobCallback = &jobCallback{callback: f}
	}
	fulfillReaction := &promiseReaction{
		capability: resultCapability,
		typ:        promiseReactionFulfill,
		handler:    onFulfilledJobCallback,
	}
	rejectReaction := &promiseReaction{
		capability: resultCapability,
		typ:        promiseReactionReject,
		handler:    onRejectedJobCallback,
	}
	switch p.state {
	case PromiseStatePending:
		p.fulfillReactions = append(p.fulfillReactions, fulfillReaction)
		p.rejectReactions = append(p.rejectReactions, rejectReaction)
	case PromiseStateFulfilled:
		r.enqueuePromiseJob(r.newPromiseReactionJob(fulfillReaction, p.result))
	default:
		reason := p.result
		if !p.handled {
			r.trackPromiseRejection(p, PromiseRejectionHandle)
		}
		r.enqueuePromiseJob(r.newPromiseReactionJob(rejectReaction, reason))
	}
	p.handled = true
	if resultCapability == nil {
		return _undefined
	}
	return resultCapability.promise
}

func (r *Runtime) promiseProto_catch(call FunctionCall) Value {
	return r.invoke(call.This, "then", _undefined, call.Argument(0))
}

func (r *Runtime) promiseResolve(c *Object, x Value) *Object {
	if obj, ok := x.(*Object); ok {
		xConstructor := nilSafe(obj.self.getStr("constructor", nil))
		if xConstructor.SameAs(c) {
			return obj
		}
	}
	pcap := r.newPromiseCapability(c)
	pcap.resolve(x)
	return pcap.promise
}

func (r *Runtime) promiseProto_finally(call FunctionCall) Value {
	promise := r.toObject(call.This)
	c := r.speciesConstructorObj(promise, r.global.Promise)
	onFinally := call.Argument(0)
	var thenFinally, catchFinally Value
	if onFinallyFn, ok := assertCallable(onFinally); !ok {
		thenFinally, catchFinally = onFinally, onFinally
	} else {
		thenFinally = r.newNativeFunc(func(call FunctionCall) Value {
			value := call.Argument(0)
			result := onFinallyFn(FunctionCall{})
			promise := r.promiseResolve(c, result)
			valueThunk := r.newNativeFunc(func(call FunctionCall) Value {
				return value
			}, nil, "", nil, 0)
			return r.invoke(promise, "then", valueThunk)
		}, nil, "", nil, 1)

		catchFinally = r.newNativeFunc(func(call FunctionCall) Value {
			reason := call.Argument(0)
			result := onFinallyFn(FunctionCall{})
			promise := r.promiseResolve(c, result)
			thrower := r.newNativeFunc(func(call FunctionCall) Value {
				panic(reason)
			}, nil, "", nil, 0)
			return r.invoke(promise, "then", thrower)
		}, nil, "", nil, 1)
	}
	return r.invoke(promise, "then", thenFinally, catchFinally)
}

func (pcap *promiseCapability) resolve(result Value) {
	pcap.promise.runtime.toCallable(pcap.resolveObj)(FunctionCall{Arguments: []Value{result}})
}

func (pcap *promiseCapability) reject(reason Value) {
	pcap.promise.runtime.toCallable(pcap.rejectObj)(FunctionCall{Arguments: []Value{reason}})
}

func (pcap *promiseCapability) try(f func()) bool {
	ex := pcap.promise.runtime.vm.try(f)
	if ex != nil {
		pcap.reject(ex.val)
		return false
	}
	return true
}

func (r *Runtime) promise_all(call FunctionCall) Value {
	c := r.toObject(call.This)
	pcap := r.newPromiseCapability(c)

	pcap.try(func() {
		promiseResolve := r.toCallable(c.self.getStr("resolve", nil))
		iter := r.getIterator(call.Argument(0), nil)
		var values []Value
		remainingElementsCount := 1
		iter.iterate(func(nextValue Value) {
			index := len(values)
			values = append(values, _undefined)
			nextPromise := promiseResolve(FunctionCall{This: c, Arguments: []Value{nextValue}})
			alreadyCalled := false
			onFulfilled := r.newNativeFunc(func(call FunctionCall) Value {
				if alreadyCalled {
					return _undefined
				}
				alreadyCalled = true
				values[index] = call.Argument(0)
				remainingElementsCount--
				if remainingElementsCount == 0 {
					pcap.resolve(r.newArrayValues(values))
				}
				return _undefined
			}, nil, "", nil, 1)
			remainingElementsCount++
			r.invoke(nextPromise, "then", onFulfilled, pcap.rejectObj)
		})
		remainingElementsCount--
		if remainingElementsCount == 0 {
			pcap.resolve(r.newArrayValues(values))
		}
	})
	return pcap.promise
}

func (r *Runtime) promise_allSettled(call FunctionCall) Value {
	c := r.toObject(call.This)
	pcap := r.newPromiseCapability(c)

	pcap.try(func() {
		promiseResolve := r.toCallable(c.self.getStr("resolve", nil))
		iter := r.getIterator(call.Argument(0), nil)
		var values []Value
		remainingElementsCount := 1
		iter.iterate(func(nextValue Value) {
			index := len(values)
			values = append(values, _undefined)
			nextPromise := promiseResolve(FunctionCall{This: c, Arguments: []Value{nextValue}})
			alreadyCalled := false
			reaction := func(status Value, valueKey unistring.String) *Object {
				return r.newNativeFunc(func(call FunctionCall) Value {
					if alreadyCalled {
						return _undefined
					}
					alreadyCalled = true
					obj := r.NewObject()
					obj.self._putProp("status", status, true, true, true)
					obj.self._putProp(valueKey, call.Argument(0), true, true, true)
					values[index] = obj
					remainingElementsCount--
					if remainingElementsCount == 0 {
						pcap.resolve(r.newArrayValues(values))
					}
					return _undefined
				}, nil, "", nil, 1)
			}
			onFulfilled := reaction(asciiString("fulfilled"), "value")
			onRejected := reaction(asciiString("rejected"), "reason")
			remainingElementsCount++
			r.invoke(nextPromise, "then", onFulfilled, onRejected)
		})
		remainingElementsCount--
		if remainingElementsCount == 0 {
			pcap.resolve(r.newArrayValues(values))
		}
	})
	return pcap.promise
}

func (r *Runtime) promise_any(call FunctionCall) Value {
	c := r.toObject(call.This)
	pcap := r.newPromiseCapability(c)

	pcap.try(func() {
		promiseResolve := r.toCallable(c.self.getStr("resolve", nil))
		iter := r.getIterator(call.Argument(0), nil)
		var errors []Value
		remainingElementsCount := 1
		iter.iterate(func(nextValue Value) {
			index := len(errors)
			errors = append(errors, _undefined)
			nextPromise := promiseResolve(FunctionCall{This: c, Arguments: []Value{nextValue}})
			alreadyCalled := false
			onRejected := r.newNativeFunc(func(call FunctionCall) Value {
				if alreadyCalled {
					return _undefined
				}
				alreadyCalled = true
				errors[index] = call.Argument(0)
				remainingElementsCount--
				if remainingElementsCount == 0 {
					_error := r.builtin_new(r.global.AggregateError, nil)
					_error.self._putProp("errors", r.newArrayValues(errors), true, false, true)
					pcap.reject(_error)
				}
				return _undefined
			}, nil, "", nil, 1)

			remainingElementsCount++
			r.invoke(nextPromise, "then", pcap.resolveObj, onRejected)
		})
		remainingElementsCount--
		if remainingElementsCount == 0 {
			_error := r.builtin_new(r.global.AggregateError, nil)
			_error.self._putProp("errors", r.newArrayValues(errors), true, false, true)
			pcap.reject(_error)
		}
	})
	return pcap.promise
}

func (r *Runtime) promise_race(call FunctionCall) Value {
	c := r.toObject(call.This)
	pcap := r.newPromiseCapability(c)

	pcap.try(func() {
		promiseResolve := r.toCallable(c.self.getStr("resolve", nil))
		iter := r.getIterator(call.Argument(0), nil)
		iter.iterate(func(nextValue Value) {
			nextPromise := promiseResolve(FunctionCall{This: c, Arguments: []Value{nextValue}})
			r.invoke(nextPromise, "then", pcap.resolveObj, pcap.rejectObj)
		})
	})
	return pcap.promise
}

func (r *Runtime) promise_reject(call FunctionCall) Value {
	pcap := r.newPromiseCapability(r.toObject(call.This))
	pcap.reject(call.Argument(0))
	return pcap.promise
}

func (r *Runtime) promise_resolve(call FunctionCall) Value {
	return r.promiseResolve(r.toObject(call.This), call.Argument(0))
}

func (r *Runtime) createPromiseProto(val *Object) objectImpl {
	o := newBaseObjectObj(val, r.global.ObjectPrototype, classObject)
	o._putProp("constructor", r.global.Promise, true, false, true)

	o._putProp("catch", r.newNativeFunc(r.promiseProto_catch, nil, "catch", nil, 1), true, false, true)
	o._putProp("finally", r.newNativeFunc(r.promiseProto_finally, nil, "finally", nil, 1), true, false, true)
	o._putProp("then", r.newNativeFunc(r.promiseProto_then, nil, "then", nil, 2), true, false, true)

	o._putSym(SymToStringTag, valueProp(asciiString(classPromise), false, false, true))

	return o
}

func (r *Runtime) createPromise(val *Object) objectImpl {
	o := r.newNativeConstructOnly(val, r.builtin_newPromise, r.global.PromisePrototype, "Promise", 1)

	o._putProp("all", r.newNativeFunc(r.promise_all, nil, "all", nil, 1), true, false, true)
	o._putProp("allSettled", r.newNativeFunc(r.promise_allSettled, nil, "allSettled", nil, 1), true, false, true)
	o._putProp("any", r.newNativeFunc(r.promise_any, nil, "any", nil, 1), true, false, true)
	o._putProp("race", r.newNativeFunc(r.promise_race, nil, "race", nil, 1), true, false, true)
	o._putProp("reject", r.newNativeFunc(r.promise_reject, nil, "reject", nil, 1), true, false, true)
	o._putProp("resolve", r.newNativeFunc(r.promise_resolve, nil, "resolve", nil, 1), true, false, true)

	r.putSpeciesReturnThis(o)

	return o
}

func (r *Runtime) initPromise() {
	r.global.PromisePrototype = r.newLazyObject(r.createPromiseProto)
	r.global.Promise = r.newLazyObject(r.createPromise)

	r.addToGlobal("Promise", r.global.Promise)
}

func (r *Runtime) wrapPromiseReaction(fObj *Object) func(interface{}) {
	f, _ := AssertFunction(fObj)
	return func(x interface{}) {
		_, _ = f(nil, r.ToValue(x))
	}
}

// NewPromise creates and returns a Promise and resolving functions for it.
//
// WARNING: The returned values are not goroutine-safe and must not be called in parallel with VM running.
// In order to make use of this method you need an event loop such as the one in goja_nodejs (https://github.com/dop251/goja_nodejs)
// where it can be used like this:
//
//  loop := NewEventLoop()
//  loop.Start()
//  defer loop.Stop()
//  loop.RunOnLoop(func(vm *goja.Runtime) {
//		p, resolve, _ := vm.NewPromise()
//		vm.Set("p", p)
//      go func() {
//   		time.Sleep(500 * time.Millisecond)   // or perform any other blocking operation
//			loop.RunOnLoop(func(*goja.Runtime) { // resolve() must be called on the loop, cannot call it here
//				resolve(result)
//			})
//		}()
//  }
func (r *Runtime) NewPromise() (promise *Promise, resolve func(result interface{}), reject func(reason interface{})) {
	p := r.newPromise(r.global.PromisePrototype)
	resolveF, rejectF := p.createResolvingFunctions()
	return p, r.wrapPromiseReaction(resolveF), r.wrapPromiseReaction(rejectF)
}

// SetPromiseRejectionTracker registers a function that will be called in two scenarios: when a promise is rejected
// without any handlers (with operation argument set to PromiseRejectionReject), and when a handler is added to a
// rejected promise for the first time (with operation argument set to PromiseRejectionHandle).
//
// Setting a tracker replaces any existing one. Setting it to nil disables the functionality.
//
// See https://tc39.es/ecma262/#sec-host-promise-rejection-tracker for more details.
func (r *Runtime) SetPromiseRejectionTracker(tracker PromiseRejectionTracker) {
	r.promiseRejectionTracker = tracker
}
