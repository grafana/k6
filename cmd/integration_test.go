package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/httpmultibin"
)

const (
	noopDefaultFunc   = `export default function() {};`
	fooLogDefaultFunc = `export default function() { console.log('foo'); };`
	noopHandleSummary = `
		export function handleSummary(data) {
			return {}; // silence the end of test summary
		};
	`
)

func TestVersion(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "version"}
	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()
	assert.Contains(t, stdOut, "k6 v"+consts.Version)
	assert.Contains(t, stdOut, runtime.Version())
	assert.Contains(t, stdOut, runtime.GOOS)
	assert.Contains(t, stdOut, runtime.GOARCH)
	assert.NotContains(t, stdOut[:len(stdOut)-1], "\n")

	assert.Empty(t, ts.stdErr.Bytes())
	assert.Empty(t, ts.loggerHook.Drain())
}

func TestSimpleTestStdin(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "run", "-"}
	ts.stdIn = bytes.NewBufferString(noopDefaultFunc)
	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()
	assert.Contains(t, stdOut, "default: 1 iterations for each of 1 VUs")
	assert.Contains(t, stdOut, "1 complete and 0 interrupted iterations")
	assert.Empty(t, ts.stdErr.Bytes())
	assert.Empty(t, ts.loggerHook.Drain())
}

func TestStdoutAndStderrAreEmptyWithQuietAndHandleSummary(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "--quiet", "run", "-"}
	ts.stdIn = bytes.NewBufferString(noopDefaultFunc + noopHandleSummary)
	newRootCommand(ts.globalState).execute()

	assert.Empty(t, ts.stdErr.Bytes())
	assert.Empty(t, ts.stdOut.Bytes())
	assert.Empty(t, ts.loggerHook.Drain())
}

func TestStdoutAndStderrAreEmptyWithQuietAndLogsForwarded(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)

	// TODO: add a test with relative path
	logFilePath := filepath.Join(ts.cwd, "test.log")

	ts.args = []string{
		"k6", "--quiet", "--log-output", "file=" + logFilePath,
		"--log-format", "raw", "run", "--no-summary", "-",
	}
	ts.stdIn = bytes.NewBufferString(fooLogDefaultFunc)
	newRootCommand(ts.globalState).execute()

	// The test state hook still catches this message
	assert.True(t, testutils.LogContains(ts.loggerHook.Drain(), logrus.InfoLevel, `foo`))

	// But it's not shown on stderr or stdout
	assert.Empty(t, ts.stdErr.Bytes())
	assert.Empty(t, ts.stdOut.Bytes())

	// Instead it should be in the log file
	logContents, err := afero.ReadFile(ts.fs, logFilePath)
	require.NoError(t, err)
	assert.Equal(t, "foo\n", string(logContents))
}

func TestRelativeLogPathWithSetupAndTeardown(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)

	ts.args = []string{"k6", "--log-output", "file=test.log", "--log-format", "raw", "run", "-i", "2", "-"}
	ts.stdIn = bytes.NewBufferString(fooLogDefaultFunc + `
		export function setup() { console.log('bar'); };
		export function teardown() { console.log('baz'); };
	`)
	newRootCommand(ts.globalState).execute()

	// The test state hook still catches these messages
	logEntries := ts.loggerHook.Drain()
	assert.True(t, testutils.LogContains(logEntries, logrus.InfoLevel, `foo`))
	assert.True(t, testutils.LogContains(logEntries, logrus.InfoLevel, `bar`))
	assert.True(t, testutils.LogContains(logEntries, logrus.InfoLevel, `baz`))

	// And check that the log file also contains everything
	logContents, err := afero.ReadFile(ts.fs, filepath.Join(ts.cwd, "test.log"))
	require.NoError(t, err)
	assert.Equal(t, "bar\nfoo\nfoo\nbaz\n", string(logContents))
}

func TestWrongCliFlagIterations(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "run", "--iterations", "foo", "-"}
	ts.stdIn = bytes.NewBufferString(noopDefaultFunc)
	// TODO: check for exitcodes.InvalidConfig after https://github.com/loadimpact/k6/issues/883 is done...
	ts.expectedExitCode = -1
	newRootCommand(ts.globalState).execute()
	assert.True(t, testutils.LogContains(ts.loggerHook.Drain(), logrus.ErrorLevel, `invalid argument "foo"`))
}

func TestWrongEnvVarIterations(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "run", "--vus", "2", "-"}
	ts.envVars = map[string]string{"K6_ITERATIONS": "4"}
	ts.stdIn = bytes.NewBufferString(noopDefaultFunc)

	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()
	t.Logf(stdOut)
	assert.Contains(t, stdOut, "4 iterations shared among 2 VUs")
	assert.Contains(t, stdOut, "4 complete and 0 interrupted iterations")
	assert.Empty(t, ts.stdErr.Bytes())
	assert.Empty(t, ts.loggerHook.Drain())
}

