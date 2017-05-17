package html

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	gohtml "golang.org/x/net/html"
)

var (
	protoPrg *goja.Program
)

type Element struct {
	sel  *Selection
	rt   *goja.Runtime
	qsel *goquery.Selection
	node *gohtml.Node
}

type Attribute struct {
	Name         string
	NamespaceURI string
	LocalName    string
	Prefix       string
	OwnerElement goja.Value
	Value        string
}

func (e Element) GetAttribute(name string) goja.Value {
	return e.sel.Attr(name)
}

func (e Element) GetAttributeNode(self goja.Value, name string) goja.Value {
	if attr := getHtmlAttr(e.node, name); attr != nil {
		return e.rt.ToValue(Attribute{attr.Key, attr.Namespace, attr.Namespace, attr.Namespace, self, attr.Val})
	} else {
		return goja.Undefined()
	}
}

func (e Element) HasAttribute(name string) bool {
	return e.sel.Attr(name) != goja.Undefined()
}

func (e Element) HasAttributes() bool {
	return e.qsel.Length() > 0 && len(e.node.Attr) > 0
}

func (e Element) Attributes(self goja.Value) map[string]Attribute {
	attrs := make(map[string]Attribute)
	for i := 0; i < len(e.node.Attr); i++ {
		attr := e.node.Attr[i]
		attrs[attr.Key] = Attribute{attr.Key, attr.Namespace, attr.Namespace, attr.Namespace, self, attr.Val}
	}
	return attrs
}

func (e Element) ToString() goja.Value {
	if e.qsel.Length() == 0 {
		return goja.Undefined()
	} else if e.node.Type == gohtml.ElementNode {
		return e.rt.ToValue("[object html.Node]")
	} else {
		return e.rt.ToValue(fmt.Sprintf("[object %s]", e.NodeName()))
	}
}

func (e Element) HasChildNodes() bool {
	return e.qsel.Length() > 0 && e.node.FirstChild != nil
}

func (e Element) TextContent() string {
	return e.qsel.Text()
}

func (e Element) Id() goja.Value {
	return e.GetAttribute("id")
}

func (e Element) IsEqualNode(v goja.Value) bool {
	if other, ok := valToElement(v); ok {
		htmlA, errA := e.qsel.Html()
		htmlB, errB := other.qsel.Html()

		return errA == nil && errB == nil && htmlA == htmlB
	} else {
		return false
	}
}

func (e Element) IsSameNode(v goja.Value) bool {
	if other, ok := valToElement(v); ok {
		return e.node == other.node
	} else {
		return false
	}
}

func (e Element) GetElementsByClassName(name string) []goja.Value {
	return elemList(Selection{e.rt, e.qsel.Find("." + name)})
}

func (e Element) GetElementsByTagName(name string) []goja.Value {
	return elemList(Selection{e.rt, e.qsel.Find(name)})
}

func (e Element) QuerySelector(selector string) goja.Value {
	return selToElement(Selection{e.rt, e.qsel.Find(selector)})
}

func (e Element) QuerySelectorAll(selector string) []goja.Value {
	return elemList(Selection{e.rt, e.qsel.Find(selector)})
}

func (e Element) NodeName() string {
	return goquery.NodeName(e.qsel)
}

func (e Element) FirstChild() goja.Value {
	return nodeToElement(e, e.node.FirstChild)
}

func (e Element) LastChild() goja.Value {
	return nodeToElement(e, e.node.LastChild)
}

func (e Element) FirstElementChild() goja.Value {
	if child := e.qsel.Children().First(); child.Length() > 0 {
		return selToElement(Selection{e.rt, child.First()})
	} else {
		return goja.Undefined()
	}
}

func (e Element) LastElementChild() goja.Value {
	if child := e.qsel.Children(); child.Length() > 0 {
		return selToElement(Selection{e.rt, child.Last()})
	} else {
		return goja.Undefined()
	}
}

func (e Element) PreviousSibling() goja.Value {
	return nodeToElement(e, e.node.PrevSibling)
}

func (e Element) NextSibling() goja.Value {
	return nodeToElement(e, e.node.NextSibling)
}

