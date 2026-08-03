package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/lib/fsext"
)

const (
	typecheckTempDirPattern = "k6-types-"
	typecheckInPlaceDir     = ".k6"
	typecheckInPlaceMarker  = "tsconfig.generated"
	typeScriptTypesHeader   = "X-Typescript-Types"
	maxDeclarationSize      = 10 << 20
)

type typecheckCmd struct {
	gs *state.GlobalState

	checker      string
	configPath   string
	inPlace      bool
	generateOnly bool
	watch        bool

	httpClient *http.Client
	lookPath   func(string) (string, error)
	runChecker func(context.Context, checkerInvocation, *state.GlobalState) error
}

type checkerInvocation struct {
	path       string
	args       []string
	workingDir string
}

type typecheckProjectEntry struct {
	scriptPath string
	imports    []string
	localFiles []string
}

type typecheckProject struct {
	CompilerOptions typecheckCompilerOptions `json:"compilerOptions"`
	Files           []string                 `json:"files"`
}

type typecheckCompilerOptions struct {
	Target                     string              `json:"target"`
	Module                     string              `json:"module"`
	ModuleResolution           string              `json:"moduleResolution"`
	AllowJS                    bool                `json:"allowJs"`
	CheckJS                    bool                `json:"checkJs"`
	NoEmit                     bool                `json:"noEmit"`
	Strict                     bool                `json:"strict"`
	SkipLibCheck               bool                `json:"skipLibCheck"`
	AllowImportingTSExtensions bool                `json:"allowImportingTsExtensions"`
	Types                      []string            `json:"types"`
	TypeRoots                  []string            `json:"typeRoots,omitempty"`
	Paths                      map[string][]string `json:"paths,omitempty"`
}

func getCmdTypecheck(gs *state.GlobalState) *cobra.Command {
	typecheck := &typecheckCmd{
		gs:         gs,
		checker:    "auto",
		httpClient: &http.Client{Timeout: time.Minute},
		lookPath:   exec.LookPath,
		runChecker: runTypeScriptChecker,
	}

	cmd := &cobra.Command{
		Use:   "typecheck [file]",
		Short: "Type-check a k6 script",
		Long: "Resolve a k6 script's dependencies, generate a TypeScript project with type mappings " +
			"for remote and extension modules, and run tsgo or tsc against it.",
		Args:    exactArgsWithMsg(1, "arg should be a path to a JavaScript or TypeScript file"),
		Example: getExampleText(gs, `  {{.}} typecheck script.js`),
		RunE:    typecheck.run,
	}

	flags := cmd.Flags()
	flags.StringVar(&typecheck.checker, "checker", typecheck.checker,
		"TypeScript checker to run: auto, tsgo, tsc, or an executable path")
	flags.StringVar(&typecheck.configPath, "tsconfig", typecheck.configPath,
		"path for the generated TypeScript configuration")
	flags.BoolVar(&typecheck.inPlace, "in-place", false,
		"write tsconfig.json locally and declarations under .k6/types")
	flags.BoolVar(&typecheck.generateOnly, "generate-only", false,
		"generate the TypeScript configuration without running a checker")
	flags.BoolVar(&typecheck.watch, "watch", false,
		"watch the local import graph and continuously type-check changes")
	flags.SortFlags = false
	flags.AddFlagSet(runtimeOptionFlagSet(false))

	return cmd
}

func (c *typecheckCmd) run(cmd *cobra.Command, args []string) error {
	if c.generateOnly && c.watch {
		return errors.New("--watch cannot be used with --generate-only")
	}
	cwd, err := c.gs.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	projectDir, configPath, typesDir, err := c.outputLocations(cwd)
	if err != nil {
		return err
	}
	if err := c.ensureInPlaceConfigAvailable(configPath); err != nil {
		return err
	}
	localFiles, err := c.generateProject(cmd.Context(), cmd, args, cwd, configPath, typesDir)
	if err != nil {
		return err
	}
	if c.inPlace && c.configPath == "" {
		if err := writeInPlaceConfigMarker(c.gs.FS, cwd, configPath); err != nil {
			return err
		}
	}
	printToStdout(c.gs, fmt.Sprintf(
		"Generated TypeScript project: %s\nTypeScript configuration: %s\n",
		projectDir, configPath))

	if c.generateOnly {
		return nil
	}

	invocation, err := c.checkerInvocation(cwd, configPath)
	if err != nil {
		return err
	}
	if c.watch {
		invocation.args = append(invocation.args, "--watch")
		return c.watchProject(cmd.Context(), cmd, args, cwd, configPath, typesDir, invocation, localFiles)
	}

	return c.runChecker(cmd.Context(), invocation, c.gs)
}

