/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package modules

import (
	"fmt"
	"sync"

	"github.com/loadimpact/k6/js/modules/k6"
	"github.com/loadimpact/k6/js/modules/k6/crypto"
	"github.com/loadimpact/k6/js/modules/k6/crypto/x509"
	"github.com/loadimpact/k6/js/modules/k6/data"
	"github.com/loadimpact/k6/js/modules/k6/encoding"
	"github.com/loadimpact/k6/js/modules/k6/grpc"
	"github.com/loadimpact/k6/js/modules/k6/html"
	"github.com/loadimpact/k6/js/modules/k6/http"
	"github.com/loadimpact/k6/js/modules/k6/metrics"
	"github.com/loadimpact/k6/js/modules/k6/ws"
)

//nolint:gochecknoglobals
var (
	modules = make(map[string]interface{})
	mx      sync.RWMutex
)

// HasModuleInstancePerVU should be implemented by all native Golang modules that
// would require per-VU state. k6 will call their NewModuleInstancePerVU() methods
// every time a VU imports the module and use its result as the returned object.
type HasModuleInstancePerVU interface {
	NewModuleInstancePerVU() interface{}
}

// Register the given mod as a JavaScript module, available
// for import from JS scripts by name.
// This function panics if a module with the same name is already registered.
func Register(name string, mod interface{}) {
	mx.Lock()
	defer mx.Unlock()

	if _, ok := modules[name]; ok {
		panic(fmt.Sprintf("module already registered: %s", name))
	}
	modules[name] = mod
}

// GetJSModules returns a map of all js modules
func GetJSModules() map[string]interface{} {
	result := map[string]interface{}{
		"k6":             k6.New(),
		"k6/crypto":      crypto.New(),
		"k6/crypto/x509": x509.New(),
		"k6/data":        data.New(),
		"k6/encoding":    encoding.New(),
		"k6/net/grpc":    grpc.New(),
		"k6/html":        html.New(),
		"k6/http":        http.New(),
		"k6/metrics":     metrics.New(),
		"k6/ws":          ws.New(),
	}

	for name, module := range modules {
		result[name] = module
	}

	return result
}
