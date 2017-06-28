package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strconv"
	"strings"
	"text/template"
)

type ElemInfo struct {
	ConstName     string
	TagName       string
	StructName    string
	PrtStructName string
}

type NodeHandler func(node ast.Node) NodeHandler

type CollectElements struct {
	handler   NodeHandler
	elemName  string
	elemInfos map[string]*ElemInfo
}

type FuncDef struct {
	ElemName   string
	ElemMethod string
	AttrMethod string
	AttrName   string
	ReturnType string
	ReturnOpts []string
}

var renameTestElems = map[string]string{
	"href":            "a",
	"mod":             "del",
	"tablecell":       "",
	"tableheadercell": "",
}

var funcDefs = []string{
	"Href Download string",
	"Href ReferrerPolicy string",
	"Href Rel string",
	"Href Href string",
	"Href Target string",
	"Href Type string",
	"Href AccessKey string",
	"Href HrefLang string",
	"Href Media string",
	"Href ToString=href string",

	"Base Href bool",
	"Base Target bool",

	"Button AccessKey string",
	"Button Autofocus bool",
	"Button Disabled bool",
	"Button Type enum=submit,button,menu,reset,menu",

	"Data Value string",

	"Embed Height string",
	"Embed Width string",
	"Embed Src string",
	"Embed Type string",

	"FieldSet Disabled bool",
	"FieldSet Name string",

	"Form Name string",
	"Form Target string",
	"Form Action string",
	"Form Enctype string",
	"Form Encoding=enctype string",
	"Form AcceptCharset=accept-charset string",
	"Form Autocomplete string",
	"Form NoValidate bool",

	"IFrame Allowfullscreen bool",
	"IFrame ReferrerPolicy string",
	"IFrame Height string",
	"IFrame Width string",
	"IFrame Name string",
	"IFrame Src string",

	"Image CurrentSrc=src string",
	"Image Sizes string",
	"Image Srcset string",
	"Image Alt string",
	"Image CrossOrigin enum=anonymous,use-credentials",
	"Image Height int",
	"Image Width int",
	"Image IsMap bool",
	"Image Name string",
	"Image Src string",
	"Image UseMap string",

	"Input Type enum=text,button,checkbox,color,date,datetime-local,email,file,hidden,image,month,number,password,radio,range,reset,search,submit,tel,time,url,week",
	"Input Disabled bool",
	"Input Autofocus bool",
	"Input Required bool",
	"Input Value string",
	"Image ReferrerPolicy string",

	"Input Checked bool",
	"Input DefaultChecked=checked bool",

	"Input Alt string",
	"Input Src string",
	"Input Height string",
	"Input Width string",

	"Input Accept string",
	"Input Autocomplete enum=off,on",
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
	"Keygen Type enum=keygen",

	"Label HtmlFor=for string",

	"Legend AccessKey string",
	"Legend Value string",

	"Li Value string",
	"Li Type enum=,1,a,A,i,I,disc,square,circle",

	"Link Crossorigin enum=anonymous,use-credentials",
	"Link ReferrerPolicy string",
	"Link Href string",
	"Link Hreflang string",
	"Link Media string",
	// The first value in enum lists gets used as the default. Putting "," at the start makes "" the default value for Rel instead of "alternate"
	"Link Rel enum=,alternate,author,dns-prefetch,help,icon,license,next,pingback,preconnect,prefetch,preload,prerender,prev,search,stylesheet",
	"Link Target string",
	"Link Type string",

	"Map Name string",

	"Meta Content string",
	"Meta HttpEquiv=http-equiv enum=content-type,default-style,refresh",
	"Meta Name enum=application-name,author,description,generator,keywords,viewport",

	"Meter Min int",
	"Meter Max int",
	"Meter High int",
	"Meter Low int",
	"Meter Optimum int",

	"Mod Cite string",
	"Mod DateTime string",

	"Object Data string",
	"Object Height string",
	"Object Name string",
	"Object Type string",
	"Object TabIndex int",
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
	"Output Type enum=output",

	"Param Name string",
	"Param Value string",

	"Pre Name string",
	"Pre Value string",

	"Quote Cite string",

	"Script CrossOrigin string",
	"Script Type string",
	"Script Src string",
	"Script HtmlFor=for string",
	"Script Charset string",
	"Script Async bool",
	"Script Defer bool",
	"Script NoModule bool",

	"Select Autofocus bool",
	"Select Disabled bool",
	"Select Multiple bool",
	"Select Name string",
	"Select Required bool",

	"Source KeySystem string",
	"Source Media string",
	"Source Sizes string",
	"Source Src string",
	"Source SrcSet string",
	"Source Type string",

	"Style Media string",
	"Style Type string",
	"Style Disabled bool",

	"Table Sortable bool",

	"TableCell ColSpan int=1",
	"TableCell RowSpan int=1",
	"TableCell Headers string",

	"TableHeaderCell Abbr string",
	"TableHeaderCell Scope string",
	"TableHeaderCell Sorted bool",

	"TextArea Type enum=textarea",
	"TextArea Value string",
	"TextArea DefaultValue=value string",
	"TextArea Placeholder string",
	"TextArea Rows int",
	"TextArea Cols int",
	"TextArea MaxLength int",
	"TextArea AccessKey string",
	"TextArea ReadOnly bool",
	"TextArea Required bool",
	"TextArea Autocomplete bool",
	"TextArea Autocapitalize enum=none,off,characters,words,sentences",
	"TextArea Wrap string",

	"Time DateTime string",

	"Title Text string",

	"UList Type string",
}

