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
func (tm *TemplateManager) GetTemplate(templateType string) (*template.Template, error) {
	// First check built-in templates
	switch templateType {
	case MinimalTemplate:
		return tm.minimalTemplate, nil
	case ProtocolTemplate:
		return tm.protocolTemplate, nil
	case BrowserTemplate:
		return tm.browserTemplate, nil
	}

	// Then check if it's a file path
	if isFilePath(templateType) {
		content, err := fsext.ReadFile(tm.fs, templateType)
		if err != nil {
			return nil, fmt.Errorf("failed to read template file %s: %w", templateType, err)
		}

		tmpl, err := template.New(filepath.Base(templateType)).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse template file %s: %w", templateType, err)
		}
		return tmpl, nil
	}

	// Check if there's a file with this name in current directory
	exists, err := fsext.Exists(tm.fs, fsext.JoinFilePath(".", templateType))
	if err == nil && exists {
		return nil, fmt.Errorf("invalid template type %q, did you mean ./%s?", templateType, templateType)
	}

	return nil, fmt.Errorf("invalid template type %q", templateType)
}

// isFilePath checks if the given string looks like a file path
// It handles both POSIX-style paths (./, ../, /) and Windows-style paths (C:\, \\, .\)
func isFilePath(path string) bool {
	// Check POSIX-style paths
	if strings.HasPrefix(path, "./") ||
		strings.HasPrefix(path, "../") ||
		strings.HasPrefix(path, "/") {
		return true
	}

	// Check Windows-style paths
	if strings.HasPrefix(path, ".\\") ||
		strings.HasPrefix(path, "..\\") ||
		strings.HasPrefix(path, "\\") ||
		strings.HasPrefix(path, "\\\\") || // UNC paths
		(len(path) >= 2 && path[1] == ':') { // Drive letter paths like C:
		return true
	}

	return false
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
