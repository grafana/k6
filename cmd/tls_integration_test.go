package cmd

// TODO: convert this into the integration tests, once https://github.com/grafana/k6/issues/2459 will be done

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/testutils/httpmultibin"
)

type metric struct {
	Avg float64 `json:"avg"`
	Min float64 `json:"min"`
	P95 float64 `json:"p(95)"`
}

type summaryMetrics struct {
	HTTPReqTLSHandshaking metric `json:"http_req_tls_handshaking"`
}

type summaryMetricsWrapper struct {
	Metrics summaryMetrics `json:"metrics"`
}

func TestTLSLoadSystemCertificates(t *testing.T) {
	t.Parallel()

	summaryFilename := "summary.json"

	tb := httpmultibin.NewHTTPMultiBin(t)
	testState := newGlobalTestState(t)
	testState.args = []string{"k6", "run", "--no-connection-reuse", "--summary-export", summaryFilename, "-i", "2", "-"}
	testState.stdIn = bytes.NewReader([]byte(tb.Replacer.Replace(`
		import http from "k6/http"
		export const options = {
		hosts: {
			"HTTPSBIN_DOMAIN": "HTTPSBIN_IP",
		},
		insecureSkipTLSVerify: true,
		}

		export default () => {
		http.get("HTTPSBIN_URL/get");
		}
  `)))
	newRootCommand(testState.globalState).execute()

	content, err := afero.ReadFile(testState.fs, summaryFilename)
	require.NoError(t, err)

	var summaryMetricsWrapperContent summaryMetricsWrapper
	err = json.Unmarshal(content, &summaryMetricsWrapperContent)
	require.NoError(t, err)

	delta := 10.0
	assert.InDelta(
		t,
		summaryMetricsWrapperContent.Metrics.HTTPReqTLSHandshaking.Min,
		summaryMetricsWrapperContent.Metrics.HTTPReqTLSHandshaking.Avg,
		delta,
	)

	assert.InDelta(
		t,
		summaryMetricsWrapperContent.Metrics.HTTPReqTLSHandshaking.Min,
		summaryMetricsWrapperContent.Metrics.HTTPReqTLSHandshaking.P95,
		delta,
	)
}
