package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	xk6types "go.k6.io/k6/v2/examples/typecheck/xk6-types"
	"go.k6.io/k6/v2/internal/cmd/tests"
	"go.k6.io/k6/v2/lib/fsext"
)

func TestLSPServerInvocation(t *testing.T) {
	t.Parallel()

	t.Run("auto prefers project-local tsgo", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		localTsgo := filepath.Join(ts.Cwd, "node_modules", ".bin", "tsgo")
		require.NoError(t, ts.FS.MkdirAll(filepath.Dir(localTsgo), 0o755))
		require.NoError(t, fsext.WriteFile(ts.FS, localTsgo, nil, 0o755))

		command := &lspCmd{
			gs:     ts.GlobalState,
			server: "auto",
			lookPath: func(string) (string, error) {
				return "", errors.New("PATH should not be used")
			},
		}
		invocation, err := command.serverInvocation(ts.Cwd)
		require.NoError(t, err)
		require.Equal(t, lspServerTsgo, invocation.kind)
		require.Equal(t, localTsgo, invocation.path)
		require.Equal(t, []string{"--lsp", "--stdio"}, invocation.args)
	})

	t.Run("auto falls back to tsserver adapter", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		command := &lspCmd{
			gs:     ts.GlobalState,
			server: "auto",
			lookPath: func(name string) (string, error) {
				if name == "typescript-language-server" {
					return "/tools/typescript-language-server", nil
				}
				return "", errors.New("not found")
			},
		}
		invocation, err := command.serverInvocation(ts.Cwd)
		require.NoError(t, err)
		require.Equal(t, lspServerTsserver, invocation.kind)
		require.Equal(t, "/tools/typescript-language-server", invocation.path)
		require.Equal(t, []string{"--stdio"}, invocation.args)
	})

	t.Run("custom path requires an explicit backend", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		command := &lspCmd{gs: ts.GlobalState, server: "auto", serverPath: "/tools/server"}
		_, err := command.serverInvocation(ts.Cwd)
		require.EqualError(t, err,
			"--server-path requires an explicit --server tsgo or --server tsserver")
	})
}

func TestLSPProjectLocations(t *testing.T) {
	t.Parallel()

	t.Run("tsgo uses a temporary project and workspace bridge", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		command := &lspCmd{gs: ts.GlobalState}
		locations, err := command.projectLocations(ts.Cwd, lspServerTsgo)
		require.NoError(t, err)
		require.True(t, locations.temporary)
		require.Equal(t, filepath.Join(locations.projectDir, "tsconfig.json"), locations.configPath)
		require.Equal(t, filepath.Join(locations.projectDir, "types"), locations.typesDir)
		require.Equal(t, filepath.Clean(ts.Cwd), filepath.Dir(locations.bridgePath))
		require.Contains(t, filepath.Base(locations.bridgePath), "tsconfig.k6.")

		require.NoError(t, writeLSPBridge(ts.FS, locations.bridgePath, locations.configPath))
		locations.bridgeCreated = true
		bridge, err := fsext.ReadFile(ts.FS, locations.bridgePath)
		require.NoError(t, err)
		var project lspBridgeProject
		require.NoError(t, json.Unmarshal(bridge, &project))
		require.Equal(t, locations.configPath, project.Extends)

		command.cleanProject(locations)
		exists, err := fsext.Exists(ts.FS, locations.bridgePath)
		require.NoError(t, err)
		require.False(t, exists)
		exists, err = fsext.Exists(ts.FS, locations.projectDir)
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("in place writes local config and keeps declarations under dot k6", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		command := &lspCmd{gs: ts.GlobalState, inPlace: true}
		locations, err := command.projectLocations(ts.Cwd, lspServerTsgo)
		require.NoError(t, err)
		require.False(t, locations.temporary)
		require.Equal(t, ts.Cwd, locations.projectDir)
		require.Equal(t, filepath.Join(ts.Cwd, "tsconfig.json"), locations.configPath)
		require.Equal(t, filepath.Join(ts.Cwd, ".k6", "types"), locations.typesDir)
		require.Empty(t, locations.bridgePath)
	})

	t.Run("in place does not overwrite a project config", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		configPath := filepath.Join(ts.Cwd, "tsconfig.json")
		require.NoError(t, fsext.WriteFile(ts.FS, configPath, []byte("{}\n"), 0o644))

		command := &lspCmd{gs: ts.GlobalState, inPlace: true}
		_, err := command.projectLocations(ts.Cwd, lspServerTsgo)
		require.ErrorContains(t, err, "refusing to overwrite existing TypeScript configuration")

		contents, readErr := fsext.ReadFile(ts.FS, configPath)
		require.NoError(t, readErr)
		require.Equal(t, "{}\n", string(contents))

		require.NoError(t, writeInPlaceConfigMarker(ts.FS, ts.Cwd, configPath))
		locations, markedErr := command.projectLocations(ts.Cwd, lspServerTsgo)
		require.NoError(t, markedErr)
		require.Equal(t, configPath, locations.configPath)
	})

	t.Run("tsserver does not overwrite a project config", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		configPath := filepath.Join(ts.Cwd, "tsconfig.json")
		require.NoError(t, fsext.WriteFile(ts.FS, configPath, []byte("{}\n"), 0o644))

		command := &lspCmd{gs: ts.GlobalState}
		_, err := command.projectLocations(ts.Cwd, lspServerTsserver)
		require.ErrorContains(t, err, "use --server tsgo")

		contents, readErr := fsext.ReadFile(ts.FS, configPath)
		require.NoError(t, readErr)
		require.Equal(t, "{}\n", string(contents))
	})
}

