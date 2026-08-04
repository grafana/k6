package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/lib/fsext"
)

const (
	lspTempDirPattern  = "k6-lsp-"
	lspConfigName      = "tsconfig.json"
	lspMaxMessageSize  = 64 << 20
	lspRegenerateDelay = 150 * time.Millisecond
	lspWatchInterval   = 500 * time.Millisecond
)

type lspCmd struct {
	gs *state.GlobalState

	server     string
	serverPath string
	inPlace    bool

	httpClient *http.Client
	lookPath   func(string) (string, error)
}

type lspServerKind string

const (
	lspServerTsgo     lspServerKind = "tsgo"
	lspServerTsserver lspServerKind = "tsserver"
)

type lspServerInvocation struct {
	kind       lspServerKind
	path       string
	args       []string
	workingDir string
}

type lspProjectLocations struct {
	projectDir    string
	configPath    string
	typesDir      string
	bridgePath    string
	bridgeCreated bool
	temporary     bool
}

type lspInput struct {
	path      string
	directory bool
}

type lspBridgeProject struct {
	Extends string `json:"extends"`
}

type lspFileStamp struct {
	exists           bool
	size             int64
	modTime          time.Time
	directoryEntries string
}

type lspFileWatcher struct {
	fs fsext.Fs

	mu    sync.Mutex
	files map[string]lspFileStamp
}

type lspServerReadiness struct {
	initialized chan struct{}
	once        sync.Once
}

func newLSPServerReadiness() *lspServerReadiness {
	return &lspServerReadiness{initialized: make(chan struct{})}
}

func (r *lspServerReadiness) markInitialized() {
	r.once.Do(func() { close(r.initialized) })
}

func (r *lspServerReadiness) isInitialized() bool {
	select {
	case <-r.initialized:
		return true
	default:
		return false
	}
}

func getCmdLSP(gs *state.GlobalState) *cobra.Command {
	lsp := &lspCmd{
		gs:         gs,
		server:     "auto",
		httpClient: &http.Client{Timeout: time.Minute},
		lookPath:   exec.LookPath,
	}

	cmd := &cobra.Command{
		Use:   "lsp [path]",
		Short: "Start a TypeScript language server for k6 scripts",
		Long: "Generate built-in declarations and type mappings for a k6 script or directory, start " +
			"tsgo's native LSP or the tsserver-backed TypeScript language server, and proxy LSP messages " +
			"over stdio.",
		Args:    exactArgsWithMsg(1, "arg should be a path to a JavaScript or TypeScript file or directory"),
		Example: getExampleText(gs, "  {{.}} lsp script.js\n  {{.}} lsp ."),
		RunE:    lsp.run,
	}

	flags := cmd.Flags()
	flags.StringVar(&lsp.server, "server", lsp.server,
		"language server backend: auto, tsgo, or tsserver")
	flags.StringVar(&lsp.serverPath, "server-path", lsp.serverPath,
		"override the language server executable path")
	flags.BoolVar(&lsp.inPlace, "in-place", false,
		"write tsconfig.json locally and declarations under .k6/types")
	flags.SortFlags = false
	flags.AddFlagSet(runtimeOptionFlagSet(false))

	return cmd
}

func (c *lspCmd) run(cmd *cobra.Command, args []string) error {
	cwd, err := c.gs.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	input, err := resolveLSPInput(c.gs.FS, cwd, args[0])
	if err != nil {
		return err
	}
	workspaceDir := cwd
	if input.directory {
		workspaceDir = input.path
	}

	invocation, err := c.serverInvocation(workspaceDir)
	if err != nil {
		return err
	}
	locations, err := c.projectLocations(workspaceDir, invocation.kind)
	if err != nil {
		return err
	}
	defer func() { c.cleanProject(locations) }()

	watchPaths, err := c.regenerateProject(cmd.Context(), cmd, input, workspaceDir, locations)
	if err != nil {
		return err
	}
	if c.inPlace {
		if err := writeInPlaceConfigMarker(c.gs.FS, workspaceDir, locations.configPath); err != nil {
			return err
		}
	}
	if locations.bridgePath != "" {
		locations.bridgeCreated = true
		if err := writeLSPBridge(c.gs.FS, locations.bridgePath, locations.configPath); err != nil {
			return err
		}
	}

	return c.proxy(cmd.Context(), cmd, input, invocation, locations, watchPaths)
}

