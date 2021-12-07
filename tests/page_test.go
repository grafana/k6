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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPageGoto(t *testing.T) {
	t.Parallel()

	b := testBrowser(t, withFileServer())

	p := b.NewPage(nil)

	url := b.staticURL("empty.html")
	r := p.Goto(url, nil)

	assert.Equal(t, url, r.URL(), `expected URL to be %q, result of navigation was %q`, url, r.URL())
}

func TestPageGotoDataURI(t *testing.T) {
	t.Parallel()

	p := testBrowser(t).NewPage(nil)

	r := p.Goto("data:text/html,hello", nil)

	assert.Nil(t, r, `expected response to be nil`)
}

func TestPageGotoWaitUntilLoad(t *testing.T) {
	t.Parallel()

	b := testBrowser(t, withFileServer())

	p := b.NewPage(nil)

	p.Goto(b.staticURL("wait_until.html"), b.rt.ToValue(struct {
		WaitUntil string `js:"waitUntil"`
	}{WaitUntil: "load"}))

	results := p.Evaluate(b.rt.ToValue("() => window.results"))
	var actual []string
	_ = b.rt.ExportTo(results.(goja.Value), &actual)

	assert.EqualValues(t, []string{"DOMContentLoaded", "load"}, actual, `expected "load" event to have fired`)
}

func TestPageGotoWaitUntilDOMContentLoaded(t *testing.T) {
	t.Parallel()

	b := testBrowser(t, withFileServer())

	p := b.NewPage(nil)

	p.Goto(b.staticURL("wait_until.html"), b.rt.ToValue(struct {
		WaitUntil string `js:"waitUntil"`
	}{WaitUntil: "domcontentloaded"}))

	results := p.Evaluate(b.rt.ToValue("() => window.results"))
	var actual []string
	_ = b.rt.ExportTo(results.(goja.Value), &actual)

	assert.EqualValues(t, "DOMContentLoaded", actual[0], `expected "DOMContentLoaded" event to have fired`)
}

func TestPageSetExtraHTTPHeaders(t *testing.T) {
	t.Parallel()

	b := testBrowser(t, withHTTPServer())

	p := b.NewPage(nil)

	headers := map[string]string{
		"Some-Header": "Some-Value",
	}
	p.SetExtraHTTPHeaders(headers)

	resp := p.Goto(b.URL("/get"), nil)

	require.NotNil(t, resp)
	var body struct{ Headers map[string][]string }
	err := json.Unmarshal(resp.Body().Bytes(), &body)
	require.NoError(t, err)
	h := body.Headers["Some-Header"]
	require.NotEmpty(t, h)
	assert.Equal(t, "Some-Value", h[0])
}
