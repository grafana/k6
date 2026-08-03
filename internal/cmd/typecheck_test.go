package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cmd/state"
	xk6types "go.k6.io/k6/v2/examples/typecheck/xk6-types"
	"go.k6.io/k6/v2/internal/cmd/tests"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib/fsext"
)

func TestTypecheckGenerateOnly(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.FS = testutils.MakeMemMapFs(t, map[string][]byte{
		filepath.Join(ts.Cwd, "script.js"): []byte(`
import http from "k6/http";

export default function () {
  http.get("https://example.com");
}
`),
	})

	cmd := getCmdTypecheck(ts.GlobalState)
	cmd.SetArgs([]string{"--generate-only", "--tsconfig", "generated/tsconfig.json", "script.js"})
	require.NoError(t, cmd.Execute())

	configPath := filepath.Join(ts.Cwd, "generated", "tsconfig.json")
	data, err := fsext.ReadFile(ts.FS, configPath)
	require.NoError(t, err)

	var project typecheckProject
	require.NoError(t, json.Unmarshal(data, &project))
	require.Equal(t, []string{filepath.Join(ts.Cwd, "script.js")}, project.Files)
	require.Equal(t, "Bundler", project.CompilerOptions.ModuleResolution)
	require.Equal(t, []string{"k6"}, project.CompilerOptions.Types)
	require.Empty(t, project.CompilerOptions.Paths)
	require.Contains(t, ts.Stdout.String(), configPath)
}

func TestTypecheckOutputLocations(t *testing.T) {
	t.Parallel()

	t.Run("temporary by default", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		command := &typecheckCmd{gs: ts.GlobalState}
		projectDir, configPath, typesDir, err := command.outputLocations(ts.Cwd)
		require.NoError(t, err)
		require.True(t, filepath.IsAbs(projectDir))
		require.True(t, strings.HasPrefix(filepath.Base(projectDir), typecheckTempDirPattern))
		require.Equal(t, filepath.Join(projectDir, "tsconfig.json"), configPath)
		require.Equal(t, filepath.Join(projectDir, "types"), typesDir)
	})

	t.Run("in place", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		command := &typecheckCmd{gs: ts.GlobalState, inPlace: true}
		projectDir, configPath, typesDir, err := command.outputLocations(ts.Cwd)
		require.NoError(t, err)
		require.Equal(t, ts.Cwd, projectDir)
		require.Equal(t, filepath.Join(ts.Cwd, "tsconfig.json"), configPath)
		require.Equal(t, filepath.Join(ts.Cwd, ".k6", "types"), typesDir)
	})
}

func TestTypecheckDoesNotExposeTypesDirectory(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	require.Nil(t, getCmdTypecheck(ts.GlobalState).Flags().Lookup("types-dir"))
}

func TestTypecheckInPlaceDoesNotImplicitlyOverwriteConfig(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	configPath := filepath.Join(ts.Cwd, "tsconfig.json")
	require.NoError(t, fsext.WriteFile(ts.FS, configPath, []byte("{}\n"), 0o644))

	command := &typecheckCmd{gs: ts.GlobalState, inPlace: true}
	require.ErrorContains(t, command.ensureInPlaceConfigAvailable(configPath),
		"refusing to overwrite existing TypeScript configuration")

	require.NoError(t, writeInPlaceConfigMarker(ts.FS, ts.Cwd, configPath))
	require.NoError(t, command.ensureInPlaceConfigAvailable(configPath))

	command.configPath = "tsconfig.json"
	require.NoError(t, command.ensureInPlaceConfigAvailable(configPath))
}

func TestTypecheckGenerateOnlyInPlaceWritesLocalConfig(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.FS = testutils.MakeMemMapFs(t, map[string][]byte{
		filepath.Join(ts.Cwd, "script.js"): []byte("export default function () {}\n"),
	})

	for range 2 {
		command := getCmdTypecheck(ts.GlobalState)
		command.SetArgs([]string{"--generate-only", "--in-place", "script.js"})
		require.NoError(t, command.Execute())
	}

	configPath := filepath.Join(ts.Cwd, "tsconfig.json")
	exists, err := fsext.Exists(ts.FS, configPath)
	require.NoError(t, err)
	require.True(t, exists)
	marked, err := isMarkedInPlaceConfig(ts.FS, ts.Cwd, configPath)
	require.NoError(t, err)
	require.True(t, marked)
}

