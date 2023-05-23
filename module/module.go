// Package module provides an entry point to the browser module.
package module

import (
	"os"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/registry"

	k6modules "go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct {
		PidRegistry    *pidRegistry
		remoteRegistry *registry.RemoteRegistry
	}

	// JSModule exposes the properties available to the JS script.
	JSModule struct {
		Chromium *goja.Object
		Devices  map[string]common.Device
		Version  string
	}

	// JSModuleInstance represents an instance of the JS module.
	JSModuleInstance struct {
		mod *JSModule
	}
)

var (
	_ k6modules.Module   = &RootModule{}
	_ k6modules.Instance = &JSModuleInstance{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{
		PidRegistry:    &pidRegistry{},
		remoteRegistry: registry.NewRemoteRegistry(os.LookupEnv),
	}
}

// NewModuleInstance implements the k6modules.Module interface to return
// a new instance for each VU.
func (m *RootModule) NewModuleInstance(vu k6modules.VU) k6modules.Instance {
	return &JSModuleInstance{
		mod: &JSModule{
			Chromium: mapBrowserToGoja(moduleVU{
				VU:             vu,
				pidRegistry:    m.PidRegistry,
				RemoteRegistry: m.remoteRegistry,
			}),
			Devices: common.GetDevices(),
		},
	}
}

// Exports returns the exports of the JS module so that it can be used in test
// scripts.
func (mi *JSModuleInstance) Exports() k6modules.Exports {
	return k6modules.Exports{Default: mi.mod}
}
