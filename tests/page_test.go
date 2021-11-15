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

func TestPageGoto(t *testing.T) {
	bt := browsertest.NewBrowserTest(t)
	setupHandlersForHTMLFiles(bt)
	defer bt.Browser.Close()

	t.Run("Page.goto", func(t *testing.T) {
		t.Run("should work", func(t *testing.T) { testPageGoto(t, bt) })
		//t.Run("should work when navigating to data URI", func(t *testing.T) { testPageGotoDataURI(t, bt) })
		t.Run("should wait for load event", func(t *testing.T) { testPageGotoWaitUntilLoad(t, bt) })
		t.Run("should wait for domcontentloaded event", func(t *testing.T) { testPageGotoWaitUntilDOMContentLoaded(t, bt) })
	})
}

func testPageGoto(t *testing.T, bt *browsertest.BrowserTest) {
	p := bt.Browser.NewPage(nil)
	defer p.Close(nil)

	url := bt.HTTPMultiBin.ServerHTTP.URL + "/empty.html"
	r := p.Goto(url, nil)

	assert.Equal(t, url, r.URL(), `expected URL to be %q, result of navigation was %q`, url, r.URL())
}

func testPageGotoDataURI(t *testing.T, bt *browsertest.BrowserTest) {
	p := bt.Browser.NewPage(nil)
	defer p.Close(nil)
	r := p.Goto("data:text/html,hello", nil)
	assert.Nil(t, r, `expected response to be nil`)
}

func testPageGotoWaitUntilLoad(t *testing.T, bt *browsertest.BrowserTest) {
	p := bt.Browser.NewPage(nil)
	defer p.Close(nil)

	p.Goto(bt.HTTPMultiBin.ServerHTTP.URL+"/wait_until.html", bt.Runtime.ToValue(struct {
		WaitUntil string `js:"waitUntil"`
	}{WaitUntil: "load"}))

	results := p.Evaluate(bt.Runtime.ToValue("() => window.results"))
	var actual []string
	bt.Runtime.ExportTo(results.(goja.Value), &actual)

	assert.EqualValues(t, []string{"DOMContentLoaded", "load"}, actual, `expected "load" event to have fired`)
}

func testPageGotoWaitUntilDOMContentLoaded(t *testing.T, bt *browsertest.BrowserTest) {
	p := bt.Browser.NewPage(nil)
	defer p.Close(nil)

	p.Goto(bt.HTTPMultiBin.ServerHTTP.URL+"/wait_until.html", bt.Runtime.ToValue(struct {
		WaitUntil string `js:"waitUntil"`
	}{WaitUntil: "domcontentloaded"}))

	results := p.Evaluate(bt.Runtime.ToValue("() => window.results"))
	var actual []string
	bt.Runtime.ExportTo(results.(goja.Value), &actual)

	assert.EqualValues(t, "DOMContentLoaded", actual[0], `expected "DOMContentLoaded" event to have fired`)
}

func TestPageSetExtraHTTPHeaders(t *testing.T) {
	bt := browsertest.NewBrowserTest(t)
	p := bt.Browser.NewPage(nil)
	t.Cleanup(func() {
		p.Close(nil)
		bt.Browser.Close()
	})

	headers := map[string]string{
		"Some-Header": "Some-Value",
	}
	p.SetExtraHTTPHeaders(headers)
	resp := p.Goto(bt.HTTPMultiBin.ServerHTTP.URL+"/get", nil)

	require.NotNil(t, resp)
	var body struct{ Headers map[string][]string }
	err := json.Unmarshal(resp.Body().Bytes(), &body)
	require.NoError(t, err)
	h := body.Headers["Some-Header"]
	require.NotEmpty(t, h)
	assert.Equal(t, "Some-Value", h[0])
}
