package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"text/template"

	log "github.com/Sirupsen/logrus"
)

const (
	stringTemplate   = "string"
	urlTemplate      = "url"
	enumTemplate     = "enum"
	boolTemplate     = "bool"
	gojaEnumTemplate = "enum-goja"
	intTemplate      = "int"
	constTemplate    = "const"
)

// The ast parser populates this struct using the <ElemName>TagName const and <ElemName>Element struct
type ElemInfo struct {
	StructName    string
	PrtStructName string
}

// The signature for functions which inspect the ast nodes
type ElemInfoCollector func(node ast.Node) ElemInfoCollector

type ElemInfoCollectorState struct {
	handler   ElemInfoCollector // The current function to check ast nodes
	elemName  string            // Only valid when a TagName const or Element struct encountered and used as an index into elemInfos
	elemInfos map[string]*ElemInfo
}

// "Elem" is the struct name for an Element. "Method" is the go method name. "Attr" is the name of the DOM attribute the method will access.
// "TemplateType" is used the "elemFuncsTemplate" and the returnType function - either string
// The Opts property:
//     for "string" and "bool" template types it is nil.
//     for "const", "int" and "url" templates it is the default return value when the attribute is unset. The "url" default should be an empty string or e.sel.URL
//     for "enum" template it is the list of valid options used in a switch statement - the default case is the first in the Opts list
//     for "enum-goja" template - same as the "enum" template except the function returns a goja.Value and the default value is always null
var funcDefs = []struct {
	Elem, Method, Attr, TemplateType string
	Opts                             []string
}{
	{"HrefElement", "Download", "download", stringTemplate, nil},
	{"HrefElement", "ReferrerPolicy", "referrerpolicy", enumTemplate, []string{"", "no-referrer", "no-referrer-when-downgrade", "origin", "origin-when-cross-origin", "unsafe-url"}},
	{"HrefElement", "Rel", "rel", stringTemplate, nil},
	{"HrefElement", "Href", "href", urlTemplate, []string{"\"\""}},
	{"HrefElement", "Target", "target", stringTemplate, nil},
	{"HrefElement", "Type", "type", stringTemplate, nil},
	{"HrefElement", "AccessKey", "accesskey", stringTemplate, nil},
	{"HrefElement", "HrefLang", "hreflang", stringTemplate, nil},
	{"HrefElement", "ToString", "href", urlTemplate, []string{"\"\""}},
	{"MediaElement", "Autoplay", "autoplay", boolTemplate, nil},
	{"MediaElement", "Controls", "controls", boolTemplate, nil},
	{"MediaElement", "Loop", "loop", boolTemplate, nil},
	{"MediaElement", "Muted", "muted", boolTemplate, nil},
	{"MediaElement", "Preload", "preload", enumTemplate, []string{"auto", "metadata", "none"}},
	{"MediaElement", "Src", "src", urlTemplate, []string{"\"\""}},
	{"MediaElement", "CrossOrigin", "crossorigin", gojaEnumTemplate, []string{"anonymous", "use-credentials"}},
	{"MediaElement", "CurrentSrc", "src", stringTemplate, nil},
	{"MediaElement", "DefaultMuted", "muted", boolTemplate, nil},
	{"MediaElement", "MediaGroup", "mediagroup", stringTemplate, nil},
	{"BaseElement", "Href", "href", urlTemplate, []string{"e.sel.URL"}},
	{"BaseElement", "Target", "target", stringTemplate, nil},
	{"ButtonElement", "AccessKey", "accesskey", stringTemplate, nil},
	{"ButtonElement", "Autofocus", "autofocus", boolTemplate, nil},
	{"ButtonElement", "Disabled", "disabled", boolTemplate, nil},
	{"ButtonElement", "TabIndex", "tabindex", intTemplate, []string{"0"}},
	{"ButtonElement", "Type", "type", enumTemplate, []string{"submit", "button", "menu", "reset"}},
	{"DataElement", "Value", "value", stringTemplate, nil},
	{"EmbedElement", "Height", "height", stringTemplate, nil},
	{"EmbedElement", "Width", "width", stringTemplate, nil},
	{"EmbedElement", "Src", "src", stringTemplate, nil},
	{"EmbedElement", "Type", "type", stringTemplate, nil},
	{"FieldSetElement", "Disabled", "disabled", boolTemplate, nil},
	{"FieldSetElement", "Name", "name", stringTemplate, nil},
	{"FormElement", "Action", "action", urlTemplate, []string{"\"\""}},
	{"FormElement", "Name", "name", stringTemplate, nil},
	{"FormElement", "Target", "target", stringTemplate, nil},
	{"FormElement", "Enctype", "enctype", enumTemplate, []string{"application/x-www-form-urlencoded", "multipart/form-data", "text/plain"}},
	{"FormElement", "Encoding", "enctype", enumTemplate, []string{"application/x-www-form-urlencoded", "multipart/form-data", "text/plain"}},
	{"FormElement", "AcceptCharset", "accept-charset", stringTemplate, nil},
	{"FormElement", "Autocomplete", "autocomplete", enumTemplate, []string{"on", "off"}},
	{"FormElement", "NoValidate", "novalidate", boolTemplate, nil},
	{"IFrameElement", "Allowfullscreen", "allowfullscreen", boolTemplate, nil},
	{"IFrameElement", "ReferrerPolicy", "referrerpolicy", enumTemplate, []string{"", "no-referrer", "no-referrer-when-downgrade", "origin", "origin-when-cross-origin", "unsafe-url"}},
	{"IFrameElement", "Height", "height", stringTemplate, nil},
	{"IFrameElement", "Width", "width", stringTemplate, nil},
	{"IFrameElement", "Name", "name", stringTemplate, nil},
	{"IFrameElement", "Src", "src", urlTemplate, []string{"\"\""}},
	{"ImageElement", "CurrentSrc", "src", urlTemplate, []string{"\"\""}},
	{"ImageElement", "Sizes", "sizes", stringTemplate, nil},
	{"ImageElement", "Srcset", "srcset", stringTemplate, nil},
	{"ImageElement", "Alt", "alt", stringTemplate, nil},
	{"ImageElement", "CrossOrigin", "crossorigin", gojaEnumTemplate, []string{"anonymous", "use-credentials"}},
	{"ImageElement", "Height", "height", intTemplate, []string{"0"}},
	{"ImageElement", "Width", "width", intTemplate, []string{"0"}},
	{"ImageElement", "IsMap", "ismap", boolTemplate, nil},
	{"ImageElement", "Name", "name", stringTemplate, nil},
	{"ImageElement", "Src", "src", urlTemplate, []string{"\"\""}},
	{"ImageElement", "UseMap", "usemap", stringTemplate, nil},
	{"ImageElement", "ReferrerPolicy", "referrerpolicy", enumTemplate, []string{"", "no-referrer", "no-referrer-when-downgrade", "origin", "origin-when-cross-origin", "unsafe-url"}},
	{"InputElement", "Name", "name", stringTemplate, nil},
	{"InputElement", "TabIndex", "tabindex", intTemplate, []string{"0"}},
	{"InputElement", "Type", "type", enumTemplate, []string{"text", "button", "checkbox", "color", "date", "datetime-local", "email", "file", "hidden", "image", "month", "number", "password", "radio", "range", "reset", "search", "submit", "tel", "time", "url", "week"}},
	{"InputElement", "Disabled", "disabled", boolTemplate, nil},
	{"InputElement", "Autofocus", "autofocus", boolTemplate, nil},
	{"InputElement", "Required", "required", boolTemplate, nil},
	{"InputElement", "Value", "value", stringTemplate, nil},
	{"InputElement", "Checked", "checked", boolTemplate, nil},
	{"InputElement", "DefaultChecked", "checked", boolTemplate, nil},
	{"InputElement", "Alt", "alt", stringTemplate, nil},
	{"InputElement", "Src", "src", urlTemplate, []string{"\"\""}},
	{"InputElement", "Height", "height", stringTemplate, nil},
	{"InputElement", "Width", "width", stringTemplate, nil},
	{"InputElement", "Accept", "accept", stringTemplate, nil},
	{"InputElement", "Autocomplete", "autocomplete", enumTemplate, []string{"on", "off"}},
	{"InputElement", "MaxLength", "maxlength", intTemplate, []string{"-1"}},
	{"InputElement", "Size", "size", intTemplate, []string{"0"}},
	{"InputElement", "Pattern", "pattern", stringTemplate, nil},
	{"InputElement", "Placeholder", "placeholder", stringTemplate, nil},
	{"InputElement", "Readonly", "readonly", boolTemplate, nil},
	{"InputElement", "Min", "min", stringTemplate, nil},
	{"InputElement", "Max", "max", stringTemplate, nil},
	{"InputElement", "DefaultValue", "value", stringTemplate, nil},
	{"InputElement", "DirName", "dirname", stringTemplate, nil},
	{"InputElement", "AccessKey", "accesskey", stringTemplate, nil},
	{"InputElement", "Multiple", "multiple", boolTemplate, nil},
	{"InputElement", "Step", "step", stringTemplate, nil},
	{"KeygenElement", "Autofocus", "autofocus", boolTemplate, nil},
	{"KeygenElement", "Challenge", "challenge", stringTemplate, nil},
	{"KeygenElement", "Disabled", "disabled", boolTemplate, nil},
	{"KeygenElement", "Keytype", "keytype", enumTemplate, []string{"RSA", "DSA", "EC"}},
	{"KeygenElement", "Name", "name", stringTemplate, nil},
	{"KeygenElement", "Type", "type", constTemplate, []string{"keygen"}},
	{"LabelElement", "HtmlFor", "for", stringTemplate, nil},
	{"LegendElement", "AccessKey", "accesskey", stringTemplate, nil},
	{"LiElement", "Value", "value", intTemplate, []string{"0"}},
	{"LiElement", "Type", "type", enumTemplate, []string{"", "1", "a", "A", "i", "I", "disc", "square", "circle"}},
	{"LinkElement", "CrossOrigin", "crossorigin", gojaEnumTemplate, []string{"anonymous", "use-credentials"}},
	{"LinkElement", "ReferrerPolicy", "referrerpolicy", enumTemplate, []string{"", "no-referrer", "no-referrer-when-downgrade", "origin", "origin-when-cross-origin", "unsafe-url"}},
	{"LinkElement", "Href", "href", urlTemplate, []string{"\"\""}},
	{"LinkElement", "Hreflang", "hreflang", stringTemplate, nil},
	{"LinkElement", "Media", "media", stringTemplate, nil},
	{"LinkElement", "Rel", "rel", stringTemplate, nil},
	{"LinkElement", "Target", "target", stringTemplate, nil},
	{"LinkElement", "Type", "type", stringTemplate, nil},
	{"MapElement", "Name", "name", stringTemplate, nil},
	{"MetaElement", "Content", "content", stringTemplate, nil},
	{"MetaElement", "Name", "name", stringTemplate, nil},
	{"MetaElement", "HttpEquiv", "http-equiv", enumTemplate, []string{"content-type", "default-style", "refresh"}},
	{"MeterElement", "Min", "min", intTemplate, []string{"0"}},
	{"MeterElement", "Max", "max", intTemplate, []string{"0"}},
	{"MeterElement", "High", "high", intTemplate, []string{"0"}},
	{"MeterElement", "Low", "low", intTemplate, []string{"0"}},
	{"MeterElement", "Optimum", "optimum", intTemplate, []string{"0"}},
	{"ModElement", "Cite", "cite", stringTemplate, nil},
	{"ModElement", "Datetime", "datetime", stringTemplate, nil},
	{"ObjectElement", "Data", "data", urlTemplate, []string{"\"\""}},
	{"ObjectElement", "Height", "height", stringTemplate, nil},
	{"ObjectElement", "Name", "name", stringTemplate, nil},
	{"ObjectElement", "Type", "type", stringTemplate, nil},
	{"ObjectElement", "TabIndex", "tabindex", intTemplate, []string{"0"}},
	{"ObjectElement", "TypeMustMatch", "typemustmatch", boolTemplate, nil},
	{"ObjectElement", "UseMap", "usemap", stringTemplate, nil},
	{"ObjectElement", "Width", "width", stringTemplate, nil},
	{"OListElement", "Reversed", "reversed", boolTemplate, nil},
	{"OListElement", "Start", "start", intTemplate, []string{"0"}},
	{"OListElement", "Type", "type", enumTemplate, []string{"1", "a", "A", "i", "I"}},
	{"OptGroupElement", "Disabled", "disabled", boolTemplate, nil},
	{"OptGroupElement", "Label", "label", stringTemplate, nil},
	{"OptionElement", "DefaultSelected", "selected", boolTemplate, nil},
	{"OptionElement", "Selected", "selected", boolTemplate, nil},
	{"OutputElement", "HtmlFor", "for", stringTemplate, nil},
	{"OutputElement", "Name", "name", stringTemplate, nil},
	{"OutputElement", "Type", "type", constTemplate, []string{"output"}},
	{"ParamElement", "Name", "name", stringTemplate, nil},
	{"ParamElement", "Value", "value", stringTemplate, nil},
	{"PreElement", "Name", "name", stringTemplate, nil},
	{"PreElement", "Value", "value", stringTemplate, nil},
	{"QuoteElement", "Cite", "cite", stringTemplate, nil},
	{"ScriptElement", "CrossOrigin", "crossorigin", stringTemplate, nil},
	{"ScriptElement", "Type", "type", stringTemplate, nil},
	{"ScriptElement", "Src", "src", urlTemplate, []string{"\"\""}},
	{"ScriptElement", "Charset", "charset", stringTemplate, nil},
	{"ScriptElement", "Async", "async", boolTemplate, nil},
	{"ScriptElement", "Defer", "defer", boolTemplate, nil},
	{"ScriptElement", "NoModule", "nomodule", boolTemplate, nil},
	{"SelectElement", "Autofocus", "autofocus", boolTemplate, nil},
	{"SelectElement", "Disabled", "disabled", boolTemplate, nil},
	{"SelectElement", "Multiple", "multiple", boolTemplate, nil},
	{"SelectElement", "Name", "name", stringTemplate, nil},
	{"SelectElement", "Required", "required", boolTemplate, nil},
	{"SelectElement", "TabIndex", "tabindex", intTemplate, []string{"0"}},
	{"SourceElement", "KeySystem", "keysystem", stringTemplate, nil},
	{"SourceElement", "Media", "media", stringTemplate, nil},
	{"SourceElement", "Sizes", "sizes", stringTemplate, nil},
	{"SourceElement", "Src", "src", urlTemplate, []string{"\"\""}},
	{"SourceElement", "Srcset", "srcset", stringTemplate, nil},
	{"SourceElement", "Type", "type", stringTemplate, nil},
	{"StyleElement", "Media", "media", stringTemplate, nil},
	{"TableElement", "Sortable", "sortable", boolTemplate, nil},
	{"TableCellElement", "ColSpan", "colspan", intTemplate, []string{"1"}},
	{"TableCellElement", "RowSpan", "rowspan", intTemplate, []string{"1"}},
	{"TableCellElement", "Headers", "headers", stringTemplate, nil},
	{"TableHeaderCellElement", "Abbr", "abbr", stringTemplate, nil},
	{"TableHeaderCellElement", "Scope", "scope", enumTemplate, []string{"", "row", "col", "colgroup", "rowgroup"}},
	{"TableHeaderCellElement", "Sorted", "sorted", boolTemplate, nil},
	{"TextAreaElement", "Type", "type", constTemplate, []string{"textarea"}},
	{"TextAreaElement", "Value", "value", stringTemplate, nil},
	{"TextAreaElement", "DefaultValue", "value", stringTemplate, nil},
	{"TextAreaElement", "Placeholder", "placeholder", stringTemplate, nil},
	{"TextAreaElement", "Rows", "rows", intTemplate, []string{"0"}},
	{"TextAreaElement", "Cols", "cols", intTemplate, []string{"0"}},
	{"TextAreaElement", "MaxLength", "maxlength", intTemplate, []string{"0"}},
	{"TextAreaElement", "TabIndex", "tabindex", intTemplate, []string{"0"}},
	{"TextAreaElement", "AccessKey", "accesskey", stringTemplate, nil},
	{"TextAreaElement", "ReadOnly", "readonly", boolTemplate, nil},
	{"TextAreaElement", "Required", "required", boolTemplate, nil},
	{"TextAreaElement", "Autocomplete", "autocomplete", enumTemplate, []string{"on", "off"}},
	{"TextAreaElement", "Autocapitalize", "autocapitalize", enumTemplate, []string{"sentences", "none", "off", "characters", "words"}},
	{"TextAreaElement", "Wrap", "wrap", enumTemplate, []string{"soft", "hard", "off"}},
	{"TimeElement", "Datetime", "datetime", stringTemplate, nil},
	{"TrackElement", "Kind", "kind", enumTemplate, []string{"subtitle", "captions", "descriptions", "chapters", "metadata"}},
	{"TrackElement", "Src", "src", urlTemplate, []string{"\"\""}},
	{"TrackElement", "Srclang", "srclang", stringTemplate, nil},
	{"TrackElement", "Label", "label", stringTemplate, nil},
	{"TrackElement", "Default", "default", boolTemplate, nil},
	{"UListElement", "Type", "type", stringTemplate, nil},
}

