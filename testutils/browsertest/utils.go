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

package browsertest

import (
	"testing"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	k6compiler "go.k6.io/k6/js/compiler"
	k6testutils "go.k6.io/k6/lib/testutils"
)

func AttachFrame(bt *BrowserTest, page api.Page, frameID string, url string) api.Frame {
	pageFn := `async (frameId, url) => {
	    const frame = document.createElement('iframe');
	    frame.src = url;
	    frame.id = frameId;
	    document.body.appendChild(frame);
	    await new Promise(x => frame.onload = x);
	    return frame;
	}`
	handle := page.EvaluateHandle(bt.Runtime.ToValue(pageFn), bt.Runtime.ToValue(frameID), bt.Runtime.ToValue(url))
	return handle.AsElement().ContentFrame()
}

func DetachFrame(bt *BrowserTest, page api.Page, frameID string) {
	pageFn := `frameId => {
        document.getElementById(frameId).remove();
    }`
	page.Evaluate(bt.Runtime.ToValue(pageFn), bt.Runtime.ToValue(frameID))
}

// runES6String Runs an ES6 string in the given runtime. Use this rather than writing ES5 in tests.
func RunES6String(tb testing.TB, rt *goja.Runtime, src string) (goja.Value, error) {
	var err error
	c := k6compiler.New(k6testutils.NewLogger(tb)) // TODO drop it ? maybe we will drop babel and this will be less needed
	src, _, err = c.Transform(src, "__string__")
	if err != nil {
		return goja.Undefined(), err
	}

	return rt.RunString(src)
}