func (c *typecheckCmd) generateProject(
	ctx context.Context, cmd *cobra.Command, args []string, cwd, configPath, typesDir string,
) ([]string, error) {
	return c.generateProjectForEntries(ctx, cmd, args, cwd, configPath, typesDir, false)
}

func (c *typecheckCmd) generateProjectForEntries(
	ctx context.Context, cmd *cobra.Command, entries []string, cwd, configPath, typesDir string,
	continueOnLoadError bool,
) ([]string, error) {
	importsSet := make(map[string]struct{})
	scriptPathsSet := make(map[string]struct{})
	localFilesSet := make(map[string]struct{})
	var scriptPaths []string

	addScriptPath := func(scriptPath string) {
		scriptPath = filepath.Clean(scriptPath)
		if _, exists := scriptPathsSet[scriptPath]; exists {
			return
		}
		scriptPathsSet[scriptPath] = struct{}{}
		scriptPaths = append(scriptPaths, scriptPath)
		localFilesSet[scriptPath] = struct{}{}
	}

	for _, entry := range entries {
		projectEntry, err := c.inspectProjectEntry(cmd, entry, cwd, continueOnLoadError)
		if err != nil {
			return nil, err
		}
		addScriptPath(projectEntry.scriptPath)
		for _, imported := range projectEntry.imports {
			importsSet[imported] = struct{}{}
		}
		for _, localFile := range projectEntry.localFiles {
			localFilesSet[localFile] = struct{}{}
		}
	}

	imports := make([]string, 0, len(importsSet))
	for imported := range importsSet {
		imports = append(imports, imported)
	}
	paths, warnings, err := c.resolveTypeMappings(ctx, imports, typesDir)
	if err != nil {
		return nil, err
	}
	for _, warning := range warnings {
		c.gs.Logger.Warn(warning)
	}

	project := newTypecheckProjectForFiles(scriptPaths, paths, typeRootCandidatesForFiles(cwd, scriptPaths))
	if err := writeTypecheckProject(c.gs.FS, configPath, project); err != nil {
		return nil, err
	}

	localFiles := make([]string, 0, len(localFilesSet))
	for localFile := range localFilesSet {
		localFiles = append(localFiles, localFile)
	}
	slices.Sort(localFiles)
	return localFiles, nil
}

func (c *typecheckCmd) inspectProjectEntry(
	cmd *cobra.Command, entry, cwd string, continueOnLoadError bool,
) (typecheckProjectEntry, error) {
	fallback := typecheckProjectEntry{scriptPath: absolutePath(cwd, entry)}
	// The k6 module loader does not yet expose a context-aware API for its own remote fetches.
	test, err := loadLocalTestWithoutRunner(c.gs, cmd, []string{entry}) //nolint:contextcheck
	if err != nil {
		var unsatisfiedErr binaryIsNotSatisfyingDependenciesError
		switch {
		case errors.As(err, &unsatisfiedErr):
			c.gs.Logger.Warnf("the running k6 binary does not provide all script dependencies: %v; "+
				"extension types can only be extracted from the binary that contains the extension", unsatisfiedErr)
		case !continueOnLoadError:
			return typecheckProjectEntry{}, err
		default:
			c.gs.Logger.Warnf("could not inspect k6 script %s for type mappings: %v", entry, err)
			return fallback, nil
		}
	}
	if test == nil {
		if continueOnLoadError {
			return fallback, nil
		}
		return typecheckProjectEntry{}, fmt.Errorf("load k6 script %s", entry)
	}

	scriptPath, err := scriptFilePath(test)
	if err != nil {
		if continueOnLoadError {
			c.gs.Logger.Warnf("could not add k6 script %s to the generated project: %v", entry, err)
			return fallback, nil
		}
		return typecheckProjectEntry{}, err
	}
	imports := test.Imports()
	return typecheckProjectEntry{
		scriptPath: scriptPath,
		imports:    imports,
		localFiles: localSourcePaths(scriptPath, imports),
	}, nil
}

