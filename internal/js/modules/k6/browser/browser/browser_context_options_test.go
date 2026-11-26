package browser

import (
	"crypto/tls"
	"crypto/x509"
	"strings"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/v2/internal/lib/testutils/tlstest"
)

func TestBrowserContextOptionsPermissions(t *testing.T) {
	t.Parallel()
	vu := k6test.NewVU(t)

	opts, err := parseBrowserContextOptions(vu.Runtime(), vu.ToSobekValue((struct {
		Permissions []any `js:"permissions"`
	}{
		Permissions: []any{"camera", "microphone"},
	})))
	assert.NoError(t, err)
	assert.Len(t, opts.Permissions, 2)
	assert.Equal(t, []string{"camera", "microphone"}, opts.Permissions)
}

func TestBrowserContextSetGeolocation(t *testing.T) {
	t.Parallel()
	vu := k6test.NewVU(t)

	opts, err := parseBrowserContextOptions(vu.Runtime(), vu.ToSobekValue((struct {
		GeoLocation *common.Geolocation `js:"geolocation"`
	}{
		GeoLocation: &common.Geolocation{
			Latitude:  1.0,
			Longitude: 2.0,
			Accuracy:  3.0,
		},
	})))
	assert.NoError(t, err)
	assert.NotNil(t, opts)
	assert.Equal(t, 1.0, opts.Geolocation.Latitude)
	assert.Equal(t, 2.0, opts.Geolocation.Longitude)
	assert.Equal(t, 3.0, opts.Geolocation.Accuracy)
}

func TestBrowserContextDefaultOptions(t *testing.T) {
	t.Parallel()
	vu := k6test.NewVU(t)

	defaults := common.DefaultBrowserContextOptions()

	// gets the default options by default
	opts, err := parseBrowserContextOptions(vu.Runtime(), nil)
	require.NoError(t, err)
	assert.Equal(t, defaults, opts)

	// merges with the default options
	opts, err = parseBrowserContextOptions(vu.Runtime(), vu.ToSobekValue((struct {
		DeviceScaleFactor float64 `js:"deviceScaleFactor"` // just to test a different field
	}{
		DeviceScaleFactor: defaults.DeviceScaleFactor + 1,
	})))
	require.NoError(t, err)
	assert.NotEqual(t, defaults.DeviceScaleFactor, opts.DeviceScaleFactor)
	assert.Equal(t, defaults.Locale, opts.Locale) // should remain as default
}

func TestBrowserContextAllOptions(t *testing.T) {
	t.Parallel()
	vu := k6test.NewVU(t)
	opts, err := vu.Runtime().RunString(`const opts = {
			acceptDownloads: true,
			downloadsPath: '/tmp',
			bypassCSP: true,
			colorScheme: 'dark',
			deviceScaleFactor: 1,
			extraHTTPHeaders: {
				'X-Header': 'value',
			},
			geolocation: { latitude: 51.509865, longitude: -0.118092, accuracy: 1 },
			hasTouch: true,
			httpCredentials: { username: 'admin', password: 'password' },
			ignoreHTTPSErrors: true,
			isMobile: true,
			javaScriptEnabled: true,
			locale: 'fr-FR',
			offline: true,
			permissions: ['camera', 'microphone'],
			reducedMotion: 'no-preference',
			screen: { width: 800, height: 600 },
			timezoneID: 'Europe/Paris',
			userAgent: 'my agent',
			viewport: { width: 800, height: 600 },
		};
		opts;
	`)
	require.NoError(t, err)

	parsedOpts, err := parseBrowserContextOptions(vu.Runtime(), opts)
	require.NoError(t, err)

	assert.Equal(t, &common.BrowserContextOptions{
		AcceptDownloads:   true,
		DownloadsPath:     "/tmp",
		BypassCSP:         true,
		ColorScheme:       common.ColorSchemeDark,
		DeviceScaleFactor: 1,
		ExtraHTTPHeaders: map[string]string{
			"X-Header": "value",
		},
		Geolocation: &common.Geolocation{
			Latitude:  51.509865,
			Longitude: -0.118092,
			Accuracy:  1,
		},
		HasTouch: true,
		HTTPCredentials: common.Credentials{
			Username: "admin",
			Password: "password",
		},
		IgnoreHTTPSErrors: true,
		IsMobile:          true,
		JavaScriptEnabled: true,
		Locale:            "fr-FR",
		Offline:           true,
		Permissions:       []string{"camera", "microphone"},
		ReducedMotion:     common.ReducedMotionNoPreference,
		Screen: common.Screen{
			Width:  800,
			Height: 600,
		},
		TimezoneID: "Europe/Paris",
		UserAgent:  "my agent",
		Viewport: common.Viewport{
			Width:  800,
			Height: 600,
		},
	}, parsedOpts)
}

