// Package templates provides the templates used by the `k6 new` command
package templates

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
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

//go:embed rest.js
var restTemplateContent string

// Constants for template types
// Template names should not contain path separators to not to be confused with file paths
const (
	MinimalTemplate  = "minimal"
	ProtocolTemplate = "protocol"
	BrowserTemplate  = "browser"
	RestTemplate     = "rest"
)

// TemplateMetadata represents optional metadata for templates
type TemplateMetadata struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags"`
	Owner           string   `json:"owner"`
	DefaultFilename string   `json:"defaultFilename"`
}

// TemplateInfo combines template information with optional metadata
type TemplateInfo struct {
	Name      string
	Path      string
	Metadata  *TemplateMetadata
	IsBuiltIn bool
	Warning   string `json:"warning,omitempty"`
}

// TemplateArgs represents arguments passed to templates
type TemplateArgs struct {
	ScriptName string
	ProjectID  string
	Project    string // Alias for ProjectID for template compatibility
	Team       string
	Env        string
}

// TemplateManager manages the pre-parsed templates and template search paths
type TemplateManager struct {
	minimalTemplate  *template.Template
	protocolTemplate *template.Template
	browserTemplate  *template.Template
	restTemplate     *template.Template
	fs               fsext.Fs
	homeDir          string
}

// NewTemplateManager initializes a new TemplateManager with parsed templates
func NewTemplateManager(fs fsext.Fs, homeDir string) (*TemplateManager, error) {
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

	restTmpl, err := template.New(RestTemplate).Parse(restTemplateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rest template: %w", err)
	}

	return &TemplateManager{
		minimalTemplate:  minimalTmpl,
		protocolTemplate: protocolTmpl,
		browserTemplate:  browserTmpl,
		restTemplate:     restTmpl,
		fs:               fs,
		homeDir:          homeDir,
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
	case RestTemplate:
		return tm.restTemplate, nil
	}

	// Then check if it's a file path
	if isFilePath(tpl) {
		return tm.loadTemplateFromPath(tpl)
	}

	// Search for templates in the specified order:
	// 1. ./templates/<name>/script.js
	// 2. ~/.k6/templates/<name>/script.js
	// 3. Built-in templates (already checked above)

	// Check local templates directory
	localPath := filepath.Join("templates", tpl, "script.js")
	if exists, _ := fsext.Exists(tm.fs, localPath); exists {
		return tm.loadTemplateFromPath(localPath)
	}

	// Check user-global templates directory
	if tm.homeDir != "" {
		globalPath := filepath.Join(tm.homeDir, ".k6", "templates", tpl, "script.js")
		if exists, _ := fsext.Exists(tm.fs, globalPath); exists {
			return tm.loadTemplateFromPath(globalPath)
		}
	}

	// Check if there's a file with this name in current directory
	exists, err := fsext.Exists(tm.fs, fsext.JoinFilePath(".", tpl))
	if err == nil && exists {
		return nil, fmt.Errorf("invalid template type %q, did you mean ./%s?", tpl, tpl)
	}

	return nil, fmt.Errorf("invalid template type %q", tpl)
}

// loadTemplateFromPath loads a template from a file path
func (tm *TemplateManager) loadTemplateFromPath(tplPath string) (*template.Template, error) {
	// For absolute paths, use as-is. For relative paths, use directly without conversion
	// since we want to work with the filesystem as provided (which may be in-memory for tests)
	pathToUse := tplPath
	if !filepath.IsAbs(tplPath) {
		pathToUse = tplPath
	} else {
		// Only convert to absolute path if we're not dealing with a memory filesystem
		// Check if the path exists first with the filesystem
		if exists, _ := fsext.Exists(tm.fs, tplPath); !exists {
			// Try to get absolute path only if the relative path doesn't exist
			if absPath, err := filepath.Abs(tplPath); err == nil {
				pathToUse = absPath
			}
		}
	}

	// Read the template content using the provided filesystem
	content, err := fsext.ReadFile(tm.fs, pathToUse)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file %s: %w", tplPath, err)
	}

	tmpl, err := template.New(filepath.Base(pathToUse)).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template file %s: %w", tplPath, err)
	}

	return tmpl, nil
}

// isFilePath checks if the given string looks like a file path by detecting path separators or file extensions
// We assume that built-in template names don't contain path separators or file extensions
func isFilePath(path string) bool {
	// Check for path separators
	if strings.ContainsRune(path, filepath.Separator) || strings.ContainsRune(path, '/') {
		return true
	}

	// Check for file extensions (files typically have extensions like .js, .json, etc.)
	// Built-in template names like "minimal", "protocol", "browser", "rest" don't have extensions
	return strings.Contains(path, ".") && filepath.Ext(path) != ""
}

// parseTemplateMetadata attempts to parse a k6.template.json file from the given directory
func (tm *TemplateManager) parseTemplateMetadata(templateDir string) (*TemplateMetadata, error) {
	metadataPath := filepath.Join(templateDir, "k6.template.json")
	exists, err := fsext.Exists(tm.fs, metadataPath)
	if err != nil || !exists {
		return nil, nil // Not an error, metadata is optional
	}

	data, err := fsext.ReadFile(tm.fs, metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file %s: %w", metadataPath, err)
	}

	var metadata TemplateMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata file %s: %w", metadataPath, err)
	}

	return &metadata, nil
}