type TestDef struct {
	ElemHtmlName string
	ElemMethod   string
	AttrName     string
	AttrVal      string
	ReturnType   string
	ReturnOpts   []string
}

var collector = &CollectElements{}

func main() {
	fs := token.NewFileSet()
	parsedFile, parseErr := parser.ParseFile(fs, "elements.go", nil, 0)
	if parseErr != nil {
		log.Fatalf("warning: internal error: could not parse elements.go: %s", parseErr)
		return
	}

	collector.handler = collector.defaultHandler
	collector.elemInfos = make(map[string]*ElemInfo)

	ast.Inspect(parsedFile, func(n ast.Node) bool {
		if n != nil {
			collector.handler = collector.handler(n)
		}
		return true
	})

	f, err := os.Create("elements_gen.go")
	if err != nil {
		log.Println("warning: internal error: invalid Go generated:", err)
	}

	elemFuncsTemplate.Execute(f, struct {
		ElemInfos map[string]*ElemInfo
		FuncDefs  []string
	}{
		collector.elemInfos,
		funcDefs,
	})
	f.Close()

	f, err = os.Create("elements_gen_test.go")
	if err != nil {
		log.Println("warning: internal error: invalid Go generated:", err)
	}

	testFuncTemplate.Execute(f, struct {
		FuncDefs []string
	}{
		funcDefs,
	})
	f.Close()
}

var elemFuncsTemplate = template.Must(template.New("").Funcs(template.FuncMap{
	"buildStruct":  buildStruct,
	"buildFuncDef": collector.buildFuncDef,
}).Parse(`// go generate
// generated by js/modules/k6/html/gen/main.go directed by js/modules/k6/html/elements.go;  DO NOT EDIT
package html

import "github.com/dop251/goja"

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

{{ range $funcDefStr := .FuncDefs }} {{ $funcDef := buildFuncDef $funcDefStr }}
func (e {{$funcDef.ElemName}}) {{$funcDef.ElemMethod}}() {{ if ne $funcDef.ReturnType "enum" }} {{$funcDef.ReturnType}}{{else}} string {{end}} {
{{ if ne $funcDef.ReturnType "enum" }} return e.{{ $funcDef.AttrMethod }}("{{ $funcDef.AttrName }}"{{ if $funcDef.ReturnOpts }}, {{ index $funcDef.ReturnOpts 0 }}{{end}}){{ else }} attrVal := e.attrAsString("{{ $funcDef.AttrName }}")
	switch attrVal { {{ range $optVal := $funcDef.ReturnOpts }}
		case "{{$optVal}}": 
			return "{{$optVal}}"
		{{ end }}
	}
	return "{{ index $funcDef.ReturnOpts 0}}" {{ end }}
}
{{ end }}
`))

