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
	"bytes"
	_ "embed"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPageScreenshot(t *testing.T) {
	bt := TestBrowser(t)

	t.Run("Page.screenshot", func(t *testing.T) {
		t.Run("should work with full page", func(t *testing.T) { testPageScreenshotFullpage(t, bt) })
	})
}

func testPageScreenshotFullpage(t *testing.T, bt *Browser) {
	p := bt.Browser.NewPage(nil)
	defer p.Close(nil)

	p.SetViewportSize(bt.Runtime.ToValue(struct {
		Width  float64 `js:"width"`
		Height float64 `js:"height"`
	}{Width: 1280, Height: 800}))
	p.Evaluate(bt.Runtime.ToValue(`
        () => {
            document.body.style.margin = '0';
            document.body.style.padding = '0';
            document.documentElement.style.margin = '0';
            document.documentElement.style.padding = '0';
            const div = document.createElement('div');
            div.style.width = '1280px';
            div.style.height = '8000px';
            div.style.background = 'linear-gradient(red, blue)';
            document.body.appendChild(div);
        }
    `))

	buf := p.Screenshot(bt.Runtime.ToValue(struct {
		FullPage bool `js:"fullPage"`
	}{FullPage: true}))

	reader := bytes.NewReader(buf.Bytes())
	img, err := png.Decode(reader)
	assert.Nil(t, err)

	assert.Equal(t, 1280, img.Bounds().Max.X, "screenshot width is not 1280px as expected, but %dpx", img.Bounds().Max.X)
	assert.Equal(t, 8000, img.Bounds().Max.Y, "screenshot height is not 8000px as expected, but %dpx", img.Bounds().Max.Y)

	r, _, b, _ := img.At(0, 0).RGBA()
	assert.Greater(t, r, uint32(128))
	assert.Less(t, b, uint32(128))
	r, _, b, _ = img.At(0, 7999).RGBA()
	assert.Less(t, r, uint32(128))
	assert.Greater(t, b, uint32(128))
}