var collector = &ElemInfoCollectorState{}

func main() {
	fs := token.NewFileSet()
	parsedFile, parseErr := parser.ParseFile(fs, "elements.go", nil, 0)
	if parseErr != nil {
		log.WithError(parseErr).Fatal("Could not parse elements.go")
	}

	collector.handler = collector.defaultHandler
	collector.elemInfos = make(map[string]*ElemInfo)

	// Populate the elemInfos data
	ast.Inspect(parsedFile, func(n ast.Node) bool {
		if n != nil {
			collector.handler = collector.handler(n)
		}
		return true
	})

	var buf bytes.Buffer
	err := elemFuncsTemplate.Execute(&buf, struct {
		ElemInfos map[string]*ElemInfo
		FuncDefs  []struct {
			Elem, Method, Attr, TemplateType string
			Opts                             []string
		}
		TemplateType struct{ String, Url, Enum, Bool, GojaEnum, Int, Const string }
	}{
		collector.elemInfos,
		funcDefs,
		struct{ String, Url, Enum, Bool, GojaEnum, Int, Const string }{stringTemplate, urlTemplate, enumTemplate, boolTemplate, gojaEnumTemplate, intTemplate, constTemplate},
	})
	if err != nil {
		log.WithError(err).Fatal("Unable to execute template")
	}

	src, err := format.Source(buf.Bytes())
	if err != nil {
		log.WithError(err).Fatal("format.Source on generated code failed")
	}

	f, err := os.Create("elements_gen.go")
	if err != nil {
		log.WithError(err).Fatal("Unable to create the file 'elements_gen.go'")
	}

	if _, err = f.Write(src); err != nil {
		log.WithError(err).Fatal("Unable to write to 'elements_gen.go'")
	}

	err = f.Close()
	if err != nil {
		log.WithError(err).Fatal("Unable to close 'elements_gen.go'")
	}
}

