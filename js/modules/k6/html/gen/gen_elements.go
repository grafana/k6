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

	"github.com/sirupsen/logrus"
)

// Generate elements_gen.go.
// There are two sections of code which need to be generated.
// The selToElement function and the attribute accessor methods.

// The first step to generate the selToElement function is parse the TagName constants and Element structs
// in elements.go using ast.Inspect
// One of NodeHandlerFunc methods is called for each ast.Node parsed by ast.Inspect
// The NodeHandlerFunc methods build ElemInfo structs and populate elemInfos in AstInspectState
// The template later iterates over elemInfos to build the selToElement function

type nodeHandlerFunc func(node ast.Node) nodeHandlerFunc

type astInspectState struct {
	handler   nodeHandlerFunc
	elemName  string
	elemInfos map[string]*elemInfo
}

type elemInfo struct {
	StructName    string
	PrtStructName string
}

// The attribute accessors are build using function definitions. Each funcion definition has a templateType.
// The number of TemplateArgs varies based on the TenplateType and is documented below.
type (
	templateType string
	templateArg  string
)

const (
	stringTemplate       templateType = "typeString"
	urlTemplate          templateType = "typeURL"
	boolTemplate         templateType = "typeBool"
	intTemplate          templateType = "typeInt"
	constTemplate        templateType = "typeConst"
	enumTemplate         templateType = "typeEnum"
	nullableEnumTemplate templateType = "typeEnumNullable"
)

// Some common TemplateArgs
//
//nolint:lll,gochecknoglobals
var (
	// Default return values for urlTemplate functions. Either an empty string or the current URL.
	defaultURLEmpty   = []templateArg{"\"\""}
	defaultURLCurrent = []templateArg{"e.sel.URL"}

	// Common default return values for intTemplates
	defaultInt0      = []templateArg{"0"}
	defaultIntMinus1 = []templateArg{"-1"}
	defaultIntPlus1  = []templateArg{"1"}

	// The following are the for various attributes using enumTemplate.
	// The first item in the list is the default value.
	autocompleteOpts = []templateArg{"on", "off"}
	referrerOpts     = []templateArg{"", "no-referrer", "no-referrer-when-downgrade", "origin", "origin-when-cross-origin", "unsafe-url"}
	preloadOpts      = []templateArg{"auto", "metadata", "none"}
	btnTypeOpts      = []templateArg{"submit", "button", "menu", "reset"}
	encTypeOpts      = []templateArg{"application/x-www-form-urlencoded", "multipart/form-data", "text/plain"}
	inputTypeOpts    = []templateArg{"text", "button", "checkbox", "color", "date", "datetime-local", "email", "file", "hidden", "image", "month", "number", "password", "radio", "range", "reset", "search", "submit", "tel", "time", "url", "week"}
	keyTypeOpts      = []templateArg{"RSA", "DSA", "EC"}
	keygenTypeOpts   = []templateArg{"keygen"}
	liTypeOpts       = []templateArg{"", "1", "a", "A", "i", "I", "disc", "square", "circle"}
	httpEquivOpts    = []templateArg{"content-type", "default-style", "refresh"}
	olistTypeOpts    = []templateArg{"1", "a", "A", "i", "I"}
	scopeOpts        = []templateArg{"", "row", "col", "colgroup", "rowgroup"}
	autocapOpts      = []templateArg{"sentences", "none", "off", "characters", "words"}
	wrapOpts         = []templateArg{"soft", "hard", "off"}
	kindOpts         = []templateArg{"subtitle", "captions", "descriptions", "chapters", "metadata"}

	// These are the values allowed for the crossorigin attribute, used by the nullableEnumTemplates is always goja.Undefined
	crossOriginOpts = []templateArg{"anonymous", "use-credentials"}
)

// Elem is one of the Element struct names from elements.go
// Method is the go method name to be generated.
// Attr is the name of the DOM attribute the method will access, usually the Method name but lowercased.
// TemplateType determines which type of function is generation by the template
// TemplateArgs is a list of values to be interpolated in the template.

