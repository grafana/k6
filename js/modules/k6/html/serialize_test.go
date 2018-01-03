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

const testSerializeHTML = `
<html>
<head>
	<title>This is the title</title>
</head>
<body>
	<h1 id="top" data-test="dataval" data-num-a="123" data-num-b="1.5" data-not-num-a="1.50" data-not-num-b="1.1e02">Lorem ipsum</h1>

	<p data-test-b="true" data-opts='{"id":101}' data-test-empty="">Lorem ipsum dolor sit amet, consectetur adipiscing elit. Donec ac dui erat. Pellentesque eu euismod odio, eget fringilla ante. In vitae nulla at est tincidunt gravida sit amet maximus arcu. Sed accumsan tristique massa, blandit sodales quam malesuada eu. Morbi vitae luctus augue. Nunc nec ligula quam. Cras fringilla nulla leo, at dignissim enim accumsan vitae. Sed eu cursus sapien, a rhoncus lorem. Etiam sed massa egestas, bibendum quam sit amet, eleifend ipsum. Maecenas mi ante, consectetur at tincidunt id, suscipit nec sem. Integer congue elit vel ligula commodo ultricies. Suspendisse condimentum laoreet ligula at aliquet.</p>
	<p>Nullam id nisi eget ex pharetra imperdiet. Maecenas augue ligula, aliquet sit amet maximus ut, vestibulum et magna. Nam in arcu sed tortor volutpat porttitor sed eget dolor. Duis rhoncus est id dui porttitor, id molestie ex imperdiet. Proin purus ligula, pretium eleifend felis a, tempor feugiat mi. Cras rutrum pulvinar neque, eu dictum arcu. Cras purus metus, fermentum eget malesuada sit amet, dignissim non dui.</p>

	<form id="form1">
		<input id="text_input" name="text_input" type="text" value="input-text-value"/>
		<input id="text_input" name="empty_text_input1" type="text" value="" />
		<input id="text_input" name="empty_text_input2" type="text" />
		<select id="select_one" name="select_one">
			<option value="not this option">no</option>
			<option value="yes this option" selected>yes</option>
		</select>
		<select id="select_text" name="select_text">
			<option>no text</option>
			<option selected>yes text</option>
		</select>
		<select id="select_multi" name="select_multi" multiple>
			<option>option 1</option>
			<option selected>option 2</option>
			<option selected>option 3</option>
		</select>
		<textarea id="textarea" name="textarea" multiple>Lorem ipsum dolor sit amet</textarea>
	</form>

	<footer>This is the footer.</footer>
</body>
`

func TestSerialize(t *testing.T) {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	ctx := common.WithRuntime(context.Background(), rt)
	rt.Set("src", testSerializeHTML)
	rt.Set("html", common.Bind(rt, New(), &ctx))

	_, err := common.RunString(rt, `let doc = html.parseHTML(src)`)
	assert.NoError(t, err)
	assert.IsType(t, Selection{}, rt.Get("doc").Export())

	t.Run("SerializeArray", func(t *testing.T) {
		v, err := common.RunString(rt, `doc.find("form").serializeArray()`)
		if assert.NoError(t, err) {
			arr := v.Export().([]FormValue)
			assert.Equal(t, 7, len(arr))

			assert.Equal(t, "text_input", arr[0].Name)
			assert.Equal(t, "input-text-value", arr[0].Value.String())

			assert.Equal(t, "empty_text_input1", arr[1].Name)
			assert.Equal(t, "", arr[1].Value.String())

			assert.Equal(t, "empty_text_input2", arr[2].Name)
			assert.Equal(t, "", arr[2].Value.String())

			assert.Equal(t, "select_one", arr[3].Name)
			assert.Equal(t, "yes this option", arr[3].Value.String())

			assert.Equal(t, "select_text", arr[4].Name)
			assert.Equal(t, "yes text", arr[4].Value.String())

			multiValues := arr[5].Value.Export().([]string)
			assert.Equal(t, "select_multi", arr[5].Name)
			assert.Equal(t, 2, len(multiValues))
			assert.Equal(t, "option 2", multiValues[0])
			assert.Equal(t, "option 3", multiValues[1])

			assert.Equal(t, "textarea", arr[6].Name)
			assert.Equal(t, "Lorem ipsum dolor sit amet", arr[6].Value.String())
		}
	})

	t.Run("SerializeObject", func(t *testing.T) {
		v, err := common.RunString(rt, `doc.find("form").serializeObject()`)
		if assert.NoError(t, err) {
			obj := v.Export().(map[string]goja.Value)
			assert.Equal(t, 7, len(obj))

			assert.Equal(t, "input-text-value", obj["text_input"].String())
			assert.Equal(t, "", obj["empty_text_input1"].String())
			assert.Equal(t, "", obj["empty_text_input2"].String())
			assert.Equal(t, "yes this option", obj["select_one"].String())
			assert.Equal(t, "yes text", obj["select_text"].String())
			assert.Equal(t, "Lorem ipsum dolor sit amet", obj["textarea"].String())

			multiValues := obj["select_multi"].Export().([]string)
			assert.Equal(t, "option 2", multiValues[0])
			assert.Equal(t, "option 3", multiValues[1])
		}
	})

	t.Run("Serialize", func(t *testing.T) {
		v, err := common.RunString(rt, `doc.find("form").serialize()`)
		if assert.NoError(t, err) {
			url := v.String()
			assert.Equal(t, "empty_text_input1=" + 
				"&empty_text_input2=" + 
				"&select_multi=option+2"+
				"&select_multi=option+3"+
				"&select_one=yes+this+option"+
				"&select_text=yes+text"+
				"&text_input=input-text-value"+
				"&textarea=Lorem+ipsum+dolor+sit+amet", url)
		}
	})
}
