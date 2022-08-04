package html

import (
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
)

const testHTMLElem = `
<html>
<head>
	<title>This is the title</title>
</head>
<body>
	<h1 id="top">Lorem ipsum</h1>
	<empty></empty>
	<p>Lorem ipsum dolor sit amet, consectetur adipiscing elit.</p>
	<p>Lorem ipsum dolor sit amet, consectetur adipiscing elit.</p>
	pretext
	<div id="div_elem" class="class1 class2" lang="en">
		innerfirst
		<h2 id="h2_elem" class="class2">Nullam id nisi eget ex pharetra imperdiet.</h2>
		<span id="span1"><b>test content</b></span>
		<svg id="svg_elem"></svg>
		<span id="span2">Maecenas augue ligula, aliquet sit amet maximus ut, vestibulum et magna</span>
		innerlast
	</div>
	aftertext
	<footer>This is the footer.</footer>
</body>
`

func TestElement(t *testing.T) {
	t.Parallel()
	t.Run("NodeName", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("#top").get(0).nodeName()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "h1", v.Export())
		}
	})
	t.Run("NodeType", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("#top").get(0).nodeType()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "1", v.String())
		}
	})
	t.Run("NodeValue", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("#top").get(0).firstChild().nodeValue()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "Lorem ipsum", v.String())
		}
	})
	t.Run("InnerHtml", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("span").get(0).innerHTML()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "<b>test content</b>", v.String())
		}
	})
	t.Run("TextContent", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("b").get(0).textContent()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "test content", v.String())
		}
	})
	t.Run("OwnerDocument", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("body").get(0).ownerDocument().nodeName()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "#document", v.String())
		}
	})
	t.Run("Attributes", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).attributes()`)
		if assert.NoError(t, err) {
			attrs, ok := v.Export().(map[string]Attribute)

			assert.True(t, ok)
			assert.Equal(t, "div_elem", attrs["id"].Value)
		}
	})
	t.Run("FirstChild", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).firstChild().nodeValue()`)
		if assert.NoError(t, err) {
			assert.Contains(t, v.Export(), "innerfirst")
		}
	})
	t.Run("LastChild", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).lastChild().nodeValue()`)
		if assert.NoError(t, err) {
			assert.Contains(t, v.Export(), "innerlast")
		}
	})
	t.Run("ChildElementCount", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("body").get(0).childElementCount()`)
		if assert.NoError(t, err) {
			assert.Equal(t, int64(6), v.Export())
		}
	})
	t.Run("FirstElementChild", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).firstElementChild().textContent()`)
		if assert.NoError(t, err) {
			assert.Contains(t, v.Export(), "Nullam id nisi ")
		}
	})
	t.Run("LastElementChild", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).lastElementChild().textContent()`)
		if assert.NoError(t, err) {
			assert.Contains(t, v.Export(), "Maecenas augue ligula")
		}
	})
	t.Run("PreviousSibling", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).previousSibling().textContent()`)
		if assert.NoError(t, err) {
			assert.Contains(t, v.Export(), "pretext")
		}
	})
	t.Run("NextSibling", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).nextSibling().textContent()`)
		if assert.NoError(t, err) {
			assert.Contains(t, v.Export(), "aftertext")
		}
	})
	t.Run("PreviousElementSibling", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).previousElementSibling().textContent()`)
		if assert.NoError(t, err) {
			assert.Contains(t, v.Export(), "consectetur adipiscing elit")
		}
	})
	t.Run("NextElementSibling", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).nextElementSibling().textContent()`)
		if assert.NoError(t, err) {
			assert.Contains(t, v.Export(), "This is the footer.")
		}
	})
	t.Run("ParentElement", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).parentElement().nodeName()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "body", v.String())
		}
	})
	t.Run("ParentNode", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		nodeVal, err1 := rt.RunString(`doc.find("html").get(0).parentNode().nodeName()`)
		nilVal, err2 := rt.RunString(`doc.find("html").get(0).parentElement()`)
		if assert.NoError(t, err1) && assert.NoError(t, err2) {
			assert.Equal(t, "#document", nodeVal.String())
			assert.Equal(t, nil, nilVal.Export())
		}
	})
	t.Run("ChildNodes", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).childNodes()`)
		if assert.NoError(t, err) {
			nodes, ok := v.Export().([]goja.Value)

			assert.True(t, ok)
			assert.Equal(t, 9, len(nodes))
			assert.Contains(t, nodes[0].Export().(Element).TextContent(), "innerfirst")
			assert.Contains(t, nodes[8].Export().(Element).TextContent(), "innerlast")
		}
	})
	t.Run("Children", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).children()`)
		if assert.NoError(t, err) {
			nodes, ok := v.Export().([]goja.Value)

			assert.True(t, ok)
			assert.Equal(t, 4, len(nodes))
			assert.Contains(t, nodes[0].Export().(Element).TextContent(), "Nullam id nisi eget ex")
			assert.Contains(t, nodes[3].Export().(Element).TextContent(), "Maecenas augue ligula")
		}
	})
	t.Run("ClassList", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).classList()`)
		if assert.NoError(t, err) {
			clsNames, ok := v.Export().([]string)

			assert.True(t, ok)
			assert.Equal(t, 2, len(clsNames))
			assert.Equal(t, []string{"class1", "class2"}, clsNames)
		}
	})
	t.Run("ClassName", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).className()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "class1 class2", v.String())
		}
	})
	t.Run("Lang", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).lang()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "en", v.String())
		}
	})
	t.Run("ToString", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v1, err1 := rt.RunString(`doc.find("div").get(0).toString()`)
		v2, err2 := rt.RunString(`doc.find("div").get(0).previousSibling().toString()`)
		if assert.NoError(t, err1) && assert.NoError(t, err2) {
			assert.Equal(t, "[object html.Node]", v1.String())
			assert.Equal(t, "[object #text]", v2.String())
		}
	})
	t.Run("HasAttribute", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v1, err1 := rt.RunString(`doc.find("div").get(0).hasAttribute("id")`)
		v2, err2 := rt.RunString(`doc.find("div").get(0).hasAttribute("noattr")`)
		if assert.NoError(t, err1) && assert.NoError(t, err2) {
			assert.Equal(t, true, v1.Export())
			assert.Equal(t, false, v2.Export())
		}
	})
	t.Run("GetAttribute", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).getAttribute("id")`)
		if assert.NoError(t, err) {
			assert.Equal(t, "div_elem", v.Export())
		}
	})
	t.Run("GetAttributeNode", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).getAttributeNode("id")`)
		if assert.NoError(t, err) && assert.IsType(t, Attribute{}, v.Export()) {
			assert.Equal(t, "div_elem", v.Export().(Attribute).Value)
		}
	})
	t.Run("HasAttributes", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v1, err1 := rt.RunString(`doc.find("h1").get(0).hasAttributes()`)
		v2, err2 := rt.RunString(`doc.find("footer").get(0).hasAttributes()`)
		if assert.NoError(t, err1) && assert.NoError(t, err2) {
			assert.Equal(t, true, v1.Export())
			assert.Equal(t, false, v2.Export())
		}
	})
	t.Run("HasChildNodes", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v1, err1 := rt.RunString(`doc.find("p").get(0).hasChildNodes()`)
		v2, err2 := rt.RunString(`doc.find("empty").get(0).hasChildNodes()`)
		if assert.NoError(t, err1) && assert.NoError(t, err2) {
			assert.Equal(t, true, v1.Export())
			assert.Equal(t, false, v2.Export())
		}
	})
	t.Run("IsSameNode", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v1, err1 := rt.RunString(`doc.find("p").get(0).isSameNode(doc.find("p").get(1))`)
		v2, err2 := rt.RunString(`doc.find("p").get(0).isSameNode(doc.find("p").get(0))`)
		if assert.NoError(t, err1) && assert.NoError(t, err2) {
			assert.Equal(t, false, v1.Export())
			assert.Equal(t, true, v2.Export())
		}
	})
	t.Run("IsEqualNode", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v1, err1 := rt.RunString(`doc.find("p").get(0).isEqualNode(doc.find("p").get(1))`)
		v2, err2 := rt.RunString(`doc.find("p").get(0).isEqualNode(doc.find("p").get(0))`)
		if assert.NoError(t, err1) && assert.NoError(t, err2) {
			assert.Equal(t, true, v1.Export())
			assert.Equal(t, true, v2.Export())
		}
	})
	t.Run("GetElementsByClassName", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("body").get(0).getElementsByClassName("class2")`)
		if assert.NoError(t, err) {
			elems, ok := v.Export().([]goja.Value)

			assert.True(t, ok)
			assert.Equal(t, "div_elem", elems[0].Export().(Element).Id())
			assert.Equal(t, "h2_elem", elems[1].Export().(Element).Id())
		}
	})
	t.Run("GetElementsByTagName", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("body").get(0).getElementsByTagName("span")`)
		if assert.NoError(t, err) {
			elems, ok := v.Export().([]goja.Value)

			assert.True(t, ok)
			assert.Equal(t, 2, len(elems))
		}
	})
	t.Run("QuerySelector", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("body").get(0).querySelector("#div_elem").id()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "div_elem", v.Export())
		}
	})
	t.Run("QuerySelectorAll", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("body").get(0).querySelectorAll("span")`)
		if assert.NoError(t, err) {
			elems, ok := v.Export().([]goja.Value)

			assert.True(t, ok)
			assert.Len(t, elems, 2)
			assert.Equal(t, "span1", elems[0].Export().(Element).Id())
			assert.Equal(t, "span2", elems[1].Export().(Element).Id())
		}
	})
	t.Run("Contains", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v1, err1 := rt.RunString(`doc.find("html").get(0).contains(doc.find("body").get(0))`)
		v2, err2 := rt.RunString(`doc.find("body").get(0).contains(doc.find("body").get(0))`)
		if assert.NoError(t, err1) && assert.NoError(t, err2) {
			assert.Equal(t, true, v1.Export())
			assert.Equal(t, false, v2.Export())
		}
	})
	t.Run("Matches", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("div").get(0).matches("#div_elem")`)
		if assert.NoError(t, err) {
			assert.Equal(t, true, v.Export())
		}
	})
	t.Run("NamespaceURI", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v, err := rt.RunString(`doc.find("#svg_elem").get(0).namespaceURI()`)
		if assert.NoError(t, err) {
			assert.Equal(t, "http://www.w3.org/2000/svg", v.Export())
		}
	})
	t.Run("IsDefaultNamespace", func(t *testing.T) {
		t.Parallel()
		rt := getTestRuntimeWithDoc(t, testHTMLElem)

		v1, err1 := rt.RunString(`doc.find("#svg_elem").get(0).isDefaultNamespace()`)
		v2, err2 := rt.RunString(`doc.find("#div_elem").get(0).isDefaultNamespace()`)
		if assert.NoError(t, err1) && assert.NoError(t, err2) {
			assert.Equal(t, false, v1.ToBoolean())
			assert.Equal(t, true, v2.ToBoolean())
		}
	})
}