func TestLSPDoesNotExposeTypesDirectory(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	require.Nil(t, getCmdLSP(ts.GlobalState).Flags().Lookup("types-dir"))
}

func TestResolveLSPInput(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	workspace := filepath.Join(ts.Cwd, "workspace")
	require.NoError(t, ts.FS.MkdirAll(workspace, 0o755))
	filename := filepath.Join(ts.Cwd, "script.js")
	require.NoError(t, fsext.WriteFile(ts.FS, filename, []byte("export {};\n"), 0o644))

	input, err := resolveLSPInput(ts.FS, ts.Cwd, "workspace")
	require.NoError(t, err)
	require.Equal(t, lspInput{path: workspace, directory: true}, input)

	input, err = resolveLSPInput(ts.FS, ts.Cwd, "script.js")
	require.NoError(t, err)
	require.Equal(t, lspInput{path: filename}, input)
}

func TestDiscoverLSPScripts(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	root := filepath.Join(ts.Cwd, "workspace")
	for _, directory := range []string{
		root,
		filepath.Join(root, "tests"),
		filepath.Join(root, ".git"),
		filepath.Join(root, ".k6"),
		filepath.Join(root, "node_modules", "dependency"),
	} {
		require.NoError(t, ts.FS.MkdirAll(directory, 0o755))
	}
	for filename, contents := range map[string]string{
		filepath.Join(root, "main.js"):                                "export {};\n",
		filepath.Join(root, "tests", "load.ts"):                       "export {};\n",
		filepath.Join(root, "tests", "helper.mjs"):                    "export {};\n",
		filepath.Join(root, "tests", "types.d.ts"):                    "export {};\n",
		filepath.Join(root, "README.md"):                              "not a script\n",
		filepath.Join(root, ".git", "hook.js"):                        "export {};\n",
		filepath.Join(root, ".k6", "generated.ts"):                    "export {};\n",
		filepath.Join(root, "node_modules", "dependency", "index.js"): "export {};\n",
	} {
		require.NoError(t, fsext.WriteFile(ts.FS, filename, []byte(contents), 0o644))
	}

	scripts, directories, err := discoverLSPScripts(ts.FS, root)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(root, "main.js"),
		filepath.Join(root, "tests", "helper.mjs"),
		filepath.Join(root, "tests", "load.ts"),
	}, scripts)
	require.Equal(t, []string{root, filepath.Join(root, "tests")}, directories)

	empty := filepath.Join(ts.Cwd, "empty")
	require.NoError(t, ts.FS.MkdirAll(empty, 0o755))
	_, _, err = discoverLSPScripts(ts.FS, empty)
	require.ErrorContains(t, err, "no JavaScript or TypeScript files found")
}

