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

package tests

import (
	"testing"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	"github.com/stretchr/testify/require"
)

func TestElementHandleBoundingBoxInvisibleElement(t *testing.T) {
	t.Parallel()

	p := testBrowser(t).NewPage(nil)
	defer p.Close(nil)

	p.SetContent(`<div style="display:none">hello</div>`, nil)
	element := p.Query("div")

	require.Nil(t, element.BoundingBox())
}

func TestElementHandleBoundingBoxSVG(t *testing.T) {
	t.Parallel()

	tb := testBrowser(t)
	p := tb.NewPage(nil)
	defer p.Close(nil)

	p.SetContent(`
        <svg xmlns="http://www.w3.org/2000/svg" width="500" height="500">
            <rect id="theRect" x="30" y="50" width="200" height="300"></rect>
        </svg>`, nil)
	element := p.Query("#therect")
	bbox := element.BoundingBox()
	pageFn := `e => {
        const rect = e.getBoundingClientRect();
        return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
    }`
	var r api.Rect
	webBbox := p.Evaluate(tb.rt.ToValue(pageFn), tb.rt.ToValue(element))
	_ = tb.rt.ExportTo(webBbox.(goja.Value), &r)

	require.EqualValues(t, bbox, &r)
}