const clientCertSentinelPassphrase = "k6-sentinel-passphrase"

type clientCertFixtures struct {
	rsaCert, rsaKey, rsa2Cert, rsa2Key, ecCert, ecKey string
}

func newClientCertFixtures(t *testing.T) clientCertFixtures {
	t.Helper()

	chain := tlstest.NewChain(t)
	pemPair := func(cn string, kt tlstest.KeyType) (string, string) {
		cert, key := chain.ClientPEM(t, cn, kt)
		return string(cert), string(key)
	}
	var f clientCertFixtures
	f.rsaCert, f.rsaKey = pemPair("k6-test-pem-rsa", tlstest.KeyRSA)
	f.rsa2Cert, f.rsa2Key = pemPair("k6-test-pem-rsa-2", tlstest.KeyRSA)
	f.ecCert, f.ecKey = pemPair("k6-test-pem-ecdsa", tlstest.KeyECDSA)
	return f
}

// runClientCertOptions evaluates script with the fixtures bound as JS globals and returns its
// result. PEM values are strings; bytes(s) encodes an ASCII string as a Uint8Array.
func runClientCertOptions(t *testing.T, rt *sobek.Runtime, f clientCertFixtures, script string) sobek.Value {
	t.Helper()

	for name, v := range map[string]any{
		"rsaCert": f.rsaCert, "rsaKey": f.rsaKey,
		"rsa2Cert": f.rsa2Cert, "rsa2Key": f.rsa2Key,
		"ecCert": f.ecCert, "ecKey": f.ecKey,
		"sentinel": clientCertSentinelPassphrase,
	} {
		require.NoError(t, rt.Set(name, v))
	}
	_, err := rt.RunString(`function bytes(s) {
		const a = new Uint8Array(s.length);
		for (let i = 0; i < s.length; i++) a[i] = s.charCodeAt(i);
		return a;
	}`)
	require.NoError(t, err)
	v, err := rt.RunString(script)
	require.NoError(t, err)
	return v
}

func leafCNs(t *testing.T, certs []tls.Certificate) []string {
	t.Helper()

	cns := make([]string, 0, len(certs))
	for _, c := range certs {
		require.NotNil(t, c.Leaf)
		cns = append(cns, c.Leaf.Subject.CommonName)
	}
	return cns
}