func (c *typecheckCmd) ensureInPlaceConfigAvailable(configPath string) error {
	if !c.inPlace || c.configPath != "" {
		return nil
	}
	exists, err := fsext.Exists(c.gs.FS, configPath)
	if err != nil {
		return fmt.Errorf("inspect local TypeScript configuration: %w", err)
	}
	if exists {
		generated, markerErr := isMarkedInPlaceConfig(c.gs.FS, filepath.Dir(configPath), configPath)
		if markerErr != nil {
			return markerErr
		}
		if generated {
			return nil
		}
		return fmt.Errorf("refusing to overwrite existing TypeScript configuration %s; "+
			"pass --tsconfig explicitly to replace it", configPath)
	}
	return nil
}

func writeInPlaceConfigMarker(fs fsext.Fs, cwd, configPath string) error {
	markerPath := filepath.Join(cwd, typecheckInPlaceDir, typecheckInPlaceMarker)
	if err := fs.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		return fmt.Errorf("create generated TypeScript configuration marker directory: %w", err)
	}
	if err := fsext.WriteFile(fs, markerPath, []byte(filepath.Clean(configPath)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write generated TypeScript configuration marker: %w", err)
	}
	return nil
}

func isMarkedInPlaceConfig(fs fsext.Fs, cwd, configPath string) (bool, error) {
	markerPath := filepath.Join(cwd, typecheckInPlaceDir, typecheckInPlaceMarker)
	exists, err := fsext.Exists(fs, markerPath)
	if err != nil {
		return false, fmt.Errorf("inspect generated TypeScript configuration marker: %w", err)
	}
	if !exists {
		return false, nil
	}
	contents, err := fsext.ReadFile(fs, markerPath)
	if err != nil {
		return false, fmt.Errorf("read generated TypeScript configuration marker: %w", err)
	}
	return filepath.Clean(strings.TrimSpace(string(contents))) == filepath.Clean(configPath), nil
}

func (c *typecheckCmd) outputLocations(cwd string) (projectDir, configPath, typesDir string, err error) {
	if c.inPlace {
		projectDir = cwd
	} else {
		projectDir, err = fsext.TempDir(c.gs.FS, "", typecheckTempDirPattern)
		if err != nil {
			return "", "", "", fmt.Errorf("create temporary TypeScript project: %w", err)
		}
	}

	if c.configPath == "" {
		configPath = filepath.Join(projectDir, "tsconfig.json")
	} else {
		configPath = absolutePath(cwd, c.configPath)
	}
	if c.inPlace {
		typesDir = filepath.Join(cwd, typecheckInPlaceDir, "types")
	} else {
		typesDir = filepath.Join(projectDir, "types")
	}
	return projectDir, configPath, typesDir, nil
}

func absolutePath(cwd, name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Join(cwd, name)
}

func scriptFilePath(test *loadedTest) (string, error) {
	if test.source.URL.Scheme != "file" || test.source.URL.Path == "/-" {
		return "", errors.New("typecheck requires a local script file")
	}
	path, err := url.PathUnescape(test.source.URL.Path)
	if err != nil {
		return "", fmt.Errorf("decode script path: %w", err)
	}
	return filepath.FromSlash(path), nil
}

func newTypecheckProject(
	scriptPath string, paths map[string][]string, typeRoots []string,
) typecheckProject {
	return newTypecheckProjectForFiles([]string{scriptPath}, paths, typeRoots)
}

func newTypecheckProjectForFiles(
	scriptPaths []string, paths map[string][]string, typeRoots []string,
) typecheckProject {
	return typecheckProject{
		CompilerOptions: typecheckCompilerOptions{
			Target:                     "ES2022",
			Module:                     "ESNext",
			ModuleResolution:           "Bundler",
			AllowJS:                    true,
			CheckJS:                    true,
			NoEmit:                     true,
			Strict:                     true,
			SkipLibCheck:               false,
			AllowImportingTSExtensions: true,
			Types:                      []string{"k6"},
			TypeRoots:                  typeRoots,
			Paths:                      paths,
		},
		Files: scriptPaths,
	}
}

func typeRootCandidates(cwd, scriptPath string) []string {
	return typeRootCandidatesForFiles(cwd, []string{scriptPath})
}

