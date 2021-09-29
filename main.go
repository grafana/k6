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
	"context"

	"github.com/dop251/goja"
	"github.com/k6io/xk6-browser/api"
	"github.com/k6io/xk6-browser/chromium"
	"github.com/k6io/xk6-browser/common"
	"github.com/pkg/errors"
	k6common "go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

const version = "v0.1.0"

// JSModule is the entrypoint into the browser JS module
type JSModule struct {
	// For when https://github.com/k6io/k6/issues/1802 is fixed
	//Chromium api.BrowserType
	Devices map[string]common.Device
	Version string
}

// NewJSModule creates a new browser module
func NewJSModule(version string) *JSModule {
	return &JSModule{
		//Chromium: chromium.NewBrowserType(),
		Devices: common.GetDevices(),
		Version: version,
	}
}

func (m *JSModule) Launch(ctx context.Context, browserName string, opts goja.Value) api.Browser {
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
		bt := chromium.NewBrowserType(ctx)
		return bt.Launch(opts)
	}

	rt := k6common.GetRuntime(ctx)
	k6common.Throw(rt, errors.Errorf("Currently 'chromium' is the only supported browser"))
	return nil
}

func init() {
	modules.Register("k6/x/browser", NewJSModule(version))
}