// ListTemplatesWithInfo returns a list of available templates with metadata
func (tm *TemplateManager) ListTemplatesWithInfo() ([]TemplateInfo, error) {
	templates := make(map[string]TemplateInfo)

	// Add built-in templates
	builtins := []string{MinimalTemplate, ProtocolTemplate, BrowserTemplate, RestTemplate}
	for _, name := range builtins {
		templates[name] = TemplateInfo{
			Name:      name,
			Path:      "",
			Metadata:  nil,
			IsBuiltIn: true,
			Warning:   "Missing k6.template.json",
		}
	}

	// Add local templates (./templates/)
	localTemplatesDir := "templates"
	if exists, _ := fsext.Exists(tm.fs, localTemplatesDir); exists {
		localTemplates, err := tm.scanTemplateDirectoryWithInfo(localTemplatesDir)
		if err == nil {
			for _, tmpl := range localTemplates {
				templates[tmpl.Name] = tmpl
			}
		}
	}

	// Add user-global templates (~/.k6/templates/)
	if tm.homeDir != "" {
		globalTemplatesDir := filepath.Join(tm.homeDir, ".k6", "templates")
		if exists, _ := fsext.Exists(tm.fs, globalTemplatesDir); exists {
			globalTemplates, err := tm.scanTemplateDirectoryWithInfo(globalTemplatesDir)
			if err == nil {
				for _, tmpl := range globalTemplates {
					templates[tmpl.Name] = tmpl
				}
			}
		}
	}

	// Convert map to sorted slice
	result := make([]TemplateInfo, 0, len(templates))
	for _, tmpl := range templates {
		result = append(result, tmpl)
	}

	// Sort by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// scanTemplateDirectoryWithInfo scans a directory for template folders and returns TemplateInfo
func (tm *TemplateManager) scanTemplateDirectoryWithInfo(dir string) ([]TemplateInfo, error) {
	var templates []TemplateInfo

	entries, err := fsext.ReadDir(tm.fs, dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			templateDir := filepath.Join(dir, entry.Name())
			scriptPath := filepath.Join(templateDir, "script.js")
			if exists, _ := fsext.Exists(tm.fs, scriptPath); exists {
				// Try to parse metadata
				metadata, err := tm.parseTemplateMetadata(templateDir)
				warning := ""

				if err != nil {
					// Log warning but continue processing
					fmt.Printf("Warning: failed to parse metadata for template %s: %v\n", entry.Name(), err)
					warning = "Invalid k6.template.json"
				} else if metadata == nil {
					warning = "Missing k6.template.json"
				}

				templates = append(templates, TemplateInfo{
					Name:      entry.Name(),
					Path:      templateDir,
					Metadata:  metadata,
					IsBuiltIn: false,
					Warning:   warning,
				})
			}
		}
	}

	return templates, nil
}

// ExecuteTemplate applies the template with provided arguments and writes to the provided writer
func ExecuteTemplate(w io.Writer, tmpl *template.Template, args TemplateArgs) error {
	return tmpl.Execute(w, args)
}

// ListTemplates returns a list of all available templates (backward compatibility)
func (tm *TemplateManager) ListTemplates() ([]string, error) {
	templatesWithInfo, err := tm.ListTemplatesWithInfo()
	if err != nil {
		return nil, err
	}

	result := make([]string, len(templatesWithInfo))
	for i, tmpl := range templatesWithInfo {
		result[i] = tmpl.Name
	}

	return result, nil
}

// CreateUserTemplate creates a new user template from an existing script
func (tm *TemplateManager) CreateUserTemplate(name, scriptPath string) error {
	if tm.homeDir == "" {
		return fmt.Errorf("home directory not available")
	}

	// Validate template name
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("template name cannot contain path separators")
	}

	// Read the source script
	content, err := fsext.ReadFile(tm.fs, scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script file %s: %w", scriptPath, err)
	}

	// Create template directory
	templateDir := filepath.Join(tm.homeDir, ".k6", "templates", name)
	if err := tm.fs.MkdirAll(templateDir, 0o755); err != nil {
		return fmt.Errorf("failed to create template directory %s: %w", templateDir, err)
	}

	// Write the template file
	templatePath := filepath.Join(templateDir, "script.js")
	if err := fsext.WriteFile(tm.fs, templatePath, content, 0o644); err != nil {
		return fmt.Errorf("failed to write template file %s: %w", templatePath, err)
	}

	// Create metadata file if it doesn't exist
	metadataPath := filepath.Join(templateDir, "k6.template.json")
	metadataExists, err := fsext.Exists(tm.fs, metadataPath)
	if err != nil {
		return fmt.Errorf("failed to check metadata file existence: %w", err)
	}

	if !metadataExists {
		// Extract the original filename for defaultFilename
		originalFilename := filepath.Base(scriptPath)

		// Create default metadata
		defaultMetadata := TemplateMetadata{
			Name:            name,
			Description:     "Describe your template here.",
			Tags:            []string{},
			Owner:           "",
			DefaultFilename: originalFilename,
		}

		metadataJSON, err := json.MarshalIndent(defaultMetadata, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal default metadata: %w", err)
		}

		if err := fsext.WriteFile(tm.fs, metadataPath, metadataJSON, 0o644); err != nil {
			return fmt.Errorf("failed to write metadata file %s: %w", metadataPath, err)
		}
	}

	return nil
}

