package html

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/dop251/goja"
)

//go:generate go run gen/gen_elements.go
//go:generate gofumpt -w .

//nolint:gochecknoglobals
var defaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
	"ftp":   "21",
}

// The code generator depends on the TagName constants being defined before the Element structs
const (
	AnchorTagName          = "a"
	AreaTagName            = "area"
	AudioTagName           = "audio"
	BaseTagName            = "base"
	ButtonTagName          = "button"
	CanvasTagName          = "canvas"
	DataTagName            = "data"
	DataListTagName        = "datalist"
	DelTagName             = "del"
	EmbedTagName           = "embed"
	FieldSetTagName        = "fieldset"
	FormTagName            = "form"
	IFrameTagName          = "iframe"
	ImageTagName           = "img"
	InputTagName           = "input"
	InsTagName             = "ins"
	KeygenTagName          = "keygen"
	LabelTagName           = "label"
	LegendTagName          = "legend"
	LiTagName              = "li"
	LinkTagName            = "link"
	MapTagName             = "map"
	MetaTagName            = "meta"
	MeterTagName           = "meter"
	ObjectTagName          = "object"
	OListTagName           = "ol"
	OptGroupTagName        = "optgroup"
	OptionTagName          = "option"
	OutputTagName          = "output"
	ParamTagName           = "param"
	PreTagName             = "pre"
	ProgressTagName        = "progress"
	QuoteTagName           = "quote"
	ScriptTagName          = "script"
	SelectTagName          = "select"
	SourceTagName          = "source"
	StyleTagName           = "style"
	TableTagName           = "table"
	TableHeadTagName       = "thead"
	TableFootTagName       = "tfoot"
	TableBodyTagName       = "tbody"
	TableRowTagName        = "tr"
	TableColTagName        = "col"
	TableDataCellTagName   = "td"
	TableHeaderCellTagName = "th"
	TextAreaTagName        = "textarea"
	TimeTagName            = "time"
	TitleTagName           = "title"
	TrackTagName           = "track"
	UListTagName           = "ul"
	VideoTagName           = "video"

	methodPost = "post"
	methodGet  = "get"
)

//revive:disable:exported

type (
	HrefElement            struct{ Element }
	MediaElement           struct{ Element }
	FormFieldElement       struct{ Element }
	ModElement             struct{ Element }
	TableSectionElement    struct{ Element }
	TableCellElement       struct{ Element }
	AnchorElement          struct{ HrefElement }
	AreaElement            struct{ HrefElement }
	AudioElement           struct{ MediaElement }
	BaseElement            struct{ Element }
	ButtonElement          struct{ FormFieldElement }
	CanvasElement          struct{ Element }
	DataElement            struct{ Element }
	DataListElement        struct{ Element }
	DelElement             struct{ ModElement }
	InsElement             struct{ ModElement }
	EmbedElement           struct{ Element }
	FieldSetElement        struct{ Element }
	FormElement            struct{ Element }
	IFrameElement          struct{ Element }
	ImageElement           struct{ Element }
	InputElement           struct{ FormFieldElement }
	KeygenElement          struct{ Element }
	LabelElement           struct{ Element }
	LegendElement          struct{ Element }
	LiElement              struct{ Element }
	LinkElement            struct{ Element }
	MapElement             struct{ Element }
	MetaElement            struct{ Element }
	MeterElement           struct{ Element }
	ObjectElement          struct{ Element }
	OListElement           struct{ Element }
	OptGroupElement        struct{ Element }
	OptionElement          struct{ Element }
	OutputElement          struct{ Element }
	ParamElement           struct{ Element }
	PreElement             struct{ Element }
	ProgressElement        struct{ Element }
	QuoteElement           struct{ Element }
	ScriptElement          struct{ Element }
	SelectElement          struct{ Element }
	SourceElement          struct{ Element }
	StyleElement           struct{ Element }
	TableElement           struct{ Element }
	TableHeadElement       struct{ TableSectionElement }
	TableFootElement       struct{ TableSectionElement }
	TableBodyElement       struct{ TableSectionElement }
	TableRowElement        struct{ Element }
	TableColElement        struct{ Element }
	TableDataCellElement   struct{ TableCellElement }
	TableHeaderCellElement struct{ TableCellElement }
	TextAreaElement        struct{ Element }
	TimeElement            struct{ Element }
	TitleElement           struct{ Element }
	TrackElement           struct{ Element }
	UListElement           struct{ Element }
	VideoElement           struct{ MediaElement }
)

//revive:enable:exported

func (h HrefElement) hrefURL() *url.URL {
	href, exists := h.attrAsURL("href")
	if !exists {
		return &url.URL{}
	}
	return href
}