func TestTypecheckWatchRunsCheckerInWatchMode(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.FS = testutils.MakeMemMapFs(t, map[string][]byte{
		filepath.Join(ts.Cwd, "script.js"): []byte("export default function () {}\n"),
	})

	var invocation checkerInvocation
	command := &typecheckCmd{
		gs:      ts.GlobalState,
		checker: "tsgo",
		watch:   true,
		lookPath: func(string) (string, error) {
			return "/tools/tsgo", nil
		},
		runChecker: func(_ context.Context, got checkerInvocation, _ *state.GlobalState) error {
			invocation = got
			return nil
		},
	}
	cobraCommand := &cobra.Command{RunE: command.run}
	cobraCommand.Flags().AddFlagSet(runtimeOptionFlagSet(false))
	cobraCommand.SetArgs([]string{"script.js"})
	require.NoError(t, cobraCommand.ExecuteContext(context.Background()))
	require.Equal(t, "/tools/tsgo", invocation.path)
	require.Equal(t, "--watch", invocation.args[len(invocation.args)-1])
}

func TestResolveRemoteAJVDeclarationFromHeader(t *testing.T) {
	t.Parallel()

	const declaration = "declare var ajv: { new(): object }; export = ajv;\n"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/ajv@6.12.5":
			if !request.URL.Query().Has("bundle") {
				http.Error(response, "missing bundle query", http.StatusBadRequest)
				return
			}
			response.Header().Set(typeScriptTypesHeader, server.URL+"/ajv@6.12.5/lib/ajv.d.ts")
			response.WriteHeader(http.StatusOK)
		case "/ajv@6.12.5/lib/ajv.d.ts":
			_, _ = response.Write([]byte(declaration))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	ts := tests.NewGlobalTestState(t)
	command := &typecheckCmd{gs: ts.GlobalState, httpClient: server.Client()}
	typesDir := filepath.Join(ts.Cwd, ".k6", "types")
	filename, found, err := command.resolveRemoteDeclaration(
		context.Background(), server.URL+"/ajv@6.12.5?bundle", typesDir)
	require.NoError(t, err)
	require.True(t, found)
	require.Contains(t, filename, filepath.Join("remotes", safePathSegment(server.Listener.Addr().String())))
	require.Regexp(t, `^ajv@6\.12\.5-[0-9a-f]{12}\.d\.ts$`, filepath.Base(filename))

	data, err := fsext.ReadFile(ts.FS, filename)
	require.NoError(t, err)
	require.Equal(t, declaration, string(data))
}

func TestResolveRemoteDeclarationFromSibling(t *testing.T) {
	t.Parallel()

	const declaration = "export declare function value(): string;\n"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/module.js":
			response.WriteHeader(http.StatusOK)
		case "/module.d.ts":
			_, _ = response.Write([]byte(declaration))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	ts := tests.NewGlobalTestState(t)
	command := &typecheckCmd{gs: ts.GlobalState, httpClient: server.Client()}
	filename, found, err := command.resolveRemoteDeclaration(
		context.Background(), server.URL+"/module.js", filepath.Join(ts.Cwd, ".k6", "types"))
	require.NoError(t, err)
	require.True(t, found)

	data, err := fsext.ReadFile(ts.FS, filename)
	require.NoError(t, err)
	require.Equal(t, declaration, string(data))
}

func TestResolveRemoteTypeScriptSource(t *testing.T) {
	t.Parallel()

	const source = "export function value(input: string): string { return input; }\n"
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		require.Equal(t, "/scope/package/1.0.0/src/mod.ts", request.URL.Path)
		_, _ = response.Write([]byte(source))
	}))
	defer server.Close()

	ts := tests.NewGlobalTestState(t)
	command := &typecheckCmd{gs: ts.GlobalState, httpClient: server.Client()}
	typesDir := filepath.Join(ts.Cwd, ".k6", "types")
	moduleSpecifier := server.URL + "/scope/package/1.0.0/src/mod.ts"

	filename, found, err := command.resolveRemoteDeclaration(context.Background(), moduleSpecifier, typesDir)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, filepath.Join(typesDir, "remotes", safePathSegment(server.Listener.Addr().String()),
		"scope", "package", "1.0.0", "src", "mod.ts"), filename)

	data, err := fsext.ReadFile(ts.FS, filename)
	require.NoError(t, err)
	require.Equal(t, source, string(data))

	secondFilename, found, err := command.resolveRemoteDeclaration(context.Background(), moduleSpecifier, typesDir)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, filename, secondFilename)
	require.Equal(t, int32(1), requests.Load())
}

func TestResolveRemoteDeclarationUsesLegacyCache(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	typesDir := filepath.Join(ts.Cwd, ".k6", "types")
	legacyPath := filepath.Join(typesDir, "legacy", "1.0.0", "index.d.ts")
	require.NoError(t, ts.FS.MkdirAll(filepath.Dir(legacyPath), 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, legacyPath, []byte("export {};\n"), 0o644))

	command := &typecheckCmd{
		gs: ts.GlobalState,
		httpClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network should not be used")
		})},
	}
	filename, found, err := command.resolveRemoteDeclaration(
		context.Background(), "https://modules.example/legacy/1.0.0/index.js", typesDir)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, legacyPath, filename)
}

