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
	"errors"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"

	k6common "go.k6.io/k6/js/common"
	k6modules "go.k6.io/k6/js/modules"

	"github.com/dop251/goja"
)

const version = "v0.3.0"

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// JSModule is the entrypoint into the browser JS module.
	JSModule struct {
		vu        k6modules.VU
		k6Metrics *k6ext.CustomMetrics
		Devices   map[string]common.Device
		Version   string
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
	k6m := k6ext.RegisterCustomMetrics(vu.InitEnv().Registry)
	return &ModuleInstance{
		mod: &JSModule{
			vu:        vu,
			k6Metrics: k6m,
			Devices:   common.GetDevices(),
			Version:   version,
		},
	}
}

// Exports returns the exports of the JS module so that it can be used in test
// scripts.
func (mi *ModuleInstance) Exports() k6modules.Exports {
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

	ctx := k6ext.WithVU(m.vu.Context(), m.vu)
	ctx = k6ext.WithCustomMetrics(ctx, m.k6Metrics)

	if browserName == "chromium" {
		bt := chromium.NewBrowserType(ctx)
		return bt.Launch(opts)
	}

	k6common.Throw(m.vu.Runtime(),
		errors.New("Currently 'chromium' is the only supported browser"))
	return nil
}

func init() {
	k6modules.Register("k6/x/browser", New())
}