var testFuncTemplate = template.Must(template.New("").Funcs(template.FuncMap{
	"buildTestDef": collector.buildTestDef,
}).Parse(`// go generate
// generated by js/modules/k6/html/gen/main.go directed by js/modules/k6/html/elements.go;  DO NOT EDIT
package html

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/stretchr/testify/assert"
)

const testGenElems = ` + "`" + `<html><body>
{{- range $index, $testDefStr := .FuncDefs -}}
	{{ $def := buildTestDef $index $testDefStr -}}
	{{ if eq $def.ElemHtmlName "" -}}
	{{ else if eq $def.ReturnType "enum" -}}
	{{ range $optIdx, $optVal := $def.ReturnOpts -}}
		<{{$def.ElemHtmlName}} id="elem_{{$index}}_{{$optIdx}}" {{$def.AttrName}}="{{$optVal}}"> {{end}}
 	{{- else if eq $def.ReturnType "bool" -}}
	  <{{$def.ElemHtmlName}} id="elem_{{$index}}" {{$def.AttrName}}></{{ $def.ElemHtmlName }}>
	{{else -}} 
	  <{{$def.ElemHtmlName}} id="elem_{{$index}}" {{$def.AttrName}}="{{$def.AttrVal}}"></{{ $def.ElemHtmlName }}>
	{{end -}}
{{- end}}
</body></html>` + "`" + `

func TestGenElements(t *testing.T) {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	ctx := common.WithRuntime(context.Background(), rt)
	rt.Set("src", testGenElems)
	rt.Set("html", common.Bind(rt, &HTML{}, &ctx))
	// compileProtoElem()

	_, err := common.RunString(rt, "let doc = html.parseHTML(src)")

	assert.NoError(t, err)
	assert.IsType(t, Selection{}, rt.Get("doc").Export())
{{ range $index, $testDefStr := .FuncDefs }} 
{{ $def := buildTestDef $index $testDefStr }} 
	{{ if ne $def.ElemHtmlName "" -}}
		t.Run("{{$def.ElemHtmlName}}.{{$def.ElemMethod}}", func(t *testing.T) { 
	{{ if eq $def.ReturnType "enum" -}} 
		{{ range $optIdx, $optVal := $def.ReturnOpts -}}
			if v, err := common.RunString(rt, "doc.find(\"#elem_{{$index}}_{{$optIdx}}\").get(0).{{$def.ElemMethod}}()"); assert.NoError(t, err) {
					assert.Equal(t, "{{$optVal}}", v.Export()) 
			} 
		{{end -}}
	{{else -}} 
	  	if v, err := common.RunString(rt, "doc.find(\"#elem_{{$index}}\").get(0).{{$def.ElemMethod}}()"); assert.NoError(t, err) {
				assert.Equal(t, {{ if eq $def.ReturnType "bool" }}{{$def.AttrVal}} {{else if eq $def.ReturnType "string" }} "{{$def.AttrVal}}" {{else if eq $def.ReturnType "int"}} int64({{$def.AttrVal}}) {{end}}, v.Export()) 
			} 
	{{end -}}
})
{{ end }}
{{- end -}}
}
`))

func buildStruct(elemInfo ElemInfo) string {
	if elemInfo.PrtStructName == "Element" {
		return elemInfo.StructName + "{elem}"
	} else {
		return elemInfo.StructName + "{" + elemInfo.PrtStructName + "{elem}}"
	}
}