func TestLSPRegenerateDirectoryProject(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	root := filepath.Join(ts.Cwd, "workspace")
	nested := filepath.Join(root, "nested")
	require.NoError(t, ts.FS.MkdirAll(nested, 0o755))
	mainFile := filepath.Join(root, "main.js")
	invalidFile := filepath.Join(root, "invalid.js")
	nestedFile := filepath.Join(nested, "scenario.ts")
	require.NoError(t, fsext.WriteFile(ts.FS, mainFile,
		[]byte(`
import { greet } from "k6/x/types-example";
export default function () { greet("k6"); }
`), 0o644))
	require.NoError(t, fsext.WriteFile(ts.FS, invalidFile,
		[]byte("export default function (\n"), 0o644))
	require.NoError(t, fsext.WriteFile(ts.FS, nestedFile,
		[]byte("export default function (): void {}\n"), 0o644))

	command := &cobra.Command{}
	command.Flags().AddFlagSet(runtimeOptionFlagSet(false))
	locations := lspProjectLocations{
		configPath: filepath.Join(ts.Cwd, "generated", "tsconfig.json"),
		typesDir:   filepath.Join(ts.Cwd, "generated", "types"),
	}
	lsp := &lspCmd{gs: ts.GlobalState, httpClient: http.DefaultClient}
	watchPaths, err := lsp.regenerateProject(context.Background(), command,
		lspInput{path: root, directory: true}, root, locations)
	require.NoError(t, err)
	require.Contains(t, watchPaths, root)
	require.Contains(t, watchPaths, nested)
	require.Contains(t, watchPaths, mainFile)
	require.Contains(t, watchPaths, invalidFile)
	require.Contains(t, watchPaths, nestedFile)

	data, err := fsext.ReadFile(ts.FS, locations.configPath)
	require.NoError(t, err)
	var project typecheckProject
	require.NoError(t, json.Unmarshal(data, &project))
	require.Equal(t, []string{invalidFile, mainFile, nestedFile}, project.Files)
	require.Contains(t, project.CompilerOptions.Paths, xk6types.ModuleName)
	require.NotEmpty(t, project.CompilerOptions.TypeRoots)
	require.Equal(t, filepath.Join(locations.typesDir, "builtin"), project.CompilerOptions.TypeRoots[0])
	httpTypes := project.CompilerOptions.Paths["k6/http"]
	require.Equal(t, []string{
		filepath.Join(locations.typesDir, "builtin", "k6", "http", "index.d.ts"),
	}, httpTypes)
	httpDeclaration, err := fsext.ReadFile(ts.FS, httpTypes[0])
	require.NoError(t, err)
	require.Contains(t, string(httpDeclaration), "export function get")

	addedFile := filepath.Join(root, "added.js")
	require.NoError(t, fsext.WriteFile(ts.FS, addedFile,
		[]byte("export default function () {}\n"), 0o644))
	watchPaths, err = lsp.regenerateProject(context.Background(), command,
		lspInput{path: root, directory: true}, root, locations)
	require.NoError(t, err)
	require.Contains(t, watchPaths, addedFile)
	data, err = fsext.ReadFile(ts.FS, locations.configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &project))
	require.Equal(t, []string{addedFile, invalidFile, mainFile, nestedFile}, project.Files)

	require.NoError(t, ts.FS.Remove(addedFile))
	_, err = lsp.regenerateProject(context.Background(), command,
		lspInput{path: root, directory: true}, root, locations)
	require.NoError(t, err)
	data, err = fsext.ReadFile(ts.FS, locations.configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &project))
	require.Equal(t, []string{invalidFile, mainFile, nestedFile}, project.Files)
}

