package html

import (
	"testing"

	"github.com/dop251/goja"
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
		<input type="checkbox" id="checkbox1" name="checkbox1" value="some checkbox" checked="checked" />
		<input type="checkbox" id="checkbox2" name="checkbox2" checked="checked" />
		<input type="checkbox" id="checkbox3" name="checkbox3" />
		<input type="radio" id="radio1" name="radio1" value="some radio" checked="checked" />
		<input type="radio" id="radio2" name="radio2" checked="checked" />
		<input type="radio" id="radio3" name="radio3" />
	</form>
	<footer>This is the footer.</footer>
</body>
`

func TestSerialize(t *testing.T) {
	t.Parallel()

	t.Run("SerializeArray", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testSerializeHTML)

		t.Run("form", func(t *testing.T) {
			v, err := rt.RunString(`doc.find("form").serializeArray()`)
			if assert.NoError(t, err) {
				arr, ok := v.Export().([]FormValue)
				assert.True(t, ok)
				assert.Len(t, arr, 11)

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

				multiValues, ok := arr[5].Value.Export().([]string)

				assert.True(t, ok)
				assert.Equal(t, "select_multi", arr[5].Name)
				assert.Equal(t, 2, len(multiValues))
				assert.Equal(t, "option 2", multiValues[0])
				assert.Equal(t, "option 3", multiValues[1])

				assert.Equal(t, "textarea", arr[6].Name)
				assert.Equal(t, "Lorem ipsum dolor sit amet", arr[6].Value.String())

				assert.Equal(t, "checkbox1", arr[7].Name)
				assert.Equal(t, "some checkbox", arr[7].Value.String())

				assert.Equal(t, "checkbox2", arr[8].Name)
				assert.Equal(t, "on", arr[8].Value.String())

				assert.Equal(t, "radio1", arr[9].Name)
				assert.Equal(t, "some radio", arr[9].Value.String())

				assert.Equal(t, "radio2", arr[10].Name)
				assert.Equal(t, "on", arr[10].Value.String())
			}
		})

		t.Run("controls", func(t *testing.T) {
			v, err := rt.RunString(`doc.find("input").serializeArray()`)
			if assert.NoError(t, err) {
				arr, ok := v.Export().([]FormValue)

				assert.True(t, ok)
				assert.Len(t, arr, 7)

				assert.Equal(t, "text_input", arr[0].Name)
				assert.Equal(t, "input-text-value", arr[0].Value.String())

				assert.Equal(t, "empty_text_input1", arr[1].Name)
				assert.Equal(t, "", arr[1].Value.String())

				assert.Equal(t, "empty_text_input2", arr[2].Name)
				assert.Equal(t, "", arr[2].Value.String())

				assert.Equal(t, "checkbox1", arr[3].Name)
				assert.Equal(t, "some checkbox", arr[3].Value.String())

				assert.Equal(t, "checkbox2", arr[4].Name)
				assert.Equal(t, "on", arr[4].Value.String())

				assert.Equal(t, "radio1", arr[5].Name)
				assert.Equal(t, "some radio", arr[5].Value.String())

				assert.Equal(t, "radio2", arr[6].Name)
				assert.Equal(t, "on", arr[6].Value.String())
			}
		})
	})

	t.Run("SerializeObject", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testSerializeHTML)

		v, err := rt.RunString(`doc.find("form").serializeObject()`)
		if assert.NoError(t, err) {
			obj, ok := v.Export().(map[string]goja.Value)

			assert.True(t, ok)
			assert.Equal(t, 11, len(obj))

			assert.Equal(t, "input-text-value", obj["text_input"].String())
			assert.Equal(t, "", obj["empty_text_input1"].String())
			assert.Equal(t, "", obj["empty_text_input2"].String())
			assert.Equal(t, "yes this option", obj["select_one"].String())
			assert.Equal(t, "yes text", obj["select_text"].String())
			assert.Equal(t, "Lorem ipsum dolor sit amet", obj["textarea"].String())
			assert.Equal(t, "some checkbox", obj["checkbox1"].String())
			assert.Equal(t, "on", obj["checkbox2"].String())
			assert.Equal(t, "some radio", obj["radio1"].String())
			assert.Equal(t, "on", obj["radio2"].String())

			multiValues, ok := obj["select_multi"].Export().([]string)

			assert.True(t, ok)
			assert.Equal(t, "option 2", multiValues[0])
			assert.Equal(t, "option 3", multiValues[1])
		}
	})

	t.Run("Serialize", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testSerializeHTML)

		v, err := rt.RunString(`doc.find("form").serialize()`)
		if assert.NoError(t, err) {
			url := v.String()
			assert.Equal(t, "checkbox1=some+checkbox"+
				"&checkbox2=on"+
				"&empty_text_input1="+
				"&empty_text_input2="+
				"&radio1=some+radio"+
				"&radio2=on"+
				"&select_multi=option+2"+
				"&select_multi=option+3"+
				"&select_one=yes+this+option"+
				"&select_text=yes+text"+
				"&text_input=input-text-value"+
				"&textarea=Lorem+ipsum+dolor+sit+amet", url)
		}
	})
}