func TestMetricsAndThresholds(t *testing.T) {
	t.Parallel()
	script := `
		import { Counter } from 'k6/metrics';

		var setupCounter = new Counter('setup_counter');
		var teardownCounter = new Counter('teardown_counter');
		var defaultCounter = new Counter('default_counter');
		let unusedCounter = new Counter('unused_counter');

		export const options = {
			scenarios: {
				sc1: {
					executor: 'per-vu-iterations',
					vus: 1,
					iterations: 1,
				},
				sc2: {
					executor: 'shared-iterations',
					vus: 1,
					iterations: 1,
				},
			},
			thresholds: {
				'setup_counter': ['count == 1'],
				'teardown_counter': ['count == 1'],
				'default_counter': ['count == 2'],
				'default_counter{scenario:sc1}': ['count == 1'],
				'default_counter{scenario:sc2}': ['count == 1'],
				'iterations': ['count == 2'],
				'iterations{scenario:sc1}': ['count == 1'],
				'iterations{scenario:sc2}': ['count == 1'],
				'default_counter{nonexistent:tag}': ['count == 0'],
				'unused_counter': ['count == 0'],
				'http_req_duration{status:200}': [' max == 0'], // no HTTP requests
			},
		};

		export function setup() {
			console.log('setup() start');
			setupCounter.add(1);
			console.log('setup() end');
			return { foo: 'bar' }
		}

		export default function (data) {
			console.log('default(' + JSON.stringify(data) + ')');
			defaultCounter.add(1);
		}

		export function teardown(data) {
			console.log('teardown(' + JSON.stringify(data) + ')');
			teardownCounter.add(1);
		}

		export function handleSummary(data) {
			console.log('handleSummary()');
			return { stdout: JSON.stringify(data, null, 4) }
		}
	`
	ts := newGlobalTestState(t)
	require.NoError(t, afero.WriteFile(ts.fs, filepath.Join(ts.cwd, "test.js"), []byte(script), 0o644))
	ts.args = []string{"k6", "run", "--quiet", "--log-format=raw", "test.js"}

	newRootCommand(ts.globalState).execute()

	expLogLines := []string{
		`setup() start`, `setup() end`, `default({"foo":"bar"})`,
		`default({"foo":"bar"})`, `teardown({"foo":"bar"})`, `handleSummary()`,
	}

	logHookEntries := ts.loggerHook.Drain()
	require.Len(t, logHookEntries, len(expLogLines))
	for i, expLogLine := range expLogLines {
		assert.Equal(t, expLogLine, logHookEntries[i].Message)
	}

	assert.Equal(t, strings.Join(expLogLines, "\n")+"\n", ts.stdErr.String())

	var summary map[string]interface{}
	require.NoError(t, json.Unmarshal(ts.stdOut.Bytes(), &summary))

	metrics, ok := summary["metrics"].(map[string]interface{})
	require.True(t, ok)

	teardownCounter, ok := metrics["teardown_counter"].(map[string]interface{})
	require.True(t, ok)

	teardownThresholds, ok := teardownCounter["thresholds"].(map[string]interface{})
	require.True(t, ok)

	expected := map[string]interface{}{"count == 1": map[string]interface{}{"ok": true}}
	require.Equal(t, expected, teardownThresholds)
}