var elemFuncsTemplate = template.Must(template.New("").Funcs(template.FuncMap{
	"buildStruct": buildStruct,
	"returnType":  returnType,
}).Parse(`// generated by js/modules/k6/html/gen/gen_elements.go directed by js/modules/k6/html/elements.go;  DO NOT EDIT
// nolint: goconst
package html

import "github.com/dop251/goja"

func selToElement(sel Selection) goja.Value {
	if sel.sel.Length() == 0 {
		return goja.Undefined()
	}

	elem := Element{sel.sel.Nodes[0], &sel}

	switch elem.node.Data { 
{{- range $elemName, $elemInfo := .ElemInfos }}
	case {{ $elemName }}TagName:
		return sel.rt.ToValue({{ buildStruct $elemInfo }})
{{- end }}
	default:
		return sel.rt.ToValue(elem)
	}
 }

{{ $templateType := .TemplateType }}
{{ range $funcDef := .FuncDefs -}} 

func (e {{$funcDef.Elem}}) {{$funcDef.Method}}() {{ returnType $funcDef.TemplateType }} {
{{- if eq $funcDef.TemplateType $templateType.Int }}
	return e.attrAsInt("{{ $funcDef.Attr }}", {{ index $funcDef.Opts 0 }})
{{- else if eq $funcDef.TemplateType $templateType.Enum }}
	attrVal := e.attrAsString("{{ $funcDef.Attr }}")
	switch attrVal { 
	{{- range $optIdx, $optVal := $funcDef.Opts }}
	{{- if ne $optIdx 0 }}
	case "{{$optVal}}":
		return attrVal
	{{- end }}
	{{- end}}
	default: 
		return "{{ index $funcDef.Opts 0 }}" 
	}
{{- else if eq $funcDef.TemplateType $templateType.GojaEnum }}
	attrVal, exists := e.sel.sel.Attr("{{ $funcDef.Attr }}")
	if !exists {
		return goja.Undefined()
	}
	switch attrVal { 
	{{- range $optVal := $funcDef.Opts }}
	case "{{$optVal}}":
		return e.sel.rt.ToValue(attrVal)
	{{- end}}
	default:
		return goja.Undefined()
	}
{{- else if eq $funcDef.TemplateType $templateType.Const }}
	return "{{ index $funcDef.Opts 0 }}"
{{- else if eq $funcDef.TemplateType $templateType.Url }}
	return e.attrAsURLString("{{ $funcDef.Attr }}", {{ index $funcDef.Opts 0 }})
{{- else if eq $funcDef.TemplateType $templateType.String }}
	return e.attrAsString("{{ $funcDef.Attr }}")
{{- else if eq $funcDef.TemplateType $templateType.Bool }}
	return e.attrIsPresent("{{ $funcDef.Attr }}")
{{- end}}
}
{{ end }}
`))

