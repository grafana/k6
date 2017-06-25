package html

import (
	"net"
	"net/url"
	"strings"

	"github.com/dop251/goja"
)

//go:generate go run gen/main.go

var defaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
	"ftp":   "21",
}

const (
	AnchorTagName   = "a"
	AreaTagName     = "area"
	BaseTagName     = "base"
	ButtonTagName   = "button"
	CanvasTagName   = "canvas"
	DataTagName     = "data"
	DataListTagName = "datalist"
	EmbedTagName    = "embed"
	FieldSetTagName = "fieldset"
)

type HrefElement struct{ Element }
type FormFieldElement struct{ Element }

type AnchorElement struct{ HrefElement }
type AreaElement struct{ HrefElement }

type BaseElement struct{ Element }
type ButtonElement struct{ FormFieldElement }
type CanvasElement struct{ Element }
type DataElement struct{ Element }
type DataListElement struct{ Element }
type EmbedElement struct{ Element }
type FieldSetElement struct{ Element }

func (h HrefElement) hrefURL() *url.URL {
	url, err := url.Parse(h.attrAsString("href"))
	if err != nil {
		url, _ = url.Parse("")
	}

	return url
}

func (h HrefElement) Hash() string {
	return "#" + h.hrefURL().Fragment
}

func (h HrefElement) Host() string {
	url := h.hrefURL()
	hostAndPort := url.Host

	host, port, err := net.SplitHostPort(hostAndPort)
	if err != nil {
		return hostAndPort
	}

	defaultPort := defaultPorts[url.Scheme]
	if defaultPort != "" && port == defaultPort {
		return strings.TrimSuffix(host, ":"+defaultPort)
	}

	return hostAndPort
}

func (h HrefElement) Hostname() string {
	host, _, err := net.SplitHostPort(h.hrefURL().Host)
	if err != nil {
		return h.hrefURL().Host
	}
	return host
}

func (h HrefElement) Port() string {
	_, port, err := net.SplitHostPort(h.hrefURL().Host)
	if err != nil {
		return ""
	}
	return port
}

func (h HrefElement) Username() string {
	user := h.hrefURL().User
	if user == nil {
		return ""
	}
	return user.Username()
}

func (h HrefElement) Password() goja.Value {
	user := h.hrefURL().User
	if user == nil {
		return goja.Undefined()
	}

	pwd, defined := user.Password()
	if !defined {
		return goja.Undefined()
	}

	return h.sel.rt.ToValue(pwd)
}

func (h HrefElement) Origin() string {
	href := h.hrefURL()

	if href.Scheme == "file" {
		return h.Href()
	}

	return href.Scheme + "://" + href.Host
}

func (h HrefElement) Pathname() string {
	return h.hrefURL().Path
}

func (h HrefElement) Protocol() string {
	return h.hrefURL().Scheme
}

func (h HrefElement) RelList() []string {
	rel := h.attrAsString("rel")

	if rel == "" {
		return make([]string, 0)
	}

	return strings.Split(rel, " ")
}

func (h HrefElement) Search() string {
	q := h.hrefURL().RawQuery
	if q == "" {
		return q
	}
	return "?" + q
}

func (h HrefElement) Text() string {
	return h.TextContent()
}

func (f FormFieldElement) Form() goja.Value {
	formSel, exists := f.ownerFormSel()
	if !exists {
		return goja.Undefined()
	}
	return selToElement(Selection{f.sel.rt, formSel})
}

// Used by the formAction, formMethod, formTarget and formEnctype methods of Button and Input elements
// Attempts to read attribute "form" + attrName on the current element or attrName on the owning form element
func (f FormFieldElement) formAttrOrElemOverride(attrName string) string {
	if elemAttr, exists := f.sel.sel.Attr("form" + attrName); exists {
		return elemAttr
	}

	formSel, exists := f.ownerFormSel()
	if !exists {
		return ""
	}

	formAttr, exists := formSel.Attr(attrName)
	if !exists {
		return ""
	}

	return formAttr
}

// Used by the formAction, formMethod, formTarget and formEnctype methods of Button and Input elements
// Attempts to read attribute "form" + attrName on the current element or attrName on the owning form element
func (f FormFieldElement) formOrElemAttrString(attrName string) string {
	if elemAttr, exists := f.sel.sel.Attr("form" + attrName); exists {
		return elemAttr
	}

	formSel, exists := f.ownerFormSel()
	if !exists {
		return ""
	}

	formAttr, exists := formSel.Attr(attrName)
	if !exists {
		return ""
	}

	return formAttr
}

func (f FormFieldElement) formOrElemAttrPresent(attrName string) bool {
	if _, exists := f.sel.sel.Attr("form" + attrName); exists {
		return true
	}

	formSel, exists := f.ownerFormSel()
	if !exists {
		return false
	}

	_, exists = formSel.Attr(attrName)
	return exists
}

func (f FormFieldElement) FormAction() string {
	return f.formOrElemAttrString("action")
}

func (f FormFieldElement) FormEnctype() string {
	return f.formOrElemAttrString("enctype")
}

func (f FormFieldElement) FormMethod() string {
	return f.formOrElemAttrString("method")
}

func (f FormFieldElement) FormNoValidate() bool {
	return f.formOrElemAttrPresent("novalidate")
}

func (f FormFieldElement) FormTarget() string {
	return f.formOrElemAttrString("target")
}

func (f FormFieldElement) elemLabels() []goja.Value {
	wrapperLbl := f.sel.sel.Closest("label")

	id := f.attrAsString("id")
	if id == "" {
		return elemList(Selection{f.sel.rt, wrapperLbl})
	}

	idLbl := f.sel.sel.Parents().Last().Find("label[for=\"" + id + "\"]")
	if idLbl.Size() == 0 {
		return elemList(Selection{f.sel.rt, wrapperLbl})
	}

	allLbls := wrapperLbl.AddSelection(idLbl)

	return elemList(Selection{f.sel.rt, allLbls})
}

func (f FormFieldElement) Labels() []goja.Value {
	return f.elemLabels()
}

func (f FormFieldElement) Name() string {
	return f.attrAsString("name")
}

func (b ButtonElement) Type() string {
	switch b.attrAsString("type") {
	case "button":
		return "button"
	case "menu":
		return "menu"
	case "reset":
		return "reset"
	default:
		return "submit"
	}
}

func (b ButtonElement) Value() string {
	return valueOrHTML(b.sel.sel)
}

func (c CanvasElement) Width() int64 {
	return c.intAttrOrDefault("width", 150)
}

func (c CanvasElement) Height() int64 {
	return c.intAttrOrDefault("height", 150)
}

func (d DataListElement) Options() (items []goja.Value) {
	return elemList(d.sel.Find("option"))
}

func (f FieldSetElement) Form() goja.Value {
	formSel, exists := f.ownerFormSel()
	if !exists {
		return goja.Undefined()
	}
	return selToElement(Selection{f.sel.rt, formSel})
}

func (f FieldSetElement) Type() string {
	return "fieldset"
}

func (f FieldSetElement) Elements() []goja.Value {
	return elemList(f.sel.Find("input,select,button,textarea"))
}

func (f FieldSetElement) Validity() goja.Value {
	return goja.Undefined()
}
