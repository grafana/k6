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
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/testutils/browsertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupHandlersForHTMLFiles(bt)
func TestPageGoto(t *testing.T) {
	bt := browsertest.New(t).WithStaticFiles()

	p := bt.Browser.NewPage(nil)
	t.Cleanup(func() { p.Close(nil) })

	url := bt.URL("/static/empty.html")
	r := p.Goto(url, nil)

	assert.Equal(t, url, r.URL(), `expected URL to be %q, result of navigation was %q`, url, r.URL())
}

func TestPageGotoDataURI(t *testing.T) {
	bt := browsertest.New(t)

	p := bt.Browser.NewPage(nil)
	t.Cleanup(func() { p.Close(nil) })

	r := p.Goto("data:text/html,hello", nil)

	assert.Nil(t, r, `expected response to be nil`)
}

func TestPageGotoWaitUntilLoad(t *testing.T) {
	bt := browsertest.New(t).WithStaticFiles()

	p := bt.Browser.NewPage(nil)
	t.Cleanup(func() { p.Close(nil) })

	p.Goto(bt.URL("/static/wait_until.html"), bt.Runtime.ToValue(struct {
		WaitUntil string `js:"waitUntil"`
	}{WaitUntil: "load"}))

	results := p.Evaluate(bt.Runtime.ToValue("() => window.results"))
	var actual []string
	bt.Runtime.ExportTo(results.(goja.Value), &actual)

	assert.EqualValues(t, []string{"DOMContentLoaded", "load"}, actual, `expected "load" event to have fired`)
}

func TestPageGotoWaitUntilDOMContentLoaded(t *testing.T) {
	bt := browsertest.New(t).WithStaticFiles()

	p := bt.Browser.NewPage(nil)
	t.Cleanup(func() { p.Close(nil) })

	p.Goto(bt.URL("/static/wait_until.html"), bt.Runtime.ToValue(struct {
		WaitUntil string `js:"waitUntil"`
	}{WaitUntil: "domcontentloaded"}))

	results := p.Evaluate(bt.Runtime.ToValue("() => window.results"))
	var actual []string
	bt.Runtime.ExportTo(results.(goja.Value), &actual)

	assert.EqualValues(t, "DOMContentLoaded", actual[0], `expected "DOMContentLoaded" event to have fired`)
}

func TestPageSetExtraHTTPHeaders(t *testing.T) {
	bt := browsertest.New(t)

	p := bt.Browser.NewPage(nil)
	t.Cleanup(func() { p.Close(nil) })

	headers := map[string]string{
		"Some-Header": "Some-Value",
	}
	p.SetExtraHTTPHeaders(headers)

	resp := p.Goto(bt.URL("/get"), nil)

	require.NotNil(t, resp)
	var body struct{ Headers map[string][]string }
	err := json.Unmarshal(resp.Body().Bytes(), &body)
	require.NoError(t, err)
	h := body.Headers["Some-Header"]
	require.NotEmpty(t, h)
	assert.Equal(t, "Some-Value", h[0])
}