func (e Element) PreviousElementSibling() goja.Value {
	if prev := e.qsel.Prev(); prev.Length() > 0 {
		return selToElement(Selection{e.rt, prev})
	} else {
		return goja.Undefined()
	}
}

func (e Element) NextElementSibling() goja.Value {
	if next := e.qsel.Next(); next.Length() > 0 {
		return selToElement(Selection{e.rt, next})
	} else {
		return goja.Undefined()
	}
}

func (e Element) ParentNode() goja.Value {
	if e.node.Parent != nil {
		return nodeToElement(e, e.node.Parent)
	} else {
		return goja.Undefined()
	}
}

func (e Element) ParentElement() goja.Value {
	if prt := e.qsel.Parent(); prt.Length() > 0 {
		return selToElement(Selection{e.rt, prt})
	} else {
		return goja.Undefined()
	}
}

func (e Element) ChildNodes() []goja.Value {
	return elemList(e.sel.Contents())
}

func (e Element) Children() []goja.Value {
	return elemList(e.sel.Children())
}

func (e Element) ChildElementCount() int {
	return e.sel.Children().Size()
}

func (e Element) ClassList() []string {
	if clsName, exists := e.qsel.Attr("class"); exists {
		return strings.Fields(clsName)
	} else {
		return nil
	}
}

func (e Element) ClassName() goja.Value {
	return e.sel.Attr("class")
}

func (e Element) Lang() goja.Value {
	if attr := getHtmlAttr(e.node, "lang"); attr != nil && attr.Namespace == "" {
		return e.rt.ToValue(attr.Val)
	} else {
		return goja.Undefined()
	}
}

func (e Element) OwnerDocument() goja.Value {
	if node := getOwnerDocNode(e.node); node != nil {
		return nodeToElement(e, node)
	} else {
		return goja.Undefined()
	}
}

func (e Element) NamespaceURI() string {
	return e.node.Namespace
}

func (e Element) IsDefaultNamespace() bool {
	// 	TODO namespace value of node always seems to be blank?
	return false
}

func getOwnerDocNode(node *gohtml.Node) *gohtml.Node {
	for ; node != nil; node = node.Parent {
		if node.Type == gohtml.DocumentNode {
			return node
		}
	}
	return nil
}

func (e Element) InnerHTML() goja.Value {
	return e.sel.Html()
}

func (e Element) NodeType() goja.Value {
	switch e.node.Type {
	case gohtml.TextNode:
		return e.rt.ToValue(3)

	case gohtml.DocumentNode:
		return e.rt.ToValue(9)

	case gohtml.ElementNode:
		return e.rt.ToValue(1)

	case gohtml.CommentNode:
		return e.rt.ToValue(8)

	case gohtml.DoctypeNode:
		return e.rt.ToValue(10)

	default:
		return goja.Undefined()
	}
}

func (e Element) NodeValue() goja.Value {
	switch e.node.Type {
	case gohtml.TextNode:
		return e.rt.ToValue(e.sel.Text())

	case gohtml.CommentNode:
		return e.rt.ToValue(e.sel.Text())

	default:
		return goja.Undefined()
	}
}

func (e Element) Contains(v goja.Value) bool {
	if other, ok := valToElement(v); ok {
		// when testing if a node contains itself, jquery + goquery Contains() return true, JS return false
		return other.node == e.node || e.qsel.Contains(other.node)
	} else {
		return false
	}
}

func (e Element) Matches(selector string) bool {
	return e.qsel.Is(selector)
}

//helper methods
func getHtmlAttr(node *gohtml.Node, name string) *gohtml.Attribute {
	for i := 0; i < len(node.Attr); i++ {
		if node.Attr[i].Key == name {
			return &node.Attr[i]
		}
	}

	return nil
}

func elemList(s Selection) (items []goja.Value) {
	for i := 0; i < s.Size(); i++ {
		items = append(items, selToElement(s.Eq(i)))
	}
	return items
}

func nodeToElement(e Element, node *gohtml.Node) goja.Value {
	emptySel := e.qsel.Eq(e.qsel.Length())
	emptySel.Nodes = append(emptySel.Nodes, node)

	sel := Selection{e.rt, emptySel}

	return selToElement(sel)
}

