package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/fsext"
)

// optsFileJSON is an options file whose options differ from the export of optsFileScript.
const optsFileJSON = `{
	"scenarios": {
		"api": {
			"executor": "shared-iterations", "vus": 2, "iterations": 4, "exec": "api",
			"env": { "FROM": "file" }, "tags": { "team": "core" }
		}
	},
	"thresholds": { "iterations": ["count==4"] },
	"tags": { "origin": "file" }
}`

const optsFileScript = `
	import exec from 'k6/execution';
	export const options = { vus: 10, duration: '1m', tags: { origin: 'script' } };
	export default function () { console.log('ran default'); }
	export function api() {
		console.log('ran ' + exec.scenario.name + ' FROM=' + __ENV.FROM +
			' team=' + exec.vu.metrics.tags.team + ' origin=' + exec.test.options.tags.origin);
	}
`

// newOptionsFileTestState writes the script to test.js and, unless options is empty, the
// options file to opts.json, both in the working directory.
func newOptionsFileTestState(
	t *testing.T, script, options string, expExitCode exitcodes.ExitCode, args ...string,
) *GlobalTestState {
	t.Helper()

	ts := NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(script), 0o644))
	if options != "" {
		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "opts.json"), []byte(options), 0o644))
	}
	ts.CmdArgs = append([]string{"k6"}, args...)
	ts.ExpectedExitCode = int(expExitCode)
	return ts
}

// inspectScriptWithOptions returns today's inspect output for a script that exports options.
func inspectScriptWithOptions(t *testing.T, options string, flags ...string) string {
	t.Helper()

	script := "export const options = " + options + ";\n" +
		"export default function () {}\nexport function api() {}\n"
	ts := newOptionsFileTestState(t, script, "", 0, append(append([]string{"inspect"}, flags...), "test.js")...)
	cmd.ExecuteWithGlobalState(ts.GlobalState)
	return ts.Stdout.String()
}

func TestInspectOptionsFile(t *testing.T) {
	t.Parallel()

	for name, flags := range map[string][]string{
		"options":                {},
		"execution requirements": {"--execution-requirements"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// The file's options give the same output as a script that exports them.
			want := inspectScriptWithOptions(t, optsFileJSON, flags...)

			for argsName, scriptArg := range map[string][]string{
				"with script":    {"test.js"},
				"without script": nil,
			} {
				t.Run(argsName, func(t *testing.T) {
					t.Parallel()

					args := append(append([]string{"inspect", "--options-file", "opts.json"}, flags...), scriptArg...)
					ts := newOptionsFileTestState(t, optsFileScript, optsFileJSON, 0, args...)
					cmd.ExecuteWithGlobalState(ts.GlobalState)

					assert.JSONEq(t, want, ts.Stdout.String())
				})
			}
		})
	}
}

func TestInspectOptionsFileExecutionRequirements(t *testing.T) {
	t.Parallel()

	ts := newOptionsFileTestState(t, optsFileScript, optsFileJSON, 0,
		"inspect", "--options-file", "opts.json", "--execution-requirements")
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, `"maxVUs": 2`)
	assert.Contains(t, stdout, `"totalDuration": "10m30s"`)
	assert.Contains(t, stdout, `"exec": "api"`)
}

func TestInspectOptionsFileDoesNotReadScript(t *testing.T) {
	t.Parallel()

	for name, script := range map[string]string{
		"unreachable remote module": `import x from 'https://127.0.0.1:1/x.js'; export default function () {}`,
		"unknown extension":         `import x from 'k6/x/doesnotexist'; export default function () {}`,
		"secret in init": `import secrets from 'k6/secrets';
			const s = await secrets.get('missing');
			export default function () {}`,
		"throw in init": `throw new Error('init ran'); export default function () {}`,
		"syntax error":  `export default function ( {`,
	} {
		for flagsName, flags := range map[string][]string{"options": {}, "requirements": {"--execution-requirements"}} {
			t.Run(name+"/"+flagsName, func(t *testing.T) {
				t.Parallel()

				args := append(append([]string{"inspect", "--options-file", "opts.json"}, flags...), "test.js")
				ts := newOptionsFileTestState(t, script, optsFileJSON, 0, args...)
				cmd.ExecuteWithGlobalState(ts.GlobalState)

				assert.Contains(t, ts.Stdout.String(), `"exec": "api"`)
			})
		}
	}
}