func (c *lspCmd) serverInvocation(cwd string) (lspServerInvocation, error) {
	if c.server != "auto" && c.server != string(lspServerTsgo) && c.server != string(lspServerTsserver) {
		return lspServerInvocation{}, fmt.Errorf("invalid language server %q; use auto, tsgo, or tsserver", c.server)
	}
	if c.server == "auto" && c.serverPath != "" {
		return lspServerInvocation{}, errors.New("--server-path requires an explicit --server tsgo or --server tsserver")
	}

	finder := &typecheckCmd{gs: c.gs, lookPath: c.lookPath}
	find := func(kind lspServerKind) (string, error) {
		if c.serverPath != "" {
			return finder.findNamedChecker(cwd, c.serverPath)
		}
		name := "tsgo"
		if kind == lspServerTsserver {
			name = "typescript-language-server"
		}
		return finder.findNamedChecker(cwd, name)
	}

	selected := lspServerKind(c.server)
	if c.server == "auto" {
		selected = lspServerTsgo
		path, findErr := find(selected)
		if findErr == nil {
			return newLSPServerInvocation(selected, path, cwd), nil
		}
		selected = lspServerTsserver
		path, findErr = find(selected)
		if findErr == nil {
			return newLSPServerInvocation(selected, path, cwd), nil
		}
		return lspServerInvocation{}, errors.New("no TypeScript language server found; install " +
			"@typescript/native-preview or typescript-language-server with typescript")
	}

	path, err := find(selected)
	if err != nil {
		if selected == lspServerTsserver {
			return lspServerInvocation{}, fmt.Errorf("find tsserver LSP adapter: %w", err)
		}
		return lspServerInvocation{}, fmt.Errorf("find tsgo language server: %w", err)
	}
	return newLSPServerInvocation(selected, path, cwd), nil
}

func newLSPServerInvocation(kind lspServerKind, path, cwd string) lspServerInvocation {
	args := []string{"--lsp", "--stdio"}
	if kind == lspServerTsserver {
		args = []string{"--stdio"}
	}
	return lspServerInvocation{kind: kind, path: path, args: args, workingDir: cwd}
}

func (c *lspCmd) projectLocations(cwd string, kind lspServerKind) (lspProjectLocations, error) {
	if c.inPlace {
		return c.inPlaceProjectLocations(cwd)
	}

	locations := lspProjectLocations{}
	if kind == lspServerTsserver {
		locations.bridgePath = filepath.Join(cwd, lspConfigName)
	} else {
		locations.bridgePath = filepath.Join(cwd, fmt.Sprintf("tsconfig.k6.%d.json", time.Now().UnixNano()))
	}
	exists, err := fsext.Exists(c.gs.FS, locations.bridgePath)
	if err != nil {
		return lspProjectLocations{}, fmt.Errorf("inspect generated TypeScript configuration bridge: %w", err)
	}
	if exists {
		if kind == lspServerTsserver {
			return lspProjectLocations{}, fmt.Errorf(
				"the tsserver backend requires a temporary %s but that file already exists; use --server tsgo",
				locations.bridgePath)
		}
		return lspProjectLocations{}, fmt.Errorf("generated TypeScript configuration bridge already exists: %s",
			locations.bridgePath)
	}

	projectDir, err := fsext.TempDir(c.gs.FS, "", lspTempDirPattern)
	if err != nil {
		return lspProjectLocations{}, fmt.Errorf("create temporary LSP project: %w", err)
	}
	locations.projectDir = projectDir
	locations.temporary = true
	locations.configPath = filepath.Join(locations.projectDir, lspConfigName)
	locations.typesDir = filepath.Join(locations.projectDir, "types")

	return locations, nil
}

