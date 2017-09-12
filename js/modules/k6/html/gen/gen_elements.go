package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"
	"text/template"
)

// The ast parser populates this struct using the <ElemName>TagName const and <ElemName>Element struct
type ElemInfo struct {
	ConstName     string
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

// Built from an entry in funcDefs by buildFuncDef(funcDef)
type FuncDef struct {
	ElemName   string
	ElemMethod string
	AttrMethod string
	AttrName   string
	ReturnType string
	ReturnBody string
	ReturnOpts []string
}

var funcDefs = []string{
	"Href Download string",
	"Href ReferrerPolicy enum=,no-referrer,no-referrer-when-downgrade,origin,origin-when-cross-origin,unsafe-url",
	"Href Rel string",
	"Href Href url-or-blank",
	"Href Target string",
	"Href Type string",
	"Href AccessKey string",
	"Href HrefLang string",
	"Href ToString=href url-or-blank",
	"Media Autoplay bool",
	"Media Controls bool",
	"Media Loop bool",
	"Media Muted bool",
	"Media Preload enum=auto,metadata,none",
	"Media Src url-or-blank",
	"Media CrossOrigin enum-nullable=anonymous,use-credentials",
	"Media CurrentSrc=src string",
	"Media DefaultMuted=muted bool",
	"Media MediaGroup string",
	"Base Href url-or-currpage",
	"Base Target string",
	"Button AccessKey string",
	"Button Autofocus bool",
	"Button Disabled bool",
	"Button TabIndex int",
	"Button Type enum=submit,button,menu,reset",
	"Data Value string",
	"Embed Height string",
	"Embed Width string",
	"Embed Src string",
	"Embed Type string",
	"FieldSet Disabled bool",
	"FieldSet Name string",
	"Form Action url-or-blank",
	"Form Name string",
	"Form Target string",
	"Form Enctype enum=application/x-www-form-urlencoded,multipart/form-data,text/plain",
	"Form Encoding=enctype enum=application/x-www-form-urlencoded,multipart/form-data,text/plain",
	"Form AcceptCharset=accept-charset string",
	"Form Autocomplete enum=on,off",
	"Form NoValidate bool",
	"IFrame Allowfullscreen bool",
	"IFrame ReferrerPolicy enum=,no-referrer,no-referrer-when-downgrade,origin,origin-when-cross-origin,unsafe-url",
	"IFrame Height string",
	"IFrame Width string",
	"IFrame Name string",
	"IFrame Src url-or-blank",
	"Image CurrentSrc=src url-or-blank",
	"Image Sizes string",
	"Image Srcset string",
	"Image Alt string",
	"Image CrossOrigin enum-nullable=anonymous,use-credentials",
	"Image Height int",
	"Image Width int",
	"Image IsMap bool",
	"Image Name string",
	"Image Src url-or-blank",
	"Image UseMap string",
	"Image ReferrerPolicy enum=,no-referrer,no-referrer-when-downgrade,origin,origin-when-cross-origin,unsafe-url",
	"Input Name string",
	"Input TabIndex int",
	"Input Type enum=text,button,checkbox,color,date,datetime-local,email,file,hidden,image,month,number,password,radio,range,reset,search,submit,tel,time,url,week",
	"Input Disabled bool",
	"Input Autofocus bool",
	"Input Required bool",
	"Input Value string",
	"Input Checked bool",
	"Input DefaultChecked=checked bool",
	"Input Alt string",
	"Input Src url-or-blank",
	"Input Height string",
	"Input Width string",
	"Input Accept string",
	"Input Autocomplete enum=on,off",
	"Input MaxLength int=-1",
	"Input Size int",
	"Input Pattern string",
	"Input Placeholder string",
	"Input Readonly bool",
	"Input Min string",
	"Input Max string",
	"Input DefaultValue=value string",
	"Input DirName string",
	"Input AccessKey string",
	"Input Multiple bool",
	"Input Step string",
	"Keygen Autofocus bool",
	"Keygen Challenge string",
	"Keygen Disabled bool",
	"Keygen Keytype enum=RSA,DSA,EC",
	"Keygen Name string",
	"Keygen Type const=keygen",
	"Label HtmlFor=for string",
	"Legend AccessKey string",
	"Li Value int=0",
	"Li Type enum=,1,a,A,i,I,disc,square,circle",
	"Link CrossOrigin enum-nullable=anonymous,use-credentials",
	"Link ReferrerPolicy enum=,no-referrer,no-referrer-when-downgrade,origin,origin-when-cross-origin,unsafe-url",
	"Link Href url-or-blank",
	"Link Hreflang string",
	"Link Media string",
	"Link Rel string",
	"Link Target string",
	"Link Type string",
	"Map Name string",
	"Meta Content string",
	"Meta Name string",
	"Meta HttpEquiv=http-equiv enum=content-type,default-style,refresh",
	"Meter Min int",
	"Meter Max int",
	"Meter High int",
	"Meter Low int",
	"Meter Optimum int",
	"Mod Cite string",
	"Mod Datetime string",
	"Object Data url-or-blank",
	"Object Height string",
	"Object Name string",
	"Object Type string",
	"Object TabIndex int=0",
	"Object TypeMustMatch bool",
	"Object UseMap string",
	"Object Width string",
	"OList Reversed bool",
	"OList Start int",
	"OList Type enum=1,a,A,i,I",
	"OptGroup Disabled bool",
	"OptGroup Label string",
	"Option DefaultSelected=selected bool",
	"Option Selected bool",
	"Output HtmlFor=for string",
	"Output Name string",
	"Output Type const=output",
	"Param Name string",
	"Param Value string",
	"Pre Name string",
	"Pre Value string",
	"Quote Cite string",
	"Script CrossOrigin string",
	"Script Type string",
	"Script Src url-or-blank",
	"Script Charset string",
	"Script Async bool",
	"Script Defer bool",
	"Script NoModule bool",
	"Select Autofocus bool",
	"Select Disabled bool",
	"Select Multiple bool",
	"Select Name string",
	"Select Required bool",
	"Select TabIndex int",
	"Source KeySystem string",
	"Source Media string",
	"Source Sizes string",
	"Source Src url-or-blank",
	"Source Srcset string",
	"Source Type string",
	"Style Media string",
	"Table Sortable bool",
	"TableCell ColSpan int=1",
	"TableCell RowSpan int=1",
	"TableCell Headers string",
	"TableHeaderCell Abbr string",
	"TableHeaderCell Scope enum=,row,col,colgroup,rowgroup",
	"TableHeaderCell Sorted bool",
	"TextArea Type const=textarea",
	"TextArea Value string",
	"TextArea DefaultValue=value string",
	"TextArea Placeholder string",
	"TextArea Rows int",
	"TextArea Cols int",
	"TextArea MaxLength int",
	"TextArea TabIndex int",
	"TextArea AccessKey string",
	"TextArea ReadOnly bool",
	"TextArea Required bool",
	"TextArea Autocomplete enum=on,off",
	"TextArea Autocapitalize enum=sentences,none,off,characters,words",
	"TextArea Wrap enum=soft,hard,off",
	"Time Datetime string",
	"Track Kind enum=subtitle,captions,descriptions,chapters,metadata",
	"Track Src url-or-blank",
	"Track Srclang string",
	"Track Label string",
	"Track Default bool",
	"UList Type string",
}

var collector = &ElemInfoCollectorState{}

func main() {
	fs := token.NewFileSet()
	parsedFile, parseErr := parser.ParseFile(fs, "elements.go", nil, 0)
	if parseErr != nil {
		log.Fatalf("warning: internal error: could not parse elements.go: %s", parseErr)
		return
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

	f, err := os.Create("elements_gen.go")
	if err != nil {
		log.Println("warning: Unable to create the file 'elements_gen.go'", err)
	}

	// Generate consts used in the enum items in funcDefs. Consts are needed to avoid warnings for the repeated use of strings
	var enumConsts = map[string]string{}
	for _, def := range funcDefs {
		funcDef := buildFuncDef(def)

		if !strings.HasPrefix(funcDef.ReturnBody, "enum") {
			continue
		}
		for _, opt := range funcDef.ReturnOpts {
			enumConsts[opt] = toConst(opt)
		}
	}

	//
	err = elemFuncsTemplate.Execute(f, struct {
		ElemInfos  map[string]*ElemInfo
		FuncDefs   []string
		EnumConsts map[string]string
	}{
		collector.elemInfos,
		funcDefs,
		enumConsts,
	})
	if err != nil {
		log.Println("error, unable to execute template:", err)
	}

	err = f.Close()
	if err != nil {
		log.Println("Unable to close the file 'elements_gen.go': ", err)
	}
}

var elemFuncsTemplate = template.Must(template.New("").Funcs(template.FuncMap{
	"buildStruct":  buildStruct,
	"buildFuncDef": buildFuncDef,
	"toConst":      toConst,
}).Parse(`// go generate
// generated by js/modules/k6/html/gen/main.go directed by js/modules/k6/html/elements.go;  DO NOT EDIT
package html

import "github.com/dop251/goja"

const (
	{{ range $constVal, $constName := .EnumConsts -}}
		{{$constName}} = "{{$constVal}}"
	{{ end -}}
)

func selToElement(sel Selection) goja.Value {
	if sel.sel.Length() == 0 {
		return goja.Undefined()
	}

	elem := Element{sel.sel.Nodes[0], &sel}

	switch elem.node.Data { {{ range $elemInfo := .ElemInfos }}
	case {{ $elemInfo.ConstName }}:
		return sel.rt.ToValue({{ buildStruct $elemInfo }})
{{ end }}
	default:
		return sel.rt.ToValue(elem)
	}
 }

{{ range $funcDefStr := .FuncDefs -}} 
{{ $funcDef := buildFuncDef $funcDefStr -}}
func (e {{$funcDef.ElemName}}) {{$funcDef.ElemMethod}}() {{$funcDef.ReturnType}} {
{{- if eq $funcDef.ReturnBody "int" }}
	return e.{{ $funcDef.AttrMethod }}("{{ $funcDef.AttrName }}", {{ index $funcDef.ReturnOpts 0 }})
{{- else if eq $funcDef.ReturnBody "enum" }}
	attrVal := e.attrAsString("{{ $funcDef.AttrName }}")
	switch attrVal { 
	{{- range $optIdx, $optVal := $funcDef.ReturnOpts }}
	{{- $optConst := toConst $optVal }}
		{{- if ne $optIdx 0 }}
	case {{$optConst}}:
		return attrVal
		{{- end }}
	{{- end}}
	default: 
		return {{ index $funcDef.ReturnOpts 0 | toConst }} 
	}
{{- else if eq $funcDef.ReturnBody "enum-nullable" }}
	attrVal, exists := e.sel.sel.Attr("{{ $funcDef.AttrName }}")
	if !exists {
		return goja.Undefined()
	}
	switch attrVal { 
	{{- range $optVal := $funcDef.ReturnOpts }}
	{{- $optConst := toConst $optVal }}
	case {{$optConst}}:
		return e.sel.rt.ToValue(attrVal)
	{{- end}}
	default:
		return goja.Undefined()
	}
{{- else if eq $funcDef.ReturnBody "const" }}
	return "{{ index $funcDef.ReturnOpts 0 }}"
{{- else if eq $funcDef.ReturnBody "url" }}
	return e.{{ $funcDef.AttrMethod }}("{{ $funcDef.AttrName }}", {{ index $funcDef.ReturnOpts 0 }})
{{- else }}
	return e.{{ $funcDef.AttrMethod }}("{{ $funcDef.AttrName }}")
{{- end}}
}
{{ end }}
`))

func constNameMapper(r rune) rune {
	if r == '-' || r == '/' {
		return '_'
	}
	return r
}

func toConst(optName string) string {
	if optName == "" {
		return "const_Blank"
	}
	return "const_" + strings.Map(constNameMapper, optName)
}

func buildStruct(elemInfo ElemInfo) string {
	if elemInfo.PrtStructName == "Element" {
		return elemInfo.StructName + "{elem}"
	} else {
		return elemInfo.StructName + "{" + elemInfo.PrtStructName + "{elem}}"
	}
}

func buildFuncDef(funcDef string) FuncDef {
	parts := strings.Split(funcDef, " ")
	// parts[0] is the element struct name (without the Element suffix)
	// parts[1] is either:
	//   MethodName               The name of method added onto that struct and converted to lowercase thenn used as the argument to elem.attrAsString(...) or elem.AttrIsPresent(...)
	//   MethodName=attrname      The MethodName is added to the struct. The attrname is the argument for attrAsString or AttrIsPresent
	// parts[2] is the return type, either string, const, bool, int, enum or enum-nullable.
	elemName := parts[0] + "Element"
	elemMethod := parts[1]
	attrName := strings.ToLower(parts[1])
	returnType := parts[2]
	returnOpts := ""

	if eqPos := strings.Index(parts[1], "="); eqPos != -1 {
		attrName = elemMethod[eqPos+1:]
		elemMethod = elemMethod[0:eqPos]
	}

	if eqPos := strings.Index(returnType, "="); eqPos != -1 {
		returnOpts = returnType[eqPos+1:]
		returnType = returnType[0:eqPos]
	}

	switch returnType {
	case "int":
		// The number following 'int=' is a default value used when the attribute is not defined.
		// "TableCell ColSpan int=1"
		// => {"TableCellElement" "ColSpan", "attrAsInt", "colspan", "int", "int", []string{"1"}}
		// => `func (e TableCellElement) ColSpan() int{ return e.attrAsInt("colspan", 1) }``
		if returnOpts == "" {
			return FuncDef{elemName, elemMethod, "attrAsInt", attrName, "int", returnType, []string{"0"}}
		} else {
			return FuncDef{elemName, elemMethod, "attrAsInt", attrName, "int", returnType, []string{returnOpts}}
		}

	case "enum":
		// "Button Type enum=submit,button,menu,reset"
		// The items in the comma separated list are used in a switch statement. The first value in the list is used as the default.
		return FuncDef{elemName, elemMethod, "", attrName, "string", returnType, strings.Split(returnOpts, ",")}

	case "enum-nullable":
		// Similar to the enum except the default is goja.Undefined()
		return FuncDef{elemName, elemMethod, "", attrName, "goja.Value", returnType, strings.Split(returnOpts, ",")}

	case "string":
		// "Button AccessKey string"
		// => {"ButtonElement" "AccessKey", "attrIsString", "accesskey", "string", "string", nil}
		// => `func (e ButtonElement) AccessKey() string{ return e.attrAsString("accessKey") }``
		return FuncDef{elemName, elemMethod, "attrAsString", attrName, returnType, returnType, nil}

	// This url function uses the current page URL as default when attribute is empty and empty string as default when the attribute is undefined
	case "url-or-blank":
		// "Href Href url-or-blank"
		// => {"HrefElement" "Href", "attrIsString", "accesskey", "string", "url", []string{"\"\""}}
		// => `func (e HrefElement) Href() string{ return e.attrAsUrlString("href", """) }``
		return FuncDef{elemName, elemMethod, "attrAsURLString", attrName, "string", "url", []string{"\"\""}}

	// This url function uses current page URL for empty and undefined attributes
	case "url-or-currpage":
		// "Base Href url-or-currpage"
		// => {"BaseElement" "Href", "attrIsString", "accesskey", "string", "url", []string{"e.sel.URL"}}
		// => `func (e HrefElement) Href() string{ return e.attrAsUrlString("href", e.sel.URL) }``
		return FuncDef{elemName, elemMethod, "attrAsURLString", attrName, "string", "url", []string{"e.sel.URL"}}

	case "const":
		// "Output Type const=output"
		// => {"OutputElement" "Type", "", "type", "string", "const", []{"output"}}
		// => `func (e OutputElement) Type() string{ return "output" }``
		return FuncDef{elemName, elemMethod, "", attrName, "string", returnType, []string{returnOpts}}

	case "bool":
		// "Button Autofocus bool"
		// => {"Button" "Autofocus", "attrIsPresent", "autofocus", "bool", "bool", nil}
		// => `func (e ButtonElement) ToString() bool { return e.attrIsPresent("autofocus") }``
		return FuncDef{elemName, elemMethod, "attrIsPresent", attrName, returnType, returnType, nil}

	default:
		panic("Unknown return type in a funcDef: " + returnType)
	}
}

// Node handler functions used in ast.Inspect to scrape TagName consts and the names of Element structs and their parent/nested struct

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

// Found a const, a Tagname suffix means it's for an Element.
func (ce *ElemInfoCollectorState) tagNameValueSpecHandler(node ast.Node) ElemInfoCollector {
	switch x := node.(type) {
	case *ast.Ident:
		if strings.HasSuffix(x.Name, "TagName") {
			ce.elemName = strings.TrimSuffix(x.Name, "TagName")
			ce.elemInfos[ce.elemName] = &ElemInfo{x.Name, "", ""}
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
			// Ignore elements which haven't had an elemInfo created by the TagName handler.
			// (Specifically intermediate structs like HrefElement, MediaElement & FormFieldElement)
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