// The number of TemplateArgs depends on the template type.
//
//	stringTemplate: doesn't use any TemplateArgs
//	boolTemplate: doesn't use any TemplateArgs
//	constTemplate: uses 1 Template Arg, the generated function always returns that value
//	intTemplate: needs 1 TemplateArg, used as the default return value (when the attribute was empty).
//	urlTemplate: needs 1 TemplateArg, used as the default, either "defaultURLEmpty" or "defaultURLCurrent"
//	enumTemplate: uses any number or more TemplateArg, the gen'd func always returns one of the values in
//	              the TemplateArgs. The first item in the list is used as the default when the attribute
//	              was invalid or unset.
//	nullableEnumTemplate: similar to the enumTemplate except the default is goja.Undefined and the
//	                      return type is goja.Value
//
//nolint:gochecknoglobals
var funcDefs = []struct {
	Elem, Method, Attr string
	TemplateType       templateType
	TemplateArgs       []templateArg
}{
	{"HrefElement", "Download", "download", stringTemplate, nil},
	{"HrefElement", "ReferrerPolicy", "referrerpolicy", enumTemplate, referrerOpts},
	{"HrefElement", "Rel", "rel", stringTemplate, nil},
	{"HrefElement", "Href", "href", urlTemplate, defaultURLEmpty},
	{"HrefElement", "Target", "target", stringTemplate, nil},
	{"HrefElement", "Type", "type", stringTemplate, nil},
	{"HrefElement", "AccessKey", "accesskey", stringTemplate, nil},
	{"HrefElement", "HrefLang", "hreflang", stringTemplate, nil},
	{"HrefElement", "ToString", "href", urlTemplate, defaultURLEmpty},
	{"MediaElement", "Autoplay", "autoplay", boolTemplate, nil},
	{"MediaElement", "Controls", "controls", boolTemplate, nil},
	{"MediaElement", "Loop", "loop", boolTemplate, nil},
	{"MediaElement", "Muted", "muted", boolTemplate, nil},
	{"MediaElement", "Preload", "preload", enumTemplate, preloadOpts},
	{"MediaElement", "Src", "src", urlTemplate, defaultURLEmpty},
	{"MediaElement", "CrossOrigin", "crossorigin", nullableEnumTemplate, crossOriginOpts},
	{"MediaElement", "CurrentSrc", "src", stringTemplate, nil},
	{"MediaElement", "DefaultMuted", "muted", boolTemplate, nil},
	{"MediaElement", "MediaGroup", "mediagroup", stringTemplate, nil},
	{"BaseElement", "Href", "href", urlTemplate, defaultURLCurrent},
	{"BaseElement", "Target", "target", stringTemplate, nil},
	{"ButtonElement", "AccessKey", "accesskey", stringTemplate, nil},
	{"ButtonElement", "Autofocus", "autofocus", boolTemplate, nil},
	{"ButtonElement", "Disabled", "disabled", boolTemplate, nil},
	{"ButtonElement", "TabIndex", "tabindex", intTemplate, defaultInt0},
	{"ButtonElement", "Type", "type", enumTemplate, btnTypeOpts},
	{"DataElement", "Value", "value", stringTemplate, nil},
	{"EmbedElement", "Height", "height", stringTemplate, nil},
	{"EmbedElement", "Width", "width", stringTemplate, nil},
	{"EmbedElement", "Src", "src", stringTemplate, nil},
	{"EmbedElement", "Type", "type", stringTemplate, nil},
	{"FieldSetElement", "Disabled", "disabled", boolTemplate, nil},
	{"FieldSetElement", "Name", "name", stringTemplate, nil},
	{"FormElement", "Action", "action", urlTemplate, defaultURLEmpty},
	{"FormElement", "Name", "name", stringTemplate, nil},
	{"FormElement", "Target", "target", stringTemplate, nil},
	{"FormElement", "Enctype", "enctype", enumTemplate, encTypeOpts},
	{"FormElement", "Encoding", "enctype", enumTemplate, encTypeOpts},
	{"FormElement", "AcceptCharset", "accept-charset", stringTemplate, nil},
	{"FormElement", "Autocomplete", "autocomplete", enumTemplate, autocompleteOpts},
	{"FormElement", "NoValidate", "novalidate", boolTemplate, nil},
	{"IFrameElement", "Allowfullscreen", "allowfullscreen", boolTemplate, nil},
	{"IFrameElement", "ReferrerPolicy", "referrerpolicy", enumTemplate, referrerOpts},
	{"IFrameElement", "Height", "height", stringTemplate, nil},
	{"IFrameElement", "Width", "width", stringTemplate, nil},
	{"IFrameElement", "Name", "name", stringTemplate, nil},
	{"IFrameElement", "Src", "src", urlTemplate, defaultURLEmpty},
	{"ImageElement", "CurrentSrc", "src", urlTemplate, defaultURLEmpty},
	{"ImageElement", "Sizes", "sizes", stringTemplate, nil},
	{"ImageElement", "Srcset", "srcset", stringTemplate, nil},
	{"ImageElement", "Alt", "alt", stringTemplate, nil},
	{"ImageElement", "CrossOrigin", "crossorigin", nullableEnumTemplate, crossOriginOpts},
	{"ImageElement", "Height", "height", intTemplate, defaultInt0},
	{"ImageElement", "Width", "width", intTemplate, defaultInt0},
	{"ImageElement", "IsMap", "ismap", boolTemplate, nil},
	{"ImageElement", "Name", "name", stringTemplate, nil},
	{"ImageElement", "Src", "src", urlTemplate, defaultURLEmpty},
	{"ImageElement", "UseMap", "usemap", stringTemplate, nil},
	{"ImageElement", "ReferrerPolicy", "referrerpolicy", enumTemplate, referrerOpts},
	{"InputElement", "Name", "name", stringTemplate, nil},
	{"InputElement", "TabIndex", "tabindex", intTemplate, defaultInt0},
	{"InputElement", "Type", "type", enumTemplate, inputTypeOpts},
	{"InputElement", "Disabled", "disabled", boolTemplate, nil},
	{"InputElement", "Autofocus", "autofocus", boolTemplate, nil},
	{"InputElement", "Required", "required", boolTemplate, nil},
	{"InputElement", "Value", "value", stringTemplate, nil},
	{"InputElement", "Checked", "checked", boolTemplate, nil},
	{"InputElement", "DefaultChecked", "checked", boolTemplate, nil},
	{"InputElement", "Alt", "alt", stringTemplate, nil},
	{"InputElement", "Src", "src", urlTemplate, defaultURLEmpty},
	{"InputElement", "Height", "height", stringTemplate, nil},
	{"InputElement", "Width", "width", stringTemplate, nil},
	{"InputElement", "Accept", "accept", stringTemplate, nil},
	{"InputElement", "Autocomplete", "autocomplete", enumTemplate, autocompleteOpts},
	{"InputElement", "MaxLength", "maxlength", intTemplate, defaultIntMinus1},
	{"InputElement", "Size", "size", intTemplate, defaultInt0},
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
	{"KeygenElement", "Keytype", "keytype", enumTemplate, keyTypeOpts},
	{"KeygenElement", "Name", "name", stringTemplate, nil},
	{"KeygenElement", "Type", "type", constTemplate, keygenTypeOpts},
	{"LabelElement", "HtmlFor", "for", stringTemplate, nil},
	{"LegendElement", "AccessKey", "accesskey", stringTemplate, nil},
	{"LiElement", "Value", "value", intTemplate, defaultInt0},
	{"LiElement", "Type", "type", enumTemplate, liTypeOpts},
	{"LinkElement", "CrossOrigin", "crossorigin", nullableEnumTemplate, crossOriginOpts},
	{"LinkElement", "ReferrerPolicy", "referrerpolicy", enumTemplate, referrerOpts},
	{"LinkElement", "Href", "href", urlTemplate, defaultURLEmpty},
	{"LinkElement", "Hreflang", "hreflang", stringTemplate, nil},
	{"LinkElement", "Media", "media", stringTemplate, nil},
	{"LinkElement", "Rel", "rel", stringTemplate, nil},
	{"LinkElement", "Target", "target", stringTemplate, nil},
	{"LinkElement", "Type", "type", stringTemplate, nil},
	{"MapElement", "Name", "name", stringTemplate, nil},
	{"MetaElement", "Content", "content", stringTemplate, nil},
	{"MetaElement", "Name", "name", stringTemplate, nil},
	{"MetaElement", "HttpEquiv", "http-equiv", enumTemplate, httpEquivOpts},
	{"MeterElement", "Min", "min", intTemplate, defaultInt0},
	{"MeterElement", "Max", "max", intTemplate, defaultInt0},
	{"MeterElement", "High", "high", intTemplate, defaultInt0},
	{"MeterElement", "Low", "low", intTemplate, defaultInt0},
	{"MeterElement", "Optimum", "optimum", intTemplate, defaultInt0},
	{"ModElement", "Cite", "cite", stringTemplate, nil},
	{"ModElement", "Datetime", "datetime", stringTemplate, nil},
	{"ObjectElement", "Data", "data", urlTemplate, defaultURLEmpty},
	{"ObjectElement", "Height", "height", stringTemplate, nil},
	{"ObjectElement", "Name", "name", stringTemplate, nil},
	{"ObjectElement", "Type", "type", stringTemplate, nil},
	{"ObjectElement", "TabIndex", "tabindex", intTemplate, defaultInt0},
	{"ObjectElement", "TypeMustMatch", "typemustmatch", boolTemplate, nil},
	{"ObjectElement", "UseMap", "usemap", stringTemplate, nil},
	{"ObjectElement", "Width", "width", stringTemplate, nil},
	{"OListElement", "Reversed", "reversed", boolTemplate, nil},
	{"OListElement", "Start", "start", intTemplate, defaultInt0},
	{"OListElement", "Type", "type", enumTemplate, olistTypeOpts},
	{"OptGroupElement", "Disabled", "disabled", boolTemplate, nil},
	{"OptGroupElement", "Label", "label", stringTemplate, nil},
	{"OptionElement", "DefaultSelected", "selected", boolTemplate, nil},
	{"OptionElement", "Selected", "selected", boolTemplate, nil},
	{"OutputElement", "HtmlFor", "for", stringTemplate, nil},
	{"OutputElement", "Name", "name", stringTemplate, nil},
	{"OutputElement", "Type", "type", constTemplate, []templateArg{"output"}},
	{"ParamElement", "Name", "name", stringTemplate, nil},
	{"ParamElement", "Value", "value", stringTemplate, nil},
	{"PreElement", "Name", "name", stringTemplate, nil},
	{"PreElement", "Value", "value", stringTemplate, nil},
	{"QuoteElement", "Cite", "cite", stringTemplate, nil},
	{"ScriptElement", "CrossOrigin", "crossorigin", stringTemplate, nil},
	{"ScriptElement", "Type", "type", stringTemplate, nil},
	{"ScriptElement", "Src", "src", urlTemplate, defaultURLEmpty},
	{"ScriptElement", "Charset", "charset", stringTemplate, nil},
	{"ScriptElement", "Async", "async", boolTemplate, nil},
	{"ScriptElement", "Defer", "defer", boolTemplate, nil},
	{"ScriptElement", "NoModule", "nomodule", boolTemplate, nil},
	{"SelectElement", "Autofocus", "autofocus", boolTemplate, nil},
	{"SelectElement", "Disabled", "disabled", boolTemplate, nil},
	{"SelectElement", "Multiple", "multiple", boolTemplate, nil},
	{"SelectElement", "Name", "name", stringTemplate, nil},
	{"SelectElement", "Required", "required", boolTemplate, nil},
	{"SelectElement", "TabIndex", "tabindex", intTemplate, defaultInt0},
	{"SourceElement", "KeySystem", "keysystem", stringTemplate, nil},
	{"SourceElement", "Media", "media", stringTemplate, nil},
	{"SourceElement", "Sizes", "sizes", stringTemplate, nil},
	{"SourceElement", "Src", "src", urlTemplate, defaultURLEmpty},
	{"SourceElement", "Srcset", "srcset", stringTemplate, nil},
	{"SourceElement", "Type", "type", stringTemplate, nil},
	{"StyleElement", "Media", "media", stringTemplate, nil},
	{"TableElement", "Sortable", "sortable", boolTemplate, nil},
	{"TableCellElement", "ColSpan", "colspan", intTemplate, defaultIntPlus1},
	{"TableCellElement", "RowSpan", "rowspan", intTemplate, defaultIntPlus1},
	{"TableCellElement", "Headers", "headers", stringTemplate, nil},
	{"TableHeaderCellElement", "Abbr", "abbr", stringTemplate, nil},
	{"TableHeaderCellElement", "Scope", "scope", enumTemplate, scopeOpts},
	{"TableHeaderCellElement", "Sorted", "sorted", boolTemplate, nil},
	{"TextAreaElement", "Type", "type", constTemplate, []templateArg{"textarea"}},
	{"TextAreaElement", "Value", "value", stringTemplate, nil},
	{"TextAreaElement", "DefaultValue", "value", stringTemplate, nil},
	{"TextAreaElement", "Placeholder", "placeholder", stringTemplate, nil},
	{"TextAreaElement", "Rows", "rows", intTemplate, defaultInt0},
	{"TextAreaElement", "Cols", "cols", intTemplate, defaultInt0},
	{"TextAreaElement", "MaxLength", "maxlength", intTemplate, defaultInt0},
	{"TextAreaElement", "TabIndex", "tabindex", intTemplate, defaultInt0},
	{"TextAreaElement", "AccessKey", "accesskey", stringTemplate, nil},
	{"TextAreaElement", "ReadOnly", "readonly", boolTemplate, nil},
	{"TextAreaElement", "Required", "required", boolTemplate, nil},
	{"TextAreaElement", "Autocomplete", "autocomplete", enumTemplate, autocompleteOpts},
	{"TextAreaElement", "Autocapitalize", "autocapitalize", enumTemplate, autocapOpts},
	{"TextAreaElement", "Wrap", "wrap", enumTemplate, wrapOpts},
	{"TimeElement", "Datetime", "datetime", stringTemplate, nil},
	{"TrackElement", "Kind", "kind", enumTemplate, kindOpts},
	{"TrackElement", "Src", "src", urlTemplate, defaultURLEmpty},
	{"TrackElement", "Srclang", "srclang", stringTemplate, nil},
	{"TrackElement", "Label", "label", stringTemplate, nil},
	{"TrackElement", "Default", "default", boolTemplate, nil},
	{"UListElement", "Type", "type", stringTemplate, nil},
}

