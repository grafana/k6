// Package templates provides the templates used by the `k6 new` command
package templates

import (
	_ "embed"
	"fmt"
	"io"
	"text/template"
)

//go:embed minimal.js
var minimalTemplateContent string

//go:embed protocol.js
var protocolTemplateContent string

//go:embed browser.js
var browserTemplateContent string

// Constants for template types
const (
	MinimalTemplate  = "minimal"
	ProtocolTemplate = "protocol"
	BrowserTemplate  = "browser"
)

// TemplateManager manages the pre-parsed templates
type TemplateManager struct {
	minimalTemplate  *template.Template
	protocolTemplate *template.Template
	browserTemplate  *template.Template
}

// NewTemplateManager initializes a new TemplateManager with parsed templates
func NewTemplateManager() (*TemplateManager, error) {
	minimalTmpl, err := template.New(MinimalTemplate).Parse(minimalTemplateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse minimal template: %w", err)
	}

	protocolTmpl, err := template.New(ProtocolTemplate).Parse(protocolTemplateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse protocol template: %w", err)
	}

	browserTmpl, err := template.New(BrowserTemplate).Parse(browserTemplateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse browser template: %w", err)
	}

	return &TemplateManager{
		minimalTemplate:  minimalTmpl,
		protocolTemplate: protocolTmpl,
		browserTemplate:  browserTmpl,
	}, nil
}

// GetTemplate selects the appropriate template based on the type
func (tm *TemplateManager) GetTemplate(templateType string) (*template.Template, error) {
	switch templateType {
	case MinimalTemplate:
		return tm.minimalTemplate, nil
	case ProtocolTemplate:
		return tm.protocolTemplate, nil
	case BrowserTemplate:
		return tm.browserTemplate, nil
	default:
		return nil, fmt.Errorf("invalid template type: %s", templateType)
	}
}

// TemplateArgs represents arguments passed to templates
type TemplateArgs struct {
	ScriptName string
	ProjectID  string
}

// ExecuteTemplate applies the template with provided arguments and writes to the provided writer
func ExecuteTemplate(w io.Writer, tmpl *template.Template, args TemplateArgs) error {
	return tmpl.Execute(w, args)
}