func typeRootCandidatesForFiles(cwd string, scriptPaths []string) []string {
	seen := make(map[string]struct{})
	var result []string
	starts := make([]string, 0, len(scriptPaths)+1)
	for _, scriptPath := range scriptPaths {
		starts = append(starts, filepath.Dir(scriptPath))
	}
	starts = append(starts, cwd)
	for _, start := range starts {
		for directory := filepath.Clean(start); ; directory = filepath.Dir(directory) {
			candidate := filepath.Join(directory, "node_modules", "@types")
			if _, ok := seen[candidate]; !ok {
				seen[candidate] = struct{}{}
				result = append(result, candidate)
			}
			parent := filepath.Dir(directory)
			if parent == directory {
				break
			}
		}
	}
	return result
}

func writeTypecheckProject(fs fsext.Fs, filename string, project typecheckProject) error {
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("encode TypeScript configuration: %w", err)
	}
	data = append(data, '\n')

	if err := fs.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return fmt.Errorf("create TypeScript configuration directory: %w", err)
	}
	if err := fsext.WriteFile(fs, filename, data, 0o644); err != nil {
		return fmt.Errorf("write TypeScript configuration: %w", err)
	}
	return nil
}

func (c *typecheckCmd) resolveTypeMappings(
	ctx context.Context, imports []string, typesDir string,
) (map[string][]string, []string, error) {
	imports = slices.Clone(imports)
	slices.Sort(imports)

	paths := make(map[string][]string)
	var warnings []string
	for _, imported := range imports {
		switch {
		case strings.HasPrefix(imported, "https://"):
			declaration, found, err := c.resolveRemoteDeclaration(ctx, imported, typesDir)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("could not discover types for %s: %v", imported, err))
				continue
			}
			if !found {
				warnings = append(warnings, fmt.Sprintf(
					"no TypeScript type information found for %s; the module must provide TypeScript source, "+
						"advertise a declaration with X-TypeScript-Types, or publish a sibling declaration", imported))
				continue
			}
			paths[imported] = []string{declaration}
		case strings.HasPrefix(imported, "k6/x/"):
			declaration, found, err := c.resolveExtensionDeclaration(imported, typesDir)
			if err != nil {
				return nil, nil, err
			}
			if !found {
				warnings = append(warnings, fmt.Sprintf(
					"no TypeScript declaration found for extension %s; the extension must embed one", imported))
				continue
			}
			paths[imported] = []string{declaration}
		}
	}
	return paths, warnings, nil
}

func (c *typecheckCmd) resolveRemoteDeclaration(
	ctx context.Context, moduleSpecifier, typesDir string,
) (string, bool, error) {
	moduleURL, err := url.Parse(moduleSpecifier)
	if err != nil {
		return "", false, fmt.Errorf("parse module URL: %w", err)
	}
	if isTypeScriptSourceURL(moduleURL) {
		cachePath := canonicalRemoteTypeScriptSourcePath(typesDir, moduleURL)
		if exists, existsErr := fsext.Exists(c.gs.FS, cachePath); existsErr != nil {
			return "", false, fmt.Errorf("inspect cached TypeScript source %s: %w", cachePath, existsErr)
		} else if exists {
			return cachePath, true, nil
		}

		data, found, fetchErr := c.fetchTypeFile(ctx, moduleURL)
		if fetchErr != nil || !found {
			return "", false, fetchErr
		}
		return c.cacheRemoteTypes(moduleSpecifier, cachePath, data)
	}

	for _, candidate := range remoteDeclarationCandidates(typesDir, moduleURL) {
		if exists, err := fsext.Exists(c.gs.FS, candidate); err != nil {
			return "", false, fmt.Errorf("inspect cached declaration %s: %w", candidate, err)
		} else if exists {
			return candidate, true, nil
		}
	}

	typesURL, found, err := c.advertisedTypesURL(ctx, moduleURL)
	if err != nil {
		return "", false, err
	}
	if found {
		data, fetched, fetchErr := c.fetchTypeFile(ctx, &typesURL)
		if fetchErr != nil {
			return "", false, fetchErr
		}
		if fetched {
			return c.cacheRemoteDeclaration(moduleSpecifier, moduleURL, typesDir, data)
		}
	}

	siblingTypesURL := siblingDeclarationURL(moduleURL)
	if siblingTypesURL == nil {
		return "", false, nil
	}

	data, found, err := c.fetchTypeFile(ctx, siblingTypesURL)
	if err != nil || !found {
		return "", false, err
	}

	return c.cacheRemoteDeclaration(moduleSpecifier, moduleURL, typesDir, data)
}