func main() {
	fs := token.NewFileSet()
	parsedFile, parseErr := parser.ParseFile(fs, "elements.go", nil, 0)
	if parseErr != nil {
		logrus.WithError(parseErr).Fatal("Could not parse elements.go")
	}

	// Initialise the AstInspectState
	collector := &astInspectState{}

	collector.handler = collector.defaultHandler
	collector.elemInfos = make(map[string]*elemInfo)

	// Populate collector.elemInfos
	ast.Inspect(parsedFile, func(n ast.Node) bool {
		if n != nil {
			collector.handler = collector.handler(n)
		}
		return true
	})

	// elemInfos and funcDefs are now complete and the template can be executed.
	var buf bytes.Buffer
	err := elemFuncsTemplate.Execute(&buf, struct {
		ElemInfos map[string]*elemInfo
		FuncDefs  []struct {
			Elem, Method, Attr string
			TemplateType       templateType
			TemplateArgs       []templateArg
		}
		TemplateTypes struct{ String, URL, Enum, Bool, GojaEnum, Int, Const templateType }
	}{
		collector.elemInfos,
		funcDefs,
		struct{ String, URL, Enum, Bool, GojaEnum, Int, Const templateType }{
			stringTemplate, urlTemplate, enumTemplate, boolTemplate, nullableEnumTemplate, intTemplate, constTemplate,
		},
	})
	if err != nil {
		logrus.WithError(err).Fatal("Unable to execute template")
	}

	src, err := format.Source(buf.Bytes())
	if err != nil {
		logrus.WithError(err).Fatal("format.Source on generated code failed")
	}

	f, err := os.Create("elements_gen.go") //nolint:forbidigo
	if err != nil {
		logrus.WithError(err).Fatal("Unable to create the file 'elements_gen.go'")
	}

	if _, err = f.Write(src); err != nil {
		logrus.WithError(err).Fatal("Unable to write to 'elements_gen.go'")
	}

	err = f.Close()
	if err != nil {
		logrus.WithError(err).Fatal("Unable to close 'elements_gen.go'")
	}
}

