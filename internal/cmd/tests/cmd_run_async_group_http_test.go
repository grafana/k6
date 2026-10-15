package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/v2/lib/fsext"
)

func TestRunAsyncGroupHTTP(t *testing.T) {
	t.Parallel()

	tb := httpmultibin.NewHTTPMultiBin(t)
	script := tb.Replacer.Replace(`
		import http from 'k6/http';
		import { check, group } from 'k6';

		export const options = {
			hosts: { 'HTTPBIN_DOMAIN': 'HTTPBIN_IP' },
		};

		export default async function () {
			await group('async request', async () => {
				const response = await http.asyncRequest('GET', 'HTTPBIN_URL/get');
				check(response, { inside: response => response.status === 200 });
			});
			check(null, { after: true });
		}
	`)

	ts := getSingleFileTestState(t, script,
		[]string{"--features", "async-metric-context", "--out", "json=results.json", "--summary-mode=disabled"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	results, err := fsext.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)
	assert.Equal(t, []float64{1}, getSampleValues(t, results, "checks", map[string]string{
		"check": "inside", "group": "::async request",
	}))
	assert.Equal(t, []float64{1}, getSampleValues(t, results, "checks", map[string]string{
		"check": "after", "group": "",
	}))
	assert.Len(t, getSampleValues(t, results, "http_reqs", map[string]string{
		"group": "::async request",
	}), 1)
}
