package tests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDigestUndefinedAlgorithmDoesNotPanic(t *testing.T) {
	t.Parallel()

	rt := newConfiguredRuntime(t)

	_, err := rt.RunOnEventLoop(`
		crypto.subtle.digest(undefined, new Uint8Array([1, 2, 3])).then(
			() => { throw new Error("digest should reject a missing algorithm"); },
			(e) => {
				if (!String(e).includes("algorithm")) {
					throw new Error("expected algorithm TypeError, got: " + e);
				}
			}
		);
	`)
	require.NoError(t, err)
}

func TestDigestNullAlgorithmDoesNotPanic(t *testing.T) {
	t.Parallel()

	rt := newConfiguredRuntime(t)

	_, err := rt.RunOnEventLoop(`
		crypto.subtle.digest(null, new Uint8Array([1, 2, 3])).then(
			() => { throw new Error("digest should reject a null algorithm"); },
			(e) => {
				if (!String(e).includes("algorithm")) {
					throw new Error("expected algorithm TypeError, got: " + e);
				}
			}
		);
	`)
	require.NoError(t, err)
}

func TestGenerateKeyUndefinedAlgorithmDoesNotPanic(t *testing.T) {
	t.Parallel()

	rt := newConfiguredRuntime(t)

	_, err := rt.RunOnEventLoop(`
		crypto.subtle.generateKey(undefined, true, ["encrypt", "decrypt"]).then(
			() => { throw new Error("generateKey should reject a missing algorithm"); },
			(e) => {
				if (!String(e).includes("algorithm")) {
					throw new Error("expected algorithm TypeError, got: " + e);
				}
			}
		);
	`)
	require.NoError(t, err)
}