//nolint:gochecknoglobals
var elemFuncsTemplate = template.Must(template.New("").Funcs(template.FuncMap{
	"buildStruct": buildStruct,
	"returnType":  returnType,
}).Parse(`// generated by js/modules/k6/html/gen/gen_elements.go;  DO NOT EDIT
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

{{ $templateTypes := .TemplateTypes }}
{{ range $funcDef := .FuncDefs -}}

func (e {{$funcDef.Elem}}) {{$funcDef.Method}}() {{ returnType $funcDef.TemplateType }} {
{{- if eq $funcDef.TemplateType $templateTypes.Int }}
	return e.attrAsInt("{{ $funcDef.Attr }}", {{ index $funcDef.TemplateArgs 0 }})
{{- else if eq $funcDef.TemplateType $templateTypes.Enum }}
	attrVal := e.attrAsString("{{ $funcDef.Attr }}")
	switch attrVal {
	{{- range $optIdx, $optVal := $funcDef.TemplateArgs }}
	{{- if ne $optIdx 0 }}
	case "{{$optVal}}":
		return attrVal
	{{- end }}
	{{- end}}
	default:
		return "{{ index $funcDef.TemplateArgs 0 }}"
	}
{{- else if eq $funcDef.TemplateType $templateTypes.GojaEnum }}
	attrVal, exists := e.sel.sel.Attr("{{ $funcDef.Attr }}")
	if !exists {
		return goja.Undefined()
	}
	switch attrVal {
	{{- range $optVal := $funcDef.TemplateArgs }}
	case "{{$optVal}}":
		return e.sel.rt.ToValue(attrVal)
	{{- end}}
	default:
		return goja.Undefined()
	}
{{- else if eq $funcDef.TemplateType $templateTypes.Const }}
	return "{{ index $funcDef.TemplateArgs 0 }}"
{{- else if eq $funcDef.TemplateType $templateTypes.URL }}
	return e.attrAsURLString("{{ $funcDef.Attr }}", {{ index $funcDef.TemplateArgs 0 }})
{{- else if eq $funcDef.TemplateType $templateTypes.String }}
	return e.attrAsString("{{ $funcDef.Attr }}")
{{- else if eq $funcDef.TemplateType $templateTypes.Bool }}
	return e.attrIsPresent("{{ $funcDef.Attr }}")
{{- end}}
}
{{ end }}
`))

