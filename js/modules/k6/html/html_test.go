/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package html

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/stretchr/testify/assert"
)

const testHTML = `
<html>
<head>
	<title>This is the title</title>
</head>
<body>
	<h1 id="top">Lorem ipsum</h1>

	<p>Lorem ipsum dolor sit amet, consectetur adipiscing elit. Donec ac dui erat. Pellentesque eu euismod odio, eget fringilla ante. In vitae nulla at est tincidunt gravida sit amet maximus arcu. Sed accumsan tristique massa, blandit sodales quam malesuada eu. Morbi vitae luctus augue. Nunc nec ligula quam. Cras fringilla nulla leo, at dignissim enim accumsan vitae. Sed eu cursus sapien, a rhoncus lorem. Etiam sed massa egestas, bibendum quam sit amet, eleifend ipsum. Maecenas mi ante, consectetur at tincidunt id, suscipit nec sem. Integer congue elit vel ligula commodo ultricies. Suspendisse condimentum laoreet ligula at aliquet.</p>
	<p>Nullam id nisi eget ex pharetra imperdiet. Maecenas augue ligula, aliquet sit amet maximus ut, vestibulum et magna. Nam in arcu sed tortor volutpat porttitor sed eget dolor. Duis rhoncus est id dui porttitor, id molestie ex imperdiet. Proin purus ligula, pretium eleifend felis a, tempor feugiat mi. Cras rutrum pulvinar neque, eu dictum arcu. Cras purus metus, fermentum eget malesuada sit amet, dignissim non dui.</p>

	<footer>This is the footer.</footer>
</body>
`

func TestParseHTML(t *testing.T) {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	ctx := common.WithRuntime(context.Background(), rt)
	rt.Set("src", testHTML)
	rt.Set("html", common.Bind(rt, New(), &ctx))

	// TODO: I literally cannot think of a snippet that makes goquery error.
	// I'm not sure if it's even possible without like, an invalid reader or something, which would
	// be impossible to cause from the JS side.
	_, err := common.RunString(rt, `let doc = html.parseHTML(src)`)
	assert.NoError(t, err)
	assert.IsType(t, Selection{}, rt.Get("doc").Export())

	t.Run("Find", func(t *testing.T) {
		v, err := common.RunString(rt, `doc.find("h1")`)
		if assert.NoError(t, err) && assert.IsType(t, Selection{}, v.Export()) {
			sel := v.Export().(Selection).sel
			assert.Equal(t, 1, sel.Length())
			assert.Equal(t, "Lorem ipsum", sel.Text())
		}
	})
	t.Run("Add", func(t *testing.T) {
		t.Run("Selector", func(t *testing.T) {
			v, err := common.RunString(rt, `doc.find("h1").add("footer")`)
			if assert.NoError(t, err) && assert.IsType(t, Selection{}, v.Export()) {
				sel := v.Export().(Selection).sel
				assert.Equal(t, 2, sel.Length())
				assert.Equal(t, "Lorem ipsumThis is the footer.", sel.Text())
			}
		})
		t.Run("Selection", func(t *testing.T) {
			v, err := common.RunString(rt, `doc.find("h1").add(doc.find("footer"))`)
			if assert.NoError(t, err) && assert.IsType(t, Selection{}, v.Export()) {
				sel := v.Export().(Selection).sel
				assert.Equal(t, 2, sel.Length())
				assert.Equal(t, "Lorem ipsumThis is the footer.", sel.Text())
			}
		})
	})

	t.Run("Text", func(t *testing.T) {
		v, err := common.RunString(rt, `doc.find("h1").text()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "Lorem ipsum", v.Export())
		}
	})
	t.Run("Attr", func(t *testing.T) {
		v, err := common.RunString(rt, `doc.find("h1").attr("id")`)
		if assert.NoError(t, err) {
			assert.Equal(t, "top", v.Export())
		}

		t.Run("Default", func(t *testing.T) {
			v, err := common.RunString(rt, `doc.find("h1").attr("id", "default")`)
			if assert.NoError(t, err) {
				assert.Equal(t, "top", v.Export())
			}
		})

		t.Run("Unset", func(t *testing.T) {
			v, err := common.RunString(rt, `doc.find("h1").attr("class")`)
			if assert.NoError(t, err) {
				assert.True(t, goja.IsUndefined(v), "v is not undefined: %v", v)
			}

			t.Run("Default", func(t *testing.T) {
				v, err := common.RunString(rt, `doc.find("h1").attr("class", "default")`)
				if assert.NoError(t, err) {
					assert.Equal(t, "default", v.Export())
				}
			})
		})
	})
}
