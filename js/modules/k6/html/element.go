package html

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	gohtml "golang.org/x/net/html"
)

type Element struct {
	sel  *Selection
	rt   *goja.Runtime
	qsel *goquery.Selection
	node *gohtml.Node
}

type Attribute struct {
	Name         string
	nsPrefix     string
	OwnerElement *Element
	Value        string
}

func namespaceURI(prefix string) string {
	switch prefix {
	case "svg":
		return "http://www.w3.org/2000/svg"
	case "math":
		return "http://www.w3.org/1998/Math/MathML"
	default:
		return "http://www.w3.org/1999/xhtml"
	}
}

func (a Attribute) Prefix() string {
	return a.nsPrefix
}

func (a Attribute) NamespaceURI() string {
	return namespaceURI(a.nsPrefix)
}

func (a Attribute) LocalName() string {
	return a.Name
}

func (e Element) GetAttribute(name string) goja.Value {
	return e.sel.Attr(name)
}

func (e Element) GetAttributeNode(name string) goja.Value {
	if attr := getHtmlAttr(e.node, name); attr != nil {
		return e.rt.ToValue(Attribute{attr.Key, attr.Namespace, &e, attr.Val})
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

func (e Element) Attributes() map[string]Attribute {
	attrs := make(map[string]Attribute)
	for i := 0; i < len(e.node.Attr); i++ {
		attr := e.node.Attr[i]
		attrs[attr.Key] = Attribute{attr.Key, attr.Namespace, &e, attr.Val}
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
	if other, ok := v.Export().(Element); ok {
		htmlA, errA := e.qsel.Html()
		htmlB, errB := other.qsel.Html()

		return errA == nil && errB == nil && htmlA == htmlB
	} else {
		return false
	}
}

func (e Element) IsSameNode(v goja.Value) bool {
	if other, ok := v.Export().(Element); ok {
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
	return namespaceURI(e.node.Namespace)
}

func (e Element) IsDefaultNamespace() bool {
	return e.node.Namespace == ""
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
	if other, ok := v.Export().(Element); ok {
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

func valToElementList(val goja.Value) (elems []Element) {
	vals := val.Export().([]goja.Value)
	for i := 0; i < len(vals); i++ {
		elems = append(elems, vals[i].Export().(Element))
	}
	return
}

func selToElement(sel Selection) goja.Value {
	if sel.sel.Length() == 0 {
		return goja.Undefined()
	} else if sel.sel.Length() > 1 {
		sel = sel.First()
	}

	elem := Element{&sel, sel.rt, sel.sel, sel.sel.Nodes[0]}

	return sel.rt.ToValue(elem)
}