func (c *lspCmd) inPlaceProjectLocations(cwd string) (lspProjectLocations, error) {
	locations := lspProjectLocations{
		projectDir: cwd,
		configPath: filepath.Join(cwd, lspConfigName),
		typesDir:   filepath.Join(cwd, typecheckInPlaceDir, "types"),
	}
	exists, err := fsext.Exists(c.gs.FS, locations.configPath)
	if err != nil {
		return lspProjectLocations{}, fmt.Errorf("inspect local TypeScript configuration: %w", err)
	}
	if !exists {
		return locations, nil
	}
	generated, err := isMarkedInPlaceConfig(c.gs.FS, cwd, locations.configPath)
	if err != nil {
		return lspProjectLocations{}, err
	}
	if generated {
		return locations, nil
	}
	return lspProjectLocations{}, fmt.Errorf(
		"refusing to overwrite existing TypeScript configuration %s", locations.configPath)
}

func (c *lspCmd) regenerateProject(
	ctx context.Context, cmd *cobra.Command, input lspInput, cwd string, locations lspProjectLocations,
) ([]string, error) {
	generator := &typecheckCmd{
		gs:         c.gs,
		httpClient: c.httpClient,
	}
	if !input.directory {
		return generator.generateProject(
			ctx, cmd, []string{input.path}, cwd, locations.configPath, locations.typesDir)
	}

	entries, directories, err := discoverLSPScripts(c.gs.FS, input.path)
	if err != nil {
		return nil, err
	}
	localFiles, err := generator.generateProjectForEntries(
		ctx, cmd, entries, cwd, locations.configPath, locations.typesDir, true)
	if err != nil {
		return nil, err
	}
	return mergeLSPWatchPaths(localFiles, directories), nil
}

func writeLSPBridge(fs fsext.Fs, bridgePath, configPath string) error {
	data, err := json.MarshalIndent(lspBridgeProject{Extends: configPath}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode TypeScript configuration bridge: %w", err)
	}
	data = append(data, '\n')
	if err := fsext.WriteFile(fs, bridgePath, data, 0o644); err != nil {
		return fmt.Errorf("write TypeScript configuration bridge: %w", err)
	}
	return nil
}

func (c *lspCmd) cleanProject(locations lspProjectLocations) {
	if locations.bridgeCreated {
		if err := c.gs.FS.Remove(locations.bridgePath); err != nil {
			c.gs.Logger.Warnf("could not remove generated TypeScript configuration bridge: %v", err)
		}
	}
	if locations.temporary {
		if err := c.gs.FS.RemoveAll(locations.projectDir); err != nil {
			c.gs.Logger.Warnf("could not remove temporary LSP project: %v", err)
		}
	}
}