func TestInspectOptionsFileValidation(t *testing.T) {
	t.Parallel()

	t.Run("checks that need init are skipped", func(t *testing.T) {
		t.Parallel()

		options := `{
			"scenarios": { "s": { "executor": "shared-iterations", "exec": "nope" } },
			"thresholds": { "my_custom": ["count>0"] }
		}`
		ts := newOptionsFileTestState(t, optsFileScript, options, 0,
			"inspect", "--options-file", "opts.json", "--execution-requirements")
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		assert.Contains(t, ts.Stdout.String(), `"maxVUs": 1`)
	})

	t.Run("invalid threshold expression", func(t *testing.T) {
		t.Parallel()

		ts := newOptionsFileTestState(t, optsFileScript, `{"thresholds": {"iterations": ["foo"]}}`,
			exitcodes.InvalidConfig, "inspect", "--options-file", "opts.json", "--execution-requirements")
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		assert.True(t, testutils.LogContains(ts.LoggerHook.Drain(), logrus.ErrorLevel, `"foo"`))
	})
}

func TestOptionsFileErrors(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		options string
		wantErr string // %s is the quoted path of the file
	}{
		"missing file":  {"", `couldn't read the options file %s`},
		"empty file":    {" \n", `the options file %s is empty`},
		"invalid JSON":  {`{"vus": }`, `couldn't parse the options file %s: invalid character '}'`},
		"unknown field": {`{"vus": 1, "vuz": 2}`, `couldn't parse the options file %s: json: unknown field "vuz"`},
		"type error": {
			`{"tags": ["a"]}`,
			`couldn't parse the options file %s: parsing options from options file got error ` +
				`while parsing "{\"tags\": [": json: cannot unmarshal array into Go struct field Options.tags`,
		},
	} {
		for _, command := range []string{"inspect", "run", "archive"} {
			t.Run(name+"/"+command, func(t *testing.T) {
				t.Parallel()

				ts := newOptionsFileTestState(t, optsFileScript, tc.options, exitcodes.InvalidConfig,
					command, "--options-file", "opts.json", "test.js")
				cmd.ExecuteWithGlobalState(ts.GlobalState)

				wantErr := fmt.Sprintf(tc.wantErr, strconv.Quote(filepath.Join(ts.Cwd, "opts.json")))
				assert.True(t, testutils.LogContains(ts.LoggerHook.Drain(), logrus.ErrorLevel, wantErr), wantErr)
			})
		}
	}
}

func TestOptionsFileFlagRepeated(t *testing.T) {
	t.Parallel()

	ts := newOptionsFileTestState(t, optsFileScript, optsFileJSON, 0,
		"inspect", "--options-file", "opts.json", "--options-file", "other.json")
	ts.ExpectedExitCode = -1 // like other flag errors
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.True(t, testutils.LogContains(ts.LoggerHook.Drain(), logrus.ErrorLevel, "can be passed only once"))
}

func TestOptionsFileFeaturesWarning(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"inspect", "run"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			ts := newOptionsFileTestState(t, optsFileScript, `{"features": ["native-histograms"]}`, 0,
				command, "--options-file", "opts.json", "test.js")
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			assert.True(t, testutils.LogContains(ts.LoggerHook.Drain(), logrus.WarnLevel,
				"Feature flags in the options file are not supported"))
		})
	}
}

const optsFileAPIRan = "ran api FROM=file team=core origin=file"

