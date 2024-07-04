package html

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/grafana/sobek"
	gohtml "golang.org/x/net/html"
)

const (
	ElementNode  = 1
	TextNode     = 3
	CommentNode  = 8
	DocumentNode = 9
	DoctypeNode  = 10
)

type Element struct {
	node *gohtml.Node
	sel  *Selection
}

type Attribute struct {
	OwnerElement *Element `json:"owner_element"`
	Name         string   `json:"name"`
	nsPrefix     string
	Value        string `json:"value"`
}

func (e Element) attrAsString(name string) string {
	val, exists := e.sel.sel.Attr(name)
	if !exists {
		return ""
	}
	return val
}

func (e Element) resolveURL(val string) (*url.URL, bool) {
	baseURL, err := url.Parse(e.sel.URL)
	if err != nil {
		return nil, false
	}

	addURL, err := url.Parse(val)
	if err != nil {
		return nil, false
	}

	return baseURL.ResolveReference(addURL), true
}

func (e Element) attrAsURL(name string) (*url.URL, bool) {
	val, exists := e.sel.sel.Attr(name)
	if !exists {
		return nil, false
	}

	return e.resolveURL(val)
}

func (e Element) attrAsURLString(name string, defaultWhenNoAttr string) string {
	if e.sel.URL == "" {
		return e.attrAsString(name)
	}

	url, ok := e.attrAsURL(name)
	if !ok {
		return defaultWhenNoAttr
	}

	return url.String()
}

func (e Element) attrAsInt(name string, defaultVal int) int {
	strVal, exists := e.sel.sel.Attr(name)
	if !exists {
		return defaultVal
	}

	intVal, err := strconv.Atoi(strVal)
	if err != nil {
		return defaultVal
	}

	return intVal
}

func (e Element) attrIsPresent(name string) bool {
	_, exists := e.sel.sel.Attr(name)
	return exists
}

func (e Element) ownerFormSel() (*goquery.Selection, bool) {
	prtForm := e.sel.sel.Closest("form")
	if prtForm.Length() > 0 {
		return prtForm, true
	}

	formID := e.attrAsString("form")
	if formID == "" {
		return nil, false
	}

	findForm := e.sel.sel.Parents().Last().Find("#" + formID)
	if findForm.Length() == 0 {
		return nil, false
	}

	return findForm, true
}

func (e Element) ownerFormVal() sobek.Value {
	formSel, exists := e.ownerFormSel()
	if !exists {
		return sobek.Undefined()
	}
	return selToElement(Selection{e.sel.rt, formSel.Eq(0), e.sel.URL})
}

func (e Element) elemLabels() []sobek.Value {
	wrapperLbl := e.sel.sel.Closest("label")

	id := e.attrAsString("id")
	if id == "" {
		return elemList(Selection{e.sel.rt, wrapperLbl, e.sel.URL})
	}

	idLbl := e.sel.sel.Parents().Last().Find("label[for=\"" + id + "\"]")
	if idLbl.Size() == 0 {
		return elemList(Selection{e.sel.rt, wrapperLbl, e.sel.URL})
	}

	allLbls := wrapperLbl.AddSelection(idLbl)

	return elemList(Selection{e.sel.rt, allLbls, e.sel.URL})
}

func (e Element) splitAttr(attrName string) []string {
	attr := e.attrAsString(attrName)

	if attr == "" {
		return make([]string, 0)
	}

	return strings.Split(attr, " ")
}

func (e Element) idOrNameAttr() (string, bool) {
	if id, exists := e.sel.sel.Attr("id"); exists {
		return id, true
	}

	if name, exists := e.sel.sel.Attr("id"); exists {
		return name, true
	}

	return "", false
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

func (e Element) GetAttribute(name string) sobek.Value {
	return e.sel.Attr(name)
}

func (e Element) GetAttributeNode(name string) sobek.Value {
	if attr := getHTMLAttr(e.node, name); attr != nil {
		return e.sel.rt.ToValue(Attribute{&e, attr.Key, attr.Namespace, attr.Val})
	}

	return sobek.Undefined()
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
		attrs[attr.Key] = Attribute{&e, attr.Key, attr.Namespace, attr.Val}
	}
	return attrs
}

