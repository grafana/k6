// Package extensionapi defines the stable API for k6 JavaScript extensions.
//
// It intentionally depends only on the Go standard library and Sobek. Host
// integrations provide the implementation of VU; extensions must not depend
// on k6 packages to use this API.
package extensionapi

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/grafana/sobek"
)

const importPrefix = "k6/x/"

// Module creates one module Instance for each JavaScript runtime that imports
// it.
type Module interface {
	NewModuleInstance(VU) Instance
}

// Instance provides the JavaScript exports for one Module in one JavaScript
// runtime.
type Instance interface {
	Exports() Exports
}

// VU exposes the capabilities available to every v1 extension module
// instance. Further host capabilities will be introduced as separate,
// optional interfaces instead of expanding this base interface.
type VU interface {
	Context() context.Context
	Runtime() *sobek.Runtime
}

// Environment is an optional VU capability for looking up environment values
// supplied by the host. Extensions obtain it with a type assertion from VU so
// hosts can provide the base API without exposing an environment.
type Environment interface {
	LookupEnv(key string) (value string, ok bool)
}

// Exports represents the ESM exports of an Instance.
type Exports struct {
	Default any
	Named   map[string]any
}

var (
	registryMu sync.RWMutex
	registry   = make(map[string]any)
)

// Register makes mod available for import under name. Names must begin with
// "k6/x/" and may be registered only once. mod may be a Module or a raw Go
// value; a host chooses how to expose raw values to JavaScript.
func Register(name string, mod any) {
	if !strings.HasPrefix(name, importPrefix) {
		panic(fmt.Errorf("extension module names must be prefixed with %q, got %q", importPrefix, name))
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := registry[name]; exists {
		panic(fmt.Errorf("extension module already registered: %s", name))
	}
	registry[name] = mod
}

// Registered returns a snapshot of registered extension modules, keyed by
// their JavaScript import specifier.
func Registered() map[string]any {
	registryMu.RLock()
	defer registryMu.RUnlock()

	return maps.Clone(registry)
}
