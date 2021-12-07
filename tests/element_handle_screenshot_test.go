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

func TestElementHandleScreenshot(t *testing.T) {
	t.Parallel()

	tb := testBrowser(t)
	p := tb.NewPage(nil)

	p.SetViewportSize(tb.rt.ToValue(struct {
		Width  float64 `js:"width"`
		Height float64 `js:"height"`
	}{Width: 800, Height: 600}))
	p.Evaluate(tb.rt.ToValue(`
         () => {
             document.body.style.margin = '0';
             document.body.style.padding = '0';
             document.documentElement.style.margin = '0';
             document.documentElement.style.padding = '0';
             const div = document.createElement('div');
             div.style.marginTop = '400px';
             div.style.marginLeft = '100px';
             div.style.width = '100px';
             div.style.height = '100px';
             div.style.background = 'red';
             document.body.appendChild(div);
         }
    `))

	elem := p.Query("div")
	buf := elem.Screenshot(nil)

	reader := bytes.NewReader(buf.Bytes())
	img, err := png.Decode(reader)
	assert.Nil(t, err)

	assert.Equal(t, 100, img.Bounds().Max.X, "screenshot width is not 100px as expected, but %dpx", img.Bounds().Max.X)
	assert.Equal(t, 100, img.Bounds().Max.Y, "screenshot height is not 100px as expected, but %dpx", img.Bounds().Max.Y)

	r, g, b, _ := img.At(0, 0).RGBA()
	assert.Equal(t, uint32(255), r>>8) // each color component has been scaled by alpha (<<8)
	assert.Equal(t, uint32(0), g)
	assert.Equal(t, uint32(0), b)
	r, g, b, _ = img.At(99, 99).RGBA()
	assert.Equal(t, uint32(255), r>>8) // each color component has been scaled by alpha (<<8)
	assert.Equal(t, uint32(0), g)
	assert.Equal(t, uint32(0), b)
}
