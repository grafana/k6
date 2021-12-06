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

	"github.com/grafana/xk6-browser/api"
	"github.com/stretchr/testify/assert"
)

var pageInputTests = map[string]func(*testing.T, api.Browser){
	"value":              testPageInputValue,
	"special_characters": testPageInputSpecialCharacters,
}

func TestPageInput(t *testing.T) {
	bt := TestBrowser(t)

	for name, test := range pageInputTests {
		t.Run(name, func(t *testing.T) {
			test(t, bt.Browser)
		})
	}
}

func testPageInputValue(t *testing.T, b api.Browser) {
	p := b.NewPage(nil)
	defer p.Close(nil)

	p.SetContent(`
		<input value="hello1">
		<select><option value="hello2" selected></option></select>
		<textarea>hello3</textarea>
     	`, nil)

	got, want := p.InputValue("input", nil), "hello1"
	assert.Equal(t, got, want)

	got, want = p.InputValue("select", nil), "hello2"
	assert.Equal(t, got, want)

	got, want = p.InputValue("textarea", nil), "hello3"
	assert.Equal(t, got, want)
}

// test for: https://github.com/grafana/xk6-browser/issues/132
func testPageInputSpecialCharacters(t *testing.T, b api.Browser) {
	p := b.NewPage(nil)
	defer p.Close(nil)

	p.SetContent(`<input id="special">`, nil)
	el := p.Query("#special")

	wants := []string{
		"test@k6.io",
		"<hello WoRlD \\/>",
		"{(hello world!)}",
		"!#$%^&*()+_|~±",
		`¯\_(ツ)_/¯`,
	}
	for _, want := range wants {
		el.Fill("", nil)
		el.Type(want, nil)

		got := el.InputValue(nil)
		assert.Equal(t, want, got)
	}
}
