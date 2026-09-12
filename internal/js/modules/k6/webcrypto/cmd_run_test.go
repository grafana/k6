package webcrypto_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/cmd"
	k6Tests "go.k6.io/k6/v2/internal/cmd/tests"
	"go.k6.io/k6/v2/lib/fsext"
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

// TestGetRandomValuesBehavior verifies the element-width, input-validation and
// quota behavior of crypto.getRandomValues (#6318, #6320).
func TestGetRandomValuesBehavior(t *testing.T) {
	t.Parallel()

	const script = `
function assertThrowsTypeError(value, label) {
    try {
        crypto.getRandomValues(value);
    } catch (error) {
        if (error.name !== "TypeError") {
            throw new Error(label + " threw " + error.name + " instead of TypeError");
        }
        return;
    }
    throw new Error(label + " was accepted");
}

function assertThrowsWebCryptoError(value, expectedErrorName, label) {
    try {
        crypto.getRandomValues(value);
    } catch (error) {
        if (!String(error.message).includes(expectedErrorName)) {
            throw new Error(label + " threw " + error.name + ": " + error.message);
        }
        return;
    }
    throw new Error(label + " was accepted");
}

export default function () {
    // Every element of a wide view must carry its full width of random bits
    // (#6318): across 1000 * 8 uint32 draws, all fitting in 8 bits is
    // impossible in practice unless only one byte per element is random.
    let max32 = 0;
    for (let i = 0; i < 1000; i++) {
        const a = new Uint32Array(8);
        crypto.getRandomValues(a);
        for (const v of a) {
            if (v > 0xffffffff || v < 0) {
                throw new Error("Uint32Array element out of range: " + v);
            }
            max32 = Math.max(max32, v);
        }
    }
    if (max32 <= 255) {
        throw new Error("Uint32Array values only carry 8 random bits: max was " + max32);
    }

    let sawWide16 = false;
    for (let i = 0; i < 200; i++) {
        const a = new Uint16Array(16);
        crypto.getRandomValues(a);
        for (const v of a) {
            if (v > 0xffff || v < 0) {
                throw new Error("Uint16Array element out of range: " + v);
            }
            if (v > 255) {
                sawWide16 = true;
            }
        }
    }
    if (!sawWide16) {
        throw new Error("Uint16Array values only carry 8 random bits");
    }

    // 8-bit views keep their ranges.
    const i8 = new Int8Array(64);
    crypto.getRandomValues(i8);
    for (const v of i8) {
        if (v < -128 || v > 127) {
            throw new Error("Int8Array element out of range: " + v);
        }
    }
    const u8 = new Uint8Array(64);
    crypto.getRandomValues(u8);
    for (const v of u8) {
        if (v < 0 || v > 255) {
            throw new Error("Uint8Array element out of range: " + v);
        }
    }
    const clamped = new Uint8ClampedArray(64);
    crypto.getRandomValues(clamped);
    for (const v of clamped) {
        if (v < 0 || v > 255) {
            throw new Error("Uint8ClampedArray element out of range: " + v);
        }
    }
    if (crypto.getRandomValues(u8) !== u8) {
        throw new Error("getRandomValues must return the view it filled");
    }

    // A missing argument and an unsupported value throw catchable errors
    // instead of taking the process down (#6320).
    assertThrowsTypeError(undefined, "missing argument");
    assertThrowsTypeError(null, "null argument");
    assertThrowsWebCryptoError({}, "TypeMismatchError", "plain object");
    assertThrowsWebCryptoError(" Uint8Array", "TypeMismatchError", "string");
    assertThrowsWebCryptoError(new Float64Array(4), "TypeMismatchError", "Float64Array");

    // Overridden JS-visible properties are not trusted: the real view length
    // is used and the call neither crashes nor throws (#6320).
    const overridden = new Uint8Array(4);
    Object.defineProperty(overridden, "length", { value: -1 });
    crypto.getRandomValues(overridden);
    for (let i = 0; i < 4; i++) {
        if (typeof overridden[i] !== "number") {
            throw new Error("element " + i + " was not filled after length override");
        }
    }

    // The quota is on the view's byteLength (#6318): exactly 65536 bytes is
    // allowed, anything above it throws a catchable QuotaExceededError.
    crypto.getRandomValues(new Uint8Array(65536));
    assertThrowsWebCryptoError(new Uint8Array(65537), "QuotaExceededError", "65537 Uint8 bytes");
    assertThrowsWebCryptoError(new Uint32Array(16385), "QuotaExceededError", "16385 Uint32 elements");

    console.log("getRandomValues behavior ok, widest uint32 seen: " + max32);
}
`
	ts := getSingleFileTestState(t, script, []string{"-v", "--log-output=stdout"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, "getRandomValues behavior ok")
	assert.NotContains(t, stdout, "Uncaught")
	assert.NotContains(t, stdout, "panic")
	assert.Empty(t, ts.Stderr.String())
}

func getFiles(t *testing.T, path string) []string {	t.Helper()

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
