package cmd

import (
	_ "embed"
	"fmt"
	"io"
	"text/template"
)

// Embed templates
//
//go:embed minimal.js
var minimalTemplateContent string

//go:embed protocol.js
var protocolTemplateContent string

//go:embed browser.js
var browserTemplateContent string

// Pre-parse templates
var (
	MinimalScriptTemplate  = template.Must(template.New("minimal").Parse(minimalTemplateContent))
	ProtocolScriptTemplate = template.Must(template.New("protocol").Parse(protocolTemplateContent))
	BrowserScriptTemplate  = template.Must(template.New("browser").Parse(browserTemplateContent))
	DefaultNewScriptName   = "script.js"
)

// TemplateArgs represents arguments passed to templates
type TemplateArgs struct {
	ScriptName string
	ProjectID  string
}

// GetTemplate selects the appropriate template based on the type
func GetTemplate(templateType string) (*template.Template, error) {
	switch templateType {
	case "minimal":
		return MinimalScriptTemplate, nil
	case "protocol":
		return ProtocolScriptTemplate, nil
	case "browser":
		return BrowserScriptTemplate, nil
	default:
		return nil, fmt.Errorf("invalid template type: %s", templateType)
	}
}

// ExecuteTemplate applies the template with provided arguments and writes to the provided writer
func ExecuteTemplate(w io.Writer, tmpl *template.Template, args TemplateArgs) error {
	return tmpl.Execute(w, args)
}
