package tls

import (
	"fmt"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/netext"
	"go.k6.io/k6/v2/lib/types"
	"go.k6.io/k6/v2/metrics"
)

func TestGetCertificateOK(t *testing.T) {
	t.Parallel()
	trt := newTestRuntime(t)

	ts := httptest.NewTLSServer(nil)
	defer ts.Close()

	testScript := fmt.Sprintf(`
		JSON.stringify(await tls.getCertificate("%s"));
	`, strings.TrimPrefix(ts.URL, "https://"))

	_, err := trt.RunOnEventLoop("(async ()=>{globalThis.result = " + testScript + "})()")
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

	_, err := trt.RunOnEventLoop(wrapInAsyncLambda(testScript))
	assert.ErrorContains(t, err, "not look like a TLS handshake")
}

func TestGetCertificateBlockedHostname(t *testing.T) {
	t.Parallel()
	trt := newTestRuntime(t)

	ts := httptest.NewTLSServer(nil)
	defer ts.Close()

	testScript := `await tls.getCertificate("blocked.net")`
	_, err := trt.RunOnEventLoop(wrapInAsyncLambda(testScript))
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

func newTestRuntime(t testing.TB) *modulestest.Runtime {
	runtime := modulestest.NewRuntime(t)
	state := &lib.State{
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(metrics.NewRegistry()),
		Dialer:         newTestDialer(),
		Tags:           lib.NewVUStateTags(metrics.NewRegistry().RootTagSet().With("tag-vu", "mytag")),
		Samples:        make(chan metrics.SampleContainer, 8),
	}
	runtime.MoveToVUContext(state)

	m, ok := New().NewModuleInstance(runtime.VU).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, runtime.VU.RuntimeField.Set("tls", m.Exports().Default))

	return runtime
}

func newTestDialer() *netext.Dialer {
	d := netext.NewDialer(net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 10 * time.Second,
	}, nil)

	trie, err := types.NewHostnameTrie([]string{"blocked.net"})
	if err != nil {
		panic(err)
	}
	d.BlockedHostnames = trie

	return d
}

// wrapInAsyncLambda is a helper function that wraps the provided input in an async lambda.
// This makes the use of `await` statements in the input possible.
func wrapInAsyncLambda(input string) string {
	return "(async () => {\n " + input + "\n })()"
}
