// Package browser provides an entry point to the browser extension.
package browser

import (
	"errors"
	"os"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/common"

	k6common "go.k6.io/k6/js/common"
	k6modules "go.k6.io/k6/js/modules"
)

const version = "0.6.0"

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// JSModule exposes the properties available to the JS script.
	JSModule struct {
		Chromium api.BrowserType
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
func (*RootModule) NewModuleInstance(vu k6modules.VU) k6modules.Instance {
	if _, ok := os.LookupEnv("K6_BROWSER_DISABLE_RUN"); ok {
		msg := "Disable run flag enabled, browser test run aborted. Please contact support."
		if m, ok := os.LookupEnv("K6_BROWSER_DISABLE_RUN_MSG"); ok {
			msg = m
		}

		k6common.Throw(vu.Runtime(), errors.New(msg))
	}

	return &ModuleInstance{
		mod: &JSModule{
			Chromium: chromium.NewBrowserType(vu),
			Devices:  common.GetDevices(),
			Version:  version,
		},
	}
}

// Exports returns the exports of the JS module so that it can be used in test
// scripts.
func (mi *ModuleInstance) Exports() k6modules.Exports {
	return k6modules.Exports{Default: mi.mod}
}