func (c *typecheckCmd) cacheRemoteDeclaration(
	moduleSpecifier string, moduleURL *url.URL, typesDir string, data []byte,
) (string, bool, error) {
	cachePath := canonicalRemoteDeclarationPath(typesDir, moduleURL)
	return c.cacheRemoteTypes(moduleSpecifier, cachePath, data)
}

func (c *typecheckCmd) cacheRemoteTypes(
	moduleSpecifier, cachePath string, data []byte,
) (string, bool, error) {
	if err := c.gs.FS.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return "", false, fmt.Errorf("create remote types directory: %w", err)
	}
	if err := fsext.WriteFile(c.gs.FS, cachePath, data, 0o644); err != nil {
		return "", false, fmt.Errorf("cache types for %s: %w", moduleSpecifier, err)
	}
	return cachePath, true, nil
}

func remoteDeclarationCandidates(typesDir string, moduleURL *url.URL) []string {
	canonical := canonicalRemoteDeclarationPath(typesDir, moduleURL)
	legacyPath := safeURLPath(moduleURL.Path)
	legacyPath = declarationFilename(legacyPath)
	legacy := filepath.Join(typesDir, filepath.FromSlash(legacyPath))
	if canonical == legacy {
		return []string{canonical}
	}
	return []string{canonical, legacy}
}

func canonicalRemoteDeclarationPath(typesDir string, moduleURL *url.URL) string {
	host := safePathSegment(moduleURL.Host)
	modulePath := declarationFilename(safeURLPath(moduleURL.EscapedPath()))
	modulePath, err := url.PathUnescape(modulePath)
	if err != nil {
		modulePath = declarationFilename(safeURLPath(moduleURL.Path))
	}
	modulePath = safeURLPath(modulePath)
	if moduleURL.RawQuery != "" {
		hash := sha256.Sum256([]byte(moduleURL.RawQuery))
		const declarationSuffix = ".d.ts"
		modulePath = strings.TrimSuffix(modulePath, declarationSuffix) + "-" +
			hex.EncodeToString(hash[:6]) + declarationSuffix
	}
	return filepath.Join(typesDir, "remotes", host, filepath.FromSlash(modulePath))
}

func canonicalRemoteTypeScriptSourcePath(typesDir string, moduleURL *url.URL) string {
	host := safePathSegment(moduleURL.Host)
	modulePath := safeURLPath(moduleURL.EscapedPath())
	modulePath, err := url.PathUnescape(modulePath)
	if err != nil {
		modulePath = safeURLPath(moduleURL.Path)
	}
	modulePath = safeURLPath(modulePath)
	if moduleURL.RawQuery != "" {
		hash := sha256.Sum256([]byte(moduleURL.RawQuery))
		ext := filepath.Ext(modulePath)
		modulePath = strings.TrimSuffix(modulePath, ext) + "-" + hex.EncodeToString(hash[:6]) + ext
	}
	return filepath.Join(typesDir, "remotes", host, filepath.FromSlash(modulePath))
}

func isTypeScriptSourceURL(moduleURL *url.URL) bool {
	switch strings.ToLower(filepath.Ext(moduleURL.Path)) {
	case ".ts", ".mts", ".cts":
		return true
	default:
		return false
	}
}

func safeURLPath(value string) string {
	result := strings.TrimPrefix(pathpkg.Clean("/"+strings.TrimPrefix(value, "/")), "/")
	if result == "." || result == "" {
		return "index.js"
	}
	return result
}

func safePathSegment(value string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, value)
}

func declarationFilename(name string) string {
	ext := filepath.Ext(name)
	if ext == ".js" || ext == ".mjs" || ext == ".cjs" || ext == ".ts" {
		return strings.TrimSuffix(name, ext) + ".d.ts"
	}
	return name + ".d.ts"
}

