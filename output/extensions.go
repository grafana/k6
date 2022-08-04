package output

import (
	"fmt"
	"sync"
)

//nolint:gochecknoglobals
var (
	extensions = make(map[string]func(Params) (Output, error))
	mx         sync.RWMutex
)

// GetExtensions returns all registered extensions.
func GetExtensions() map[string]func(Params) (Output, error) {
	mx.RLock()
	defer mx.RUnlock()
	res := make(map[string]func(Params) (Output, error), len(extensions))
	for k, v := range extensions {
		res[k] = v
	}
	return res
}

// RegisterExtension registers the given output extension constructor. This
// function panics if a module with the same name is already registered.
func RegisterExtension(name string, mod func(Params) (Output, error)) {
	mx.Lock()
	defer mx.Unlock()

	if _, ok := extensions[name]; ok {
		panic(fmt.Sprintf("output extension already registered: %s", name))
	}
	extensions[name] = mod
}