func TestBrowserContextOptionsClientCertificates(t *testing.T) {
	t.Parallel()

	f := newClientCertFixtures(t)
	tests := []struct {
		name   string
		script string
		want   map[string][]string
	}{
		{
			name:   "pem_strings",
			script: `({clientCertificates: [{origin: 'https://x.test', cert: rsaCert, key: rsaKey}]})`,
			want:   map[string][]string{"https://x.test": {"k6-test-pem-rsa"}},
		},
		{
			name: "pem_array_buffer",
			script: `({clientCertificates: [{origin: 'https://x.test',
				cert: bytes(rsaCert).buffer, key: bytes(rsaKey).buffer}]})`,
			want: map[string][]string{"https://x.test": {"k6-test-pem-rsa"}},
		},
		{
			name: "pem_uint8array_subarray",
			script: `(() => {
				const sub = (s) => { const b = bytes('xx' + s + 'yy'); return b.subarray(2, b.length - 2); };
				return {clientCertificates: [{origin: 'https://x.test', cert: sub(rsaCert), key: sub(rsaKey)}]};
			})()`,
			want: map[string][]string{"https://x.test": {"k6-test-pem-rsa"}},
		},
		{
			name: "pem_uint16array",
			script: `(() => {
				const u16 = (s) => new Uint16Array(bytes(s.length % 2 ? s + '\n' : s).buffer);
				return {clientCertificates: [{origin: 'https://x.test', cert: u16(rsaCert), key: u16(rsaKey)}]};
			})()`,
			want: map[string][]string{"https://x.test": {"k6-test-pem-rsa"}},
		},
		{
			name: "pem_uint8array_after_isview_overwrite",
			script: `ArrayBuffer.isView = () => false;
				({clientCertificates: [{origin: 'https://x.test', cert: bytes(rsaCert), key: bytes(rsaKey)}]})`,
			want: map[string][]string{"https://x.test": {"k6-test-pem-rsa"}},
		},
		{
			name:   "origin_normalized",
			script: `({clientCertificates: [{origin: 'https://Example.com:443/x', cert: rsaCert, key: rsaKey}]})`,
			want:   map[string][]string{"https://example.com": {"k6-test-pem-rsa"}},
		},
		{
			name: "two_key_types_in_option_order",
			script: `({clientCertificates: [
				{origin: 'https://x.test', cert: ecCert, key: ecKey},
				{origin: 'https://x.test', cert: rsaCert, key: rsaKey},
			]})`,
			want: map[string][]string{"https://x.test": {"k6-test-pem-ecdsa", "k6-test-pem-rsa"}},
		},
		{
			name: "two_origins",
			script: `({clientCertificates: [
				{origin: 'https://a.test', cert: rsaCert, key: rsaKey},
				{origin: 'https://b.test:8443', cert: ecCert, key: ecKey},
			]})`,
			want: map[string][]string{
				"https://a.test":      {"k6-test-pem-rsa"},
				"https://b.test:8443": {"k6-test-pem-ecdsa"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vu := k6test.NewVU(t)

			opts, err := parseBrowserContextOptions(vu.Runtime(), runClientCertOptions(t, vu.Runtime(), f, tt.script))
			require.NoError(t, err)
			require.Len(t, opts.ClientCertificates, len(tt.want))
			for origin, cns := range tt.want {
				assert.Equal(t, cns, leafCNs(t, opts.ClientCertificates[origin]), origin)
			}
		})
	}

	t.Run("buffer_mutated_after_parse", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		rt := vu.Runtime()

		v := runClientCertOptions(t, rt, f, `const certBytes = bytes(rsaCert);
			({clientCertificates: [{origin: 'https://x.test', cert: certBytes, key: rsaKey}]})`)
		opts, err := parseBrowserContextOptions(rt, v)
		require.NoError(t, err)
		_, err = rt.RunString(`certBytes.fill(0)`)
		require.NoError(t, err)

		certs := opts.ClientCertificates["https://x.test"]
		require.Len(t, certs, 1)
		leaf, err := x509.ParseCertificate(certs[0].Certificate[0])
		require.NoError(t, err)
		assert.Equal(t, "k6-test-pem-rsa", leaf.Subject.CommonName)
	})

	for name, script := range map[string]string{"absent": `({})`, "empty_array": `({clientCertificates: []})`} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			vu := k6test.NewVU(t)

			opts, err := parseBrowserContextOptions(vu.Runtime(), runClientCertOptions(t, vu.Runtime(), f, script))
			require.NoError(t, err)
			assert.Empty(t, opts.ClientCertificates)
		})
	}
}