func (c *typecheckCmd) advertisedTypesURL(ctx context.Context, moduleURL *url.URL) (url.URL, bool, error) {
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		typesURL, found, err := c.requestAdvertisedTypesURL(ctx, method, moduleURL)
		if err != nil {
			return url.URL{}, false, err
		}
		if found {
			return typesURL, true, nil
		}
	}
	return url.URL{}, false, nil
}

func (c *typecheckCmd) requestAdvertisedTypesURL(
	ctx context.Context, method string, moduleURL *url.URL,
) (url.URL, bool, error) {
	req, err := http.NewRequestWithContext(ctx, method, moduleURL.String(), nil)
	if err != nil {
		return url.URL{}, false, fmt.Errorf("create type metadata request: %w", err)
	}
	if method == http.MethodGet {
		req.Header.Set("Range", "bytes=0-0")
	}
	resp, err := c.httpClient.Do(req) //nolint:gosec // The script author supplied the remote module URL.
	if err != nil {
		return url.URL{}, false, fmt.Errorf("request type metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return url.URL{}, false, nil
	}
	header := strings.TrimSpace(resp.Header.Get(typeScriptTypesHeader))
	if header == "" {
		return url.URL{}, false, nil
	}
	reference, err := url.Parse(header)
	if err != nil {
		return url.URL{}, false, fmt.Errorf("parse X-TypeScript-Types header %q: %w", header, err)
	}
	return *resp.Request.URL.ResolveReference(reference), true, nil
}

func siblingDeclarationURL(moduleURL *url.URL) *url.URL {
	ext := strings.ToLower(filepath.Ext(moduleURL.Path))
	if ext != ".js" && ext != ".mjs" && ext != ".cjs" {
		return nil
	}
	result := *moduleURL
	result.Path = strings.TrimSuffix(result.Path, filepath.Ext(result.Path)) + ".d.ts"
	result.RawPath = ""
	return &result
}

func (c *typecheckCmd) fetchTypeFile(ctx context.Context, typesURL *url.URL) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, typesURL.String(), nil)
	if err != nil {
		return nil, false, fmt.Errorf("create type file request: %w", err)
	}
	resp, err := c.httpClient.Do(req) //nolint:gosec // The declaration URL belongs to the imported module.
	if err != nil {
		return nil, false, fmt.Errorf("download type file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, false, fmt.Errorf("download type file: server returned %s", resp.Status)
	}
	if resp.ContentLength > maxDeclarationSize {
		return nil, false, fmt.Errorf("type file exceeds %d bytes", maxDeclarationSize)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDeclarationSize+1))
	if err != nil {
		return nil, false, fmt.Errorf("read type file: %w", err)
	}
	if len(data) > maxDeclarationSize {
		return nil, false, fmt.Errorf("type file exceeds %d bytes", maxDeclarationSize)
	}
	return data, true, nil
}

func (c *typecheckCmd) resolveExtensionDeclaration(
	moduleSpecifier, typesDir string,
) (string, bool, error) {
	extension, registered := ext.Get(ext.JSExtension)[moduleSpecifier]
	version := ""
	if registered {
		version = strings.TrimPrefix(extension.Version, "v")
	}

	for _, candidate := range extensionDeclarationCandidates(typesDir, moduleSpecifier, version) {
		if exists, err := fsext.Exists(c.gs.FS, candidate); err != nil {
			return "", false, fmt.Errorf("inspect extension declaration %s: %w", candidate, err)
		} else if exists {
			return candidate, true, nil
		}
	}

	if !registered {
		return "", false, nil
	}
	return c.cacheProvidedExtensionDeclaration(moduleSpecifier, version, typesDir, extension.Module)
}

func (c *typecheckCmd) cacheProvidedExtensionDeclaration(
	moduleSpecifier, version, typesDir string, module any,
) (string, bool, error) {
	provider, ok := module.(modules.TypeScriptTypeProvider)
	if !ok {
		return "", false, nil
	}
	data := provider.TypeScriptTypes()
	if len(data) == 0 {
		return "", false, fmt.Errorf("extension %s returned an empty TypeScript declaration", moduleSpecifier)
	}

	target := extensionDeclarationCandidates(typesDir, moduleSpecifier, version)[0]
	if err := c.gs.FS.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", false, fmt.Errorf("create extension types directory: %w", err)
	}
	if err := fsext.WriteFile(c.gs.FS, target, data, 0o644); err != nil {
		return "", false, fmt.Errorf("cache declaration for extension %s: %w", moduleSpecifier, err)
	}
	return target, true, nil
}

