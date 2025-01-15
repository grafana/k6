// Package modules defines interfaces used for developing k6's JavaScript modules.
package modules

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/sobek"

	"go.k6.io/k6/ext"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
)

const extPrefix string = "k6/x/"

// Register the given mod as an external JavaScript module that can be imported
// by name. The name must be unique across all registered modules and must be
// prefixed with "k6/x/", otherwise this function will panic.
func Register(name string, mod interface{}) {
	if !strings.HasPrefix(name, extPrefix) {
		panic(fmt.Errorf("external module names must be prefixed with '%s', tried to register: %s", extPrefix, name))
	}

	ext.Register(name, ext.JSExtension, mod)
}

// Module is the interface js modules should implement in order to get access to the VU
type Module interface {
	// NewModuleInstance will get modules.VU that should provide the module with a way to interact with the VU.
	// This method will be called for *each* VU that imports the module *once* per that VU.
	NewModuleInstance(VU) Instance
}

// Instance is what a module needs to return
type Instance interface {
	Exports() Exports
}

// VU gives access to the currently executing VU to a module Instance
type VU interface {
	// Context return the context.Context about the current VU
	Context() context.Context

	// Events allows subscribing to global k6 execution events, such as Init and
	// Exit, and to local (per-VU) events, such as IterStart and IterEnd.
	// NOTE: This API is EXPERIMENTAL and may be changed, renamed or
	// completely removed in a later k6 release.
	// FIXME: Subscribing to global events shouldn't be part of this VU (local)
	// interface.
	Events() common.Events

	// InitEnv returns common.InitEnvironment instance if present
	InitEnv() *common.InitEnvironment

	// State returns lib.State if any is present
	State() *lib.State

	// Runtime returns the sobek.Runtime for the current VU
	Runtime() *sobek.Runtime

	// RegisterCallback lets a JS module declare that it wants to run a function
	// on the event loop *at a later point in time*. See the documentation for
	// `EventLoop.RegisterCallback()` in the `k6/js/eventloop` Go module for
	// the very important details on its usage and restrictions.
	RegisterCallback() (enqueueCallback func(func() error))

	// sealing field will help probably with pointing users that they just need to embed this in their Instance
	// implementations
}

// Exports is representation of ESM exports of a module
type Exports struct {
	// Default is what will be the `default` export of a module
	Default interface{}
	// Named is the named exports of a module
	Named map[string]interface{}
}
