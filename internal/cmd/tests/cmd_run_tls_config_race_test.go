package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/v2/lib/fsext"
)

// TestRunHTTPAndWebSocketConcurrentlyOnFreshVU starts a VU's first HTTP request and a WebSocket
// connection in the same event loop turn. net/http lazily configures HTTP/2 on the transport's
// TLS config during the first request, while the WebSocket dial clones the VU's TLS config from
// its own goroutine. When both used the same *tls.Config, the race detector reported a data race.
func TestRunHTTPAndWebSocketConcurrentlyOnFreshVU(t *testing.T) {
	t.Parallel()

	const vus = 10
	tb := httpmultibin.NewHTTPMultiBin(t)
	script := tb.Replacer.Replace(`
		import { check } from 'k6';
		import http from 'k6/http';
		import { WebSocket } from 'k6/websockets';

		export const options = {
			hosts: { 'HTTPBIN_DOMAIN': 'HTTPBIN_IP' },
			scenarios: {
				fresh_vus: { executor: 'per-vu-iterations', vus: 10, iterations: 1 },
			},
			thresholds: { checks: ['rate==1'] },
		};

		export default async function () {
			const echoed = new Promise((resolve, reject) => {
				const ws = new WebSocket('WSBIN_URL/ws-echo');
				ws.addEventListener('open', () => ws.send('hello'));
				ws.addEventListener('message', event => {
					check(event, { 'websocket echo': e => e.data === 'hello' });
					ws.close();
				});
				ws.addEventListener('close', resolve);
				ws.addEventListener('error', () => reject(new Error('WebSocket failed')));
			});
			const response = http.asyncRequest('GET', 'HTTPBIN_URL/get');

			check(await response, { 'http status 200': r => r.status === 200 });
			await echoed;
		}
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json", "--summary-mode=disabled"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	results, err := fsext.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)
	for _, name := range []string{"http status 200", "websocket echo"} {
		assert.Len(t, getSampleValues(t, results, "checks", map[string]string{"check": name}), vus, name)
	}
}