func (c *lspCmd) proxy(
	ctx context.Context, cmd *cobra.Command, input lspInput,
	invocation lspServerInvocation, locations lspProjectLocations, watchPaths []string,
) error {
	serverCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	command, serverIn, serverOut, err := c.startServer(serverCtx, invocation)
	if err != nil {
		return err
	}

	writer := &lockedLSPWriter{writer: serverIn}
	readiness := newLSPServerReadiness()
	regenerate := make(chan struct{}, 1)
	watcher := &lspFileWatcher{fs: c.gs.FS}
	watcher.update(watchPaths)
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		watcher.run(serverCtx, regenerate)
	}()
	regenerationDone := make(chan struct{})
	go func() {
		defer close(regenerationDone)
		c.regenerationLoop(
			serverCtx, cmd, input, invocation, locations, writer, watcher, readiness, regenerate)
	}()

	proxyDone := make(chan error, 1)
	go func() {
		configName := filepath.Base(locations.configPath)
		if locations.bridgePath != "" {
			configName = filepath.Base(locations.bridgePath)
		}
		proxyDone <- proxyLSPClient(c.gs.Stdin, writer, configName, readiness, regenerate)
	}()
	outputDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(c.gs.Stdout.Writer, serverOut)
		outputDone <- copyErr
	}()
	waitDone := make(chan error, 1)
	go func() { waitDone <- command.Wait() }()

	select {
	case proxyErr := <-proxyDone:
		_ = serverIn.Close()
		if proxyErr != nil {
			cancel()
			<-waitDone
			<-regenerationDone
			<-watcherDone
			<-outputDone
			return proxyErr
		}
		waitErr := <-waitDone
		cancel()
		<-regenerationDone
		<-watcherDone
		if copyErr := <-outputDone; copyErr != nil {
			return fmt.Errorf("proxy language server stdout: %w", copyErr)
		}
		if waitErr != nil {
			return fmt.Errorf("TypeScript language server exited: %w", waitErr)
		}
		return nil
	case waitErr := <-waitDone:
		cancel()
		<-regenerationDone
		<-watcherDone
		copyErr := <-outputDone
		if waitErr != nil {
			return fmt.Errorf("TypeScript language server exited: %w", waitErr)
		}
		if copyErr != nil {
			return fmt.Errorf("proxy language server stdout: %w", copyErr)
		}
		return nil
	}
}

func (c *lspCmd) startServer(
	ctx context.Context, invocation lspServerInvocation,
) (*exec.Cmd, io.WriteCloser, io.ReadCloser, error) {
	command := exec.CommandContext(ctx, invocation.path, invocation.args...) //nolint:gosec
	command.Dir = invocation.workingDir
	command.Stderr = c.gs.Stderr
	command.Env = environmentList(c.gs.Env)

	serverIn, err := command.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open language server stdin: %w", err)
	}
	serverOut, err := command.StdoutPipe()
	if err != nil {
		_ = serverIn.Close()
		return nil, nil, nil, fmt.Errorf("open language server stdout: %w", err)
	}
	if err := command.Start(); err != nil {
		_ = serverIn.Close()
		_ = serverOut.Close()
		return nil, nil, nil, fmt.Errorf("start TypeScript language server: %w", err)
	}
	return command, serverIn, serverOut, nil
}

func (c *lspCmd) regenerationLoop(
	ctx context.Context, cmd *cobra.Command, input lspInput,
	invocation lspServerInvocation, locations lspProjectLocations,
	writer *lockedLSPWriter, watcher *lspFileWatcher, readiness *lspServerReadiness,
	regenerate <-chan struct{},
) {
	for {
		if !waitForTypeMappingRefresh(ctx, regenerate) {
			return
		}
		watchPaths, err := c.regenerateProject(ctx, cmd, input, invocation.workingDir, locations)
		if err != nil {
			c.gs.Logger.Warnf("could not refresh k6 type mappings: %v", err)
			continue
		}
		watcher.update(watchPaths)
		if err := notifyLSPWatchedFiles(writer, readiness, locations); err != nil {
			c.gs.Logger.Warnf("could not notify TypeScript language server about refreshed mappings: %v", err)
		}
	}
}

func waitForTypeMappingRefresh(ctx context.Context, regenerate <-chan struct{}) bool {
	select {
	case <-ctx.Done():
		return false
	case <-regenerate:
	}

	timer := time.NewTimer(lspRegenerateDelay)
	for {
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return false
		case <-regenerate:
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(lspRegenerateDelay)
		case <-timer.C:
			return true
		}
	}
}