func extensionDeclarationCandidates(typesDir, moduleSpecifier, version string) []string {
	name := strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(moduleSpecifier)
	base := filepath.Join(typesDir, "extensions", name)
	if version == "" {
		return []string{filepath.Join(base, "index.d.ts")}
	}
	return []string{
		filepath.Join(base, safePathSegment(version), "index.d.ts"),
		filepath.Join(base, "index.d.ts"),
	}
}

func (c *typecheckCmd) checkerInvocation(cwd, configPath string) (checkerInvocation, error) {
	checker, err := c.findChecker(cwd)
	if err != nil {
		return checkerInvocation{}, err
	}
	return checkerInvocation{
		path:       checker,
		args:       []string{"--project", configPath},
		workingDir: cwd,
	}, nil
}

func (c *typecheckCmd) watchProject(
	ctx context.Context, cmd *cobra.Command, args []string, cwd, configPath, typesDir string,
	invocation checkerInvocation, localFiles []string,
) error {
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	regenerate := make(chan struct{}, 1)
	watcher := &lspFileWatcher{fs: c.gs.FS}
	watcher.update(localFiles)
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		watcher.run(watchCtx, regenerate)
	}()

	refreshDone := make(chan struct{})
	go func() {
		defer close(refreshDone)
		for waitForTypeMappingRefresh(watchCtx, regenerate) {
			files, err := c.generateProject(watchCtx, cmd, args, cwd, configPath, typesDir)
			if err != nil {
				c.gs.Logger.Warnf("could not refresh k6 type mappings: %v", err)
				continue
			}
			watcher.update(files)
		}
	}()

	err := c.runChecker(watchCtx, invocation, c.gs)
	cancel()
	<-watcherDone
	<-refreshDone
	return err
}

func (c *typecheckCmd) findChecker(cwd string) (string, error) {
	if c.checker != "auto" {
		return c.findNamedChecker(cwd, c.checker)
	}
	for _, name := range []string{"tsgo", "tsc"} {
		checker, err := c.findNamedChecker(cwd, name)
		if err == nil {
			return checker, nil
		}
	}
	return "", errors.New("no TypeScript checker found; install @typescript/native-preview or typescript, " +
		"or pass --checker with an executable path")
}

func (c *typecheckCmd) findNamedChecker(cwd, name string) (string, error) {
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		if exists, err := fsext.Exists(c.gs.FS, absolutePath(cwd, name)); err != nil {
			return "", fmt.Errorf("inspect TypeScript checker %q: %w", name, err)
		} else if !exists {
			return "", fmt.Errorf("TypeScript checker %q does not exist", name)
		}
		return absolutePath(cwd, name), nil
	}

	for directory := filepath.Clean(cwd); ; directory = filepath.Dir(directory) {
		local := filepath.Join(directory, "node_modules", ".bin", name)
		localCandidates := []string{local}
		if runtime.GOOS == "windows" {
			localCandidates = append(localCandidates, local+".cmd")
		}
		for _, candidate := range localCandidates {
			if exists, err := fsext.Exists(c.gs.FS, candidate); err == nil && exists {
				return candidate, nil
			}
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
	}
	checker, err := c.lookPath(name)
	if err != nil {
		return "", fmt.Errorf("TypeScript checker %q was not found", name)
	}
	return checker, nil
}

func runTypeScriptChecker(ctx context.Context, invocation checkerInvocation, gs *state.GlobalState) error {
	command := exec.CommandContext(ctx, invocation.path, invocation.args...) //nolint:gosec
	command.Dir = invocation.workingDir
	command.Stdin = gs.Stdin
	command.Stdout = gs.Stdout
	command.Stderr = gs.Stderr
	command.Env = environmentList(gs.Env)

	err := command.Run()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return errext.WithExitCodeIfNone(errAlreadyReported, exitcodes.ExitCode(exitError.ExitCode())) //nolint:gosec
	}
	if err != nil {
		return fmt.Errorf("run TypeScript checker: %w", err)
	}
	return nil
}

func environmentList(environment map[string]string) []string {
	result := make([]string, 0, len(environment))
	for name, value := range environment {
		result = append(result, name+"="+value)
	}
	slices.Sort(result)
	return result
}
