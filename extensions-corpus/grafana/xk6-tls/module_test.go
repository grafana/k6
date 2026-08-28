package tls

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

func TestGetCertificateOK(t *testing.T) {
	t.Parallel()
	trt := newTestRuntime(t)

	ts := httptest.NewTLSServer(nil)
	defer ts.Close()

	testScript := fmt.Sprintf(`
		JSON.stringify(await tls.getCertificate("%s"));
	`, strings.TrimPrefix(ts.URL, "https://"))

	err := trt.EventLoop.Start(func() error {
		_, err := trt.VU.Runtime().RunString("(async ()=>{globalThis.result = " + testScript + "})()")
		return err
	})
	require.NoError(t, err)
	v := trt.VU.Runtime().GlobalObject().Get("result")

	exp := `{"subject":{"common_name":""},"issuer":{"common_name":""},"issued":0,"expires":3600000000000,"fingerprint":"468174fd18ae990a0a1e10568e30f9819a8acd23224c319f4ec3eb4f6f2980d9"}`
	assert.JSONEq(t, exp, v.ToString().String())
}

func TestGetCertificateNoTLS(t *testing.T) {
	t.Parallel()
	trt := newTestRuntime(t)

	ts := httptest.NewServer(nil)
	defer ts.Close()

	testScript := fmt.Sprintf(`await tls.getCertificate("%s")`, strings.TrimPrefix(ts.URL, "http://"))

	err := runOnEventLoop(trt, wrapInAsyncLambda(testScript))
	assert.ErrorContains(t, err, "not look like a TLS handshake")
}

func TestGetCertificateBlockedHostname(t *testing.T) {
	t.Parallel()
	trt := newTestRuntime(t)

	ts := httptest.NewTLSServer(nil)
	defer ts.Close()

	testScript := `await tls.getCertificate("blocked.net")`
	err := runOnEventLoop(trt, wrapInAsyncLambda(testScript))
	assert.ErrorContains(t, err, "blocked pattern")
}

func TestParseTargetAddr(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		target  string
		expAddr string
		expErr  string
	}{
		{"", "", "target address was not provided"},
		{"htt://", "", "not contain a valid port"},
		{"http://", "", "not contain a valid port"},
		{"http://notok.com", "", "not contain a valid port"},
		{"https://ok.com", "", "not contain a valid port"},
		{"https://ok.com:", "", "too many colons"},
		{"ok.com", "ok.com:443", ""},
		{"ok.com:", "ok.com:443", ""},
		{"ok.com:443", "ok.com:443", ""},
		{"ok.com:1234", "ok.com:1234", ""},
		{"ok.com:65536", "", "not contain a valid port"}, // over the max allowed
	}
	for _, tc := range testcases {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()
			addr, err := parseTargetAddr(tc.target)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expAddr, addr.uri)
			}
		})
	}
}

func newTestRuntime(t testing.TB) *extensionapitest.Runtime {
	t.Helper()
	runtime := extensionapitest.NewRuntime()
	runtime.VU.DialContextFunc = testDialContext

	m, ok := New().NewModuleInstance(runtime.VU).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, runtime.VU.Runtime().Set("tls", m.Exports().Default))

	return runtime
}

func testDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if host == "blocked.net" {
		return nil, errors.New("blocked pattern")
	}

	return (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 10 * time.Second}).DialContext(ctx, network, address)
}

func runOnEventLoop(runtime *extensionapitest.Runtime, script string) error {
	return runtime.EventLoop.Start(func() error {
		_, err := runtime.VU.Runtime().RunString(script)
		return err
	})
}

// wrapInAsyncLambda is a helper function that wraps the provided input in an async lambda.
// This makes the use of `await` statements in the input possible.
func wrapInAsyncLambda(input string) string {
	return "(async () => {\n " + input + "\n })()"
}
