package html

import (
	"net"
	"net/url"
	"strings"

	"strconv"

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
	DelTagName      = "del"
	EmbedTagName    = "embed"
	FieldSetTagName = "fieldset"
	FormTagName     = "form"
	IFrameTagName   = "iframe"
	ImageTagName    = "img"
	InputTagName    = "input"
	InsTagName      = "ins"
	KeygenTagName   = "keygen"
	LabelTagName    = "label"
	LegendTagName   = "legend"
	LiTagName       = "li"
	LinkTagName     = "link"
	MapTagName      = "map"
	MetaTagName     = "meta"
	MeterTagName    = "meter"
	ObjectTagName   = "object"
	OListTagName    = "olist"
	OptGroupTagName = "optgroup"
	OptionTagName   = "option"
	OutputTagName   = "output"
	ParamTagName    = "param"
	PreTagName      = "pre"
	ProgressTagName = "progress"
	QuoteTagName    = "progress"
)

type HrefElement struct{ Element }
type MediaElement struct{ Element }
type FormFieldElement struct{ Element }
type ModElement struct{ Element }

type AnchorElement struct{ HrefElement }
type AreaElement struct{ HrefElement }

type BaseElement struct{ Element }
type ButtonElement struct{ FormFieldElement }
type CanvasElement struct{ Element }
type DataElement struct{ Element }
type DataListElement struct{ Element }
type DelElement struct{ ModElement }
type InsElement struct{ ModElement }

type EmbedElement struct{ Element }
type FieldSetElement struct{ Element }
type FormElement struct{ Element }
type IFrameElement struct{ Element }
type ImageElement struct{ Element }
type InputElement struct{ FormFieldElement }
type KeygenElement struct{ Element }
type LabelElement struct{ Element }
type LegendElement struct{ Element }
type LiElement struct{ Element }
type LinkElement struct{ Element }
type MapElement struct{ Element }
type MetaElement struct{ Element }
type MeterElement struct{ Element }
type ObjectElement struct{ Element }
type OListElement struct{ Element }
type OptGroupElement struct{ Element }
type OptionElement struct{ Element }
type OutputElement struct{ Element }
type ParamElement struct{ Element }
type PreElement struct{ Element }
type ProgressElement struct{ Element }
type QuoteElement struct{ Element }

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
	return h.splitAttr("rel")
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
	return f.ownerFormVal()
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
	if method := strings.ToLower(f.formOrElemAttrString("method")); method == "post" {
		return "post"
	}

	return "get"
}

func (f FormFieldElement) FormNoValidate() bool {
	return f.formOrElemAttrPresent("novalidate")
}

func (f FormFieldElement) FormTarget() string {
	return f.formOrElemAttrString("target")
}

func (f FormFieldElement) Labels() []goja.Value {
	return f.elemLabels()
}

func (f FormFieldElement) Name() string {
	return f.attrAsString("name")
}

func (b ButtonElement) Value() string {
	return valueOrHTML(b.sel.sel)
}

func (c CanvasElement) Width() int {
	return c.attrAsInt("width", 150)
}