func TestRunOptionsFile(t *testing.T) {
	t.Parallel()

	for name, scriptArg := range map[string]string{"script file": "test.js", "script from stdin": "-"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ts := newOptionsFileTestState(t, optsFileScript, optsFileJSON, 0,
				"run", "--log-output=stdout", "--options-file", "opts.json", scriptArg)
			ts.Stdin = bytes.NewBufferString(optsFileScript)
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			stdout := ts.Stdout.String()
			assert.Contains(t, stdout, "4 iterations shared among 2 VUs")
			assert.Equal(t, 4, strings.Count(stdout, optsFileAPIRan))
			assert.NotContains(t, stdout, "ran default")
		})
	}
}

func TestRunOptionsFilePrecedence(t *testing.T) {
	t.Parallel()

	script := `
		export const options = { vus: 10, iterations: 20 };
		export default function () {}
	`
	for name, tc := range map[string]struct {
		config string
		env    map[string]string
		flags  []string
		want   string
	}{
		"options file":      {want: "6 iterations shared among 2 VUs"},
		"-c below the file": {config: `{"vus": 5, "iterations": 9}`, want: "6 iterations shared among 2 VUs"},
		"K6_VUS above":      {env: map[string]string{"K6_VUS": "3"}, want: "6 iterations shared among 3 VUs"},
		"--vus above":       {flags: []string{"--vus", "4"}, want: "6 iterations shared among 4 VUs"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			args := append([]string{"run", "--options-file", "opts.json"}, tc.flags...)
			ts := newOptionsFileTestState(t, script, `{"vus": 2, "iterations": 6}`, 0, append(args, "test.js")...)
			if tc.config != "" {
				require.NoError(t, ts.FS.MkdirAll(filepath.Dir(ts.Flags.ConfigFilePath), 0o755))
				require.NoError(t, fsext.WriteFile(ts.FS, ts.Flags.ConfigFilePath, []byte(tc.config), 0o644))
			}
			maps.Copy(ts.Env, tc.env)
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			assert.Contains(t, ts.Stdout.String(), tc.want)
		})
	}
}

func TestRunOptionsFileOnce(t *testing.T) {
	t.Parallel()

	// vus is a shortcut next to the scenario, which --once drops from the file layer.
	options := strings.Replace(optsFileJSON, `"scenarios"`, `"vus": 3, "scenarios"`, 1)
	options = strings.Replace(options, `"count==4"`, `"count==1"`, 1)
	ts := newOptionsFileTestState(t, optsFileScript, options, 0,
		"run", "--log-output=stdout", "--options-file", "opts.json", "--once", "test.js")
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, `--once overrode vus in \"options file\" configuration`)
	assert.Contains(t, stdout, "1 iterations shared among 1 VUs")
	assert.Equal(t, 1, strings.Count(stdout, optsFileAPIRan))
}

func TestRunOptionsFileScenarioNotFound(t *testing.T) {
	t.Parallel()

	ts := newOptionsFileTestState(t, optsFileScript, `{"vus": 2}`, exitcodes.InvalidConfig,
		"run", "--log-output=stdout", "--options-file", "opts.json", "--scenario", "api", "test.js")
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, `--scenario overrode vus in \"options file\" configuration`)
	assert.Contains(t, stdout, `scenario \"api\" not found; the options file does not have any named scenario`)
}

func TestRunOptionsFileWrongExec(t *testing.T) {
	t.Parallel()

	options := `{"scenarios": {"s": {"executor": "shared-iterations", "exec": "nope"}}}`
	ts := newOptionsFileTestState(t, optsFileScript, options, exitcodes.InvalidConfig,
		"run", "--log-output=stdout", "--options-file", "opts.json", "test.js")
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, "There were problems with the specified options file configuration")
	assert.Contains(t, stdout, "executor s: function 'nope' not found in exports")
	assert.NotContains(t, stdout, "ran ")
}

