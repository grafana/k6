package cmd

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/cmd/tests"
	"go.k6.io/k6/v2/subcommand"
)

const testCatalogJSON = `{
"example.com/x-alpha": {"subcommands":["alpha"],"description":"alpha sub","tier":"official"},
"example.com/x-bravo": {"subcommands":["bravo"],"description":"bravo sub","tier":"official"},
"example.com/x-charlie": {"subcommands":["charlie"],"description":"charlie sub","tier":"community"}
}`

func newCatalogServer(t *testing.T, body ...string) *httptest.Server {
	t.Helper()
	catalog := testCatalogJSON
	if len(body) > 0 {
		catalog = body[0]
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(catalog))
	}))
	t.Cleanup(s.Close)
	return s
}

func TestExtensionSubcommands(t *testing.T) {
	t.Parallel()

	registerTestSubcommandExtensions(t)

	t.Run("returns all extension subcommands", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		commands := extensionSubcommands(ts.GlobalState)

		// Should have at least the 3 test extensions we registered
		require.GreaterOrEqual(t, len(commands), 2)

		// Check that our test commands are present
		commandNames := make(map[string]bool)
		for _, cmd := range commands {
			commandNames[cmd.Name()] = true
		}

		require.True(t, commandNames["test-cmd-1"], "test-cmd-1 should be present")
		require.True(t, commandNames["test-cmd-2"], "test-cmd-2 should be present")
		require.True(t, commandNames["test-cmd-3"], "test-cmd-3 should be present")
	})

	t.Run("returns commands with correct properties", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		commands := extensionSubcommands(ts.GlobalState)

		for _, cmd := range commands {
			require.NotEmpty(t, cmd.Use, "command should have a Use field")

			switch cmd.Use {
			case "test-cmd-1":
				require.Equal(t, "Test command 1", cmd.Short)
			case "test-cmd-2":
				require.Equal(t, "Test command 2", cmd.Short)
			case "test-cmd-3":
				require.Equal(t, "Test command 3", cmd.Short)
			}
		}
	})
}

func TestXCommandHelpDisplayCommands(t *testing.T) {
	t.Parallel()

	registerTestSubcommandExtensions(t)

	testCases := []struct {
		name               string
		wantStdoutContains string
	}{
		{
			name:               "should have test-cmd-1 command",
			wantStdoutContains: "  test-cmd-1  Test command 1",
		},
		{
			name:               "should have test-cmd-2 command",
			wantStdoutContains: "  test-cmd-2  Test command 2",
		},
		{
			name:               "should have test-cmd-3 command",
			wantStdoutContains: "  test-cmd-3  Test command 3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)
			ts.CmdArgs = []string{"k6", "x", "help"}
			newRootCommand(ts.GlobalState).execute()

			require.Contains(t, ts.Stdout.String(), tc.wantStdoutContains)
		})
	}
}

func TestXCommandRegistryStubs(t *testing.T) {
	t.Parallel()

	registerTestSubcommandExtensions(t)
	server := newCatalogServer(t)

	tt := []struct {
		name      string
		catalog   string
		autoOff   bool
		wantStubs bool
	}{
		{name: "reachable catalog shows stubs", catalog: server.URL, wantStubs: true},
		{name: "unreachable catalog hides stubs", catalog: "http://127.0.0.1:1/unreachable"},
		{name: "auto-extension-resolution off hides stubs", catalog: server.URL, autoOff: true},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)
			ts.Env[state.ProvisionCatalogURL] = tc.catalog
			if tc.autoOff {
				ts.Env[state.AutoExtensionResolution] = "false"
			}
			ts.CmdArgs = []string{"k6", "x"}
			ts.ReparseFlags()
			newRootCommand(ts.GlobalState).execute()

			out := ts.Stdout.String()
			require.Contains(t, out, "Available Commands:")
			require.Regexp(t, `(?m)^  test-cmd-1\s+Test command 1$`, out)

			for _, name := range []string{"alpha", "bravo", "charlie"} {
				pattern := `(?m)^  ` + regexp.QuoteMeta(name) + `\s`
				if tc.wantStubs {
					require.Regexp(t, pattern+`+\S`, out, "missing stub row %q", name)
				} else {
					require.NotRegexp(t, pattern, out, "unexpected stub row %q", name)
				}
			}
		})
	}
}

func Test_dependenciesFromSubcommand(t *testing.T) {
	t.Parallel()

	t.Run("without manifest", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		deps, err := dependenciesFromSubcommand(ts.GlobalState, "test-cmd")

		require.NoError(t, err)
		require.Contains(t, deps, "subcommand:test-cmd")
		require.Nil(t, deps["subcommand:test-cmd"])
	})

	t.Run("with manifest", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)

		ts.Flags.DependenciesManifest = `{"subcommand:test-cmd": "1.2.3"}`

		deps, err := dependenciesFromSubcommand(ts.GlobalState, "test-cmd")

		require.NoError(t, err)
		require.Contains(t, deps, "subcommand:test-cmd")
		require.Equal(t, "1.2.3", deps["subcommand:test-cmd"].String())
	})

	t.Run("with malformed manifest", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)

		ts.Flags.DependenciesManifest = `{subcommand:test-cmd": "1.2.3"}`

		deps, err := dependenciesFromSubcommand(ts.GlobalState, "test-cmd")

		require.Error(t, err)
		require.Nil(t, deps)
	})
}

