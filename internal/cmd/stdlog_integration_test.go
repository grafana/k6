package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/internal/lib/testutils/httpmultibin"
)

// SetOutput sets the global log so it is racy with other tests
//
//nolint:paralleltest
func TestStdLogOutputIsSet(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	ts := tests.NewGlobalTestState(t)
	// Sometimes the Go runtime uses the standard log output to
	// log some messages directly.
	// It does when an invalid char is found in a Cookie.
	// Check for details https://github.com/grafana/k6/issues/711#issue-341414887
	ts.Stdin = bytes.NewReader([]byte(tb.Replacer.Replace(`
import http from 'k6/http';
export const options = {
  hosts: {
    "HTTPSBIN_DOMAIN": "HTTPSBIN_IP",
  },
  insecureSkipTLSVerify: true,
}
export default function() {
  http.get("HTTPSBIN_URL/get", {
    "cookies": {
      "test": "\""
    },
  })
}`)))

	ts.CmdArgs = []string{"k6", "run", "-i", "1", "-"}
	newRootCommand(ts.GlobalState).execute()

	entries := ts.LoggerHook.Drain()
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0].Message, "Cookie.Value; dropping invalid bytes")
}
