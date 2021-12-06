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

	"github.com/stretchr/testify/assert"
)

func TestElementHandleInputValue(t *testing.T) {
	bt := TestBrowser(t)

	t.Run("ElementHandle.inputValue", func(t *testing.T) {
		t.Run("should work", func(t *testing.T) { testElementHandleInputValue(t, bt) })
	})
}

func testElementHandleInputValue(t *testing.T, bt *Browser) {
	p := bt.Browser.NewPage(nil)
	defer p.Close(nil)

	p.SetContent(`
        <input value="hello1">
        <select><option value="hello2" selected></option></select>
        <textarea>hello3</textarea>
    `, nil)

	element := p.Query("input")
	value := element.InputValue(nil)
	element.Dispose()
	assert.Equal(t, value, "hello1", `expected input value "hello1", got %q`, value)

	element = p.Query("select")
	value = element.InputValue(nil)
	element.Dispose()
	assert.Equal(t, value, "hello2", `expected input value "hello2", got %q`, value)

	element = p.Query("textarea")
	value = element.InputValue(nil)
	element.Dispose()
	assert.Equal(t, value, "hello3", `expected input value "hello3", got %q`, value)
}