func TestCanonicalRemoteDeclarationPathCannotEscapeTypesDirectory(t *testing.T) {
	t.Parallel()

	typesDir := filepath.Join(t.TempDir(), "types")
	moduleURL, err := url.Parse("https://example.com/../../outside.js")
	require.NoError(t, err)

	filename := canonicalRemoteDeclarationPath(typesDir, moduleURL)
	relative, err := filepath.Rel(typesDir, filename)
	require.NoError(t, err)
	require.NotEqual(t, "..", relative)
	require.NotContains(t, relative, ".."+string(filepath.Separator))
}

func TestFindCheckerPrefersProjectLocalTsgo(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	localTsgo := filepath.Join(ts.Cwd, "node_modules", ".bin", "tsgo")
	require.NoError(t, ts.FS.MkdirAll(filepath.Dir(localTsgo), 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, localTsgo, []byte(""), 0o755))

	command := &typecheckCmd{
		gs:      ts.GlobalState,
		checker: "auto",
		lookPath: func(string) (string, error) {
			return "", errors.New("PATH should not be used")
		},
	}
	checker, err := command.findChecker(ts.Cwd)
	require.NoError(t, err)
	require.Equal(t, localTsgo, checker)
}

func TestFindCheckerSearchesAncestorNodeModules(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	localTsgo := filepath.Join(ts.Cwd, "node_modules", ".bin", "tsgo")
	require.NoError(t, ts.FS.MkdirAll(filepath.Dir(localTsgo), 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, localTsgo, nil, 0o755))
	workspace := filepath.Join(ts.Cwd, "tests", "load")
	require.NoError(t, ts.FS.MkdirAll(workspace, 0o755))

	command := &typecheckCmd{
		gs: ts.GlobalState,
		lookPath: func(string) (string, error) {
			return "", errors.New("PATH should not be used")
		},
	}
	checker, err := command.findNamedChecker(workspace, "tsgo")
	require.NoError(t, err)
	require.Equal(t, localTsgo, checker)
}

func TestCacheProvidedExtensionDeclaration(t *testing.T) {
	t.Parallel()

	const declaration = "export default function value(): string;\n"
	ts := tests.NewGlobalTestState(t)
	command := &typecheckCmd{gs: ts.GlobalState}
	typesDir := filepath.Join(ts.Cwd, ".k6", "types")

	filename, found, err := command.cacheProvidedExtensionDeclaration(
		"k6/x/example", "1.2.3", typesDir, staticTypeScriptTypes(declaration))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t,
		filepath.Join(typesDir, "extensions", "k6-x-example", "1.2.3", "index.d.ts"), filename)

	data, err := fsext.ReadFile(ts.FS, filename)
	require.NoError(t, err)
	require.Equal(t, declaration, string(data))
}

func TestTypecheckExtractsDeclarationFromCompiledExtension(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.FS = testutils.MakeMemMapFs(t, map[string][]byte{
		filepath.Join(ts.Cwd, "extension.js"): []byte(`
import { greet } from "k6/x/types-example";

export default function () {
  greet("k6");
}
`),
	})

	cmd := getCmdTypecheck(ts.GlobalState)
	cmd.SetArgs([]string{
		"--generate-only",
		"--tsconfig", "generated/tsconfig.json",
		"extension.js",
	})
	require.NoError(t, cmd.Execute())

	configPath := filepath.Join(ts.Cwd, "generated", "tsconfig.json")
	data, err := fsext.ReadFile(ts.FS, configPath)
	require.NoError(t, err)
	var project typecheckProject
	require.NoError(t, json.Unmarshal(data, &project))

	mapping := project.CompilerOptions.Paths[xk6types.ModuleName]
	require.Len(t, mapping, 1)
	declaration, err := fsext.ReadFile(ts.FS, mapping[0])
	require.NoError(t, err)
	require.Contains(t, string(declaration), "export declare function greet(name: string): string;")
}

func TestNewTypecheckProject(t *testing.T) {
	t.Parallel()

	paths := map[string][]string{
		"https://example.com/module.js": {"/types/module.d.ts"},
	}
	typeRoots := []string{"/project/node_modules/@types"}
	project := newTypecheckProject("/project/script.ts", paths, typeRoots)

	require.True(t, project.CompilerOptions.AllowJS)
	require.True(t, project.CompilerOptions.CheckJS)
	require.True(t, project.CompilerOptions.NoEmit)
	require.False(t, project.CompilerOptions.SkipLibCheck)
	require.True(t, project.CompilerOptions.AllowImportingTSExtensions)
	require.Equal(t, typeRoots, project.CompilerOptions.TypeRoots)
	require.Equal(t, paths, project.CompilerOptions.Paths)
	require.Equal(t, []string{"/project/script.ts"}, project.Files)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type staticTypeScriptTypes string

func (types staticTypeScriptTypes) TypeScriptTypes() []byte {
	return []byte(types)
}
