package webcrypto_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/cmd"
	k6Tests "go.k6.io/k6/v2/internal/cmd/tests"
	"go.k6.io/k6/v2/lib/fsext"
)

func getSingleFileTestState(tb testing.TB, script string, cliFlags []string) *k6Tests.GlobalTestState {
	if cliFlags == nil {
		cliFlags = []string{"-v", "--log-output=stdout"}
	}

	ts := k6Tests.NewGlobalTestState(tb)
	require.NoError(tb, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(script), 0o644))
	ts.CmdArgs = append(append([]string{"k6", "run"}, cliFlags...), "test.js")
	ts.ExpectedExitCode = 0

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

				ts := getSingleFileTestState(t, string(script), []string{"-v", "--log-output=stdout"})

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

func TestDigestBufferSources(t *testing.T) {
	t.Parallel()

	const script = `
function hex(buffer) {
    return Array.from(new Uint8Array(buffer), byte => byte.toString(16).padStart(2, "0")).join("");
}

async function expectDigest(input, expectedBytes, label) {
    const actual = await crypto.subtle.digest("SHA-256", input);
    const expected = await crypto.subtle.digest("SHA-256", new Uint8Array(expectedBytes).buffer);
    if (hex(actual) !== hex(expected)) {
        throw new Error(label + " digested the wrong bytes");
    }
}

export default async function () {
    const bytes = new Uint8Array([9, 1, 2, 3, 8]);
    await expectDigest(bytes.buffer, [9, 1, 2, 3, 8], "ArrayBuffer");
    await expectDigest(bytes.subarray(1, 4), [1, 2, 3], "Uint8Array subarray");
    await expectDigest(new DataView(bytes.buffer, 1, 3), [1, 2, 3], "DataView");
    await expectDigest(bytes.subarray(3, 3), [], "empty view");
    await expectDigest(new DataView(bytes.buffer, 4, 0), [], "empty DataView");

    const arrayTypes = [
        Int8Array, Uint8Array, Uint8ClampedArray, Int16Array, Uint16Array,
        Int32Array, Uint32Array, Float32Array, Float64Array, BigInt64Array, BigUint64Array,
    ];
    for (const ArrayType of arrayTypes) {
        const width = ArrayType.BYTES_PER_ELEMENT;
        const buffer = new ArrayBuffer(width * 4);
        const raw = new Uint8Array(buffer);
        raw.forEach((_, index) => { raw[index] = index + 1; });
        await expectDigest(
            new ArrayType(buffer, width, 2),
            raw.slice(width, width * 3),
            ArrayType.name,
        );
    }

    const snapshot = new Uint8Array([9, 1, 2, 3, 8]);
    const pending = crypto.subtle.digest("SHA-256", snapshot.subarray(1, 4));
    snapshot.fill(0);
    const expected = await crypto.subtle.digest("SHA-256", new Uint8Array([1, 2, 3]));
    if (hex(await pending) !== hex(expected)) {
        throw new Error("digest did not copy view bytes before returning");
    }

    ArrayBuffer.isView = () => false;
    await expectDigest(bytes.subarray(1, 4), [1, 2, 3], "overridden isView");
    const viewWithConstructorOverride = bytes.subarray(1, 4);
    viewWithConstructorOverride.constructor = Object;
    await expectDigest(viewWithConstructorOverride, [1, 2, 3], "overridden constructor");
    class CustomDataView extends DataView {}
    await expectDigest(new CustomDataView(bytes.buffer, 1, 3), [1, 2, 3], "DataView subclass");
    class CustomArrayBuffer extends ArrayBuffer {}
    const customBuffer = new CustomArrayBuffer(3);
    new Uint8Array(customBuffer).set([1, 2, 3]);
    await expectDigest(customBuffer, [1, 2, 3], "ArrayBuffer subclass");

    const spoofed = { constructor: Uint8Array, buffer: bytes.buffer };
    try {
        await crypto.subtle.digest("SHA-256", spoofed);
        throw new Error("constructor-spoofed input was accepted");
    } catch (error) {
        if (error.message === "constructor-spoofed input was accepted") {
            throw error;
        }
        if (error.name !== "TypeError") {
            throw new Error("constructor-spoofed input rejected with " + error.name);
        }
    }

    try {
        await crypto.subtle.digest("SHA-256", null);
        throw new Error("null input was accepted");
    } catch (error) {
        if (error.message === "null input was accepted") {
            throw error;
        }
        if (error.name !== "TypeError") {
            throw new Error("null input rejected with " + error.name);
        }
    }

    console.log("digest BufferSource checks passed");
}
`

	ts := getSingleFileTestState(t, script, []string{"--quiet", "--no-color", "--log-output=stdout"})
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	require.Contains(t, stdout, "digest BufferSource checks passed")
	assert.NotContains(t, stdout, "Uncaught")
	assert.Empty(t, ts.Stderr.String())
}