func TestBrowserContextOptionsClientCertificates_Validation(t *testing.T) {
	t.Parallel()

	const bytesErr = "expected string, ArrayBuffer or TypedArray"
	f := newClientCertFixtures(t)
	tests := []struct {
		name    string
		script  string
		wantErr string
	}{
		{
			name:   "cert_path",
			script: `[{origin: 'https://x.test', certPath: '/c.pem', keyPath: '/k.pem'}]`,
			wantErr: "clientCertificates[0].certPath: file paths are not supported; " +
				"read the file with open() in the init context and pass its contents as cert",
		},
		{
			name:   "key_path_with_inline_cert",
			script: `[{origin: 'https://x.test', cert: rsaCert, key: rsaKey, keyPath: '/k.pem'}]`,
			wantErr: "clientCertificates[0].keyPath: file paths are not supported; " +
				"read the file with open() in the init context and pass its contents as key",
		},
		{
			name:   "pfx_path",
			script: `[{origin: 'https://x.test', pfxPath: '/c.p12', passphrase: sentinel}]`,
			wantErr: "clientCertificates[0].pfxPath: file paths are not supported; " +
				"read the file with open() in the init context and pass its contents as pfx",
		},
		{
			name:    "pfx",
			script:  `[{pfx: bytes('x').buffer, passphrase: sentinel}]`,
			wantErr: "clientCertificates[0].pfx: PFX is not supported yet; pass cert and key as PEM",
		},
		{
			name:    "missing_origin",
			script:  `[{cert: rsaCert, key: rsaKey}]`,
			wantErr: "clientCertificates[0]: clientCertificates.origin is required",
		},
		{
			name:    "nothing_specified",
			script:  `[{origin: 'https://x.test'}]`,
			wantErr: "clientCertificates[0]: None of cert, key, passphrase or pfx is specified",
		},
		{
			name:   "passphrase_only",
			script: `[{origin: 'https://x.test', passphrase: sentinel}]`,
			wantErr: "clientCertificates[0]: failed to load client certificate: " +
				"cert and key, or pfx, are required",
		},
		{
			name:    "cert_without_key",
			script:  `[{origin: 'https://x.test', cert: rsaCert, passphrase: sentinel}]`,
			wantErr: "clientCertificates[0]: cert is specified without key",
		},
		{
			name:    "key_without_cert",
			script:  `[{origin: 'https://x.test', key: rsaKey, passphrase: sentinel}]`,
			wantErr: "clientCertificates[0]: key is specified without cert",
		},
		{
			name:    "http_origin",
			script:  `[{origin: 'http://x.test', cert: rsaCert, key: rsaKey}]`,
			wantErr: "clientCertificates[0]: origin must use https",
		},
		{
			name:    "bad_origin",
			script:  `[{origin: 'not an origin', cert: rsaCert, key: rsaKey}]`,
			wantErr: `clientCertificates[0]: invalid origin "not an origin"`,
		},
		{
			name:    "wrong_type",
			script:  `[{origin: 'https://x.test', cert: 42, key: rsaKey}]`,
			wantErr: "clientCertificates[0].cert: " + bytesErr,
		},
		{
			name:    "plain_array_cert",
			script:  `[{origin: 'https://x.test', cert: [1, 2, 3], key: rsaKey}]`,
			wantErr: "clientCertificates[0].cert: " + bytesErr,
		},
		{
			name:    "plain_object_key",
			script:  `[{origin: 'https://x.test', cert: rsaCert, key: {}}]`,
			wantErr: "clientCertificates[0].key: " + bytesErr,
		},
		{
			name:    "fake_view_object",
			script:  `[{origin: 'https://x.test', cert: {buffer: new ArrayBuffer(4), byteLength: 4}, key: rsaKey}]`,
			wantErr: "clientCertificates[0].cert: " + bytesErr,
		},
		{
			name:    "dataview_cert",
			script:  `[{origin: 'https://x.test', cert: new DataView(bytes(rsaCert).buffer), key: rsaKey}]`,
			wantErr: "clientCertificates[0].cert: " + bytesErr,
		},
		{
			name: "plain_array_after_isview_overwrite",
			script: `ArrayBuffer.isView = () => true;
				[{origin: 'https://x.test', cert: [1, 2, 3], key: rsaKey}]`,
			wantErr: "clientCertificates[0].cert: " + bytesErr,
		},
		{
			name:    "numeric_origin",
			script:  `[{origin: 1, cert: rsaCert, key: rsaKey}]`,
			wantErr: "clientCertificates[0].origin: expected string",
		},
		{
			name:    "numeric_passphrase",
			script:  `[{origin: 'https://x.test', cert: rsaCert, key: rsaKey, passphrase: 1}]`,
			wantErr: "clientCertificates[0].passphrase: expected string",
		},
		{
			name:    "null_entry",
			script:  `[null]`,
			wantErr: "clientCertificates[0]: expected object",
		},
		{
			name:    "primitive_entry",
			script:  `["x"]`,
			wantErr: "clientCertificates[0]: expected object",
		},
		{
			name:    "not_an_array",
			script:  `({})`,
			wantErr: "clientCertificates must be an array",
		},
		{
			name: "duplicate_key_type",
			script: `[
				{origin: 'https://x.test', cert: rsaCert, key: rsaKey},
				{origin: 'https://X.test:443', cert: rsa2Cert, key: rsa2Key},
			]`,
			wantErr: "clientCertificates[1]: more than one RSA certificate for origin https://x.test; " +
				"only one per key type is supported",
		},
		{
			name:    "load_failure",
			script:  `[{origin: 'https://x.test', cert: 'not a pem', key: rsaKey, passphrase: sentinel}]`,
			wantErr: "clientCertificates[0]: failed to load client certificate: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vu := k6test.NewVU(t)
			rt := vu.Runtime()

			v := runClientCertOptions(t, rt, f, tt.script)
			opts := rt.NewObject()
			require.NoError(t, opts.Set("clientCertificates", v))

			_, err := parseBrowserContextOptions(rt, opts)
			require.Error(t, err)
			if strings.HasSuffix(tt.wantErr, ": ") {
				assert.True(t, strings.HasPrefix(err.Error(), tt.wantErr), err.Error())
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
			assert.NotContains(t, err.Error(), clientCertSentinelPassphrase)
		})
	}
}