// IsDirectoryBasedTemplate checks if a template is directory-based (not built-in or file-based)
func (tm *TemplateManager) IsDirectoryBasedTemplate(tpl string) bool {
	// Built-in templates are not directory-based
	switch tpl {
	case MinimalTemplate, ProtocolTemplate, BrowserTemplate, RestTemplate:
		return false
	}

	// File paths are not directory-based templates
	if isFilePath(tpl) {
		return false
	}

	// Check if this template exists as a directory
	return tm.getTemplateDirectoryPath(tpl) != ""
}

// getTemplateDirectoryPath returns the directory path for a directory-based template, or empty string if not found
func (tm *TemplateManager) getTemplateDirectoryPath(tpl string) string {
	// Check local templates directory
	localPath := filepath.Join("templates", tpl)
	if exists, _ := fsext.Exists(tm.fs, filepath.Join(localPath, "script.js")); exists {
		return localPath
	}

	// Check user-global templates directory
	if tm.homeDir != "" {
		globalPath := filepath.Join(tm.homeDir, ".k6", "templates", tpl)
		if exists, _ := fsext.Exists(tm.fs, filepath.Join(globalPath, "script.js")); exists {
			return globalPath
		}
	}

	return ""
}

// CopyTemplateFiles copies all files from a directory-based template to the current directory
// Excludes k6.template.json and hidden files, and handles file conflicts
func (tm *TemplateManager) CopyTemplateFiles(templateName string, args TemplateArgs, overwrite bool, stdout io.Writer) error {
	templateDir := tm.getTemplateDirectoryPath(templateName)
	if templateDir == "" {
		return fmt.Errorf("template %s is not a directory-based template", templateName)
	}

	return tm.copyDirectoryFiles(templateDir, ".", args, overwrite, stdout)
}

// copyDirectoryFiles recursively copies files from source to destination directory
func (tm *TemplateManager) copyDirectoryFiles(srcDir, dstDir string, args TemplateArgs, overwrite bool, stdout io.Writer) error {
	entries, err := fsext.ReadDir(tm.fs, srcDir)
	if err != nil {
		return fmt.Errorf("failed to read template directory %s: %w", srcDir, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		// Skip metadata file
		if entry.Name() == "k6.template.json" {
			continue
		}

		// Skip hidden files and directories (starting with .)
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		if entry.IsDir() {
			// Create destination directory if it doesn't exist
			if err := tm.fs.MkdirAll(dstPath, 0o755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dstPath, err)
			}

			// Recursively copy directory contents
			if err := tm.copyDirectoryFiles(srcPath, dstPath, args, overwrite, stdout); err != nil {
				return err
			}
		} else {
			// Handle file copying
			if err := tm.copyTemplateFile(srcPath, dstPath, args, overwrite, stdout); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyTemplateFile copies a single file from template, applying template processing and handling conflicts
func (tm *TemplateManager) copyTemplateFile(srcPath, dstPath string, args TemplateArgs, overwrite bool, stdout io.Writer) error {
	// Check if destination file already exists
	exists, err := fsext.Exists(tm.fs, dstPath)
	if err != nil {
		return fmt.Errorf("failed to check if file exists %s: %w", dstPath, err)
	}

	if exists && !overwrite {
		// Print warning and skip the file
		fmt.Fprintf(stdout, "Warning: File %s already exists, skipping.\n", dstPath)
		return nil
	}

	// Read the source file
	content, err := fsext.ReadFile(tm.fs, srcPath)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %w", srcPath, err)
	}

	// Get file info to preserve permissions
	fileInfo, err := tm.fs.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", srcPath, err)
	}

	// Check if the file is likely a template (contains template syntax)
	contentStr := string(content)
	if strings.Contains(contentStr, "{{") && strings.Contains(contentStr, "}}") {
		// Process as template
		tmpl, err := template.New(filepath.Base(srcPath)).Parse(contentStr)
		if err != nil {
			return fmt.Errorf("failed to parse template file %s: %w", srcPath, err)
		}

		// Execute template
		var buf strings.Builder
		if err := tmpl.Execute(&buf, args); err != nil {
			return fmt.Errorf("failed to execute template %s: %w", srcPath, err)
		}

		// Write processed content
		if err := fsext.WriteFile(tm.fs, dstPath, []byte(buf.String()), fileInfo.Mode()); err != nil {
			return fmt.Errorf("failed to write file %s: %w", dstPath, err)
		}
	} else {
		// Copy file as-is
		if err := fsext.WriteFile(tm.fs, dstPath, content, fileInfo.Mode()); err != nil {
			return fmt.Errorf("failed to write file %s: %w", dstPath, err)
		}
	}

	return nil
}
