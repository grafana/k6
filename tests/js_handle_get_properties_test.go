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
	"testing"

	"github.com/k6io/xk6-browser/testutils/browsertest"
	"github.com/stretchr/testify/assert"
)

func TestJSHandleGetProperties(t *testing.T) {
	bt := browsertest.NewBrowserTest(t, false)
	defer bt.Browser.Close()

	t.Run("JSHandle.getProperties", func(t *testing.T) {
		t.Run("should work", func(t *testing.T) { testJSHandleGetProperties(t, bt) })
	})
}

func testJSHandleGetProperties(t *testing.T, bt *browsertest.BrowserTest) {
	p := bt.Browser.NewPage(nil)
	defer p.Close(nil)

	handle := p.EvaluateHandle(bt.Runtime.ToValue(`() => {
        return {
            prop1: "one",
            prop2: "two",
            prop3: "three"
        };
    }`))

	props := handle.GetProperties()
	value := props["prop1"].JSONValue().String()
	assert.Equal(t, value, "one", `expected property value of "one", got %q`, value)
}