func (ce *CollectElements) buildFuncDef(funcDef string) FuncDef {
	parts := strings.Split(funcDef, " ")
	// parts[0] is the element struct name (without the Element suffix for brevity)
	// parts[1] is either:
	//   MethodName               The name of method added onto that struct and converted to lowercase thenn used as the argument to elem.attrAsString(...) or elem.AttrIsPresent(...)
	//   MethodName=attrname      The MethodName is added to the struct. The attrname is the argument for attrAsString or AttrIsPresent
	// parts[2] is the return type, either string or bool
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
		// "Button AccessKey string" => {"ButtonElement" "AccessKey", "attrIsString", "accesskey", "string"} => `func (e ButtonElement) AccessKey() string{ return e.attrAsString("accessKey") }``
		if returnOpts == "" {
			return FuncDef{elemName, elemMethod, "attrAsInt", attrName, returnType, []string{"0"}}
		} else {
			return FuncDef{elemName, elemMethod, "attrAsInt", attrName, returnType, []string{returnOpts}}
		}

	case "enum":
		return FuncDef{elemName, elemMethod, "attrAsInt", attrName, returnType, strings.Split(returnOpts, ",")}

	case "string":
		// "Button AccessKey string" => {"ButtonElement" "AccessKey", "attrIsString", "accesskey", "string"} => `func (e ButtonElement) AccessKey() string{ return e.attrAsString("accessKey") }``
		return FuncDef{elemName, elemMethod, "attrAsString", attrName, returnType, nil}

	case "bool":
		// "Button Autofocus bool" {"Button" "Autofocus", "attrIsPresent", "autofocus", "bool"} => `func (e ButtonElement) ToString() bool { return e.attrIsPresent("autofocus") }``
		return FuncDef{elemName, elemMethod, "attrIsPresent", attrName, returnType, nil}
	default:
		panic("Unknown return type in a funcDef: " + returnType)
	}
}

func (ce *CollectElements) buildTestDef(index int, testDef string) TestDef {
	parts := strings.Split(testDef, " ")

	elemHtmlName := strings.ToLower(parts[0])
	elemMethod := strings.ToLower(parts[1][0:1]) + parts[1][1:]
	attrName := strings.ToLower(parts[1])
	returnType := parts[2]
	returnOpts := ""

	if useElemName, ok := renameTestElems[elemHtmlName]; ok {
		elemHtmlName = useElemName
	} else if elemInfo, ok := ce.elemInfos[parts[0]]; ok {
		elemHtmlName = strings.Trim(elemInfo.TagName, "\"")
	}

	if eqPos := strings.Index(elemMethod, "="); eqPos != -1 {
		attrName = elemMethod[eqPos+1:]
		elemMethod = elemMethod[0:eqPos]
	}

	if eqPos := strings.Index(returnType, "="); eqPos != -1 {
		returnOpts = returnType[eqPos+1:]
		returnType = returnType[0:eqPos]
	}

	switch returnType {
	case "bool":
		return TestDef{elemHtmlName, elemMethod, attrName, "true", returnType, nil}
	case "string":
		return TestDef{elemHtmlName, elemMethod, attrName, "attrval_" + strconv.Itoa(index), returnType, nil}
	case "int":
		return TestDef{elemHtmlName, elemMethod, attrName, strconv.Itoa(index), returnType, nil}
	case "enum":
		return TestDef{elemHtmlName, elemMethod, attrName, "attrval_" + strconv.Itoa(index), returnType, strings.Split(returnOpts, ",")}
	default:
		panic("Unknown return type in a funcDef:" + returnType)

	}
}

// Node handler functions used in ast.Inspect to scrape TagName consts and the names of Element structs and their parent/nested struct

func (ce *CollectElements) defaultHandler(node ast.Node) NodeHandler {
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

func (ce *CollectElements) tagNameValueSpecHandler(node ast.Node) NodeHandler {
	switch x := node.(type) {
	case *ast.Ident:
		if strings.HasSuffix(x.Name, "TagName") {
			ce.elemName = strings.TrimSuffix(x.Name, "TagName")
			ce.elemInfos[ce.elemName] = &ElemInfo{x.Name, "", "", ""}
			return ce.tagNameValueSpecHandler
		}

		return ce.defaultHandler
	case *ast.BasicLit:
		if _, ok := ce.elemInfos[ce.elemName]; !ok {
			return ce.defaultHandler
		}

		ce.elemInfos[ce.elemName].TagName = x.Value
		return ce.defaultHandler

	default:
		return ce.defaultHandler
	}
}

func (ce *CollectElements) elemTypeSpecHandler(node ast.Node) NodeHandler {
	switch x := node.(type) {
	case *ast.Ident:
		if !strings.HasSuffix(x.Name, "Element") {
			return ce.defaultHandler
		}

		if ce.elemName == "" {
			ce.elemName = strings.TrimSuffix(x.Name, "Element")
			// Ignore HrefElement and MediaElement structs. They are subclassed by AnchorElement/AreaElement/VideoElement and do not have their own entry in ElemInfos
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
