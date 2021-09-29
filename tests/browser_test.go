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
	"regexp"
	"testing"

	"github.com/k6io/xk6-browser/api"
	"github.com/k6io/xk6-browser/testutils/browsertest"
	"github.com/stretchr/testify/assert"
)

func TestBrowserModule(t *testing.T) {
	bt := browsertest.NewBrowserTest(t, false)
	defer bt.Browser.Close()

	t.Run("Browser", func(t *testing.T) {
		t.Run("newPage", func(t *testing.T) { testBrowserNewPage(t, bt.Browser) })
		t.Run("version", func(t *testing.T) { testBrowserVersion(t, bt.Browser) })
	})
}

func testBrowserNewPage(t *testing.T, b api.Browser) {
	p := b.NewPage(nil)
	l := len(b.Contexts())
	assert.Equal(t, 1, l, "expected there to be 1 browser context, but found %d", l)

	p2 := b.NewPage(nil)
	l = len(b.Contexts())
	assert.Equal(t, 2, l, "expected there to be 2 browser context, but found %d", l)

	p.Close(nil)
	l = len(b.Contexts())
	assert.Equal(t, 1, l, "expected there to be 1 browser context after first page close, but found %d", l)
	p2.Close(nil)
	l = len(b.Contexts())
	assert.Equal(t, 0, l, "expected there to be 0 browser context after second page close, but found %d", l)
}

func testBrowserVersion(t *testing.T, b api.Browser) {
	version := b.Version()
	r, _ := regexp.Compile(`^\d+\.\d+\.\d+\.\d+$`) // This only works for Chrome!
	assert.Regexp(t, r, version, "expected browser version to match regex '^\\d+\\.\\d+\\.\\d+\\.\\d+', but found %q", version)
}
