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


// TestAlgorithmParamsSnapshot verifies that BufferSource algorithm parameters
// (iv, counter, additionalData, label, salt) are copied when the call is made,
// not read after the returned promise settles (#6319).
func TestAlgorithmParamsSnapshot(t *testing.T) {
	t.Parallel()

	const script = `
const hex = (b) => Array.from(new Uint8Array(b), (x) => x.toString(16).padStart(2, "0")).join("");

async function importAesKey(name, usages) {
    return await crypto.subtle.importKey(
        "raw", new Uint8Array(16).fill(7), { name }, false, usages,
    );
}

export default async function () {
    const data = new Uint8Array([1, 2, 3, 4]);

    // The exact shape of #6319: mutating the iv while the promise is pending
    // must not change the ciphertext — the all-zero iv snapshot must win.
    const key = await importAesKey("AES-GCM", ["encrypt"]);
    const iv = new Uint8Array(12);
    const pending = crypto.subtle.encrypt({ name: "AES-GCM", iv }, key, data);
    iv.fill(0xff);
    const mutatedIvResult = hex(await pending);
    const zeroIvResult = hex(await crypto.subtle.encrypt(
        { name: "AES-GCM", iv: new Uint8Array(12) }, key, data,
    ));
    if (mutatedIvResult !== zeroIvResult) {
        throw new Error("encrypt used the mutated iv, not the snapshot");
    }

    // Same property for AES-GCM additionalData.
    const aad = new Uint8Array(8);
    const aadPending = crypto.subtle.encrypt(
        { name: "AES-GCM", iv: new Uint8Array(12), additionalData: aad }, key, data,
    );
    aad.fill(0xee);
    const mutatedAadResult = hex(await aadPending);
    const zeroAadResult = hex(await crypto.subtle.encrypt(
        { name: "AES-GCM", iv: new Uint8Array(12), additionalData: new Uint8Array(8) }, key, data,
    ));
    if (mutatedAadResult !== zeroAadResult) {
        throw new Error("encrypt used the mutated additionalData, not the snapshot");
    }

    // Same property for the AES-CTR counter.
    const counterKey = await importAesKey("AES-CTR", ["encrypt"]);
    const counter = new Uint8Array(16);
    const counterPending = crypto.subtle.encrypt(
        { name: "AES-CTR", counter, length: 64 }, counterKey, new Uint8Array([5, 6]),
    );
    counter.fill(0xab);
    const counterMutated = hex(await counterPending);
    const counterControl = hex(await crypto.subtle.encrypt(
        { name: "AES-CTR", counter: new Uint8Array(16), length: 64 }, counterKey, new Uint8Array([5, 6]),
    ));
    if (counterMutated !== counterControl) {
        throw new Error("AES-CTR used the mutated counter, not the snapshot");
    }

    // Reusing the same buffers across calls must re-snapshot: after the
    // buffers change, the next call uses the new bytes, not the first
    // call's snapshot.
    const sharedIv = new Uint8Array(12);
    const sharedAlgorithm = { name: "AES-GCM", iv: sharedIv };
    const firstShared = hex(await crypto.subtle.encrypt(sharedAlgorithm, key, data));
    sharedIv.fill(0x5a);
    const secondShared = hex(await crypto.subtle.encrypt(sharedAlgorithm, key, data));
    const sharedControl = hex(await crypto.subtle.encrypt(
        { name: "AES-GCM", iv: new Uint8Array(12).fill(0x5a) }, key, data,
    ));
    if (secondShared !== sharedControl || secondShared === firstShared) {
        throw new Error("a reused iv buffer was not re-snapshotted");
    }

    // PBKDF2's salt is derived against later, in the callback goroutine.
    const baseKey = await crypto.subtle.importKey(
        "raw", new Uint8Array(16).fill(3), { name: "PBKDF2" }, false, ["deriveBits"],
    );
    const salt = new Uint8Array(16);
    const saltPending = crypto.subtle.deriveBits(
        { name: "PBKDF2", hash: "SHA-256", iterations: 1000, salt }, baseKey, 128,
    );
    salt.fill(0xff);
    const saltMutated = hex(await saltPending);
    const saltControl = hex(await crypto.subtle.deriveBits(
        { name: "PBKDF2", hash: "SHA-256", iterations: 1000, salt: new Uint8Array(16) },
        baseKey, 128,
    ));
    if (saltMutated !== saltControl) {
        throw new Error("deriveBits used the mutated salt, not the snapshot");
    }

    console.log("algorithm params snapshot ok");
}
`
	ts := getSingleFileTestState(t, script, []string{"-v", "--log-output=stdout"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, "algorithm params snapshot ok")
	assert.NotContains(t, stdout, "Uncaught")
	assert.NotContains(t, stdout, "panic")
	assert.Empty(t, ts.Stderr.String())
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
