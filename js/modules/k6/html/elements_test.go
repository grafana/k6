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
<head><link rel="alternate next"/></head>
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
	
	<form id="form1" action="action_url" enctype="text/plain" target="_self">
		<label for="form_btn" id="form_btn_label"></label>
		<button id="form_btn" name="form_btn" accesskey="b" autofocus disabled></button>
		<label for="form_btn_2" id="form_btn_2_label"></label>
		<label id="wrapper_label">
			<button id="form_btn_2" type="button" formaction="override_action_url" formenctype="multipart/form-data" formmethod="post" formnovalidate formtarget="_top" value="form_btn_2_initval"></button>
		</label>
	</form>
	<form id="form2"></form>
	<button id="named_form_btn" form="form2"></button>
	<button id="no_form_btn"></button>
	<canvas width="200"></canvas>
	<datalist id="datalist1"><option id="dl_opt_1"/><option id="dl_opt_2"/></datalist>
	<form method="post" id="fieldset_form"><fieldset id="fieldset_1"><legend id="legend_1">Legend title</legend><input id="test_dl_input" type="text" list="datalist1"><input type="text"><select></select><button></button><textarea></textarea></fieldset></form>
	<ul><li id="li_nil"/></ul>
	<ol><li id="li_first"/><li value="10"/><li id="li_eleven"/></ol>
	<map id="not_this_map"></map>
	<map id="find_this_map"><area/><area/><area/></map>
	<img usemap="#find_this_map"/><object usemap="#find_this_map"/><img usemap="#not_this_map"/>
	<object id="obj_1" form="form1"/>
	<form id="form3"><select><option id="opt_1">txt_label</option><option id="opt_2" disabled label="attr_label" value="attr_val"/><optgroup disabled><option id="opt_3"/></optgroup></select></form>
	<select form="form3"><option id="opt_4"/></select>
	<label for="output1"></label>
	<label><output id="output1" form="form3">defaultVal</output></label>
	<progress id="progress1" max="100" value="70"></progress>
	<progress id="progress2"></progress>
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
				assert.Equal(t, "form_btn_label", v.Export().([]goja.Value)[0].Export().(LabelElement).Id())
			}
			if v, err := common.RunString(rt, `doc.find("#form_btn_2").get(0).labels()`); assert.NoError(t, err) {
				assert.Equal(t, 2, len(v.Export().([]goja.Value)))
				assert.Equal(t, "wrapper_label", v.Export().([]goja.Value)[0].Export().(LabelElement).Id())
				assert.Equal(t, "form_btn_2_label", v.Export().([]goja.Value)[1].Export().(LabelElement).Id())
			}
		})
		t.Run("name", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn").get(0).name()`); assert.NoError(t, err) {
				assert.Equal(t, "form_btn", v.Export())
			}
		})
		t.Run("value", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#form_btn_2").get(0).value()`); assert.NoError(t, err) {
				assert.Equal(t, "form_btn_2_initval", v.Export())
			}
		})
	})
	t.Run("CanvasElement", func(t *testing.T) {
		t.Run("width", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("canvas").get(0).width()`); assert.NoError(t, err) {
				assert.Equal(t, int64(200), v.Export())
			}
		})
		t.Run("height", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("canvas").get(0).height()`); assert.NoError(t, err) {
				assert.Equal(t, int64(150), v.Export())
			}
		})
	})
	t.Run("DataListElement options", func(t *testing.T) {
		if v, err := common.RunString(rt, `doc.find("datalist").get(0).options()`); assert.NoError(t, err) {
			assert.Equal(t, 2, len(v.Export().([]goja.Value)))
			assert.Equal(t, "dl_opt_1", v.Export().([]goja.Value)[0].Export().(OptionElement).Id())
			assert.Equal(t, "dl_opt_2", v.Export().([]goja.Value)[1].Export().(OptionElement).Id())
		}
	})
	t.Run("FieldSetElement", func(t *testing.T) {
		t.Run("elements", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("fieldset").get(0).elements()`); assert.NoError(t, err) {
				assert.Equal(t, 5, len(v.Export().([]goja.Value)))
			}
		})
		t.Run("type", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("fieldset").get(0).type()`); assert.NoError(t, err) {
				assert.Equal(t, "fieldset", v.Export())
			}
		})
		t.Run("form", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("fieldset").get(0).form().id()`); assert.NoError(t, err) {
				assert.Equal(t, "fieldset_form", v.Export())
			}
		})
	})
	t.Run("FormElement", func(t *testing.T) {
		t.Run("elements", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#fieldset_form").get(0).elements()`); assert.NoError(t, err) {
				assert.Equal(t, 6, len(v.Export().([]goja.Value)))
			}
		})
		t.Run("length", func(t *testing.T) {
			if v, err := common.RunString(rt, `doc.find("#fieldset_form").get(0).length()`); assert.NoError(t, err) {
				assert.Equal(t, int64(6), v.Export())
			}
		})
		t.Run("method", func(t *testing.T) {
			v1, err1 := common.RunString(rt, `doc.find("#form1").get(0).method()`)
			v2, err2 := common.RunString(rt, `doc.find("#fieldset_form").get(0).method()`)
			if assert.NoError(t, err1) && assert.NoError(t, err2) {
				assert.Equal(t, "get", v1.Export())
				assert.Equal(t, "post", v2.Export())
			}
		})
		t.Run("InputElement", func(t *testing.T) {
			t.Run("form", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#test_dl_input").get(0).list().options()`); assert.NoError(t, err) {
					assert.Equal(t, 2, len(v.Export().([]goja.Value)))
				}
			})
		})
		t.Run("LabelElement", func(t *testing.T) {
			t.Run("control", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#form_btn_2_label").get(0).control().value()`); assert.NoError(t, err) {
					assert.Equal(t, "form_btn_2_initval", v.Export())
				}
			})
			t.Run("form", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#form_btn_2_label").get(0).form().id()`); assert.NoError(t, err) {
					assert.Equal(t, "form1", v.Export())
				}
			})
		})
		t.Run("LegendElement", func(t *testing.T) {
			t.Run("form", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#legend_1").get(0).form().id()`); assert.NoError(t, err) {
					assert.Equal(t, "fieldset_form", v.Export())
				}
			})
		})
		t.Run("LiElement", func(t *testing.T) {
			t.Run("value nil", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#li_nil").get(0).value()`); assert.NoError(t, err) {
					assert.Equal(t, nil, v.Export())
				}
			})
			t.Run("value 1", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#li_first").get(0).value()`); assert.NoError(t, err) {
					assert.Equal(t, int64(1), v.Export())
				}
			})
			t.Run("value 11", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#li_eleven").get(0).value()`); assert.NoError(t, err) {
					assert.Equal(t, int64(11), v.Export())
				}
			})
		})
		t.Run("LinkElement", func(t *testing.T) {
			t.Run("rel list", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("link").get(0).relList()`); assert.NoError(t, err) {
					assert.Equal(t, []string{"alternate", "next"}, v.Export())
				}
			})
		})
		t.Run("MapElement", func(t *testing.T) {
			t.Run("areas", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#find_this_map").get(0).areas()`); assert.NoError(t, err) {
					assert.Equal(t, 3, len(v.Export().([]goja.Value)))
				}
			})
			t.Run("images", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#find_this_map").get(0).images()`); assert.NoError(t, err) {
					assert.Equal(t, 2, len(v.Export().([]goja.Value)))
				}
			})
		})
		t.Run("ObjectElement", func(t *testing.T) {
			t.Run("form", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#obj_1").get(0).form().id()`); assert.NoError(t, err) {
					assert.Equal(t, "form1", v.Export())
				}
			})
		})
		t.Run("OptionElement", func(t *testing.T) {
			t.Run("disabled", func(t *testing.T) {
				v1, err1 := common.RunString(rt, `doc.find("#opt_1").get(0).disabled()`)
				v2, err2 := common.RunString(rt, `doc.find("#opt_2").get(0).disabled()`)
				v3, err3 := common.RunString(rt, `doc.find("#opt_3").get(0).disabled()`)
				if assert.NoError(t, err1) && assert.NoError(t, err2) && assert.NoError(t, err3) {
					assert.Equal(t, false, v1.Export())
					assert.Equal(t, true, v2.Export())
					assert.Equal(t, true, v3.Export())
				}
			})
			t.Run("form", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#opt_4").get(0).form().id()`); assert.NoError(t, err) {
					assert.Equal(t, "form3", v.Export())
				}
			})
			t.Run("index", func(t *testing.T) {
				v1, err1 := common.RunString(rt, `doc.find("#dl_opt_2").get(0).index()`)
				v2, err2 := common.RunString(rt, `doc.find("#opt_3").get(0).index()`)
				if assert.NoError(t, err1) && assert.NoError(t, err2) {
					assert.Equal(t, int64(1), v1.Export())
					assert.Equal(t, int64(2), v2.Export())
				}
			})
			t.Run("label", func(t *testing.T) {
				v1, err1 := common.RunString(rt, `doc.find("#opt_1").get(0).label()`)
				v2, err2 := common.RunString(rt, `doc.find("#opt_2").get(0).label()`)
				if assert.NoError(t, err1) && assert.NoError(t, err2) {
					assert.Equal(t, "txt_label", v1.Export())
					assert.Equal(t, "attr_label", v2.Export())
				}
			})
			t.Run("text", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#opt_1").get(0).text()`); assert.NoError(t, err) {
					assert.Equal(t, "txt_label", v.Export())
				}
			})
			t.Run("value", func(t *testing.T) {
				v1, err1 := common.RunString(rt, `doc.find("#opt_1").get(0).value()`)
				v2, err2 := common.RunString(rt, `doc.find("#opt_2").get(0).value()`)
				if assert.NoError(t, err1) && assert.NoError(t, err2) {
					assert.Equal(t, "txt_label", v1.Export())
					assert.Equal(t, "attr_val", v2.Export())
				}
			})
		})
		t.Run("OutputElement", func(t *testing.T) {
			t.Run("value", func(t *testing.T) {
				v1, err1 := common.RunString(rt, `doc.find("#output1").get(0).value()`)
				v2, err2 := common.RunString(rt, `doc.find("#output1").get(0).defaultValue()`)
				if assert.NoError(t, err1) && assert.NoError(t, err2) {
					assert.Equal(t, "defaultVal", v1.Export())
					assert.Equal(t, "defaultVal", v2.Export())
				}
			})
			t.Run("labels", func(t *testing.T) {
				if v, err := common.RunString(rt, `doc.find("#output1").get(0).labels()`); assert.NoError(t, err) {
					assert.Equal(t, 2, len(v.Export().([]goja.Value)))
				}
			})
		})
		t.Run("ProgressElement", func(t *testing.T) {
			t.Run("max", func(t *testing.T) {
				v1, err1 := common.RunString(rt, `doc.find("#progress1").get(0).max()`)
				v2, err2 := common.RunString(rt, `doc.find("#progress2").get(0).max()`)
				if assert.NoError(t, err1) && assert.NoError(t, err2) {
					assert.Equal(t, float64(100), v1.Export())
					assert.Equal(t, float64(1), v2.Export())
				}
			})
			t.Run("value", func(t *testing.T) {
				v1, err1 := common.RunString(rt, `doc.find("#progress1").get(0).value()`)
				v2, err2 := common.RunString(rt, `doc.find("#progress2").get(0).value()`)
				if assert.NoError(t, err1) && assert.NoError(t, err2) {
					assert.Equal(t, float64(0.7), v1.Export())
					assert.Equal(t, float64(0), v2.Export())
				}
			})
			t.Run("position", func(t *testing.T) {
				v1, err1 := common.RunString(rt, `doc.find("#progress1").get(0).position()`)
				v2, err2 := common.RunString(rt, `doc.find("#progress2").get(0).position()`)
				if assert.NoError(t, err1) && assert.NoError(t, err2) {
					assert.Equal(t, float64(0.7), v1.Export())
					assert.Equal(t, float64(-1), v2.Export())
				}
			})
		})
	})
}
