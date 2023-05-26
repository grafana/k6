// Package browser provides an entry point to the browser module.
package browser

import (
	"os"
	"sync"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"

	k6modules "go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct {
		PidRegistry    *pidRegistry
		remoteRegistry *remoteRegistry
		once           *sync.Once
	}

	// JSModule exposes the properties available to the JS script.
	JSModule struct {
		Chromium *goja.Object
		Devices  map[string]common.Device
		Version  string
	}

	// ModuleInstance represents an instance of the JS module.
	ModuleInstance struct {
		mod *JSModule
	}
)

var (
	_ k6modules.Module   = &RootModule{}
	_ k6modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{
		PidRegistry: &pidRegistry{},
		once:        &sync.Once{},
	}
}

// NewModuleInstance implements the k6modules.Module interface to return
// a new instance for each VU.
func (m *RootModule) NewModuleInstance(vu k6modules.VU) k6modules.Instance {
	m.once.Do(func() {
		// remoteRegistry should only be initialized once as it is
		// used globally across the whole test run and not just the
		// current vu. Since newRemoteRegistry can fail with an error,
		// we've had to place it here so that if an error occurs a
		// panic can be initiated and safely handled by k6.
		rr, err := newRemoteRegistry(os.LookupEnv)
		if err != nil {
			ctx := k6ext.WithVU(vu.Context(), vu)
			k6ext.Panic(ctx, "failed to create remote registry: %v", err.Error())
		}
		m.remoteRegistry = rr
	})

	return &ModuleInstance{
		mod: &JSModule{
			Chromium: mapBrowserToGoja(moduleVU{
				VU:             vu,
				pidRegistry:    m.PidRegistry,
				remoteRegistry: m.remoteRegistry,
			}),
			Devices: common.GetDevices(),
		},
	}
}

// Exports returns the exports of the JS module so that it can be used in test
// scripts.
func (mi *ModuleInstance) Exports() k6modules.Exports {
	return k6modules.Exports{Default: mi.mod}
}
