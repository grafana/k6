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
	"github.com/loadimpact/k6/js/modules/k6"
	"github.com/loadimpact/k6/js/modules/k6/html"
	"github.com/loadimpact/k6/js/modules/k6/http"
	"github.com/loadimpact/k6/js/modules/k6/metrics"
)

// Index of module implementations.
var Index = map[string]interface{}{
	"k6":         &k6.K6{},
	"k6/http":    &http.HTTP{},
	"k6/metrics": &metrics.Metrics{},
	"k6/html":    &html.HTML{},
}