// generate the nested struct, either one or two levels of nesting,
// ie "BaseElement{elem}" or "ButtonElement{FormFieldElement{elem}})"
func buildStruct(ei elemInfo) string {
	if ei.PrtStructName == "Element" {
		return ei.StructName + "{elem}"
	}
	return ei.StructName + "{" + ei.PrtStructName + "{elem}}"
}

// Select the correct return type for one of the attribute accessor methods
func returnType(tt templateType) string {
	switch tt {
	case boolTemplate:
		return "bool"
	case intTemplate:
		return "int"
	case nullableEnumTemplate:
		return "goja.Value"
	default:
		return "string"
	}
}

// Default node handler functions for ast.Inspect. Return itself unless it's found a "const" or "struct" keyword
func (ce *astInspectState) defaultHandler(node ast.Node) nodeHandlerFunc {
	ce.elemName = ""
	switch node.(type) {
	case *ast.TypeSpec: // struct keyword
		return ce.elementStructHandler

	case *ast.ValueSpec: // const keyword
		return ce.elementTagNameHandler

	default:
		return ce.defaultHandler
	}
}

// Found a tagname constant. The code 'const AnchorTagName = "a"' will add an ElemInfo called "Anchor",
// like elemInfos["Anchor"] = ElemInfo{"", ""}
func (ce *astInspectState) elementTagNameHandler(node ast.Node) nodeHandlerFunc {
	switch x := node.(type) {
	case *ast.Ident:
		if strings.HasSuffix(x.Name, "TagName") {
			ce.elemName = strings.TrimSuffix(x.Name, "TagName")
			ce.elemInfos[ce.elemName] = &elemInfo{"", ""}
		}

		return ce.defaultHandler

	default:
		return ce.defaultHandler
	}
}