func valToElementList(val goja.Value) (elems []*Element) {
	vals := val.Export().([]goja.Value)
	for i := 0; i < len(vals); i++ {
		if elem, ok := valToElement(vals[i]); ok {
			elems = append(elems, elem)
		}
	}
	return
}

func valToElement(v goja.Value) (*Element, bool) {
	obj, ok := v.Export().(map[string]interface{})

	if !ok {
		return nil, false
	}

	other, ok := obj["__elem__"]

	if !ok {
		return nil, false
	}

	if elem, ok := other.(*Element); ok {
		return elem, true
	} else {
		return nil, false
	}
}

func selToElement(sel Selection) goja.Value {
	if sel.sel.Length() == 0 {
		return goja.Undefined()
	} else if sel.sel.Length() > 1 {
		sel = sel.First()
	}

	elem := sel.rt.NewObject()

	e := Element{&sel, sel.rt, sel.sel, sel.sel.Nodes[0]}

	proto, ok := initJsElem(sel.rt)
	if !ok {
		return goja.Undefined()
	}

	elem.Set("__proto__", proto)
	elem.Set("__elem__", sel.rt.ToValue(&e))

	return sel.rt.ToValue(elem)
}

func initJsElem(rt *goja.Runtime) (goja.Value, bool) {
	if protoPrg == nil {
		compileProtoElem()
	}

	obj, err := rt.RunProgram(protoPrg)
	if err != nil {
		panic(err)
	}

	return obj, true
}

func compileProtoElem() {
	protoPrg = common.MustCompile("Element proto", `Object.freeze({
	get id() { return this.__elem__.id(); },
	get nodeName() { return this.__elem__.nodeName(); },
	get nodeType() { return this.__elem__.nodeType(); },
	get nodeValue() { return this.__elem__.nodeValue(); },
	get innerHTML() { return this.__elem__.innerHTML(); },
	get textContent() { return this.__elem__.textContent(); },

	get attributes() { return this.__elem__.attributes(this); },

	get firstChild() { return this.__elem__.firstChild(); },
	get lastChild() { return this.__elem__.lastChild(); },
	get firstElementChild() { return this.__elem__.firstElementChild(); },
	get lastElementChild() { return this.__elem__.lastElementChild(); },

	get previousSibling() { return this.__elem__.previousSibling(); },
	get nextSibling() { return this.__elem__.nextSibling(); },

	get previousElementSibling() { return this.__elem__.previousElementSibling(); },
	get nextElementSibling() { return this.__elem__.nextElementSibling(); },

	get parentNode() { return this.__elem__.parentNode(); },
	get parentElement() { return this.__elem__.parentElement(); },

	get childNodes() { return this.__elem__.childNodes(); },
	get childElementCount() { return this.__elem__.childElementCount(); },
	get children() { return this.__elem__.children(); },

	get classList() { return this.__elem__.classList(); },
	get className() { return this.__elem__.className(); },

	get lang() { return this.__elem__.lang(); },
	get ownerDocument() { return this.__elem__.ownerDocument(); },
	get namespaceURI() { return this.__elem__.namespaceURI(); },


	toString: function() { return this.__elem__.toString(); },
	hasAttribute: function(name) { return this.__elem__.hasAttribute(name); },
	getAttribute: function(name) { return this.__elem__.getAttribute(name); },
	getAttributeNode: function(name) { return this.__elem__.getAttributeNode(this, name); },
	hasAttributes: function() { return this.__elem__.hasAttributes(); },
	hasChildNodes: function() { return this.__elem__.hasChildNodes(); },
	isSameNode: function(val) { return this.__elem__.isSameNode(val); },
	isEqualNode: function(val) { return this.__elem__.isEqualNode(val); },
	getElementsByClassName: function(val) { return this.__elem__.getElementsByClassName(val); },
	getElementsByTagName: function(val) { return this.__elem__.getElementsByTagName(val); },

	querySelector: function(val) { return this.__elem__.querySelector(val); },
	querySelectorAll: function(val) { return this.__elem__.querySelectorAll(val); },

	contains: function(node) { return this.__elem__.contains(node); }
	matches: function(str) { return this.__elem__.matches(str); }

});
`, true)
}
