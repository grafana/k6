package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runAsync runs the body of an async function and returns what it resolved to, or the name of
// the error it was rejected with, prefixed with "rejected: ".
func runAsync(t *testing.T, body string) string {
	t.Helper()

	ts := newConfiguredRuntime(t)

	var result string
	err := ts.EventLoop.Start(func() error {
		rt := ts.VU.Runtime()
		require.NoError(t, rt.Set("report", func(s string) { result = s }))

		_, err := rt.RunString(`(async () => {` + body + `})().then(
			(v) => report(String(v)),
			(e) => report("rejected: " + e.name),
		)`)
		return err
	})
	require.NoError(t, err)

	return result
}

// These cases cover behaviors of the Ed25519 and X25519 implementations that the Web Platform
// Tests don't exercise.
func TestOKPKeys(t *testing.T) {
	t.Parallel()

	const ed25519JWK = `
		const pair = await crypto.subtle.generateKey("Ed25519", true, ["sign", "verify"]);
		const jwk = await crypto.subtle.exportKey("jwk", pair.privateKey);
	`

	testCases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "exported raw Ed25519 public key doesn't share memory with the key",
			body: `
				const pair = await crypto.subtle.generateKey("Ed25519", true, ["sign", "verify"]);
				const data = new Uint8Array([1, 2, 3]);
				const signature = await crypto.subtle.sign("Ed25519", pair.privateKey, data);

				const exported = new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey));
				const original = exported.slice();
				exported.fill(0);

				const again = new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey));
				const unchanged = again.every((b, i) => b === original[i]);
				const verified = await crypto.subtle.verify("Ed25519", pair.publicKey, signature, data);
				return unchanged + " " + verified;
			`,
			want: "true true",
		},
		{
			name: "X25519 is not an ECDH named curve",
			body: `
				const pair = await crypto.subtle.generateKey("X25519", true, ["deriveBits"]);
				const raw = await crypto.subtle.exportKey("raw", pair.publicKey);
				await crypto.subtle.importKey("raw", raw, { name: "ECDH", namedCurve: "X25519" }, true, []);
				return "imported";
			`,
			want: "rejected: NotSupportedError",
		},
		{
			name: "JWK with ext false can't be imported as extractable",
			body: ed25519JWK + `
				jwk.ext = false;
				await crypto.subtle.importKey("jwk", jwk, "Ed25519", true, ["sign"]);
				return "imported";
			`,
			want: "rejected: DataError",
		},
		{
			name: "JWK with ext false can be imported as non-extractable",
			body: ed25519JWK + `
				jwk.ext = false;
				const key = await crypto.subtle.importKey("jwk", jwk, "Ed25519", false, ["sign"]);
				return key.type + " " + key.extractable;
			`,
			want: "private false",
		},
		{
			name: "JWK with empty key_ops doesn't allow any usage",
			body: ed25519JWK + `
				jwk.key_ops = [];
				await crypto.subtle.importKey("jwk", jwk, "Ed25519", true, ["sign"]);
				return "imported";
			`,
			want: "rejected: DataError",
		},
		{
			name: "JWK with empty key_ops and no usages",
			body: `
				const pair = await crypto.subtle.generateKey("X25519", true, ["deriveBits"]);
				const jwk = await crypto.subtle.exportKey("jwk", pair.publicKey);
				jwk.key_ops = [];
				const key = await crypto.subtle.importKey("jwk", jwk, "X25519", true, []);
				return key.type;
			`,
			want: "public",
		},
		{
			name: "JWK with an empty d is not a public key",
			body: ed25519JWK + `
				jwk.d = "";
				delete jwk.key_ops;
				await crypto.subtle.importKey("jwk", jwk, "Ed25519", true, ["sign"]);
				return "imported";
			`,
			want: "rejected: DataError",
		},
		{
			name: "X25519 deriveBits rejects a base key of another algorithm",
			body: `
				const x25519 = await crypto.subtle.generateKey("X25519", true, ["deriveBits"]);
				const ecdh = await crypto.subtle.generateKey(
					{ name: "ECDH", namedCurve: "P-256" }, true, ["deriveBits"],
				);
				await crypto.subtle.deriveBits(
					{ name: "X25519", public: x25519.publicKey }, ecdh.privateKey, 256,
				);
				return "derived";
			`,
			want: "rejected: InvalidAccessError",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, runAsync(t, tc.body))
		})
	}
}
