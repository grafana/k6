package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"
	"text/template"
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
	{"HrefElement", "Download", "download", "string", nil},
	{"HrefElement", "ReferrerPolicy", "referrerpolicy", "enum", []string{"", "no-referrer", "no-referrer-when-downgrade", "origin", "origin-when-cross-origin", "unsafe-url"}},
	{"HrefElement", "Rel", "rel", "string", nil},
	{"HrefElement", "Href", "href", "url", []string{"\"\""}},
	{"HrefElement", "Target", "target", "string", nil},
	{"HrefElement", "Type", "type", "string", nil},
	{"HrefElement", "AccessKey", "accesskey", "string", nil},
	{"HrefElement", "HrefLang", "hreflang", "string", nil},
	{"HrefElement", "ToString", "href", "url", []string{"\"\""}},
	{"MediaElement", "Autoplay", "autoplay", "bool", nil},
	{"MediaElement", "Controls", "controls", "bool", nil},
	{"MediaElement", "Loop", "loop", "bool", nil},
	{"MediaElement", "Muted", "muted", "bool", nil},
	{"MediaElement", "Preload", "preload", "enum", []string{"auto", "metadata", "none"}},
	{"MediaElement", "Src", "src", "url", []string{"\"\""}},
	{"MediaElement", "CrossOrigin", "crossorigin", "enum-goja", []string{"anonymous", "use-credentials"}},
	{"MediaElement", "CurrentSrc", "src", "string", nil},
	{"MediaElement", "DefaultMuted", "muted", "bool", nil},
	{"MediaElement", "MediaGroup", "mediagroup", "string", nil},
	{"BaseElement", "Href", "href", "url", []string{"e.sel.URL"}},
	{"BaseElement", "Target", "target", "string", nil},
	{"ButtonElement", "AccessKey", "accesskey", "string", nil},
	{"ButtonElement", "Autofocus", "autofocus", "bool", nil},
	{"ButtonElement", "Disabled", "disabled", "bool", nil},
	{"ButtonElement", "TabIndex", "tabindex", "int", []string{"0"}},
	{"ButtonElement", "Type", "type", "enum", []string{"submit", "button", "menu", "reset"}},
	{"DataElement", "Value", "value", "string", nil},
	{"EmbedElement", "Height", "height", "string", nil},
	{"EmbedElement", "Width", "width", "string", nil},
	{"EmbedElement", "Src", "src", "string", nil},
	{"EmbedElement", "Type", "type", "string", nil},
	{"FieldSetElement", "Disabled", "disabled", "bool", nil},
	{"FieldSetElement", "Name", "name", "string", nil},
	{"FormElement", "Action", "action", "url", []string{"\"\""}},
	{"FormElement", "Name", "name", "string", nil},
	{"FormElement", "Target", "target", "string", nil},
	{"FormElement", "Enctype", "enctype", "enum", []string{"application/x-www-form-urlencoded", "multipart/form-data", "text/plain"}},
	{"FormElement", "Encoding", "enctype", "enum", []string{"application/x-www-form-urlencoded", "multipart/form-data", "text/plain"}},
	{"FormElement", "AcceptCharset", "accept-charset", "string", nil},
	{"FormElement", "Autocomplete", "autocomplete", "enum", []string{"on", "off"}},
	{"FormElement", "NoValidate", "novalidate", "bool", nil},
	{"IFrameElement", "Allowfullscreen", "allowfullscreen", "bool", nil},
	{"IFrameElement", "ReferrerPolicy", "referrerpolicy", "enum", []string{"", "no-referrer", "no-referrer-when-downgrade", "origin", "origin-when-cross-origin", "unsafe-url"}},
	{"IFrameElement", "Height", "height", "string", nil},
	{"IFrameElement", "Width", "width", "string", nil},
	{"IFrameElement", "Name", "name", "string", nil},
	{"IFrameElement", "Src", "src", "url", []string{"\"\""}},
	{"ImageElement", "CurrentSrc", "src", "url", []string{"\"\""}},
	{"ImageElement", "Sizes", "sizes", "string", nil},
	{"ImageElement", "Srcset", "srcset", "string", nil},
	{"ImageElement", "Alt", "alt", "string", nil},
	{"ImageElement", "CrossOrigin", "crossorigin", "enum-goja", []string{"anonymous", "use-credentials"}},
	{"ImageElement", "Height", "height", "int", []string{"0"}},
	{"ImageElement", "Width", "width", "int", []string{"0"}},
	{"ImageElement", "IsMap", "ismap", "bool", nil},
	{"ImageElement", "Name", "name", "string", nil},
	{"ImageElement", "Src", "src", "url", []string{"\"\""}},
	{"ImageElement", "UseMap", "usemap", "string", nil},
	{"ImageElement", "ReferrerPolicy", "referrerpolicy", "enum", []string{"", "no-referrer", "no-referrer-when-downgrade", "origin", "origin-when-cross-origin", "unsafe-url"}},
	{"InputElement", "Name", "name", "string", nil},
	{"InputElement", "TabIndex", "tabindex", "int", []string{"0"}},
	{"InputElement", "Type", "type", "enum", []string{"text", "button", "checkbox", "color", "date", "datetime-local", "email", "file", "hidden", "image", "month", "number", "password", "radio", "range", "reset", "search", "submit", "tel", "time", "url", "week"}},
	{"InputElement", "Disabled", "disabled", "bool", nil},
	{"InputElement", "Autofocus", "autofocus", "bool", nil},
	{"InputElement", "Required", "required", "bool", nil},
	{"InputElement", "Value", "value", "string", nil},
	{"InputElement", "Checked", "checked", "bool", nil},
	{"InputElement", "DefaultChecked", "checked", "bool", nil},
	{"InputElement", "Alt", "alt", "string", nil},
	{"InputElement", "Src", "src", "url", []string{"\"\""}},
	{"InputElement", "Height", "height", "string", nil},
	{"InputElement", "Width", "width", "string", nil},
	{"InputElement", "Accept", "accept", "string", nil},
	{"InputElement", "Autocomplete", "autocomplete", "enum", []string{"on", "off"}},
	{"InputElement", "MaxLength", "maxlength", "int", []string{"-1"}},
	{"InputElement", "Size", "size", "int", []string{"0"}},
	{"InputElement", "Pattern", "pattern", "string", nil},
	{"InputElement", "Placeholder", "placeholder", "string", nil},
	{"InputElement", "Readonly", "readonly", "bool", nil},
	{"InputElement", "Min", "min", "string", nil},
	{"InputElement", "Max", "max", "string", nil},
	{"InputElement", "DefaultValue", "value", "string", nil},
	{"InputElement", "DirName", "dirname", "string", nil},
	{"InputElement", "AccessKey", "accesskey", "string", nil},
	{"InputElement", "Multiple", "multiple", "bool", nil},
	{"InputElement", "Step", "step", "string", nil},
	{"KeygenElement", "Autofocus", "autofocus", "bool", nil},
	{"KeygenElement", "Challenge", "challenge", "string", nil},
	{"KeygenElement", "Disabled", "disabled", "bool", nil},
	{"KeygenElement", "Keytype", "keytype", "enum", []string{"RSA", "DSA", "EC"}},
	{"KeygenElement", "Name", "name", "string", nil},
	{"KeygenElement", "Type", "type", "const", []string{"keygen"}},
	{"LabelElement", "HtmlFor", "for", "string", nil},
	{"LegendElement", "AccessKey", "accesskey", "string", nil},
	{"LiElement", "Value", "value", "int", []string{"0"}},
	{"LiElement", "Type", "type", "enum", []string{"", "1", "a", "A", "i", "I", "disc", "square", "circle"}},
	{"LinkElement", "CrossOrigin", "crossorigin", "enum-goja", []string{"anonymous", "use-credentials"}},
	{"LinkElement", "ReferrerPolicy", "referrerpolicy", "enum", []string{"", "no-referrer", "no-referrer-when-downgrade", "origin", "origin-when-cross-origin", "unsafe-url"}},
	{"LinkElement", "Href", "href", "url", []string{"\"\""}},
	{"LinkElement", "Hreflang", "hreflang", "string", nil},
	{"LinkElement", "Media", "media", "string", nil},
	{"LinkElement", "Rel", "rel", "string", nil},
	{"LinkElement", "Target", "target", "string", nil},
	{"LinkElement", "Type", "type", "string", nil},
	{"MapElement", "Name", "name", "string", nil},
	{"MetaElement", "Content", "content", "string", nil},
	{"MetaElement", "Name", "name", "string", nil},
	{"MetaElement", "HttpEquiv", "http-equiv", "enum", []string{"content-type", "default-style", "refresh"}},
	{"MeterElement", "Min", "min", "int", []string{"0"}},
	{"MeterElement", "Max", "max", "int", []string{"0"}},
	{"MeterElement", "High", "high", "int", []string{"0"}},
	{"MeterElement", "Low", "low", "int", []string{"0"}},
	{"MeterElement", "Optimum", "optimum", "int", []string{"0"}},
	{"ModElement", "Cite", "cite", "string", nil},
	{"ModElement", "Datetime", "datetime", "string", nil},
	{"ObjectElement", "Data", "data", "url", []string{"\"\""}},
	{"ObjectElement", "Height", "height", "string", nil},
	{"ObjectElement", "Name", "name", "string", nil},
	{"ObjectElement", "Type", "type", "string", nil},
	{"ObjectElement", "TabIndex", "tabindex", "int", []string{"0"}},
	{"ObjectElement", "TypeMustMatch", "typemustmatch", "bool", nil},
	{"ObjectElement", "UseMap", "usemap", "string", nil},
	{"ObjectElement", "Width", "width", "string", nil},
	{"OListElement", "Reversed", "reversed", "bool", nil},
	{"OListElement", "Start", "start", "int", []string{"0"}},
	{"OListElement", "Type", "type", "enum", []string{"1", "a", "A", "i", "I"}},
	{"OptGroupElement", "Disabled", "disabled", "bool", nil},
	{"OptGroupElement", "Label", "label", "string", nil},
	{"OptionElement", "DefaultSelected", "selected", "bool", nil},
	{"OptionElement", "Selected", "selected", "bool", nil},
	{"OutputElement", "HtmlFor", "for", "string", nil},
	{"OutputElement", "Name", "name", "string", nil},
	{"OutputElement", "Type", "type", "const", []string{"output"}},
	{"ParamElement", "Name", "name", "string", nil},
	{"ParamElement", "Value", "value", "string", nil},
	{"PreElement", "Name", "name", "string", nil},
	{"PreElement", "Value", "value", "string", nil},
	{"QuoteElement", "Cite", "cite", "string", nil},
	{"ScriptElement", "CrossOrigin", "crossorigin", "string", nil},
	{"ScriptElement", "Type", "type", "string", nil},
	{"ScriptElement", "Src", "src", "url", []string{"\"\""}},
	{"ScriptElement", "Charset", "charset", "string", nil},
	{"ScriptElement", "Async", "async", "bool", nil},
	{"ScriptElement", "Defer", "defer", "bool", nil},
	{"ScriptElement", "NoModule", "nomodule", "bool", nil},
	{"SelectElement", "Autofocus", "autofocus", "bool", nil},
	{"SelectElement", "Disabled", "disabled", "bool", nil},
	{"SelectElement", "Multiple", "multiple", "bool", nil},
	{"SelectElement", "Name", "name", "string", nil},
	{"SelectElement", "Required", "required", "bool", nil},
	{"SelectElement", "TabIndex", "tabindex", "int", []string{"0"}},
	{"SourceElement", "KeySystem", "keysystem", "string", nil},
	{"SourceElement", "Media", "media", "string", nil},
	{"SourceElement", "Sizes", "sizes", "string", nil},
	{"SourceElement", "Src", "src", "url", []string{"\"\""}},
	{"SourceElement", "Srcset", "srcset", "string", nil},
	{"SourceElement", "Type", "type", "string", nil},
	{"StyleElement", "Media", "media", "string", nil},
	{"TableElement", "Sortable", "sortable", "bool", nil},
	{"TableCellElement", "ColSpan", "colspan", "int", []string{"1"}},
	{"TableCellElement", "RowSpan", "rowspan", "int", []string{"1"}},
	{"TableCellElement", "Headers", "headers", "string", nil},
	{"TableHeaderCellElement", "Abbr", "abbr", "string", nil},
	{"TableHeaderCellElement", "Scope", "scope", "enum", []string{"", "row", "col", "colgroup", "rowgroup"}},
	{"TableHeaderCellElement", "Sorted", "sorted", "bool", nil},
	{"TextAreaElement", "Type", "type", "const", []string{"textarea"}},
	{"TextAreaElement", "Value", "value", "string", nil},
	{"TextAreaElement", "DefaultValue", "value", "string", nil},
	{"TextAreaElement", "Placeholder", "placeholder", "string", nil},
	{"TextAreaElement", "Rows", "rows", "int", []string{"0"}},
	{"TextAreaElement", "Cols", "cols", "int", []string{"0"}},
	{"TextAreaElement", "MaxLength", "maxlength", "int", []string{"0"}},
	{"TextAreaElement", "TabIndex", "tabindex", "int", []string{"0"}},
	{"TextAreaElement", "AccessKey", "accesskey", "string", nil},
	{"TextAreaElement", "ReadOnly", "readonly", "bool", nil},
	{"TextAreaElement", "Required", "required", "bool", nil},
	{"TextAreaElement", "Autocomplete", "autocomplete", "enum", []string{"on", "off"}},
	{"TextAreaElement", "Autocapitalize", "autocapitalize", "enum", []string{"sentences", "none", "off", "characters", "words"}},
	{"TextAreaElement", "Wrap", "wrap", "enum", []string{"soft", "hard", "off"}},
	{"TimeElement", "Datetime", "datetime", "string", nil},
	{"TrackElement", "Kind", "kind", "enum", []string{"subtitle", "captions", "descriptions", "chapters", "metadata"}},
	{"TrackElement", "Src", "src", "url", []string{"\"\""}},
	{"TrackElement", "Srclang", "srclang", "string", nil},
	{"TrackElement", "Label", "label", "string", nil},
	{"TrackElement", "Default", "default", "bool", nil},
	{"UListElement", "Type", "type", "string", nil},
}