func TestSSLKEYLOGFILE(t *testing.T) {
	t.Parallel()

	filepaths := [...]string{"./ssl.log", "/test/ssl.log"}
	for _, filePath := range filepaths {
		filePath := filePath
		t.Run(filePath, func(t *testing.T) {
			// TODO don't use insecureSkipTLSVerify when/if tlsConfig is given to the runner from outside
			tb := httpmultibin.NewHTTPMultiBin(t)
			ts := newGlobalTestState(t)
			ts.args = []string{"k6", "run", "-"}
			ts.envVars = map[string]string{"SSLKEYLOGFILE": filePath}
			ts.stdIn = bytes.NewReader([]byte(tb.Replacer.Replace(`
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

			newRootCommand(ts.globalState).execute()

			assert.True(t,
				testutils.LogContains(ts.loggerHook.Drain(), logrus.WarnLevel, "SSLKEYLOGFILE was specified"))
			sslloglines, err := afero.ReadFile(ts.fs, filepath.Join(ts.cwd, "ssl.log"))
			require.NoError(t, err)
			// TODO maybe have multiple depending on the ciphers used as that seems to change it
			require.Regexp(t, "^CLIENT_[A-Z_]+ [0-9a-f]+ [0-9a-f]+\n", string(sslloglines))
		})
	}
}

func TestThresholdDeprecationWarnings(t *testing.T) {
	t.Parallel()

	// TODO: adjust this test after we actually make url, error, iter and vu non-indexable

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "run", "--system-tags", "url,error,vu,iter", "-"}
	ts.stdIn = bytes.NewReader([]byte(`
		export const options = {
			thresholds: {
				'http_req_duration{url:https://test.k6.io}': ['p(95)<500', 'p(99)<1000'],
				'http_req_duration{error:foo}': ['p(99)<1000'],
				'iterations{vu:1,iter:0}': ['count == 1'],
			},
		};

		export default function () { }`,
	))

	newRootCommand(ts.globalState).execute()

	logs := ts.loggerHook.Drain()
	assert.True(t, testutils.LogContains(logs, logrus.WarnLevel,
		"Thresholds like 'http_req_duration{url:https://test.k6.io}', based on the high-cardinality 'url' metric tag, are deprecated",
	))
	assert.True(t, testutils.LogContains(logs, logrus.WarnLevel,
		"Thresholds like 'http_req_duration{error:foo}', based on the high-cardinality 'error' metric tag, are deprecated",
	))
	assert.True(t, testutils.LogContains(logs, logrus.WarnLevel,
		"Thresholds like 'iterations{vu:1,iter:0}', based on the high-cardinality 'vu' metric tag, are deprecated",
	))
	assert.True(t, testutils.LogContains(logs, logrus.WarnLevel,
		"Thresholds like 'iterations{vu:1,iter:0}', based on the high-cardinality 'iter' metric tag, are deprecated",
	))
}

// TODO: add a hell of a lot more integration tests, including some that spin up
// a test HTTP server and actually check if k6 hits it

// TODO: also add a test that starts multiple k6 "instances", for example:
//  - one with `k6 run --paused` and another with `k6 resume`
//  - one with `k6 run` and another with `k6 stats` or `k6 status`

func TestExecutionTestOptionsDefaultValues(t *testing.T) {
	t.Parallel()
	script := `
		import exec from 'k6/execution';

		export default function () {
			console.log(exec.test.options)
		}
	`

	ts := newGlobalTestState(t)
	require.NoError(t, afero.WriteFile(ts.fs, filepath.Join(ts.cwd, "test.js"), []byte(script), 0o644))
	ts.args = []string{"k6", "run", "--iterations", "1", "test.js"}

	newRootCommand(ts.globalState).execute()

	loglines := ts.loggerHook.Drain()
	require.Len(t, loglines, 1)

	expected := `{"paused":null,"executionSegment":null,"executionSegmentSequence":null,"noSetup":null,"setupTimeout":null,"noTeardown":null,"teardownTimeout":null,"rps":null,"dns":{"ttl":null,"select":null,"policy":null},"maxRedirects":null,"userAgent":null,"batch":null,"batchPerHost":null,"httpDebug":null,"insecureSkipTLSVerify":null,"tlsCipherSuites":null,"tlsVersion":null,"tlsAuth":null,"throw":null,"thresholds":null,"blacklistIPs":null,"blockHostnames":null,"hosts":null,"noConnectionReuse":null,"noVUConnectionReuse":null,"minIterationDuration":null,"ext":null,"summaryTrendStats":["avg", "min", "med", "max", "p(90)", "p(95)"],"summaryTimeUnit":null,"systemTags":["check","error","error_code","expected_response","group","method","name","proto","scenario","service","status","subproto","tls_version","url"],"tags":null,"metricSamplesBufferSize":null,"noCookiesReset":null,"discardResponseBodies":null,"consoleOutput":null,"scenarios":{"default":{"vus":null,"iterations":1,"executor":"shared-iterations","maxDuration":null,"startTime":null,"env":null,"tags":null,"gracefulStop":null,"exec":null}},"localIPs":null}`
	assert.JSONEq(t, expected, loglines[0].Message)
}

func TestSubMetricThresholdNoData(t *testing.T) {
	t.Parallel()
	script := `
		import { Counter } from 'k6/metrics';

		const counter1 = new Counter("one");
		const counter2 = new Counter("two");

		export const options = {
			thresholds: {
				'one{tag:xyz}': [],
			},
		};

		export default function () {
			counter2.add(42);
		}
	`
	ts := newGlobalTestState(t)
	require.NoError(t, afero.WriteFile(ts.fs, filepath.Join(ts.cwd, "test.js"), []byte(script), 0o644))
	ts.args = []string{"k6", "run", "--quiet", "test.js"}

	newRootCommand(ts.globalState).execute()

	require.Len(t, ts.loggerHook.Drain(), 0)
	require.Contains(t, ts.stdOut.String(), `
     one..................: 0   0/s
       { tag:xyz }........: 0   0/s
     two..................: 42`)
}
