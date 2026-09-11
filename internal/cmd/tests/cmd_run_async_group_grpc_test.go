package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib/fsext"
)

func TestRunAsyncGroupGRPCStreamListeners(t *testing.T) {
	t.Parallel()

	tb := NewGRPC(t)
	script := tb.Replacer.Replace(`
		import { check, group } from 'k6';
		import exec from 'k6/execution';
		import { Client, Stream } from 'k6/net/grpc';

		const client = new Client();
		client.load([], './route_guide.proto');

		export default async function () {
			client.connect('GRPCBIN_ADDR', { plaintext: true });
			const stream = new Stream(client, 'main.FeatureExplorer/ListFeatures');
			await new Promise((resolve, reject) => {
				exec.vu.metrics.tags.owner = 'registered';
				group('grpcgroup', () => {
					stream.on('data', data => check(data, { data: value => value.name !== '' }));
				});
				exec.vu.metrics.tags.owner = 'active';
				stream.on('end', () => {
					check(null, { end: true });
					client.close();
					resolve();
				});
				stream.on('error', reject);
				stream.write({
					lo: { latitude: 407838351, longitude: -746143763 },
					hi: { latitude: 407838351, longitude: -746143763 },
				});
				stream.end();
			});
		}
	`)

	ts := getSingleFileTestState(t, script,
		[]string{"--features", "async-metric-context", "--out", "json=results.json", "--summary-mode=disabled"}, 0)
	proto, err := os.ReadFile(projectRootPath + "internal/lib/testutils/grpcservice/route_guide.proto") //nolint:forbidigo
	require.NoError(t, err)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "route_guide.proto"), proto, 0o644))
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	results, err := fsext.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)
	assert.NotEmpty(t, getSampleValues(t, results, "checks", map[string]string{
		"check": "data", "group": "::grpcgroup", "owner": "registered",
	}))
	assert.Equal(t, []float64{1}, getSampleValues(t, results, "checks", map[string]string{
		"check": "end", "group": "", "owner": "active",
	}))
}