func TestSubtleCryptoRejectsInvalidBufferSources(t *testing.T) {
	t.Parallel()

	const script = `
async function expectTypeError(label, operation) {
    const pending = operation();
    if (!pending || typeof pending.then !== "function") {
        throw new Error(label + " did not return a Promise");
    }
    try {
        await pending;
    } catch (error) {
        if (error.name === "TypeError") {
            return;
        }
        throw new Error(label + " rejected with " + error.name + " instead of TypeError");
    }
    throw new Error(label + " was accepted");
}

export default async function () {
    const data = new Uint8Array([1, 2, 3, 4]);
    const hmacKey = await crypto.subtle.importKey(
        "raw", data, { name: "HMAC", hash: "SHA-256" }, false, ["sign", "verify"]);
    const aesKey = await crypto.subtle.importKey(
        "raw", new Uint8Array(16), "AES-GCM", false, ["encrypt", "decrypt"]);
    const iv = new Uint8Array(12);
    const signature = new Uint8Array(await crypto.subtle.sign("HMAC", hmacKey, data));
    const ciphertext = new Uint8Array(await crypto.subtle.encrypt({ name: "AES-GCM", iv }, aesKey, data));
    const operations = [
        ["digest", data, input => crypto.subtle.digest("SHA-256", input)],
        ["importKey", data, input => crypto.subtle.importKey(
            "raw", input, { name: "HMAC", hash: "SHA-256" }, false, ["sign"])],
        ["encrypt", data, input => crypto.subtle.encrypt({ name: "AES-GCM", iv }, aesKey, input)],
        ["decrypt", ciphertext, input => crypto.subtle.decrypt({ name: "AES-GCM", iv }, aesKey, input)],
        ["sign", data, input => crypto.subtle.sign("HMAC", hmacKey, input)],
        ["verify signature", signature, input => crypto.subtle.verify("HMAC", hmacKey, input, data)],
        ["verify data", data, input => crypto.subtle.verify("HMAC", hmacKey, signature, input)],
    ];
    const invalidInputs = [
        ["plain object", {}],
        ["constructor spoof", { constructor: Uint8Array, buffer: data.buffer }],
        ["invalid buffer spoof", { constructor: Uint8Array, buffer: 42 }],
        ["missing buffer spoof", { constructor: Uint8Array }],
        ["null", null],
        ["undefined", undefined],
    ];

    for (const [name, validInput, operation] of operations) {
        await operation(validInput);
        for (const [label, input] of invalidInputs) {
            await expectTypeError(name + " / " + label, () => operation(input));
        }
    }
    console.log("invalid BufferSource checks passed");
}
`

	ts := getSingleFileTestState(t, script, []string{"--quiet", "--no-color", "--log-output=stdout"})
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	require.Contains(t, stdout, "invalid BufferSource checks passed")
	assert.NotContains(t, stdout, "Uncaught")
	assert.Empty(t, ts.Stderr.String())
}

func TestVerifyBufferSourceViews(t *testing.T) {
	t.Parallel()

	const script = `
export default async function () {
    const key = await crypto.subtle.importKey(
        "raw", new Uint8Array([1, 2, 3, 4]), { name: "HMAC", hash: "SHA-256" }, false, ["sign", "verify"]);
    const data = new Uint8Array([5, 6, 7, 8]);
    const signature = new Uint8Array(await crypto.subtle.sign("HMAC", key, data));
    const paddedSignature = new Uint8Array(signature.length + 2);
    paddedSignature.set(signature, 1);
    const signatureView = paddedSignature.subarray(1, signature.length + 1);

    if (!await crypto.subtle.verify("HMAC", key, signatureView, data)) {
        throw new Error("signature subarray did not verify");
    }

    const paddedData = new Uint8Array([0, 5, 6, 7, 8, 0]);
    if (!await crypto.subtle.verify("HMAC", key, signature, new DataView(paddedData.buffer, 1, 4))) {
        throw new Error("data view did not verify");
    }

    const truncated = signature.subarray(0, signature.byteLength - 1);
    if (await crypto.subtle.verify("HMAC", key, truncated, data)) {
        throw new Error("truncated signature subarray verified");
    }

    console.log("verify BufferSource checks passed");
}
`

	ts := getSingleFileTestState(t, script, []string{"--quiet", "--no-color", "--log-output=stdout"})
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	require.Contains(t, stdout, "verify BufferSource checks passed")
	assert.NotContains(t, stdout, "Uncaught")
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
