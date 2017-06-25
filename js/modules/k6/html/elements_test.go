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

const testHTMLElems = `
<html>
<head></head>
<body>
	<a href="/testhref?querytxt#hashtext">0</a>
	<a href="http://example.com:80">1</a>
	<a href="http://example.com:81/path/file">2</a>
	<a href="https://ssl.example.com:443/">3</a>
	<a href="https://ssl.example.com:444/">4</a>
	<a href="http://username:password@example.com:80">5</a>
	<a href="http://example.com" rel="prev next" target="_self" type="rare" accesskey="q" hreflang="en-US" media="print">6</a>
	<area href="web.address.com"></area>
	<base href="/rel/path" target="_self"></base>
	
	<form id="form1" action="action_url" enctype="text/plain" method="get" target="_self">
		<label for="form_btn" id="form_btn_label"></label>
		<button id="form_btn" name="form_btn" accesskey="b" autofocus disabled></button>
		<label for="form_btn_2" id="form_btn_2_label"></label>
		<label id="wrapper_label">
			<button id="form_btn_2" type="button" formaction="override_action_url" formenctype="multipart/form-data" formmethod="post" formnovalidate formtarget="_top" value="initval"></button>
		</label>
	</form>
	<form id="form2"></form>
	<button id="named_form_btn" form="form2"></button>
	
	<button id="no_form_btn"></button>
</body>
`

func TestElements(t *testing.T) {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	ctx := common.WithRuntime(context.Background(), rt)
	rt.Set("src", testHTMLElems)
	rt.Set("html", common.Bind(rt, &HTML{}, &ctx))
	// compileProtoElem()

	_, err := common.RunString(rt, `let doc = html.parseHTML(src)`)

	assert.NoError(t, err)
	assert.IsType(t, Selection{}, rt.Get("doc").Export())

	t.Run("AnchorElement", func(t *testing.T) {
		t.Run("Hash", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(0).hash()`); assert.NoError(t, err) {
				assert.Equal(t, "#hashtext", v.Export())
			}
		})
		t.Run("Host", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(1).host()`); assert.NoError(t, err) {
				assert.Equal(t, "example.com", v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("a").get(2).host()`); assert.NoError(t, err) {
				assert.Equal(t, "example.com:81", v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("a").get(3).host()`); assert.NoError(t, err) {
				assert.Equal(t, "ssl.example.com", v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("a").get(4).host()`); assert.NoError(t, err) {
				assert.Equal(t, "ssl.example.com:444", v.Export())
			}
		})
		t.Run("Hostname", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(1).hostname()`); assert.NoError(t, err) {
				assert.Equal(t, "example.com", v.Export())
			}
		})
		t.Run("Port", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(5).port()`); assert.NoError(t, err) {
				assert.Equal(t, "80", v.Export())
			}
		})
		t.Run("Username", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(5).username()`); assert.NoError(t, err) {
				assert.Equal(t, "username", v.Export())
			}
		})
		t.Run("Password", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(5).password()`); assert.NoError(t, err) {
				assert.Equal(t, "password", v.Export())
			}
		})
		t.Run("Origin", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(5).origin()`); assert.NoError(t, err) {
				assert.Equal(t, "http://example.com:80", v.Export())
			}
		})
		t.Run("Pathname", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(1).pathname()`); assert.NoError(t, err) {
				assert.Equal(t, "", v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("a").get(2).pathname()`); assert.NoError(t, err) {
				assert.Equal(t, "/path/file", v.Export())
			}
		})
		t.Run("Protocol", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(4).protocol()`); assert.NoError(t, err) {
				assert.Equal(t, "https", v.Export())
			}
		})
		t.Run("RelList", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(6).relList()`); assert.NoError(t, err) {
				assert.Equal(t, []string{"prev", "next"}, v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("a").get(5).relList()`); assert.NoError(t, err) {
				assert.Equal(t, []string{}, v.Export())
			}
		})
		t.Run("Search", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(0).search()`); assert.NoError(t, err) {
				assert.Equal(t, "?querytxt", v.Export())
			}
		})
		t.Run("Text", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("a").get(6).text()`); assert.NoError(t, err) {
				assert.Equal(t, "6", v.Export())
			}
		})
	})
	t.Run("AreaElement", func(t *testing.T) {
		if v, err := common.RunString(rt, `doc.find("area").get(0).toString()`); assert.NoError(t, err) {
			assert.Equal(t, "web.address.com", v.Export())
		}
	})
	t.Run("ButtonElement", func(t *testing.T) {
		t.Run("form", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn").get(0).form().id()`); assert.NoError(t, err) {
				assert.Equal(t, "form1", v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("#named_form_btn").get(0).form().id()`); assert.NoError(t, err) {
				assert.Equal(t, "form2", v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("#no_form_btn").get(0).form()`); assert.NoError(t, err) {
				assert.Equal(t, nil, v.Export())
			}
		})
		t.Run("formaction", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn").get(0).formAction()`); assert.NoError(t, err) {
				assert.Equal(t, "action_url", v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("#form_btn_2").get(0).formAction()`); assert.NoError(t, err) {
				assert.Equal(t, "override_action_url", v.Export())
			}
		})
		t.Run("formenctype", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn").get(0).formEnctype()`); assert.NoError(t, err) {
				assert.Equal(t, "text/plain", v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("#form_btn_2").get(0).formEnctype()`); assert.NoError(t, err) {
				assert.Equal(t, "multipart/form-data", v.Export())
			}
		})
		t.Run("formmethod", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn").get(0).formMethod()`); assert.NoError(t, err) {
				assert.Equal(t, "get", v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("#form_btn_2").get(0).formMethod()`); assert.NoError(t, err) {
				assert.Equal(t, "post", v.Export())
			}
		})
		t.Run("formnovalidate", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn").get(0).formNoValidate()`); assert.NoError(t, err) {
				assert.Equal(t, false, v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("#form_btn_2").get(0).formNoValidate()`); assert.NoError(t, err) {
				assert.Equal(t, true, v.Export())
			}
		})
		t.Run("formtarget", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn").get(0).formTarget()`); assert.NoError(t, err) {
				assert.Equal(t, "_self", v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("#form_btn_2").get(0).formTarget()`); assert.NoError(t, err) {
				assert.Equal(t, "_top", v.Export())
			}
		})
		t.Run("labels", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn").get(0).labels()`); assert.NoError(t, err) {
				assert.Equal(t, 1, len(v.Export().([]goja.Value)))
				assert.Equal(t, "form_btn_label", v.Export().([]goja.Value)[0].Export().(Element).Id())
			}
			if v, err := common.RunString(rt, `doc.find("#form_btn_2").get(0).labels()`); assert.NoError(t, err) {
				assert.Equal(t, 2, len(v.Export().([]goja.Value)))
				assert.Equal(t, "wrapper_label", v.Export().([]goja.Value)[0].Export().(Element).Id())
				assert.Equal(t, "form_btn_2_label", v.Export().([]goja.Value)[1].Export().(Element).Id())
			}
		})
		t.Run("name", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn").get(0).name()`); assert.NoError(t, err) {
				assert.Equal(t, "form_btn", v.Export())
			}
		})
		t.Run("type", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn").get(0).type()`); assert.NoError(t, err) {
				assert.Equal(t, "submit", v.Export())
			}
			if v, err := common.RunString(rt, `doc.find("#form_btn_2").get(0).type()`); assert.NoError(t, err) {
				assert.Equal(t, "button", v.Export())
			}
		})
		t.Run("value", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn_2").get(0).value()`); assert.NoError(t, err) {
				assert.Equal(t, "initval", v.Export())
			}
		})
	})
}
