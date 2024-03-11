package json

import (
	"fmt"
	"reflect"
	"sync"
)

type Registry struct {
	mu   *sync.RWMutex
	data map[string]reflect.Type
}

func NewRegistry() *Registry {
	return &Registry{
		mu:   &sync.RWMutex{},
		data: make(map[string]reflect.Type),
	}
}

func (r *Registry) Register(name string, object interface{}) {
	if object == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.data, name)
		return
	}

	typ := reflect.TypeOf(object)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[name] = typ
}

func (r *Registry) Decode(dec *Decoder, name string) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if typ, ok := r.data[name]; ok {
		ptr := reflect.New(typ).Interface()
		if err := dec.Decode(ptr); err != nil {
			return nil, fmt.Errorf(`failed to decode field %s: %w`, name, err)
		}
		return reflect.ValueOf(ptr).Elem().Interface(), nil
	}

	var decoded interface{}
	if err := dec.Decode(&decoded); err != nil {
		return nil, fmt.Errorf(`failed to decode field %s: %w`, name, err)
	}
	return decoded, nil
}