func TestXCompletion(t *testing.T) {
	t.Parallel()

	registerTestSubcommandExtensions(t)

	tt := []struct {
		name    string
		args    []string
		want    []string
		notWant []string
	}{
		{
			name:    "committed extension name completes its args",
			args:    []string{"k6", "__complete", "x", "test-cmd-1", ""},
			want:    []string{"alpha", "bravo", "charlie"},
			notWant: []string{"test-cmd-1"},
		},
		{
			name:    "deeper args complete within extension",
			args:    []string{"k6", "__complete", "x", "test-cmd-1", "alpha", ""},
			want:    []string{"deep-one", "deep-two"},
			notWant: []string{"alpha"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)
			ts.CmdArgs = tc.args
			newRootCommand(ts.GlobalState).execute()

			out := ts.Stdout.String()
			for _, w := range tc.want {
				require.Contains(t, out, w)
			}
			for _, nw := range tc.notWant {
				require.NotContains(t, out, nw)
			}
		})
	}
}

func Test_detectExtensionCompletion(t *testing.T) {
	t.Parallel()

	registerTestSubcommandExtensions(t)

	tt := []struct {
		name    string
		args    []string
		env     map[string]string
		want    bool
		wantExt string
	}{
		{
			name: "partial name single char",
			args: []string{"k6", "__complete", "x", "d"},
		},
		{
			name: "partial name without trailing space",
			args: []string{"k6", "__complete", "x", "docs"},
		},
		{
			name: "not a completion request",
			args: []string{"k6", "x", "docs", ""},
		},
		{
			name: "completion not targeting x",
			args: []string{"k6", "__complete", "run", "script.js", ""},
		},
		{
			name: "auto resolution disabled",
			args: []string{"k6", "__complete", "x", "unknown-ext", ""},
			env:  map[string]string{"K6_AUTO_EXTENSION_RESOLUTION": "false"},
		},
		{
			name: "registered extension handled by cobra",
			args: []string{"k6", "__complete", "x", "test-cmd-1", ""},
		},
		{
			name:    "unregistered extension with committed name",
			args:    []string{"k6", "__complete", "x", "docs", ""},
			want:    true,
			wantExt: "docs",
		},
		{
			name:    "unregistered extension with deeper args",
			args:    []string{"k6", "__complete", "x", "docs", "http", ""},
			want:    true,
			wantExt: "docs",
		},
		{
			name:    "__completeNoDesc variant",
			args:    []string{"k6", "__completeNoDesc", "x", "docs", ""},
			want:    true,
			wantExt: "docs",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)
			ts.CmdArgs = tc.args
			maps.Copy(ts.Env, tc.env)
			if len(tc.env) > 0 {
				ts.ReparseFlags()
			}

			root := newRootCommand(ts.GlobalState)
			ext, ok := detectExtensionCompletion(root.cmd, ts.GlobalState)

			require.Equal(t, tc.want, ok)
			require.Equal(t, tc.wantExt, ext)
		})
	}
}

func Test_buildExtensionDeps(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	err := buildExtensionDeps(ts.GlobalState, "docs")

	var derr binaryIsNotSatisfyingDependenciesError
	require.ErrorAs(t, err, &derr)
	require.Contains(t, derr.deps, "subcommand:docs")
}

var registerTestSubcommandExtensionsOnce sync.Once //nolint:gochecknoglobals

func registerTestSubcommandExtensions(t *testing.T) {
	t.Helper()

	registerTestSubcommandExtensionsOnce.Do(func() {
		subcommand.RegisterExtension("test-cmd-1", func(_ *state.GlobalState) *cobra.Command {
			return &cobra.Command{
				Use:   "test-cmd-1",
				Short: "Test command 1",
				Run:   func(_ *cobra.Command, _ []string) {},
				ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
					if len(args) > 0 {
						return []string{"deep-one", "deep-two"}, cobra.ShellCompDirectiveNoFileComp
					}
					return []string{"alpha", "bravo", "charlie"}, cobra.ShellCompDirectiveNoFileComp
				},
			}
		})

		subcommand.RegisterExtension("test-cmd-2", func(_ *state.GlobalState) *cobra.Command {
			return &cobra.Command{
				Use:   "test-cmd-2",
				Short: "Test command 2",
				Run:   func(_ *cobra.Command, _ []string) {},
			}
		})

		subcommand.RegisterExtension("test-cmd-3", func(_ *state.GlobalState) *cobra.Command {
			return &cobra.Command{
				Use:   "test-cmd-3",
				Short: "Test command 3",
				Run:   func(_ *cobra.Command, _ []string) {},
			}
		})
	})
}