func (e Element) ToString() sobek.Value {
	if e.sel.sel.Length() == 0 {
		return sobek.Undefined()
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

//nolint:revive,stylecheck // var-naming wants this to be ID but this will break the API
func (e Element) Id() string {
	return e.attrAsString("id")
}

func (e Element) IsEqualNode(v sobek.Value) bool {
	if other, ok := v.Export().(Element); ok {
		htmlA, errA := e.sel.sel.Html()
		htmlB, errB := other.sel.sel.Html()

		return errA == nil && errB == nil && htmlA == htmlB
	}

	return false
}

func (e Element) IsSameNode(v sobek.Value) bool {
	if other, ok := v.Export().(Element); ok {
		return e.node == other.node
	}

	return false
}

// Selection returns a Selection object based on the current Element.
//
// This function is used to create a Selection object that represents the same HTML
// content as the Element. It is useful for performing operations or manipulations
// on the HTML content within the scope of this Element.
//
// Example:
// sel := element.Selection()
func (e Element) Selection() Selection {
	return *e.sel
}

func (e Element) GetElementsByClassName(name string) []sobek.Value {
	return elemList(Selection{e.sel.rt, e.sel.sel.Find("." + name), e.sel.URL})
}

func (e Element) GetElementsByTagName(name string) []sobek.Value {
	return elemList(Selection{e.sel.rt, e.sel.sel.Find(name), e.sel.URL})
}

func (e Element) QuerySelector(selector string) sobek.Value {
	return selToElement(Selection{e.sel.rt, e.sel.sel.Find(selector), e.sel.URL})
}

func (e Element) QuerySelectorAll(selector string) []sobek.Value {
	return elemList(Selection{e.sel.rt, e.sel.sel.Find(selector), e.sel.URL})
}

func (e Element) NodeName() string {
	return goquery.NodeName(e.sel.sel)
}

func (e Element) FirstChild() sobek.Value {
	return nodeToElement(e, e.node.FirstChild)
}

func (e Element) LastChild() sobek.Value {
	return nodeToElement(e, e.node.LastChild)
}

func (e Element) FirstElementChild() sobek.Value {
	if child := e.sel.sel.Children().First(); child.Length() > 0 {
		return selToElement(Selection{e.sel.rt, child.First(), e.sel.URL})
	}

	return sobek.Undefined()
}

func (e Element) LastElementChild() sobek.Value {
	if child := e.sel.sel.Children(); child.Length() > 0 {
		return selToElement(Selection{e.sel.rt, child.Last(), e.sel.URL})
	}

	return sobek.Undefined()
}

func (e Element) PreviousSibling() sobek.Value {
	return nodeToElement(e, e.node.PrevSibling)
}

func (e Element) NextSibling() sobek.Value {
	return nodeToElement(e, e.node.NextSibling)
}

func (e Element) PreviousElementSibling() sobek.Value {
	if prev := e.sel.sel.Prev(); prev.Length() > 0 {
		return selToElement(Selection{e.sel.rt, prev, e.sel.URL})
	}

	return sobek.Undefined()
}

func (e Element) NextElementSibling() sobek.Value {
	if next := e.sel.sel.Next(); next.Length() > 0 {
		return selToElement(Selection{e.sel.rt, next, e.sel.URL})
	}

	return sobek.Undefined()
}

func (e Element) ParentNode() sobek.Value {
	if e.node.Parent != nil {
		return nodeToElement(e, e.node.Parent)
	}

	return sobek.Undefined()
}

func (e Element) ParentElement() sobek.Value {
	if prt := e.sel.sel.Parent(); prt.Length() > 0 {
		return selToElement(Selection{e.sel.rt, prt, e.sel.URL})
	}

	return sobek.Undefined()
}

func (e Element) ChildNodes() []sobek.Value {
	return elemList(e.sel.Contents())
}

func (e Element) Children() []sobek.Value {
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

func (e Element) ClassName() sobek.Value {
	return e.sel.Attr("class")
}

func (e Element) Lang() sobek.Value {
	if attr := getHTMLAttr(e.node, "lang"); attr != nil && attr.Namespace == "" {
		return e.sel.rt.ToValue(attr.Val)
	}

	return sobek.Undefined()
}

func (e Element) OwnerDocument() sobek.Value {
	if node := getOwnerDocNode(e.node); node != nil {
		return nodeToElement(e, node)
	}

	return sobek.Undefined()
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

func (e Element) InnerHTML() sobek.Value {
	return e.sel.Html()
}

func (e Element) NodeType() sobek.Value {
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
		return sobek.Undefined()
	}
}

func (e Element) NodeValue() sobek.Value {
	switch e.node.Type {
	case gohtml.TextNode:
		return e.sel.rt.ToValue(e.sel.Text())

	case gohtml.CommentNode:
		return e.sel.rt.ToValue(e.sel.Text())

	default:
		return sobek.Undefined()
	}
}

func (e Element) Contains(v sobek.Value) bool {
	if other, ok := v.Export().(Element); ok {
		// When testing if a node contains itself, jquery's + goquery's version of Contains()
		// return true while the DOM API returns false.
		return other.node != e.node && e.sel.sel.Contains(other.node)
	}

	return false
}

func (e Element) Matches(selector string) bool {
	return e.sel.sel.Is(selector)
}
