package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/v2/lib/fsext"
)

func TestRunConcurrentAsyncGroupsAcrossModules(t *testing.T) {
	t.Parallel()

	httpBin := httpmultibin.NewHTTPMultiBin(t)
	grpcBin := NewGRPC(t)
	script := httpBin.Replacer.Replace(grpcBin.Replacer.Replace(`
		import { check, group } from 'k6';
		import exec from 'k6/execution';
		import http from 'k6/http';
		import { Client, Stream } from 'k6/net/grpc';
		import { WebSocket } from 'k6/websockets';

		export const options = {
			hosts: { 'HTTPBIN_DOMAIN': 'HTTPBIN_IP' },
		};

		const client = new Client();
		client.load([], './route_guide.proto');
		const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
		const phases = ['first', 'second', 'third'];
		const changeContext = (name, phase) => {
			const value = name + ':' + phase;
			exec.vu.metrics.tags.phase = value;
			exec.vu.metrics.metadata.step = value;
		};
		const crossBoundary = phase => phase === 'second' ? delay(0) : Promise.resolve();
		const recordContext = (name, phase) => check(null, {[name + ':' + phase]: true});

		export default async function () {
			client.connect('GRPCBIN_ADDR', { plaintext: true });
			exec.vu.metrics.tags.owner = 'root';
			exec.vu.metrics.tags.phase = 'root';
			exec.vu.metrics.metadata.trace = 'root';
			exec.vu.metrics.metadata.step = 'root';

			let release;
			const gate = new Promise(resolve => { release = resolve; });
			let started = 0;
			const start = (name, operation) => {
				exec.vu.metrics.tags.owner = name;
				exec.vu.metrics.metadata.trace = name;
				const pending = group(name, () => {
					started++;
					check(null, {[name + ':start']: true});
					return operation();
				});
				exec.vu.metrics.tags.owner = 'root';
				exec.vu.metrics.metadata.trace = 'root';
				return pending;
			};

			const pending = [
				start('http', async () => {
					await gate;
					for (const phase of phases) {
						changeContext('http', phase);
						await crossBoundary(phase);
						recordContext('http', phase);
					}
					const response = await http.asyncRequest('GET', 'HTTPBIN_URL/get');
					check(response, {'http:finish': value => value.status === 200});
				}),
				start('websocket', () => new Promise((resolve, reject) => {
					const ws = new WebSocket('WSBIN_URL/ws-echo');
					ws.addEventListener('open', () => ws.send('hello'));
					ws.addEventListener('message', async event => {
						try {
							check(event, {'websocket:message': value => value.data === 'hello'});
							for (const phase of phases) {
								changeContext('websocket', phase);
								await crossBoundary(phase);
								recordContext('websocket', phase);
							}
							ws.close();
						} catch (error) {
							reject(error);
						}
					});
					ws.addEventListener('close', () => {
						check(null, {'websocket:finish': true});
						resolve();
					});
					ws.addEventListener('error', () => reject(new Error('WebSocket failed')));
				})),
				start('grpc', () => new Promise((resolve, reject) => {
					const stream = new Stream(client, 'main.FeatureExplorer/ListFeatures');
					let handlingData = false;
					let dataHandled = false;
					let streamEnded = false;
					const finish = () => {
						if (dataHandled && streamEnded) resolve();
					};
					stream.on('data', async data => {
						if (handlingData) return;
						handlingData = true;
						try {
							check(data, {'grpc:data': value => value.name !== ''});
							for (const phase of phases) {
								changeContext('grpc', phase);
								await crossBoundary(phase);
								recordContext('grpc', phase);
							}
							dataHandled = true;
							finish();
						} catch (error) {
							reject(error);
						}
					});
					stream.on('end', () => {
						check(null, {'grpc:finish': true});
						streamEnded = true;
						finish();
					});
					stream.on('error', reject);
					stream.write({
						lo: { latitude: 407838351, longitude: -746143763 },
						hi: { latitude: 407838351, longitude: -746143763 },
					});
					stream.end();
				})),
				start('timer', () => new Promise((resolve, reject) => {
					setTimeout(async () => {
						try {
							for (const phase of phases) {
								changeContext('timer', phase);
								await crossBoundary(phase);
								recordContext('timer', phase);
							}
							check(null, {'timer:finish': true});
							resolve();
						} catch (error) {
							reject(error);
						}
					}, 1);
				})),
				start('promise', () => gate.then(async () => {
					for (const phase of phases) {
						changeContext('promise', phase);
						await crossBoundary(phase);
						recordContext('promise', phase);
					}
					check(null, {'promise:finish': true});
				})),
			];

			// The event loop cannot run any protocol or timer callback before this point.
			check(null, {'all-started': () => started === pending.length});
			release();
			await Promise.all(pending);
			check(null, {'outside:finish': true});
			client.close();
		}
	`))

	ts := getSingleFileTestState(t, script,
		[]string{"--features", "async-metric-context", "--out", "json=results.json", "--summary-mode=disabled"}, 0)
	proto, err := os.ReadFile(projectRootPath + "internal/lib/testutils/grpcservice/route_guide.proto") //nolint:forbidigo
	require.NoError(t, err)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "route_guide.proto"), proto, 0o644))
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	results, err := fsext.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)
	for _, name := range []string{"http", "websocket", "grpc", "timer", "promise"} {
		assert.Equal(t, []float64{1}, getSampleValuesWithMetadata(t, results, "checks", map[string]string{
			"check": name + ":start", "group": "::" + name, "owner": name,
		}, map[string]string{"trace": name}))
		assert.Len(t, getSampleValuesWithMetadata(t, results, "group_duration", map[string]string{
			"group": "::" + name, "owner": name, "phase": "root",
		}, map[string]string{"trace": name, "step": "root"}), 1)
		for _, phase := range []string{"first", "second", "third"} {
			value := name + ":" + phase
			assert.Equal(t, []float64{1}, getSampleValuesWithMetadata(t, results, "checks", map[string]string{
				"check": value, "group": "::" + name, "owner": name, "phase": value,
			}, map[string]string{"trace": name, "step": value}))
		}
	}

	for _, expected := range []struct {
		check string
		group string
		phase string
	}{
		{"http:finish", "http", "http:third"},
		{"websocket:message", "websocket", "root"},
		{"websocket:finish", "websocket", "root"},
		{"grpc:finish", "grpc", "root"},
		{"timer:finish", "timer", "timer:third"},
		{"promise:finish", "promise", "promise:third"},
	} {
		assert.Equal(t, []float64{1}, getSampleValuesWithMetadata(t, results, "checks", map[string]string{
			"check": expected.check, "group": "::" + expected.group, "owner": expected.group,
			"phase": expected.phase,
		}, map[string]string{"trace": expected.group, "step": expected.phase}))
	}
	assert.NotEmpty(t, getSampleValuesWithMetadata(t, results, "checks", map[string]string{
		"check": "grpc:data", "group": "::grpc", "owner": "grpc", "phase": "root",
	}, map[string]string{"trace": "grpc", "step": "root"}))

	for _, expected := range []struct {
		metric string
		group  string
		phase  string
	}{
		{"http_reqs", "http", "http:third"},
		{"ws_msgs_received", "websocket", "root"},
		{"grpc_streams_msgs_received", "grpc", "root"},
	} {
		assert.NotEmpty(t, getSampleValuesWithMetadata(t, results, expected.metric, map[string]string{
			"group": "::" + expected.group, "owner": expected.group, "phase": expected.phase,
		}, map[string]string{"trace": expected.group, "step": expected.phase}))
	}
	assert.Equal(t, []float64{1}, getSampleValuesWithMetadata(t, results, "checks", map[string]string{
		"check": "all-started", "group": "", "owner": "root", "phase": "root",
	}, map[string]string{"trace": "root", "step": "root"}))
	assert.Equal(t, []float64{1}, getSampleValuesWithMetadata(t, results, "checks", map[string]string{
		"check": "outside:finish", "group": "", "owner": "root", "phase": "root",
	}, map[string]string{"trace": "root", "step": "root"}))
}
