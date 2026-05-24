package js

import "github.com/grafana/sobek"

// envDynamicObject wraps the __ENV map as a read-only JS object.
// Any attempt to set or delete a property panics with a TypeError so that
// scripts get a clear error instead of a silent no-op.
type envDynamicObject struct {
	runtime *sobek.Runtime
	env     map[string]string
}

func (o *envDynamicObject) Get(key string) sobek.Value {
	v, ok := o.env[key]
	if !ok {
		return nil
	}
	return o.runtime.ToValue(v)
}

func (o *envDynamicObject) Set(_ string, _ sobek.Value) bool {
	panic(o.runtime.NewTypeError("__ENV is read-only"))
}

func (o *envDynamicObject) Has(key string) bool {
	_, ok := o.env[key]
	return ok
}

func (o *envDynamicObject) Delete(_ string) bool {
	panic(o.runtime.NewTypeError("__ENV is read-only"))
}

func (o *envDynamicObject) Keys() []string {
	keys := make([]string, 0, len(o.env))
	for k := range o.env {
		keys = append(keys, k)
	}
	return keys
}