func buildStruct(elemInfo ElemInfo) string {
	if elemInfo.PrtStructName == "Element" {
		return elemInfo.StructName + "{elem}"
	} else {
		return elemInfo.StructName + "{" + elemInfo.PrtStructName + "{elem}}"
	}
}

func returnType(templateType string) string {
	switch templateType {
	case boolTemplate:
		return "bool"
	case intTemplate:
		return "int"
	case gojaEnumTemplate:
		return "goja.Value"
	default:
		return "string"
	}
}

// Node handler functions for ast.Inspect.
func (ce *ElemInfoCollectorState) defaultHandler(node ast.Node) ElemInfoCollector {
	ce.elemName = ""
	switch node.(type) {
	case *ast.TypeSpec:
		return ce.elemTypeSpecHandler

	case *ast.ValueSpec:
		return ce.tagNameValueSpecHandler

	default:
		return ce.defaultHandler
	}
}

// Found a tagname constant. eg AnchorTagName = "a" adds the entry ce.elemInfos["Anchor"] = {""", ""}
func (ce *ElemInfoCollectorState) tagNameValueSpecHandler(node ast.Node) ElemInfoCollector {
	switch x := node.(type) {
	case *ast.Ident:
		if strings.HasSuffix(x.Name, "TagName") {
			ce.elemName = strings.TrimSuffix(x.Name, "TagName")
			ce.elemInfos[ce.elemName] = &ElemInfo{"", ""}
		}

		return ce.defaultHandler

	default:
		return ce.defaultHandler
	}
}

func (ce *ElemInfoCollectorState) elemTypeSpecHandler(node ast.Node) ElemInfoCollector {
	switch x := node.(type) {
	case *ast.Ident:
		if !strings.HasSuffix(x.Name, "Element") {
			return ce.defaultHandler
		}

		if ce.elemName == "" {
			ce.elemName = strings.TrimSuffix(x.Name, "Element")
			// Ignore elements which don't have a tag name constant meaning no elemInfo structure was created by the TagName handle.
			// It skips the Href, Media, FormField, Mod, TableSection or TableCell structs as these structs are inherited by other elements and not created indepedently.
			if _, ok := ce.elemInfos[ce.elemName]; !ok {
				return ce.defaultHandler
			}

			ce.elemInfos[ce.elemName].StructName = x.Name
			return ce.elemTypeSpecHandler
		} else {
			ce.elemInfos[ce.elemName].PrtStructName = x.Name
			return ce.defaultHandler
		}

	case *ast.StructType:
		return ce.elemTypeSpecHandler

	case *ast.FieldList:
		return ce.elemTypeSpecHandler

	case *ast.Field:
		return ce.elemTypeSpecHandler

	default:
		return ce.defaultHandler
	}
}