func localSourcePaths(scriptPath string, imports []string) []string {
	paths := []string{filepath.Clean(scriptPath)}
	seen := map[string]struct{}{filepath.Clean(scriptPath): {}}
	for _, imported := range imports {
		moduleURL, err := url.Parse(imported)
		if err != nil || moduleURL.Scheme != "file" {
			continue
		}
		path, err := url.PathUnescape(moduleURL.Path)
		if err != nil {
			continue
		}
		path = filepath.Clean(filepath.FromSlash(path))
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	slices.Sort(paths[1:])
	return paths
}

func validateLSPEntryPath(cwd, scriptPath string) error {
	relative, err := filepath.Rel(cwd, scriptPath)
	if err != nil {
		return fmt.Errorf("resolve script relative to LSP workspace: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("script %s is outside the LSP workspace %s; run k6 lsp from a parent directory",
			scriptPath, cwd)
	}
	return nil
}

func resolveLSPInput(fileSystem fsext.Fs, cwd, value string) (lspInput, error) {
	inputPath := absolutePath(cwd, value)
	info, err := fileSystem.Stat(inputPath)
	if err != nil {
		return lspInput{}, fmt.Errorf("inspect LSP input %s: %w", inputPath, err)
	}
	if info.IsDir() {
		return lspInput{path: inputPath, directory: true}, nil
	}
	if !info.Mode().IsRegular() {
		return lspInput{}, fmt.Errorf("LSP input %s is not a regular file or directory", inputPath)
	}
	if err := validateLSPEntryPath(cwd, inputPath); err != nil {
		return lspInput{}, err
	}
	return lspInput{path: inputPath}, nil
}

func discoverLSPScripts(fileSystem fsext.Fs, root string) ([]string, []string, error) {
	var scripts []string
	var directories []string
	err := fsext.Walk(fileSystem, root, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if path != root && isIgnoredLSPDirectory(info.Name()) {
				return filepath.SkipDir
			}
			directories = append(directories, filepath.Clean(path))
			return nil
		}
		if isLSPSourceFile(info.Name()) {
			scripts = append(scripts, filepath.Clean(path))
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("discover k6 scripts under %s: %w", root, err)
	}
	if len(scripts) == 0 {
		return nil, nil, fmt.Errorf("no JavaScript or TypeScript files found under %s", root)
	}
	slices.Sort(scripts)
	slices.Sort(directories)
	return scripts, directories, nil
}

func isIgnoredLSPDirectory(name string) bool {
	switch name {
	case ".git", ".k6", "node_modules":
		return true
	default:
		return false
	}
}

func isLSPSourceFile(name string) bool {
	name = strings.ToLower(name)
	if strings.HasSuffix(name, ".d.ts") || strings.HasSuffix(name, ".d.mts") ||
		strings.HasSuffix(name, ".d.cts") {
		return false
	}
	switch filepath.Ext(name) {
	case ".js", ".mjs", ".cjs", ".ts", ".mts", ".cts":
		return true
	default:
		return false
	}
}

func mergeLSPWatchPaths(pathGroups ...[]string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, paths := range pathGroups {
		for _, path := range paths {
			path = filepath.Clean(path)
			if _, exists := seen[path]; exists {
				continue
			}
			seen[path] = struct{}{}
			result = append(result, path)
		}
	}
	slices.Sort(result)
	return result
}

func (w *lspFileWatcher) update(paths []string) {
	files := make(map[string]lspFileStamp, len(paths))
	for _, path := range paths {
		files[path] = w.stamp(path)
	}
	w.mu.Lock()
	w.files = files
	w.mu.Unlock()
}

func (w *lspFileWatcher) run(ctx context.Context, regenerate chan<- struct{}) {
	ticker := time.NewTicker(lspWatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !w.changed() {
				continue
			}
			select {
			case regenerate <- struct{}{}:
			default:
			}
		}
	}
}

func (w *lspFileWatcher) changed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	changed := false
	for path, previous := range w.files {
		current := w.stamp(path)
		if current != previous {
			w.files[path] = current
			changed = true
		}
	}
	return changed
}

