package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/v2/lib/fsext"
)

func TestRunAsyncGroupWebSocketListeners(t *testing.T) {
	t.Parallel()

	tb := httpmultibin.NewHTTPMultiBin(t)
	script := tb.Replacer.Replace(`
		import { check, group } from 'k6';
		import exec from 'k6/execution';
		import { WebSocket } from 'k6/websockets';

		export const options = {
			hosts: { 'HTTPBIN_DOMAIN': 'HTTPBIN_IP' },
		};

		export default function () {
			exec.vu.metrics.tags.owner = 'registered';
			const ws = new WebSocket('WSBIN_URL/ws-echo');
			group('wsgroup', () => {
				ws.onopen = () => {
					check(null, { open: true });
					exec.vu.metrics.tags.callback = 'open';
					ws.send('something');
				};
				ws.addEventListener('message', () => {
					check(null, { message: true });
					ws.close();
				});
			});
			exec.vu.metrics.tags.owner = 'active';
			ws.onclose = () => check(null, { close: true });
		}
	`)

	ts := getSingleFileTestState(t, script,
		[]string{"--features", "async-metric-context", "--out", "json=results.json", "--summary-mode=disabled"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	results, err := fsext.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)
	for _, check := range []string{"open", "message"} {
		assert.Equal(t, []float64{1}, getSampleValues(t, results, "checks", map[string]string{
			"check": check, "group": "::wsgroup", "owner": "registered",
		}))
	}
	assert.Equal(t, []float64{1}, getSampleValues(t, results, "checks", map[string]string{
		"check": "close", "group": "", "owner": "active",
	}))
}