func (h HrefElement) Hash() string {
	frag := h.hrefURL().Fragment
	if frag == "" {
		return ""
	}
	return "#" + frag
}

func (h HrefElement) Host() string {
	href := h.hrefURL()
	if href.Host == "" {
		return ""
	}

	host, port, err := net.SplitHostPort(href.Host)
	if err != nil {
		return href.Host
	}

	defaultPort := defaultPorts[href.Scheme]
	if defaultPort != "" && port == defaultPort {
		return strings.TrimSuffix(host, ":"+defaultPort)
	}

	return href.Host
}

func (h HrefElement) Hostname() string {
	hostAndPort := h.hrefURL().Host
	if hostAndPort == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(hostAndPort)
	if err != nil {
		return hostAndPort
	}

	return host
}

func (h HrefElement) Port() string {
	hostAndPort := h.hrefURL().Host
	if hostAndPort == "" {
		return ""
	}

	_, port, err := net.SplitHostPort(hostAndPort)
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

func (h HrefElement) Password() string {
	user := h.hrefURL().User
	if user == nil {
		return ""
	}

	pwd, defined := user.Password()
	if !defined {
		return ""
	}

	return pwd
}

func (h HrefElement) Origin() string {
	href := h.hrefURL()

	if href.Scheme == "" {
		return ""
	}

	if href.Scheme == "file" {
		return h.Href()
	}

	return href.Scheme + "://" + href.Host
}

func (h HrefElement) Pathname() string {
	return h.hrefURL().Path
}

func (h HrefElement) Protocol() string {
	scheme := h.hrefURL().Scheme
	if scheme == "" {
		return ":"
	}
	return scheme
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

func (f FormFieldElement) formOrElemAttr(attrName string) (string, bool) {
	if elemAttr, exists := f.sel.sel.Attr("form" + attrName); exists {
		return elemAttr, true
	}

	formSel, exists := f.ownerFormSel()
	if !exists {
		return "", false
	}

	formAttr, exists := formSel.Attr(attrName)
	if !exists {
		return "", false
	}

	return formAttr, true
}

func (f FormFieldElement) FormAction() string {
	action, exists := f.formOrElemAttr("action")
	if f.sel.URL == "" {
		return action
	}

	if !exists || action == "" {
		return f.sel.URL
	}

	actionURL, ok := f.resolveURL(action)
	if !ok {
		return action
	}

	return actionURL.String()
}

func (f FormFieldElement) FormEnctype() string {
	enctype, _ := f.formOrElemAttr("enctype")

	switch enctype {
	case "multipart/form-data":
		return enctype
	case "text/plain":
		return enctype
	default:
		return "application/x-www-form-urlencoded"
	}
}

func (f FormFieldElement) FormMethod() string {
	method, _ := f.formOrElemAttr("method")

	switch strings.ToLower(method) {
	case methodPost:
		return methodPost
	default:
		return methodGet
	}
}

func (f FormFieldElement) FormNoValidate() bool {
	_, exists := f.formOrElemAttr("novalidate")
	return exists
}

func (f FormFieldElement) FormTarget() string {
	target, _ := f.formOrElemAttr("target")
	return target
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

func (d DataListElement) Options() []goja.Value {
	return elemList(d.sel.Find("option"))
}

func (f FieldSetElement) Form() goja.Value {
	formSel, exists := f.ownerFormSel()
	if !exists {
		return goja.Undefined()
	}
	return selToElement(Selection{f.sel.rt, formSel, f.sel.URL})
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
	if method := f.attrAsString("method"); method == methodPost {
		return methodPost
	}

	return methodGet
}

func (i InputElement) List() goja.Value {
	listID := i.attrAsString("list")

	if listID == "" {
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

	datalist := i.sel.sel.Parents().Last().Find("datalist[id=\"" + listID + "\"]")
	if datalist.Length() == 0 {
		return goja.Undefined()
	}

	return selToElement(Selection{i.sel.rt, datalist.Eq(0), i.sel.URL})
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

	return selToElement(Selection{l.sel.rt, findControl.Eq(0), l.sel.URL})
}

func (l LabelElement) Form() goja.Value {
	return l.ownerFormVal()
}

func (l LegendElement) Form() goja.Value {
	return l.ownerFormVal()
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

	imgs := m.sel.sel.Parents().Last().Find("img[usemap=\"#" + name + "\"],object[usemap=\"#" + name + "\"]")
	return elemList(Selection{m.sel.rt, imgs, m.sel.URL})
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
		return selToElement(Selection{o.sel.rt, prtForm.First(), o.sel.URL})
	}

	prtSelect := o.sel.sel.ParentsFiltered("select")
	formID, exists := prtSelect.Attr("form")
	if !exists {
		return goja.Undefined()
	}

	ownerForm := prtSelect.Parents().Last().Find("form#" + formID)
	if ownerForm.Length() == 0 {
		return goja.Undefined()
	}

	return selToElement(Selection{o.sel.rt, ownerForm.First(), o.sel.URL})
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

func (s ScriptElement) Text() string {
	return s.TextContent()
}

func (s SelectElement) Form() goja.Value {
	return s.ownerFormVal()
}

func (s SelectElement) Labels() []goja.Value {
	return s.elemLabels()
}

func (s SelectElement) Length() int {
	return s.sel.Find("option").Size()
}

func (s SelectElement) Options() []goja.Value {
	return elemList(Selection{s.sel.rt, s.sel.sel.Find("option"), s.sel.URL})
}

func (s SelectElement) SelectedIndex() int {
	option := s.sel.sel.Find("option[selected]")
	if option.Length() == 0 {
		return -1
	}
	return s.sel.sel.Find("option").IndexOfSelection(option)
}

func (s SelectElement) SelectedOptions() []goja.Value {
	return elemList(Selection{s.sel.rt, s.sel.sel.Find("option[selected]"), s.sel.URL})
}

func (s SelectElement) Size() int {
	if s.attrIsPresent("multiple") {
		return 4
	}
	return 1
}

func (s SelectElement) Type() string {
	if s.attrIsPresent("multiple") {
		return "select-multiple"
	}
	return "select"
}

func (s SelectElement) Value() string {
	option := s.sel.sel.Find("option[selected]")
	if option.Length() == 0 {
		return ""
	}
	return valueOrHTML(option.First())
}

func (s StyleElement) Type() string {
	typeVal := s.attrAsString("type")
	if typeVal == "" {
		return "text/css"
	}
	return typeVal
}

func (t TableElement) firstChild(elemName string) goja.Value {
	child := t.sel.sel.ChildrenFiltered(elemName)
	if child.Size() == 0 {
		return goja.Undefined()
	}
	return selToElement(Selection{t.sel.rt, child, t.sel.URL})
}

func (t TableElement) Caption() goja.Value {
	return t.firstChild("caption")
}

func (t TableElement) THead() goja.Value {
	return t.firstChild("thead")
}

func (t TableElement) TFoot() goja.Value {
	return t.firstChild("tfoot")
}

func (t TableElement) Rows() []goja.Value {
	return elemList(Selection{t.sel.rt, t.sel.sel.Find("tr"), t.sel.URL})
}

func (t TableElement) TBodies() []goja.Value {
	return elemList(Selection{t.sel.rt, t.sel.sel.Find("tbody"), t.sel.URL})
}

func (t TableSectionElement) Rows() []goja.Value {
	return elemList(Selection{t.sel.rt, t.sel.sel.Find("tr"), t.sel.URL})
}

func (t TableCellElement) CellIndex() int {
	prtRow := t.sel.sel.ParentsFiltered("tr")
	if prtRow.Length() == 0 {
		return -1
	}
	return prtRow.Find("th,td").IndexOfSelection(t.sel.sel)
}

func (t TableRowElement) Cells() []goja.Value {
	return elemList(Selection{t.sel.rt, t.sel.sel.Find("th,td"), t.sel.URL})
}

func (t TableRowElement) RowIndex() int {
	table := t.sel.sel.ParentsFiltered("table")
	if table.Length() == 0 {
		return -1
	}
	return table.Find("tr").IndexOfSelection(t.sel.sel)
}

func (t TableRowElement) SectionRowIndex() int {
	section := t.sel.sel.ParentsFiltered("thead,tbody,tfoot")
	if section.Length() == 0 {
		return -1
	}
	return section.Find("tr").IndexOfSelection(t.sel.sel)
}

func (t TextAreaElement) Form() goja.Value {
	return t.ownerFormVal()
}

func (t TextAreaElement) Length() int {
	return len(t.attrAsString("value"))
}

func (t TextAreaElement) Labels() []goja.Value {
	return t.elemLabels()
}

func (t TableColElement) Span() int {
	span := t.attrAsInt("span", 1)
	if span < 1 {
		return 1
	}
	return span
}

func (m MediaElement) TextTracks() []goja.Value {
	return elemList(Selection{m.sel.rt, m.sel.sel.Find("track"), m.sel.URL})
}

func (t TitleElement) Text() string {
	return t.TextContent()
}