var collector = &ElemInfoCollectorState{}

func main() {
	fs := token.NewFileSet()
	parsedFile, parseErr := parser.ParseFile(fs, "elements.go", nil, 0)
	if parseErr != nil {
		log.Fatalf("error: could not parse elements.go\n", parseErr)
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

	buf := new(bytes.Buffer)
	err := elemFuncsTemplate.Execute(buf, struct {
		ElemInfos map[string]*ElemInfo
		FuncDefs  []struct {
			Elem, Method, Attr, TemplateType string
			Opts                             []string
		}
	}{
		collector.elemInfos,
		funcDefs,
	})
	if err != nil {
		log.Fatalf("error: unable to execute template\n", err)
	}

	src, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatalf("error: format.Source on generated code failed\n", err)
	}

	f, err := os.Create("elements_gen.go")
	if err != nil {
		log.Fatalf("error: Unable to create the file 'elements_gen.go'\n", err)
	}

	if _, err = f.Write(src); err != nil {
		log.Fatalf("error: Unable to write to 'elements_gen.go'\n", err)
	}

	err = f.Close()
	if err != nil {
		log.Fatalf("error: unable to close the file 'elements_gen.go'\n", err)
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

{{ range $funcDef := .FuncDefs -}} 

func (e {{$funcDef.Elem}}) {{$funcDef.Method}}() {{ returnType $funcDef.TemplateType }} {
{{- if eq $funcDef.TemplateType "int" }}
	return e.attrAsInt("{{ $funcDef.Attr }}", {{ index $funcDef.Opts 0 }})
{{- else if eq $funcDef.TemplateType "enum" }}
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
{{- else if eq $funcDef.TemplateType "enum-goja" }}
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
{{- else if eq $funcDef.TemplateType "const" }}
	return "{{ index $funcDef.Opts 0 }}"
{{- else if eq $funcDef.TemplateType "url" }}
	return e.attrAsURLString("{{ $funcDef.Attr }}", {{ index $funcDef.Opts 0 }})
{{- else if eq $funcDef.TemplateType "string" }}
	return e.attrAsString("{{ $funcDef.Attr }}")
{{- else if eq $funcDef.TemplateType "bool" }}
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
	case "bool":
		return "bool"
	case "int":
		return "int"
	case "enum-goja":
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