func (c CanvasElement) Height() int {
	return c.attrAsInt("height", 150)
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

func (f FormElement) Elements() []goja.Value {
	return elemList(f.sel.Find("input,select,button,textarea,fieldset"))
}

func (f FormElement) Length() int {
	return f.sel.sel.Find("input,select,button,textarea,fieldset").Size()
}

func (f FormElement) Method() string {
	if method := f.attrAsString("method"); method == "post" {
		return "post"
	}

	return "get"
}

func (i InputElement) List() goja.Value {
	listId := i.attrAsString("list")

	if listId == "" {
		return goja.Undefined()
	}

	switch i.attrAsString("type") {
	case "hidden":
		return goja.Undefined()
	case "checkbox":
		return goja.Undefined()
	case "radio":
		return goja.Undefined()
	case "file":
		return goja.Undefined()
	case "button":
		return goja.Undefined()
	}

	datalist := i.sel.sel.Parents().Last().Find("datalist[id=\"" + listId + "\"]")
	if datalist.Length() == 0 {
		return goja.Undefined()
	}

	return selToElement(Selection{i.sel.rt, datalist.Eq(0)})
}

func (k KeygenElement) Form() goja.Value {
	return k.ownerFormVal()
}

func (k KeygenElement) Labels() []goja.Value {
	return k.elemLabels()
}

func (l LabelElement) Control() goja.Value {
	forAttr, exists := l.sel.sel.Attr("for")
	if !exists {
		return goja.Undefined()
	}

	findControl := l.sel.sel.Parents().Last().Find("#" + forAttr)
	if findControl.Length() == 0 {
		return goja.Undefined()
	}

	return selToElement(Selection{l.sel.rt, findControl.Eq(0)})
}

func (l LabelElement) Form() goja.Value {
	return l.ownerFormVal()
}

func (l LegendElement) Form() goja.Value {
	return l.ownerFormVal()
}

func (l LiElement) Value() goja.Value {
	if l.sel.sel.ParentFiltered("ol").Size() == 0 {
		return goja.Undefined()
	}

	prev := l.sel.sel.PrevAllFiltered("li")
	len := prev.Length()

	if len == 0 {
		return l.sel.rt.ToValue(1)
	}

	for idx := 0; idx < len; idx++ {
		val, exists := prev.Eq(idx).Attr("value")
		if !exists {
			continue
		}

		intVal, err := strconv.Atoi(val)
		if err != nil {
			continue
		}

		return l.sel.rt.ToValue(intVal + idx + 1)
	}

	return l.sel.rt.ToValue(len + 1)
}

func (l LinkElement) RelList() []string {
	return l.splitAttr("rel")
}

func (m MapElement) Areas() []goja.Value {
	return elemList(m.sel.Find("area"))
}

func (m MapElement) Images() []goja.Value {
	name, exists := m.idOrNameAttr()

	if !exists {
		return make([]goja.Value, 0)
	}

	return elemList(Selection{m.sel.rt, m.sel.sel.Parents().Last().Find("img[usemap=\"#" + name + "\"],object[usemap=\"#" + name + "\"]")})
}

func (m MeterElement) Labels() []goja.Value {
	return m.elemLabels()
}

func (o ObjectElement) Form() goja.Value {
	return o.ownerFormVal()
}

func (o OptionElement) Disabled() bool {
	if o.attrIsPresent("disabled") {
		return true
	}

	optGroup := o.sel.sel.ParentsFiltered("optgroup")
	if optGroup.Length() == 0 {
		return false
	}

	_, exists := optGroup.Attr("disabled")
	return exists
}

func (o OptionElement) Form() goja.Value {
	prtForm := o.sel.sel.ParentsFiltered("form")
	if prtForm.Length() != 0 {
		return selToElement(Selection{o.sel.rt, prtForm.First()})
	}

	prtSelect := o.sel.sel.ParentsFiltered("select")
	formId, exists := prtSelect.Attr("form")
	if !exists {
		return goja.Undefined()
	}

	ownerForm := prtSelect.Parents().Last().Find("form[id=\"" + formId + "\"]")
	if ownerForm.Length() == 0 {
		return goja.Undefined()
	}

	return selToElement(Selection{o.sel.rt, ownerForm.First()})
}

func (o OptionElement) Index() int {
	optsHolder := o.sel.sel.ParentsFiltered("select,datalist")
	if optsHolder.Length() == 0 {
		return 0
	}

	return optsHolder.Find("option").IndexOfSelection(o.sel.sel)
}

func (o OptionElement) Label() string {
	if lbl, exists := o.sel.sel.Attr("label"); exists {
		return lbl
	}

	return o.TextContent()
}

func (o OptionElement) Text() string {
	return o.TextContent()
}

func (o OptionElement) Value() string {
	return valueOrHTML(o.sel.sel)
}

func (o OutputElement) Form() goja.Value {
	return o.ownerFormVal()
}

func (o OutputElement) Labels() []goja.Value {
	return o.elemLabels()
}

func (o OutputElement) Value() string {
	return o.TextContent()
}

func (o OutputElement) DefaultValue() string {
	return o.TextContent()
}

func (p ProgressElement) Max() float64 {
	maxStr, exists := p.sel.sel.Attr("max")
	if !exists {
		return 1.0
	}

	maxVal, err := strconv.ParseFloat(maxStr, 64)
	if err != nil || maxVal < 0 {
		return 1.0
	}

	return maxVal
}

func (p ProgressElement) calcProgress(defaultVal float64) float64 {
	valStr, exists := p.sel.sel.Attr("value")
	if !exists {
		return defaultVal
	}

	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil || val < 0 {
		return defaultVal
	}

	return val / p.Max()
}

func (p ProgressElement) Value() float64 {
	return p.calcProgress(0.0)
}

func (p ProgressElement) Position() float64 {
	return p.calcProgress(-1.0)
}

func (p ProgressElement) Labels() []goja.Value {
	return p.elemLabels()
}
