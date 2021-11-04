/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package browser

import (
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/common"
	"github.com/pkg/errors"
	k6common "go.k6.io/k6/js/common"
	k6modules "go.k6.io/k6/js/modules"
)

const version = "v0.1.0"

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// JSModule is the entrypoint into the browser JS module
	JSModule struct {
		k6modules.InstanceCore
		Devices map[string]common.Device
		Version string
	}

	// ModuleInstance represents an instance of the JS module.
	ModuleInstance struct {
		k6modules.InstanceCore
		mod *JSModule
	}
)

var (
	_ k6modules.IsModuleV2 = &RootModule{}
	_ k6modules.Instance   = &ModuleInstance{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.IsModuleV2 interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(m k6modules.InstanceCore) k6modules.Instance {
	return &ModuleInstance{
		InstanceCore: m,
		mod: &JSModule{
			InstanceCore: m,
			Devices:      common.GetDevices(),
			Version:      version,
		},
	}
}

// GetExports returns the exports of the JS module so that it can be used in
// test scripts.
func (mi *ModuleInstance) GetExports() k6modules.Exports {
	return k6modules.Exports{Default: mi.mod}
}

func (m *JSModule) Launch(browserName string, opts goja.Value) api.Browser {
	/*go func() {
		f, err := os.Create("./cpu.profile")
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
		<-ctx.Done()
	}()*/

	if browserName == "chromium" {
		bt := chromium.NewBrowserType(m.GetContext())
		return bt.Launch(opts)
	}

	k6common.Throw(m.GetRuntime(),
		errors.Errorf("Currently 'chromium' is the only supported browser"))
	return nil
}

func init() {
	k6modules.Register("k6/x/browser", New())
}