// A struct definition was found, keep the elem handler if it's for an Element struct
// Element structs nest the "Element" struct or an intermediate struct like "HrefElement",
// the name of the 'parent' struct is contained in the
// *ast.Ident node located a few nodes after the TypeSpec node containing struct keyword
// The nodes in between the ast.TypeSpec and ast.Ident are ignored
func (ce *astInspectState) elementStructHandler(node ast.Node) nodeHandlerFunc {
	switch x := node.(type) {
	case *ast.Ident:
		if !strings.HasSuffix(x.Name, "Element") {
			return ce.defaultHandler
		}

		if ce.elemName == "" {
			ce.elemName = strings.TrimSuffix(x.Name, "Element")
			// Ignore elements which don't have a tag name constant meaning no elemInfo
			// structure was created by the TagName handle.
			// It skips the Href, Media, FormField, Mod, TableSection or TableCell structs
			// as these structs are inherited by other elements and not created indepedently.
			if _, ok := ce.elemInfos[ce.elemName]; !ok {
				return ce.defaultHandler
			}

			ce.elemInfos[ce.elemName].StructName = x.Name
			return ce.elementStructHandler
		}

		ce.elemInfos[ce.elemName].PrtStructName = x.Name
		return ce.defaultHandler

	case *ast.StructType:
		return ce.elementStructHandler

	case *ast.FieldList:
		return ce.elementStructHandler

	case *ast.Field:
		return ce.elementStructHandler

	default:
		return ce.defaultHandler
	}
}
