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

package common

import k6stats "go.k6.io/k6/stats"

var (
	BrowserDOMContentLoaded     = k6stats.New("browser_dom_content_loaded", k6stats.Trend, k6stats.Time)
	BrowserFirstPaint           = k6stats.New("browser_first_paint", k6stats.Trend, k6stats.Time)
	BrowserFirstContentfulPaint = k6stats.New("browser_first_contentful_paint", k6stats.Trend, k6stats.Time)
	BrowserFirstMeaningfulPaint = k6stats.New("browser_first_meaningful_paint", k6stats.Trend, k6stats.Time)
	BrowserLoaded               = k6stats.New("browser_loaded", k6stats.Trend, k6stats.Time)
)
