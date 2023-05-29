// Package browser provides an entry point to the browser module.
package browser

import (
	"fmt"
	"os"
	"time"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"

	k6event "go.k6.io/k6/event"
	k6modules "go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct {
		PidRegistry    *pidRegistry
		remoteRegistry *remoteRegistry
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
func New(state *k6modules.State) *RootModule {
	// TODO: Only subscribe to events if there are browser scenarios configured.
	// For this to work, state.Options should be accessible here.
	_, evtCh := state.Events.Subscribe(k6event.Init, k6event.TestStart, k6event.TestEnd)
	go func() {
		for evt := range evtCh {
			fmt.Printf(">>> received event: %#+v\n", evt)
			switch evt.Type {
			case k6event.Init:
				fmt.Printf(">>> starting browser processes...\n")
				// Start browser processes here...
				go func() {
					time.Sleep(time.Second)
					fmt.Printf(">>> done starting browser processes...\n")
					evt.Done()
				}()
			case k6event.TestStart:
				fmt.Printf(">>> test started in browser...\n")
			case k6event.TestEnd:
				fmt.Printf(">>> stopping browser processes...\n")
				// Stop browser processes here...
				go func() {
					time.Sleep(time.Second)
					// Don't forget to call this to signal k6 that it can
					// continue shutting down!
					fmt.Printf(">>> done stopping browser processes...\n")
					evt.Done()
				}()
			}
		}
	}()

	return &RootModule{
		PidRegistry:    &pidRegistry{},
		remoteRegistry: newRemoteRegistry(os.LookupEnv),
	}
}

// NewModuleInstance implements the k6modules.Module interface to return
// a new instance for each VU.
func (m *RootModule) NewModuleInstance(vu k6modules.VU) k6modules.Instance {
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
