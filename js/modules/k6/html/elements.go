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
	AnchorTagName = "a"
	AreaTagName   = "area"
	BaseTagName   = "base"
	ButtonTagName = "button"
)

type HrefElement struct{ Element }
type AnchorElement struct{ HrefElement }
type AreaElement struct{ HrefElement }

type BaseElement struct{ Element }
type ButtonElement struct{ Element }

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

func (h HrefElement) Rel() string {
	return h.attrAsString("rel")
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

func (h HrefElement) Target() string {
	return h.attrAsString("target")
}

func (h HrefElement) Text() string {
	return h.TextContent()
}

func (h HrefElement) Type() string {
	return h.attrAsString("type")
}

func (h HrefElement) AccessKey() string {
	return h.attrAsString("accesskey")
}

func (h HrefElement) HrefLang() string {
	return h.attrAsString("hreflang")
}

func (h HrefElement) Media() string {
	return h.attrAsString("media")
}

func (h HrefElement) ToString() string {
	return h.attrAsString("href")
}

func (h HrefElement) Href() string {
	return h.attrAsString("href")
}

func (h BaseElement) Href() string {
	return h.attrAsString("href")
}

func (h BaseElement) Target() string {
	return h.attrAsString("target")
}

func (b ButtonElement) AccessKey() string {
	return b.attrAsString("accesskey")
}

func (b ButtonElement) Autofocus() bool {
	return b.attrIsPresent("autofocus")
}

func (b ButtonElement) Disabled() bool {
	return b.attrIsPresent("disabled")
}

func (b ButtonElement) Form() goja.Value {
	formSel, exists := b.ownerFormSel()
	if !exists {
		return goja.Undefined()
	}
	return selToElement(Selection{b.sel.rt, formSel})
}

// Used by the formAction, formMethod, formTarget and formEnctype methods of Button and Input elements
// Attempts to read attribute "form" + attrName on the current element or attrName on the owning form element
func (e Element) formAttrOrElemOverride(attrName string) string {
	if elemAttr, exists := e.sel.sel.Attr("form" + attrName); exists {
		return elemAttr
	}

	formSel, exists := e.ownerFormSel()
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
func (e Element) formOrElemAttrString(attrName string) string {
	if elemAttr, exists := e.sel.sel.Attr("form" + attrName); exists {
		return elemAttr
	}

	formSel, exists := e.ownerFormSel()
	if !exists {
		return ""
	}

	formAttr, exists := formSel.Attr(attrName)
	if !exists {
		return ""
	}

	return formAttr
}

func (e Element) formOrElemAttrPresent(attrName string) bool {
	if _, exists := e.sel.sel.Attr("form" + attrName); exists {
		return true
	}

	formSel, exists := e.ownerFormSel()
	if !exists {
		return false
	}

	_, exists = formSel.Attr(attrName)
	return exists
}

func (b ButtonElement) FormAction() string {
	return b.formOrElemAttrString("action")
}

func (b ButtonElement) FormEnctype() string {
	return b.formOrElemAttrString("enctype")
}

func (b ButtonElement) FormMethod() string {
	return b.formOrElemAttrString("method")
}

func (b ButtonElement) FormNoValidate() bool {
	return b.formOrElemAttrPresent("novalidate")
}

func (b ButtonElement) FormTarget() string {
	return b.formOrElemAttrString("target")
}

func (e Element) elemLabels() (items []goja.Value) {
	wrapperLbl := e.sel.sel.Closest("label")

	id := e.attrAsString("id")
	if id == "" {
		return elemList(Selection{e.sel.rt, wrapperLbl})
	}

	idLbl := e.sel.sel.Parents().Last().Find("label[for=\"" + id + "\"]")
	if idLbl.Size() == 0 {
		return elemList(Selection{e.sel.rt, wrapperLbl})
	}

	allLbls := wrapperLbl.AddSelection(idLbl)

	return elemList(Selection{e.sel.rt, allLbls})
}

func (b ButtonElement) Labels() (items []goja.Value) {
	return b.elemLabels()
}

func (b ButtonElement) Name() string {
	return b.attrAsString("name")
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
	return b.attrAsString("value")
}
