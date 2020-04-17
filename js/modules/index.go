/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

	"github.com/loadimpact/k6/js/modules/k6"
	"github.com/loadimpact/k6/js/modules/k6/crypto"
	"github.com/loadimpact/k6/js/modules/k6/crypto/x509"
	"github.com/loadimpact/k6/js/modules/k6/encoding"
	"github.com/loadimpact/k6/js/modules/k6/html"
	"github.com/loadimpact/k6/js/modules/k6/http"
	"github.com/loadimpact/k6/js/modules/k6/metrics"
	"github.com/loadimpact/k6/js/modules/k6/ws"
)

// Index of module implementations.
var Index = map[string]interface{}{
	"k6":             k6.New(),
	"k6/crypto":      crypto.New(),
	"k6/crypto/x509": x509.New(),
	"k6/encoding":    encoding.New(),
	"k6/http":        http.New(),
	"k6/metrics":     metrics.New(),
	"k6/html":        html.New(),
	"k6/ws":          ws.New(),
}

// PluginIndex holds a map of plugin modules to their respective implementations.
var PluginIndex = map[string]interface{}{}

// RegisterPluginModules takes care of registering a map of paths that a plugin exposes so they can be
// loaded from the JavaScript VM.
func RegisterPluginModules(modules map[string]interface{}) {
	for path, impl := range modules {
		importPath := fmt.Sprintf("k6-plugin/%s", path)
		PluginIndex[importPath] = impl
	}
}
