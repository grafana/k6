// Package browser provides an entry point to the browser extension.
package browser

import (
	"sync"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"

	k6modules "go.k6.io/k6/js/modules"
)

const version = "0.8.1"

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct {
		muBrowserProcessIDs sync.RWMutex
		browserProcessIDs   []int
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
	return &RootModule{}
}

// NewModuleInstance implements the k6modules.Module interface to return
// a new instance for each VU.
func (m *RootModule) NewModuleInstance(vu k6modules.VU) k6modules.Instance {
	return &ModuleInstance{
		mod: &JSModule{
			Chromium: mapBrowserToGoja(moduleVU{
				VU:   vu,
				root: m,
			}),
			Devices: common.GetDevices(),
			Version: version,
		},
	}
}

// pids returns the launched browser process IDs.
func (m *RootModule) pids() []int {
	m.muBrowserProcessIDs.RLock()
	defer m.muBrowserProcessIDs.RUnlock()

	pids := make([]int, len(m.browserProcessIDs))
	copy(pids, m.browserProcessIDs)

	return pids
}

// addPid keeps track of the launched browser process IDs.
func (m *RootModule) addPid(pid int) {
	m.muBrowserProcessIDs.Lock()
	defer m.muBrowserProcessIDs.Unlock()

	m.browserProcessIDs = append(m.browserProcessIDs, pid)
}

// Exports returns the exports of the JS module so that it can be used in test
// scripts.
func (mi *ModuleInstance) Exports() k6modules.Exports {
	return k6modules.Exports{Default: mi.mod}
}
