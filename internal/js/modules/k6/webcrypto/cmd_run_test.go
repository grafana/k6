package webcrypto_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/cmd"
	k6Tests "go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/lib/fsext"
)

func getSingleFileTestState(tb testing.TB, script string, cliFlags []string, expExitCode exitcodes.ExitCode) *k6Tests.GlobalTestState {
	if cliFlags == nil {
		cliFlags = []string{"-v", "--log-output=stdout"}
	}

	ts := k6Tests.NewGlobalTestState(tb)
	require.NoError(tb, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(script), 0o644))
	ts.CmdArgs = append(append([]string{"k6", "run"}, cliFlags...), "test.js")
	ts.ExpectedExitCode = int(expExitCode)

	return ts
}

// TestExamplesInputOutput runs same k6's scripts that we have in example folder
// it check that output contains/not contains cetane things
// it's not a real test, but it's a good way to check that examples are working
// between changes
//
// We also do use a convention that successful output should contain `level=info` (at least one info message from console.log), e.g.:
// INFO[0000] deciphered text == original text:  true       source=console
// and should not contain `level=error` or "Uncaught", e.g. outputs like:
// ERRO[0000] Uncaught (in promise) OperationError: length is too large  executor=per-vu-iterations scenario=default
func TestExamplesInputOutput(t *testing.T) {
	t.Parallel()

	outputShouldContain := []string{
		"output: -",
		"default: 1 iterations for each of 1 VUs",
		"1 complete and 0 interrupted iterations",
		"level=info", // at least one info message
	}

	outputShouldNotContain := []string{
		"Uncaught",
		"level=error", // no error messages
	}

	const examplesDir = "../../../../../examples/webcrypto"

	// List of the directories containing the examples
	// that we should run and check that they produce the expected output
	// and not the unexpected one
	// it could be a file (ending with .js) or a directory
	examples := []string{
		examplesDir + "/digest.js",
		examplesDir + "/getRandomValues.js",
		examplesDir + "/randomUUID.js",
		examplesDir + "/generateKey",
		examplesDir + "/derive_bits",
		examplesDir + "/encrypt_decrypt",
		examplesDir + "/sign_verify",
		examplesDir + "/import_export",
	}

	for _, path := range examples {
		list := getFiles(t, path)

		for _, file := range list {
			name := filepath.Base(file)
			file := file

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				script, err := os.ReadFile(filepath.Clean(file)) //nolint:forbidigo // we read an example directly
				require.NoError(t, err)

				ts := getSingleFileTestState(t, string(script), []string{"-v", "--log-output=stdout"}, 0)

				cmd.ExecuteWithGlobalState(ts.GlobalState)

				stdout := ts.Stdout.String()

				for _, s := range outputShouldContain {
					assert.Contains(t, stdout, s)
				}
				for _, s := range outputShouldNotContain {
					assert.NotContains(t, stdout, s)
				}

				assert.Empty(t, ts.Stderr.String())
			})
		}
	}
}

func getFiles(t *testing.T, path string) []string {
	t.Helper()

	result := []string{}

	// If the path is a file, return it as is
	if strings.HasSuffix(path, ".js") {
		return append(result, path)
	}

	// If the path is a directory, return all the files in it
	list, err := os.ReadDir(path) //nolint:forbidigo // we read a directory
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	for _, file := range list {
		if file.IsDir() {
			continue
		}

		result = append(result, filepath.Join(path, file.Name()))
	}

	return result
}