func (w *lspFileWatcher) stamp(path string) lspFileStamp {
	info, err := w.fs.Stat(path)
	if err != nil {
		return lspFileStamp{}
	}
	stamp := lspFileStamp{exists: true, size: info.Size(), modTime: info.ModTime()}
	if !info.IsDir() {
		return stamp
	}

	directory, err := w.fs.Open(path)
	if err != nil {
		return stamp
	}
	entries, readErr := directory.Readdir(-1)
	_ = directory.Close()
	if readErr != nil {
		return stamp
	}
	entryNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		entryNames = append(entryNames, entry.Name()+":"+entry.Mode().String())
	}
	slices.Sort(entryNames)
	stamp.directoryEntries = strings.Join(entryNames, "\x00")
	return stamp
}

type lockedLSPWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *lockedLSPWriter) writeJSON(message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode LSP message: %w", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return writeLSPMessage(w.writer, body)
}

func (w *lockedLSPWriter) writeBody(body []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return writeLSPMessage(w.writer, body)
}

func proxyLSPClient(
	client io.Reader, server *lockedLSPWriter, customConfigName string,
	readiness *lspServerReadiness, regenerate chan<- struct{},
) error {
	reader := bufio.NewReader(client)
	for {
		body, err := readLSPMessage(reader)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read LSP client message: %w", err)
		}
		method := lspMessageMethod(body)
		if method == "workspace/didChangeWatchedFiles" && !readiness.isInitialized() {
			requestTypeMappingRegeneration(regenerate)
			continue
		}
		if err := server.writeBody(body); err != nil {
			return fmt.Errorf("proxy LSP client message: %w", err)
		}
		switch method {
		case "initialized", "workspace/didChangeConfiguration":
			if err := server.writeJSON(lspConfigurationNotification(customConfigName)); err != nil {
				return fmt.Errorf("configure TypeScript language server: %w", err)
			}
		}
		if method == "initialized" {
			// Mark the server ready only after initialization and configuration have
			// been written. tsgo processes initialization and file events serially.
			readiness.markInitialized()
		}
		switch method {
		case "textDocument/didSave", "workspace/didChangeWatchedFiles":
			requestTypeMappingRegeneration(regenerate)
		}
	}
}

func requestTypeMappingRegeneration(regenerate chan<- struct{}) {
	select {
	case regenerate <- struct{}{}:
	default:
	}
}

func notifyLSPWatchedFiles(
	server *lockedLSPWriter, readiness *lspServerReadiness, locations lspProjectLocations,
) error {
	if !readiness.isInitialized() {
		return nil
	}
	return server.writeJSON(lspWatchedFilesNotification(locations))
}

func readLSPMessage(reader *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && line == "" {
				return nil, io.EOF
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, found := strings.Cut(line, ":")
		if !found || !strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			continue
		}
		contentLength, err = strconv.Atoi(strings.TrimSpace(value))
		if err != nil || contentLength < 0 || contentLength > lspMaxMessageSize {
			return nil, fmt.Errorf("invalid Content-Length %q", strings.TrimSpace(value))
		}
	}
	if contentLength < 0 {
		return nil, errors.New("LSP message has no Content-Length header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return body, nil
}

func writeLSPMessage(writer io.Writer, body []byte) error {
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err := writer.Write(body)
	return err
}

func lspMessageMethod(body []byte) string {
	var message struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(body, &message); err != nil {
		return ""
	}
	return message.Method
}

func lspConfigurationNotification(customConfigName string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"method":  "workspace/didChangeConfiguration",
		"params": map[string]any{
			"settings": map[string]any{
				"js/ts": map[string]any{
					"customConfigFileName": customConfigName,
				},
			},
		},
	}
}

func lspWatchedFilesNotification(locations lspProjectLocations) map[string]any {
	changes := []map[string]any{{"uri": fileURI(locations.configPath), "type": 2}}
	if locations.bridgePath != "" {
		changes = append(changes, map[string]any{"uri": fileURI(locations.bridgePath), "type": 2})
	}
	return map[string]any{
		"jsonrpc": "2.0",
		"method":  "workspace/didChangeWatchedFiles",
		"params":  map[string]any{"changes": changes},
	}
}

func fileURI(filename string) string {
	path := filepath.ToSlash(filename)
	if runtime.GOOS == "windows" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}
