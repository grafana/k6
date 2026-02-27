package common

import (
	"fmt"
	"strconv"

	"github.com/grafana/sobek"
)

// JSErrorConfig describes a JS-visible error type.
type JSErrorConfig struct {
	// Constructor is the name of the constructor function exposed to JS (required).
	Constructor string
	// Name is assigned to the error's `name` property.
	Name string
	// Message is the error message.
	Message string
	// Properties contains additional properties to copy onto the JS error object.
	Properties map[string]interface{}
	// Decorator runs after the error has been instantiated, allowing callers to
	// set additional state or computed properties.
	Decorator func(rt *sobek.Runtime, obj *sobek.Object)
}

// JSError implements both error and JSException and can be embedded into custom error structs.
type JSError struct {
	constructor string
	name        string
	message     string
	properties  map[string]interface{}
	decorator   func(rt *sobek.Runtime, obj *sobek.Object)
}

// ExportNamedError returns a function suitable for use in module exports. It returns
// a constructor proxy bound to the caller's runtime.
func ExportNamedError(name string) func(rt *sobek.Runtime) sobek.Value {
	return func(rt *sobek.Runtime) sobek.Value {
		return ExportErrorConstructor(rt, name)
	}
}

// NewJSError returns a new JS-aware error helper.
func NewJSError(cfg JSErrorConfig) *JSError {
	if cfg.Constructor == "" {
		panic("common.NewJSError: Constructor is required")
	}

	return &JSError{
		constructor: cfg.Constructor,
		name:        cfg.Name,
		message:     cfg.Message,
		properties:  cfg.Properties,
		decorator:   cfg.Decorator,
	}
}

// Error satisfies the Go error interface.
func (e *JSError) Error() string {
	if e == nil {
		return ""
	}

	switch {
	case e.name == "" && e.message == "":
		return e.constructor
	case e.name == "":
		return e.message
	case e.message == "":
		return e.name
	default:
		return e.name + ": " + e.message
	}
}

// JSValue materializes the error as a JS Error instance.
func (e *JSError) JSValue(rt *sobek.Runtime) sobek.Value {
	if e == nil {
		return sobek.Null()
	}

	ctor, err := EnsureErrorConstructor(rt, e.constructor)
	if err != nil {
		panic(err)
	}

	jsConstructor, ok := sobek.AssertConstructor(ctor)
	if !ok {
		panic(rt.NewGoError(fmt.Errorf("%s constructor is not callable", e.constructor)))
	}

	obj, err := jsConstructor(nil, rt.ToValue(e.message))
	if err != nil {
		panic(err)
	}

	if e.name != "" {
		if setErr := obj.Set("name", e.name); setErr != nil {
			panic(setErr)
		}
	}

	for key, value := range e.properties {
		if setErr := obj.Set(key, value); setErr != nil {
			panic(setErr)
		}
	}

	if e.decorator != nil {
		e.decorator(rt, obj)
	}

	return obj
}

var (
	_ error       = (*JSError)(nil)
	_ JSException = (*JSError)(nil)
)

// EnsureErrorConstructor ensures a named Error subclass exists in the runtime and
// returns the constructor function. The constructor is stored on the global object
// so user scripts can import it and perform instanceof checks.
func EnsureErrorConstructor(rt *sobek.Runtime, name string) (*sobek.Object, error) {
	if name == "" {
		name = "Error"
	}

	if val := rt.GlobalObject().Get(name); val != nil && !sobek.IsUndefined(val) {
		if obj, ok := val.(*sobek.Object); ok {
			return obj, nil
		}
	}

	script := fmt.Sprintf(`(function(name) {
		if (typeof globalThis[name] === "function") {
			return globalThis[name];
		}

		const ctor = class extends Error {
			constructor(message) {
				super(message);
				this.name = name;
			}
		};

		Object.defineProperty(ctor, "name", { value: name, configurable: true });
		globalThis[name] = ctor;
		return ctor;
	})(%s);`, strconv.Quote(name))

	value, err := rt.RunString(script)
	if err != nil {
		return nil, err
	}

	return value.ToObject(rt), nil
}

// ExportErrorConstructor returns a proxy constructor suitable for module exports that
// always delegates to the runtime-local Error constructor, keeping instanceof checks working.
//
// Note: We intentionally don't cache proxies because runtimes are short-lived per-VU,
// and caching would cause memory leaks by preventing runtime garbage collection.
func ExportErrorConstructor(rt *sobek.Runtime, name string) sobek.Value {
	// Make sure the constructor exists so the proxy can reference it.
	if _, err := EnsureErrorConstructor(rt, name); err != nil {
		panic(err)
	}

	script := fmt.Sprintf(`(function(name) {
		const getCtor = () => globalThis[name];
		return new Proxy(function () {}, {
			construct(_target, args, newTarget) {
				return Reflect.construct(getCtor(), args, newTarget);
			},
			apply(_target, thisArg, args) {
				return Reflect.apply(getCtor(), thisArg, args);
			},
			get(_target, prop, receiver) {
				if (prop === Symbol.hasInstance) {
					return function (value) {
						return value instanceof getCtor();
					};
				}
				return Reflect.get(getCtor(), prop, receiver);
			},
			set(_target, prop, value, receiver) {
				return Reflect.set(getCtor(), prop, value, receiver);
			}
		});
	})(%s);`, strconv.Quote(name))

	proxy, err := rt.RunString(script)
	if err != nil {
		panic(err)
	}

	return proxy
}