func TestOptionsFileRejectsArchive(t *testing.T) {
	t.Parallel()

	tarData := buildArchive(t, optsFileScript)
	for _, command := range []string{"run", "archive"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			ts := newOptionsFileTestState(t, optsFileScript, optsFileJSON, exitcodes.InvalidConfig,
				command, "--options-file", "opts.json", "archive.tar")
			require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), tarData, 0o644))
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			assert.True(t, testutils.LogContains(ts.LoggerHook.Drain(), logrus.ErrorLevel,
				"--options-file can't be used with an archive"))
		})
	}
}

func TestArchiveOptionsFile(t *testing.T) {
	t.Parallel()

	ts := newOptionsFileTestState(t, optsFileScript, optsFileJSON, 0)
	tarPath := filepath.Join(ts.Cwd, "archive.tar")
	ts.CmdArgs = []string{
		"k6", "archive", "--options-file", "opts.json", "--max-redirects", "3", "-O", tarPath, "test.js",
	}
	ts.Env["K6_BATCH"] = "7"
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	tarData, err := fsext.ReadFile(ts.FS, tarPath)
	require.NoError(t, err)
	arc, err := lib.ReadArchive(bytes.NewReader(tarData))
	require.NoError(t, err)

	// The file's options, with env and CLI overrides, and nothing from the script.
	require.Contains(t, arc.Options.Scenarios, "api")
	assert.Equal(t, "api", arc.Options.Scenarios["api"].GetExec())
	assert.Equal(t, map[string]string{"origin": "file"}, arc.Options.RunTags)
	assert.Equal(t, null.IntFrom(7), arc.Options.Batch)
	assert.Equal(t, null.IntFrom(3), arc.Options.MaxRedirects)
	assert.False(t, arc.Options.VUs.Valid)
	assert.False(t, arc.Options.Duration.Valid)

	t.Run("run", func(t *testing.T) {
		t.Parallel()

		ts := newOptionsFileTestState(t, optsFileScript, "", 0, "run", "--log-output=stdout", "archive.tar")
		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), tarData, 0o644))
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, "4 iterations shared among 2 VUs")
		assert.Equal(t, 4, strings.Count(stdout, optsFileAPIRan))
		// The archive's script exports options too, but without the flag there is nothing to report.
		assert.NotContains(t, stdout, "--options-file")
	})

	for name, flags := range map[string][]string{
		"inspect":                        {},
		"inspect execution requirements": {"--execution-requirements"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			args := append(append([]string{"inspect"}, flags...), "archive.tar")
			ts := newOptionsFileTestState(t, optsFileScript, "", 0, args...)
			require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), tarData, 0o644))
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			var got lib.Options
			require.NoError(t, json.Unmarshal(ts.Stdout.Bytes(), &got))
			require.Contains(t, got.Scenarios, "api")
			assert.Equal(t, "api", got.Scenarios["api"].GetExec())
			assert.Equal(t, null.IntFrom(7), got.Batch)
			assert.False(t, got.VUs.Valid)
		})
	}
}

func TestRunOptionsFileIgnoresScriptOptions(t *testing.T) {
	t.Parallel()

	const functions = "export default function () {}\nexport function api() {}\n"
	const ignored = "The script's options are ignored because of --options-file"
	for name, tc := range map[string]struct {
		export string
		warns  bool
	}{
		// Without the flag, the next three exports fail the test or warn.
		"undecodable":      {"export const options = { tags: ['a'] };\n", true},
		"function":         {"export function options() {}\n", true},
		"unknown fields":   {"export const options = { myCustom: 1 };\n", true},
		"no export":        {"", false},
		"undefined export": {"export let options;\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ts := newOptionsFileTestState(t, tc.export+functions, optsFileJSON, 0,
				"run", "--options-file", "opts.json", "--no-thresholds", "test.js")
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			assert.Contains(t, ts.Stdout.String(), "4 iterations shared among 2 VUs")
			warnings := testutils.FilterEntries(ts.LoggerHook.Drain(), logrus.WarnLevel, "options")
			if !tc.warns {
				assert.Empty(t, warnings)
				return
			}
			require.Len(t, warnings, 1)
			assert.Equal(t, ignored, warnings[0].Message)
		})
	}
}
