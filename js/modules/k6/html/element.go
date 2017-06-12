package html

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	gohtml "golang.org/x/net/html"
)

const (
	TextNode     = 3
	DocumentNode = 9
	ElementNode  = 1
	CommentNode  = 1
	DoctypeNode  = 10
)

type Element struct {
	node *gohtml.Node
	sel  *Selection
}

type Attribute struct {
	Name         string
	nsPrefix     string
	OwnerElement *Element
	Value        string
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
		return e.sel.rt.ToValue(Attribute{attr.Key, attr.Namespace, &e, attr.Val})
	}

	return goja.Undefined()
}

func (e Element) HasAttribute(name string) bool {
	_, exists := e.sel.sel.Attr(name)
	return exists
}

func (e Element) HasAttributes() bool {
	return e.sel.sel.Length() > 0 && len(e.node.Attr) > 0
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
	if e.sel.sel.Length() == 0 {
		return goja.Undefined()
	}

	if e.node.Type == gohtml.ElementNode {
		return e.sel.rt.ToValue("[object html.Node]")
	}

	return e.sel.rt.ToValue(fmt.Sprintf("[object %s]", e.NodeName()))
}

func (e Element) HasChildNodes() bool {
	return e.sel.sel.Length() > 0 && e.node.FirstChild != nil
}

func (e Element) TextContent() string {
	return e.sel.sel.Text()
}

func (e Element) Id() goja.Value {
	return e.GetAttribute("id")
}

func (e Element) IsEqualNode(v goja.Value) bool {
	if other, ok := v.Export().(Element); ok {
		htmlA, errA := e.sel.sel.Html()
		htmlB, errB := other.sel.sel.Html()

		return errA == nil && errB == nil && htmlA == htmlB
	}

	return false
}

func (e Element) IsSameNode(v goja.Value) bool {
	if other, ok := v.Export().(Element); ok {
		return e.node == other.node
	}

	return false
}

func (e Element) GetElementsByClassName(name string) []goja.Value {
	return elemList(Selection{e.sel.rt, e.sel.sel.Find("." + name)})
}

func (e Element) GetElementsByTagName(name string) []goja.Value {
	return elemList(Selection{e.sel.rt, e.sel.sel.Find(name)})
}

func (e Element) QuerySelector(selector string) goja.Value {
	return selToElement(Selection{e.sel.rt, e.sel.sel.Find(selector)})
}

func (e Element) QuerySelectorAll(selector string) []goja.Value {
	return elemList(Selection{e.sel.rt, e.sel.sel.Find(selector)})
}

func (e Element) NodeName() string {
	return goquery.NodeName(e.sel.sel)
}

func (e Element) FirstChild() goja.Value {
	return nodeToElement(e, e.node.FirstChild)
}

func (e Element) LastChild() goja.Value {
	return nodeToElement(e, e.node.LastChild)
}

func (e Element) FirstElementChild() goja.Value {
	if child := e.sel.sel.Children().First(); child.Length() > 0 {
		return selToElement(Selection{e.sel.rt, child.First()})
	}

	return goja.Undefined()
}

func (e Element) LastElementChild() goja.Value {
	if child := e.sel.sel.Children(); child.Length() > 0 {
		return selToElement(Selection{e.sel.rt, child.Last()})
	}

	return goja.Undefined()
}

func (e Element) PreviousSibling() goja.Value {
	return nodeToElement(e, e.node.PrevSibling)
}

func (e Element) NextSibling() goja.Value {
	return nodeToElement(e, e.node.NextSibling)
}

func (e Element) PreviousElementSibling() goja.Value {
	if prev := e.sel.sel.Prev(); prev.Length() > 0 {
		return selToElement(Selection{e.sel.rt, prev})
	}

	return goja.Undefined()
}

func (e Element) NextElementSibling() goja.Value {
	if next := e.sel.sel.Next(); next.Length() > 0 {
		return selToElement(Selection{e.sel.rt, next})
	}

	return goja.Undefined()
}

func (e Element) ParentNode() goja.Value {
	if e.node.Parent != nil {
		return nodeToElement(e, e.node.Parent)
	}

	return goja.Undefined()
}

func (e Element) ParentElement() goja.Value {
	if prt := e.sel.sel.Parent(); prt.Length() > 0 {
		return selToElement(Selection{e.sel.rt, prt})
	}

	return goja.Undefined()
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
	if clsName, exists := e.sel.sel.Attr("class"); exists {
		return strings.Fields(clsName)
	}

	return nil
}

func (e Element) ClassName() goja.Value {
	return e.sel.Attr("class")
}

func (e Element) Lang() goja.Value {
	if attr := getHtmlAttr(e.node, "lang"); attr != nil && attr.Namespace == "" {
		return e.sel.rt.ToValue(attr.Val)
	}

	return goja.Undefined()
}

func (e Element) OwnerDocument() goja.Value {
	if node := getOwnerDocNode(e.node); node != nil {
		return nodeToElement(e, node)
	}

	return goja.Undefined()
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
		return e.sel.rt.ToValue(TextNode)

	case gohtml.DocumentNode:
		return e.sel.rt.ToValue(DocumentNode)

	case gohtml.ElementNode:
		return e.sel.rt.ToValue(ElementNode)

	case gohtml.CommentNode:
		return e.sel.rt.ToValue(CommentNode)

	case gohtml.DoctypeNode:
		return e.sel.rt.ToValue(DoctypeNode)

	default:
		return goja.Undefined()
	}
}

func (e Element) NodeValue() goja.Value {
	switch e.node.Type {
	case gohtml.TextNode:
		return e.sel.rt.ToValue(e.sel.Text())

	case gohtml.CommentNode:
		return e.sel.rt.ToValue(e.sel.Text())

	default:
		return goja.Undefined()
	}
}

func (e Element) Contains(v goja.Value) bool {
	if other, ok := v.Export().(Element); ok {
		// When testing if a node contains itself, jquery's + goquery's version of Contains() return true while the DOM API returns false.
		return other.node != e.node && e.sel.sel.Contains(other.node)
	}

	return false
}

func (e Element) Matches(selector string) bool {
	return e.sel.sel.Is(selector)
}