func TestProxyLSPClient(t *testing.T) {
	t.Parallel()

	var client bytes.Buffer
	require.NoError(t, writeLSPMessage(&client, []byte(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)))
	require.NoError(t, writeLSPMessage(&client, []byte(
		`{"jsonrpc":"2.0","method":"textDocument/didSave","params":{}}`)))

	var server bytes.Buffer
	readiness := newLSPServerReadiness()
	regenerate := make(chan struct{}, 1)
	require.NoError(t, proxyLSPClient(
		&client, &lockedLSPWriter{writer: &server}, "tsconfig.k6.42.json", readiness, regenerate))
	require.True(t, readiness.isInitialized())

	reader := bufio.NewReader(&server)
	first, err := readLSPMessage(reader)
	require.NoError(t, err)
	require.Equal(t, "initialized", lspMessageMethod(first))

	configuration, err := readLSPMessage(reader)
	require.NoError(t, err)
	require.Equal(t, "workspace/didChangeConfiguration", lspMessageMethod(configuration))
	var message map[string]any
	require.NoError(t, json.Unmarshal(configuration, &message))
	settings := message["params"].(map[string]any)["settings"].(map[string]any)
	jsTS := settings["js/ts"].(map[string]any)
	require.Equal(t, "tsconfig.k6.42.json", jsTS["customConfigFileName"])

	last, err := readLSPMessage(reader)
	require.NoError(t, err)
	require.Equal(t, "textDocument/didSave", lspMessageMethod(last))
	_, err = readLSPMessage(reader)
	require.ErrorIs(t, err, io.EOF)

	select {
	case <-regenerate:
	default:
		t.Fatal("didSave did not request type mapping regeneration")
	}
}

func TestProxyLSPClientDropsWatchedFilesBeforeInitialized(t *testing.T) {
	t.Parallel()

	var client bytes.Buffer
	require.NoError(t, writeLSPMessage(&client, []byte(
		`{"jsonrpc":"2.0","method":"workspace/didChangeWatchedFiles","params":{"changes":[]}}`)))

	var server bytes.Buffer
	readiness := newLSPServerReadiness()
	regenerate := make(chan struct{}, 1)
	require.NoError(t, proxyLSPClient(
		&client, &lockedLSPWriter{writer: &server}, "tsconfig.k6.42.json", readiness, regenerate))

	require.Empty(t, server.Bytes())
	require.False(t, readiness.isInitialized())
	select {
	case <-regenerate:
	default:
		t.Fatal("didChangeWatchedFiles did not request type mapping regeneration")
	}
}

func TestLSPServerReadiness(t *testing.T) {
	t.Parallel()

	readiness := newLSPServerReadiness()
	require.False(t, readiness.isInitialized())
	readiness.markInitialized()
	require.True(t, readiness.isInitialized())
	readiness.markInitialized()
}

func TestNotifyLSPWatchedFilesWaitsForInitialization(t *testing.T) {
	t.Parallel()

	var server bytes.Buffer
	writer := &lockedLSPWriter{writer: &server}
	readiness := newLSPServerReadiness()
	locations := lspProjectLocations{configPath: "/tmp/k6-lsp/tsconfig.json"}

	require.NoError(t, notifyLSPWatchedFiles(writer, readiness, locations))
	require.Empty(t, server.Bytes())

	readiness.markInitialized()
	require.NoError(t, notifyLSPWatchedFiles(writer, readiness, locations))
	message, err := readLSPMessage(bufio.NewReader(&server))
	require.NoError(t, err)
	require.Equal(t, "workspace/didChangeWatchedFiles", lspMessageMethod(message))
}

func TestLSPFileWatcher(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	filename := filepath.Join(ts.Cwd, "script.js")
	require.NoError(t, fsext.WriteFile(ts.FS, filename, []byte("export {};\n"), 0o644))

	watcher := &lspFileWatcher{fs: ts.FS}
	watcher.update([]string{filename})
	require.False(t, watcher.changed())

	require.NoError(t, fsext.WriteFile(ts.FS, filename, []byte("export default function () {};\n"), 0o644))
	require.True(t, watcher.changed())
	require.False(t, watcher.changed())

	watcher.update([]string{ts.Cwd, filename})
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "added.js"), []byte("export {};\n"), 0o644))
	require.True(t, watcher.changed())
	require.False(t, watcher.changed())
}

func TestLocalSourcePaths(t *testing.T) {
	t.Parallel()

	paths := localSourcePaths("/work/script.js", []string{
		"file:///work/helper.js",
		"file:///work/helper.js",
		"https://example.test/module.js",
		"k6/http",
	})
	require.Equal(t, []string{"/work/script.js", "/work/helper.js"}, paths)
}

func TestValidateLSPEntryPath(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateLSPEntryPath("/work", "/work/tests/script.js"))
	require.ErrorContains(t, validateLSPEntryPath("/work", "/tmp/script.js"),
		"run k6 lsp from a parent directory")
}
