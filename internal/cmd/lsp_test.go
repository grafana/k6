package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

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
