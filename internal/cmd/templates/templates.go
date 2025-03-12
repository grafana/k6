// Package templates provides the templates used by the `k6 new` command
package templates

import (
	_ "embed"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/template"

	"go.k6.io/k6/lib/fsext"
)

//go:embed minimal.js
var minimalTemplateContent string

//go:embed protocol.js
var protocolTemplateContent string

//go:embed browser.js
var browserTemplateContent string

// Constants for template types
// Template names should not contain path separators to not to be confused with file paths
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
	fs               fsext.Fs
}

// NewTemplateManager initializes a new TemplateManager with parsed templates
func NewTemplateManager(fs fsext.Fs) (*TemplateManager, error) {
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
		fs:               fs,
	}, nil
}

// GetTemplate selects the appropriate template based on the type
func (tm *TemplateManager) GetTemplate(tpl string) (*template.Template, error) {
	// First check built-in templates
	switch tpl {
	case MinimalTemplate:
		return tm.minimalTemplate, nil
	case ProtocolTemplate:
		return tm.protocolTemplate, nil
	case BrowserTemplate:
		return tm.browserTemplate, nil
	}

	// Then check if it's a file path
	if isFilePath(tpl) {
		tplPath, err := filepath.Abs(tpl)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for template %s: %w", tpl, err)
		}

		// Read the template content using the provided filesystem
		content, err := fsext.ReadFile(tm.fs, tplPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read template file %s: %w", tpl, err)
		}

		tmpl, err := template.New(filepath.Base(tplPath)).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse template file %s: %w", tpl, err)
		}

		return tmpl, nil
	}

	// Check if there's a file with this name in current directory
	exists, err := fsext.Exists(tm.fs, fsext.JoinFilePath(".", tpl))
	if err == nil && exists {
		return nil, fmt.Errorf("invalid template type %q, did you mean ./%s?", tpl, tpl)
	}

	return nil, fmt.Errorf("invalid template type %q", tpl)
}

// isFilePath checks if the given string looks like a file path by detecting path separators
// We assume that built-in template names don't contain path separators
func isFilePath(path string) bool {
	return strings.ContainsRune(path, filepath.Separator) || strings.ContainsRune(path, '/')
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
