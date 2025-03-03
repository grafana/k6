package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/cmd"
	"go.k6.io/k6/internal/cmd/tests/events"
	"go.k6.io/k6/internal/event"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib/fsext"
)

func TestVersion(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args string
	}{
		"flag":       {"--version"},
		"subcommand": {"version"},
	}

	ts := NewGlobalTestState(t)

	for _, tc := range tests {
		ts.CmdArgs = []string{"k6", tc.args}
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, "k6 v"+build.Version)
		assert.Contains(t, stdout, runtime.Version())
		assert.Contains(t, stdout, runtime.GOOS)
		assert.Contains(t, stdout, runtime.GOARCH)
	}

	assert.Empty(t, ts.Stderr.Bytes())
	assert.Empty(t, ts.LoggerHook.Drain())
}

func TestSimpleTestStdin(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "run", "-"}
	ts.Stdin = bytes.NewBufferString(`export default function() {};`)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, "output: -")
	assert.Contains(t, stdout, "default: 1 iterations for each of 1 VUs")
	assert.Contains(t, stdout, "1 complete and 0 interrupted iterations")
	assert.Empty(t, ts.Stderr.Bytes())
	assert.Empty(t, ts.LoggerHook.Drain())
}

func TestBinaryNameStdout(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)
	ts.BinaryName = "customBinaryName"
	ts.CmdArgs = []string{ts.BinaryName}
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, fmt.Sprintf("%s [command]", ts.BinaryName))
	assert.Empty(t, ts.Stderr.Bytes())
	assert.Empty(t, ts.LoggerHook.Drain())
}

func TestBinaryNameHelpStdout(t *testing.T) {
	t.Parallel()
	ts := NewGlobalTestState(t)
	ts.BinaryName = "customBinaryName"

	tests := []struct {
		cmdName        string
		extraCmd       string // For the `login cloud` cmd
		containsOutput string
	}{
		{
			cmdName:        "archive",
			containsOutput: fmt.Sprintf("%s archive -u 10 -d 10s -O myarchive.tar script.js", ts.BinaryName),
		},
		{
			cmdName:        "cloud",
			containsOutput: fmt.Sprintf("%s cloud script.js", ts.BinaryName),
		},
		{
			cmdName:        "login",
			extraCmd:       "cloud",
			containsOutput: fmt.Sprintf("%s login cloud -t YOUR_TOKEN", ts.BinaryName),
		},
		{
			cmdName:        "run",
			containsOutput: fmt.Sprintf("%s run -i 10 script.js", ts.BinaryName),
		},
	}

	for _, tt := range tests {
		ts.CmdArgs = []string{ts.BinaryName, "help", tt.cmdName, tt.extraCmd}
		cmd.ExecuteWithGlobalState(ts.GlobalState)
		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, tt.containsOutput)
		assert.Empty(t, ts.Stderr.Bytes())
		assert.Empty(t, ts.LoggerHook.Drain())
	}
}

// TODO: Remove this? It doesn't test anything AFAICT...
func TestStdoutAndStderrAreEmptyWithQuietAndHandleSummary(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "--quiet", "run", "-"}
	ts.Stdin = bytes.NewBufferString(`
		export default function() {};
		export function handleSummary(data) {
			return {}; // silence the end of test summary
		};
	`)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Empty(t, ts.Stderr.Bytes())
	assert.Empty(t, ts.Stdout.Bytes())
	assert.Empty(t, ts.LoggerHook.Drain())
}

func TestStdoutAndStderrAreEmptyWithQuietAndLogsForwarded(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)

	// TODO: add a test with relative path
	logFilePath := filepath.Join(ts.Cwd, "test.log")

	ts.CmdArgs = []string{
		"k6", "--quiet", "--log-output", "file=" + logFilePath,
		"--log-format", "raw", "run", "--no-summary", "-",
	}
	ts.Stdin = bytes.NewBufferString(`
		console.log('init');
		export default function() { console.log('foo'); };
	`)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	// The test state hook still catches this message
	assert.True(t, testutils.LogContains(ts.LoggerHook.Drain(), logrus.InfoLevel, `foo`))

	// But it's not shown on stderr or stdout
	assert.Empty(t, ts.Stderr.Bytes())
	assert.Empty(t, ts.Stdout.Bytes())

	// Instead it should be in the log file
	logContents, err := fsext.ReadFile(ts.FS, logFilePath)
	require.NoError(t, err)
	assert.Equal(t, "init\ninit\nfoo\n", string(logContents)) //nolint:dupword
}

func TestRelativeLogPathWithSetupAndTeardown(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)

	ts.CmdArgs = []string{"k6", "--log-output", "file=test.log", "--log-format", "raw", "run", "-i", "2", "-"}
	ts.Stdin = bytes.NewBufferString(`
		console.log('init');
		export default function() { console.log('foo'); };
		export function setup() { console.log('bar'); };
		export function teardown() { console.log('baz'); };
	`)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	// The test state hook still catches these messages
	logEntries := ts.LoggerHook.Drain()
	assert.True(t, testutils.LogContains(logEntries, logrus.InfoLevel, `foo`))
	assert.True(t, testutils.LogContains(logEntries, logrus.InfoLevel, `bar`))
	assert.True(t, testutils.LogContains(logEntries, logrus.InfoLevel, `baz`))

	// And check that the log file also contains everything
	logContents, err := fsext.ReadFile(ts.FS, filepath.Join(ts.Cwd, "test.log"))
	require.NoError(t, err)
	assert.Equal(t, "init\ninit\ninit\nbar\nfoo\nfoo\ninit\nbaz\ninit\n", string(logContents)) //nolint:dupword
}

func TestWrongCliFlagIterations(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "run", "--iterations", "foo", "-"}
	ts.Stdin = bytes.NewBufferString(`export default function() {};`)
	// TODO: check for exitcodes.InvalidConfig after https://github.com/loadimpact/k6/issues/883 is done...
	ts.ExpectedExitCode = -1
	cmd.ExecuteWithGlobalState(ts.GlobalState)
	assert.True(t, testutils.LogContains(ts.LoggerHook.Drain(), logrus.ErrorLevel, `invalid argument "foo"`))
}

func TestWrongEnvVarIterations(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "run", "--vus", "2", "-"}
	ts.Env["K6_ITERATIONS"] = "4"
	ts.Stdin = bytes.NewBufferString(`export default function() {};`)

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, "4 iterations shared among 2 VUs")
	assert.Contains(t, stdout, "4 complete and 0 interrupted iterations")
	assert.Empty(t, ts.Stderr.Bytes())
	assert.Empty(t, ts.LoggerHook.Drain())
}

func getSingleFileTestState(tb testing.TB, script string, cliFlags []string, expExitCode exitcodes.ExitCode) *GlobalTestState {
	if cliFlags == nil {
		cliFlags = []string{"-v", "--log-output=stdout"}
	}

	ts := NewGlobalTestState(tb)
	require.NoError(tb, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(script), 0o644))
	ts.CmdArgs = append(append([]string{"k6", "run"}, cliFlags...), "test.js")
	ts.ExpectedExitCode = int(expExitCode)

	return ts
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
	ts := getSingleFileTestState(t, script, []string{"--quiet", "--log-format=raw"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	expLogLines := []string{
		`setup() start`, `setup() end`, `default({"foo":"bar"})`,
		`default({"foo":"bar"})`, `teardown({"foo":"bar"})`, `handleSummary()`,
	}

	logHookEntries := ts.LoggerHook.Drain()
	require.Len(t, logHookEntries, len(expLogLines))
	for i, expLogLine := range expLogLines {
		assert.Equal(t, expLogLine, logHookEntries[i].Message)
	}

	assert.Equal(t, strings.Join(expLogLines, "\n")+"\n", ts.Stderr.String())

	var summary map[string]interface{}
	require.NoError(t, json.Unmarshal(ts.Stdout.Bytes(), &summary))

	metrics, ok := summary["metrics"].(map[string]interface{})
	require.True(t, ok)

	teardownCounter, ok := metrics["teardown_counter"].(map[string]interface{})
	require.True(t, ok)

	teardownThresholds, ok := teardownCounter["thresholds"].(map[string]interface{})
	require.True(t, ok)

	expected := map[string]interface{}{"count == 1": map[string]interface{}{"ok": true}}
	require.Equal(t, expected, teardownThresholds)
}

func TestSSLKEYLOGFILEAbsolute(t *testing.T) {
	t.Parallel()
	ts := NewGlobalTestState(t)
	testSSLKEYLOGFILE(t, ts, filepath.Join(ts.Cwd, "ssl.log"))
}

func TestSSLKEYLOGFILEARelative(t *testing.T) {
	t.Parallel()
	ts := NewGlobalTestState(t)
	testSSLKEYLOGFILE(t, ts, "./ssl.log")
}

func testSSLKEYLOGFILE(t *testing.T, ts *GlobalTestState, filePath string) {
	t.Helper()

	// TODO don't use insecureSkipTLSVerify when/if tlsConfig is given to the runner from outside
	tb := httpmultibin.NewHTTPMultiBin(t)
	ts.CmdArgs = []string{"k6", "run", "-"}
	ts.Env["SSLKEYLOGFILE"] = filePath
	ts.Stdin = bytes.NewReader([]byte(tb.Replacer.Replace(`
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

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.True(t,
		testutils.LogContains(ts.LoggerHook.Drain(), logrus.WarnLevel, "SSLKEYLOGFILE was specified"))
	sslloglines, err := fsext.ReadFile(ts.FS, filepath.Join(ts.Cwd, "ssl.log"))
	require.NoError(t, err)
	// TODO maybe have multiple depending on the ciphers used as that seems to change it
	assert.Regexp(t, "^CLIENT_[A-Z_]+ [0-9a-f]+ [0-9a-f]+\n", string(sslloglines))
}

func TestThresholdDeprecationWarnings(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "run", "--system-tags", "url,error,vu,iter,scenario", "-"}
	ts.Stdin = bytes.NewReader([]byte(`
		export const options = {
			thresholds: {
				'http_req_duration{url:https://test.k6.io}': ['p(95)<500', 'p(99)<1000'],
				'http_req_duration{error:foo}': ['p(99)<1000'],
				'iterations{scenario:default}': ['count == 1'],
				'iterations{vu:1,iter:0}': ['count == 0'], // iter and vu are now unindexable
			},
		};

		export default function () { }`,
	))

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	logs := ts.LoggerHook.Drain()

	// We no longer warn about this
	assert.False(t, testutils.LogContains(logs, logrus.WarnLevel, "http_req_duration{url:https://test.k6.io}"))
	assert.False(t, testutils.LogContains(logs, logrus.WarnLevel, "http_req_duration{error:foo}"))
	assert.True(t, testutils.LogContains(logs, logrus.WarnLevel,
		"The high-cardinality 'vu' metric tag was made non-indexable in k6 v0.41.0, so thresholds like 'iterations{vu:1,iter:0}'",
	))
	assert.True(t, testutils.LogContains(logs, logrus.WarnLevel,
		"The high-cardinality 'iter' metric tag was made non-indexable in k6 v0.41.0, so thresholds like 'iterations{vu:1,iter:0}'",
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

	ts := getSingleFileTestState(t, script, []string{"--iterations", "1"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	loglines := ts.LoggerHook.Drain()
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
	ts := getSingleFileTestState(t, script, []string{"--quiet"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Len(t, ts.LoggerHook.Drain(), 0)
	assert.Contains(t, ts.Stdout.String(), `
     one..................: 0   0/s
       { tag:xyz }........: 0   0/s
     two..................: 42`)
}

func getTestServer(tb testing.TB, routes map[string]http.Handler) *httptest.Server {
	mux := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		for methodAndRoute, handler := range routes {
			method, route, _ := strings.Cut(methodAndRoute, " ")
			routeRegexp, err := regexp.Compile(route)
			require.NoError(tb, err)

			if req.Method == method && routeRegexp.Match([]byte(req.URL.String())) {
				handler.ServeHTTP(resp, req)
				return
			}
		}

		// By default, respond with 200 OK to all unmatched requests
		resp.WriteHeader(http.StatusOK)
	})
	return httptest.NewServer(mux)
}

func getCloudTestEndChecker(
	tb testing.TB, testRunID int,
	testStart http.Handler, expRunStatus cloudapi.RunStatus, expResultStatus cloudapi.ResultStatus,
) *httptest.Server {
	testFinished := false

	if testStart == nil {
		testStart = cloudTestStartSimple(tb, testRunID)
	}

	srv := getTestServer(tb, map[string]http.Handler{
		"POST ^/v1/tests$": testStart,
		fmt.Sprintf("POST ^/v1/tests/%d$", testRunID): http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
			require.NotNil(tb, req.Body)
			buf := &bytes.Buffer{}
			_, err := io.Copy(buf, req.Body)
			require.NoError(tb, err)
			require.NoError(tb, req.Body.Close())

			body := buf.Bytes()
			require.True(tb, gjson.ValidBytes(body))

			runStatus := gjson.GetBytes(body, "run_status")
			require.True(tb, runStatus.Exists()) // important to check, since run_status can be 0
			assert.Equalf(
				tb, expRunStatus, cloudapi.RunStatus(runStatus.Int()),
				"received wrong run_status value",
			)

			resultStatus := gjson.GetBytes(body, "result_status")
			require.True(tb, resultStatus.Exists())
			assert.Equalf(
				tb, expResultStatus, cloudapi.ResultStatus(resultStatus.Int()),
				"received wrong result_status value",
			)
			testFinished = true
		}),
	})

	tb.Cleanup(func() {
		assert.Truef(tb, testFinished, "expected test to have called the cloud API endpoint to finish the test")
		srv.Close()
	})

	return srv
}

func getSimpleCloudOutputTestState(
	tb testing.TB, script string, cliFlags []string,
	expRunStatus cloudapi.RunStatus, expResultStatus cloudapi.ResultStatus, expExitCode exitcodes.ExitCode,
) *GlobalTestState {
	if cliFlags == nil {
		cliFlags = []string{"-v", "--log-output=stdout"}
	}
	cliFlags = append(cliFlags, "--out", "cloud")

	srv := getCloudTestEndChecker(tb, 111, nil, expRunStatus, expResultStatus)
	ts := getSingleFileTestState(tb, script, cliFlags, expExitCode)
	ts.Env["K6_CLOUD_HOST"] = srv.URL
	return ts
}

func TestSetupTeardownThresholds(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import { Counter } from "k6/metrics";

		let statusCheck = { "status is 200": (r) => r.status === 200 }
		let myCounter = new Counter("setup_teardown");

		export let options = {
			iterations: 5,
			thresholds: {
				"setup_teardown": ["count == 3"],
				"iterations": ["count == 5"],
				"http_reqs": ["count == 8"],
				"checks": ["rate == 1"]
			},
		};

		export function setup() {
			check(http.get("HTTPBIN_IP_URL"), statusCheck) && myCounter.add(1);
		};

		export default function () {
			check(http.get("HTTPBIN_IP_URL"), statusCheck);
		};

		export function teardown() {
			check(http.get("HTTPBIN_IP_URL"), statusCheck) && myCounter.add(1);
		};
	`)

	cliFlags := []string{"-v", "--log-output=stdout", "--linger"}
	ts := getSimpleCloudOutputTestState(t, script, cliFlags, cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed, 0)

	sendSignal := injectMockSignalNotifier(ts)
	asyncWaitForStdoutAndRun(t, ts, 20, 500*time.Millisecond, "waiting for Ctrl+C to continue", func() {
		defer func() {
			sendSignal <- syscall.SIGINT
			<-sendSignal
		}()
		t.Logf("Linger reached, running teardown again and stopping the test...")
		req, err := http.NewRequestWithContext(
			ts.Ctx, http.MethodPost, fmt.Sprintf("http://%s/v1/teardown", ts.Flags.Address), nil,
		)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		t.Logf("Response body: %s", body)
		assert.NoError(t, resp.Body.Close())
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdOut := ts.Stdout.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, `✓ checks.........................: 100.00% 8 out of 8`)
	assert.Contains(t, stdOut, `✓ http_reqs......................: 8`)
	assert.Contains(t, stdOut, `✓ iterations.....................: 5`)
	assert.Contains(t, stdOut, `✓ setup_teardown.................: 3`)

	logMsgs := ts.LoggerHook.Drain()
	for _, msg := range logMsgs {
		if msg.Level != logrus.DebugLevel {
			assert.Failf(t, "unexpected log message", "level %s, msg '%s'", msg.Level, msg.Message)
		}
	}
	assert.True(t, testutils.LogContains(logMsgs, logrus.DebugLevel, "Running thresholds on 4 metrics..."))
	assert.True(t, testutils.LogContains(logMsgs, logrus.DebugLevel, "Finalizing thresholds..."))
	assert.True(t, testutils.LogContains(logMsgs, logrus.DebugLevel, "Metrics emission of VUs and VUsMax metrics stopped"))
	assert.True(t, testutils.LogContains(logMsgs, logrus.DebugLevel, "Metrics and traces processing finished!"))
}

func TestThresholdsFailed(t *testing.T) {
	t.Parallel()
	script := `
		export let options = {
			scenarios: {
				sc1: {
					executor: 'per-vu-iterations',
					vus: 1, iterations: 1,
				},
				sc2: {
					executor: 'shared-iterations',
					vus: 1, iterations: 2,
				},
			},
			thresholds: {
				'iterations': ['count == 3'],
				'iterations{scenario:sc1}': ['count == 2'],
				'iterations{scenario:sc2}': ['count == 1'],
				'iterations{scenario:sc3}': ['count == 0'],
			},
		};

		export default function () {};
	`

	// Since these thresholds don't have an abortOnFail property, the run_status
	// in the cloud will still be Finished, even if the test itself failed.
	ts := getSimpleCloudOutputTestState(
		t, script, nil, cloudapi.RunStatusFinished, cloudapi.ResultStatusFailed, exitcodes.ThresholdsHaveFailed,
	)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	expErr := "thresholds on metrics 'iterations{scenario:sc1}, iterations{scenario:sc2}' have been crossed"
	assert.True(t, testutils.LogContains(ts.LoggerHook.Drain(), logrus.ErrorLevel, expErr))
	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, `   ✓ iterations...........: 3`)
	assert.Contains(t, stdout, `     ✗ { scenario:sc1 }...: 1`)
	assert.Contains(t, stdout, `     ✗ { scenario:sc2 }...: 2`)
	assert.Contains(t, stdout, `     ✓ { scenario:sc3 }...: 0   0/s`)
}

func TestAbortedByThreshold(t *testing.T) {
	t.Parallel()
	script := `
		export const options = {
			scenarios: {
				sc1: {
					executor: 'constant-arrival-rate',
					duration: '30s',
					rate: 1,
					preAllocatedVUs: 2,
				},
			},
			thresholds: {
				'iterations': [{
					threshold: 'count == 1',
					abortOnFail: true,
				}],
			},
		};

		export default function () {};

		export function teardown() {
			console.log('teardown() called');
		}
	`

	ts := getSimpleCloudOutputTestState(
		t, script, nil, cloudapi.RunStatusAbortedThreshold, cloudapi.ResultStatusFailed, exitcodes.ThresholdsHaveFailed,
	)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	expErr := "thresholds on metrics 'iterations' were crossed; at least one has abortOnFail enabled, stopping test prematurely"
	assert.True(t, testutils.LogContains(ts.LoggerHook.Drain(), logrus.ErrorLevel, expErr))
	stdOut := ts.Stdout.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, `✗ iterations`)
	assert.Contains(t, stdOut, `teardown() called`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics and traces processing finished!"`)
	assert.Contains(t, stdOut, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=8 tainted=true`)
}

func TestAbortedByUserWithGoodThresholds(t *testing.T) {
	t.Parallel()
	script := `
		import { Counter } from 'k6/metrics';
		import exec from 'k6/execution';

		export const options = {
			scenarios: {
				sc1: {
					executor: 'constant-arrival-rate',
					duration: '30s',
					rate: 1,
					preAllocatedVUs: 2,
				},
			},
			thresholds: {
				'iterations': ['count >= 1'],
				'tc': ['count == 1'],
				'tc{group:::setup}': ['count == 0'],
				'tc{group:::teardown}': ['count == 1'],
			},
		};

		let tc = new Counter('tc');
		export function teardown() {
			tc.add(1);
		}

		export default function () {
			console.log('simple iter ' + exec.scenario.iterationInTest);
		};
	`

	ts := getSimpleCloudOutputTestState(t, script, nil, cloudapi.RunStatusAbortedUser, cloudapi.ResultStatusPassed, exitcodes.ExternalAbort)

	asyncWaitForStdoutAndStopTestWithInterruptSignal(t, ts, 30, 300*time.Millisecond, "simple iter 2")

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	logs := ts.LoggerHook.Drain()
	assert.False(t, testutils.LogContains(logs, logrus.ErrorLevel, `thresholds on metrics`))
	assert.True(t, testutils.LogContains(logs, logrus.ErrorLevel, `test run was aborted because k6 received a 'interrupt' signal`))
	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, `✓ iterations`)
	assert.Contains(t, stdout, `✓ tc`)
	assert.Contains(t, stdout, `✓ { group:::teardown }`)
	assert.Contains(t, stdout, `Stopping k6 in response to signal`)
	assert.Contains(t, stdout, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdout, `level=debug msg="Metrics and traces processing finished!"`)
	assert.Contains(t, stdout, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=5 tainted=false`)
}

func asyncWaitForStdoutAndRun(
	t *testing.T, ts *GlobalTestState, attempts int, interval time.Duration, expText string, callback func(),
) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		reachedCondition := false
		for i := 0; i < attempts; i++ {
			ts.OutMutex.Lock()
			stdOut := ts.Stdout.String()
			ts.OutMutex.Unlock()

			if strings.Contains(stdOut, expText) {
				t.Logf("found '%s' in the process stdout on try %d at t=%s", expText, i, time.Now())
				reachedCondition = true
				break
			}

			t.Logf("did not find the text '%s' in the process stdout on try %d at t=%s", expText, i, time.Now())
			time.Sleep(interval)
		}
		if reachedCondition {
			callback()
			return // everything is fine
		}

		ts.OutMutex.Lock()
		stdOut := ts.Stdout.String()
		ts.OutMutex.Unlock()
		t.Log(stdOut)
		require.FailNow(
			t, "expected output not found", "did not find the text '%s' in the process stdout after %d attempts (%s)",
			expText, attempts, time.Duration(attempts)*interval,
		)
	}()

	t.Cleanup(wg.Wait) // ensure the test waits for the goroutine to finish
}

func injectMockSignalNotifier(ts *GlobalTestState) (sendSignal chan os.Signal) {
	sendSignal = make(chan os.Signal)
	ts.GlobalState.SignalNotify = func(c chan<- os.Signal, signals ...os.Signal) {
		isAbortNotify := false
		for _, s := range signals {
			if s == os.Interrupt {
				isAbortNotify = true
				break
			}
		}
		if !isAbortNotify {
			return
		}
		go func() {
			sig := <-sendSignal
			c <- sig
			close(sendSignal)
		}()
	}
	ts.GlobalState.SignalStop = func(_ chan<- os.Signal) { /* noop */ }
	return sendSignal
}

func asyncWaitForStdoutAndStopTestWithInterruptSignal(
	t *testing.T, ts *GlobalTestState, attempts int, interval time.Duration, expText string,
) {
	sendSignal := injectMockSignalNotifier(ts)
	asyncWaitForStdoutAndRun(t, ts, attempts, interval, expText, func() {
		t.Log("expected stdout text was found, sending interrupt signal...")
		sendSignal <- syscall.SIGINT
		<-sendSignal
	})
}

func asyncWaitForStdoutAndStopTestFromRESTAPI(
	t *testing.T, ts *GlobalTestState, attempts int, interval time.Duration, expText string,
) {
	asyncWaitForStdoutAndRun(t, ts, attempts, interval, expText, func() {
		req, err := http.NewRequestWithContext(
			ts.Ctx, http.MethodPatch, fmt.Sprintf("http://%s/v1/status", ts.Flags.Address),
			bytes.NewBufferString(`{"data":{"type":"status","id":"default","attributes":{"stopped":true}}}`),
		)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		t.Logf("Response body: %s", body)
		assert.NoError(t, resp.Body.Close())
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TODO: add more abort scenario tests, see
// https://github.com/grafana/k6/issues/2804

func TestAbortedByUserWithRestAPI(t *testing.T) {
	t.Parallel()
	script := `
		import { sleep } from 'k6';
		export default function () {
			console.log('a simple iteration')
			sleep(1);
		};

		export function teardown() {
			console.log('teardown() called');
		}
	`

	ts := getSimpleCloudOutputTestState(
		t, script, []string{"-v", "--log-output=stdout", "--iterations", "20"},
		cloudapi.RunStatusAbortedUser, cloudapi.ResultStatusPassed, exitcodes.ScriptStoppedFromRESTAPI,
	)

	asyncWaitForStdoutAndStopTestFromRESTAPI(t, ts, 15, time.Second, "a simple iteration")

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, `a simple iteration`)
	assert.Contains(t, stdout, `teardown() called`)
	assert.Contains(t, stdout, `PATCH /v1/status`)
	assert.Contains(t, stdout, `level=error msg="test run stopped from REST API`)
	assert.Contains(t, stdout, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdout, `level=debug msg="Metrics and traces processing finished!"`)
	assert.Contains(t, stdout, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=5 tainted=false`)
	assert.NotContains(t, stdout, `Running thresholds`)
	assert.NotContains(t, stdout, `Finalizing thresholds`)
}

func TestAbortedByScriptSetupErrorWithDependency(t *testing.T) {
	t.Parallel()
	depScript := `
		export default function () {
			baz();
		}
		function baz() {
			throw new Error("baz");
		}
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`
	mainScript := `
		import bar from "./bar.js";
		export function setup() {
			console.log('wonky setup');
			bar();
		};
		export default function() {};

		export { handleSummary } from "./bar.js";
	`

	srv := getCloudTestEndChecker(t, 123, nil, cloudapi.RunStatusAbortedScriptError, cloudapi.ResultStatusPassed)

	ts := NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(mainScript), 0o644))
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "bar.js"), []byte(depScript), 0o644))

	ts.Env["K6_CLOUD_HOST"] = srv.URL
	ts.CmdArgs = []string{"k6", "run", "-v", "--out", "cloud", "--log-output=stdout", "test.js"}
	ts.ExpectedExitCode = int(exitcodes.ScriptException)

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, `wonky setup`)

	rootPath := "file:///"
	if runtime.GOOS == "windows" {
		rootPath += "c:/"
	}
	assert.Contains(t, stdout, `level=error msg="Error: baz\n\tat baz (`+rootPath+`test/bar.js:6:10(3))\n\tat default (`+
		rootPath+`test/bar.js:3:7(3))\n\tat setup (`+rootPath+`test/test.js:5:7(8))\n" hint="script exception"`)
	assert.Contains(t, stdout, `level=debug msg="Sending test finished" output=cloud ref=123 run_status=7 tainted=false`)
	assert.Contains(t, stdout, "bogus summary")
}

func runTestWithNoLinger(_ *testing.T, ts *GlobalTestState) {
	cmd.ExecuteWithGlobalState(ts.GlobalState)
}

func runTestWithLinger(t *testing.T, ts *GlobalTestState) {
	ts.CmdArgs = append(ts.CmdArgs, "--linger")
	asyncWaitForStdoutAndStopTestWithInterruptSignal(t, ts, 15, time.Second, "waiting for Ctrl+C to continue")
	cmd.ExecuteWithGlobalState(ts.GlobalState)
}

func TestAbortedByScriptSetupError(t *testing.T) {
	t.Parallel()
	script := `
		export function setup() {
			console.log('wonky setup');
			throw new Error('foo');
		}

		export function teardown() {
			console.log('nice teardown');
		}

		export default function () {};

		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`

	doChecks := func(t *testing.T, ts *GlobalTestState) {
		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, "Error: foo")
		assert.Contains(t, stdout, "wonky setup")
		assert.NotContains(t, stdout, "nice teardown") // do not execute teardown if setup failed
		assert.Contains(t, stdout, "bogus summary")
	}

	t.Run("noLinger", func(t *testing.T) {
		t.Parallel()
		ts := testAbortedByScriptError(t, script, runTestWithNoLinger)
		doChecks(t, ts)
	})

	t.Run("withLinger", func(t *testing.T) {
		t.Parallel()
		ts := testAbortedByScriptError(t, script, runTestWithLinger)
		doChecks(t, ts)
	})
}

func TestAbortedByScriptTeardownError(t *testing.T) {
	t.Parallel()

	script := `
		export function setup() {
			console.log('nice setup');
		}

		export function teardown() {
			console.log('wonky teardown');
			throw new Error('foo');
		}

		export default function () {};

		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`

	doChecks := func(t *testing.T, ts *GlobalTestState) {
		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, "Error: foo")
		assert.Contains(t, stdout, "nice setup")
		assert.Contains(t, stdout, "wonky teardown")
		assert.Contains(t, stdout, "bogus summary")
	}

	t.Run("noLinger", func(t *testing.T) {
		t.Parallel()
		ts := testAbortedByScriptError(t, script, runTestWithNoLinger)
		doChecks(t, ts)
	})

	t.Run("withLinger", func(t *testing.T) {
		t.Parallel()
		ts := testAbortedByScriptError(t, script, runTestWithLinger)
		doChecks(t, ts)
	})
}

func testAbortedByScriptError(t *testing.T, script string, runTest func(*testing.T, *GlobalTestState)) *GlobalTestState {
	ts := getSimpleCloudOutputTestState(
		t, script, nil, cloudapi.RunStatusAbortedScriptError, cloudapi.ResultStatusPassed, exitcodes.ScriptException,
	)
	runTest(t, ts)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdout, `level=debug msg="Metrics and traces processing finished!"`)
	assert.Contains(t, stdout, `level=debug msg="Everything has finished, exiting k6 with an error!"`)
	assert.Contains(t, stdout, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=7 tainted=false`)
	return ts
}

func TestAbortedByTestAbortFirstInitCode(t *testing.T) {
	t.Parallel()
	script := `
		import exec from 'k6/execution';
		exec.test.abort('foo');
		export default function () {};

		// Should not be called, since error is in the init context
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`

	ts := getSingleFileTestState(t, script, nil, exitcodes.ScriptAborted)
	cmd.ExecuteWithGlobalState(ts.GlobalState)
	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, "test aborted: foo")
	assert.NotContains(t, stdout, "bogus summary")
}

func TestAbortedByTestAbortInNonFirstInitCode(t *testing.T) {
	t.Parallel()
	script := `
		import exec from 'k6/execution';

		export const options = {vus: 3, duration: '5s'};

		if (__VU > 1) {
			exec.test.abort('foo');
		}

		export default function () {};

		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`

	testAbortedByScriptTestAbort(t, script, runTestWithNoLinger)
}

func TestAbortedByScriptAbortInVUCode(t *testing.T) {
	t.Parallel()
	script := `
		import exec from 'k6/execution';
		export default function () {
			exec.test.abort('foo');
		};
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`

	t.Run("noLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, script, runTestWithNoLinger)
	})

	t.Run("withLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, script, runTestWithLinger)
	})
}

func TestAbortedByScriptAbortInVUCodeInGroup(t *testing.T) {
	t.Parallel()
	script := `
		import exec from 'k6/execution';
        import { group } from 'k6';
		export default function () {
            group("here", () => {
                exec.test.abort('foo');
            });
		};
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`

	t.Run("noLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, script, runTestWithNoLinger)
	})

	t.Run("withLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, script, runTestWithLinger)
	})
}

func TestAbortedByScriptAbortInSetup(t *testing.T) {
	t.Parallel()
	script := `
		import exec from 'k6/execution';
		export function setup() {
			exec.test.abort('foo');
		}
		export default function () {};
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`

	t.Run("noLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, script, runTestWithNoLinger)
	})
	t.Run("withLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, script, runTestWithLinger)
	})
}

func TestAbortedByScriptAbortInTeardown(t *testing.T) {
	t.Parallel()
	script := `
		import exec from 'k6/execution';
		export function teardown() {
			exec.test.abort('foo');
		}
		export default function () {};
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`

	t.Run("noLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, script, runTestWithNoLinger)
	})

	t.Run("withLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, script, runTestWithLinger)
	})
}

func testAbortedByScriptTestAbort(t *testing.T, script string, runTest func(*testing.T, *GlobalTestState)) {
	ts := getSimpleCloudOutputTestState(
		t, script, nil, cloudapi.RunStatusAbortedUser, cloudapi.ResultStatusPassed, exitcodes.ScriptAborted,
	)
	runTest(t, ts)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, "execution: local")
	assert.Contains(t, stdout, "output: cloud (https://app.k6.io/runs/111)")
	assert.Contains(t, stdout, "test aborted: foo")
	assert.Contains(t, stdout, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=5 tainted=false`)
	assert.Contains(t, stdout, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdout, `level=debug msg="Metrics and traces processing finished!"`)
	assert.Contains(t, stdout, "bogus summary")
}

func TestAbortedByInterruptDuringVUInit(t *testing.T) {
	t.Parallel()
	script := `
		import { sleep } from 'k6';
		export const options = {
			vus: 5,
			duration: '10s',
		};

		if (__VU > 1) {
			console.log('VU init sleeping for a while');
			sleep(100);
		}

		export default function () {};
	`

	ts := getSimpleCloudOutputTestState(
		t, script, nil, cloudapi.RunStatusAbortedUser, cloudapi.ResultStatusPassed, exitcodes.ExternalAbort,
	)
	asyncWaitForStdoutAndStopTestWithInterruptSignal(t, ts, 15, time.Second, "VU init sleeping for a while")
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdOut := ts.Stdout.String()
	t.Log(stdOut)

	assert.Contains(t, stdOut, `level=debug msg="Stopping k6 in response to signal..." sig=interrupt`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdOut, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=5 tainted=false`)
	assert.Contains(t, stdOut, `level=error msg="test run was aborted because k6 received a 'interrupt' signal"`)
}

func TestAbortedByInterruptWhenPaused(t *testing.T) {
	t.Parallel()
	script := `export default function () {};`
	ts := getSimpleCloudOutputTestState(
		t, script, []string{"-v", "--log-output=stdout", "--paused"},
		cloudapi.RunStatusAbortedUser, cloudapi.ResultStatusPassed, exitcodes.ExternalAbort,
	)
	asyncWaitForStdoutAndStopTestWithInterruptSignal(
		t, ts, 10, time.Second, "Execution is paused, waiting for resume or interrupt...",
	)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdOut := ts.Stdout.String()
	t.Log(stdOut)

	assert.Contains(t, stdOut, `level=debug msg="Stopping k6 in response to signal..." sig=interrupt`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdOut, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=5 tainted=false`)
	assert.Contains(t, stdOut, `level=error msg="test run was aborted because k6 received a 'interrupt' signal"`)
}

func TestAbortedByScriptInitError(t *testing.T) {
	t.Parallel()
	script := `
		export const options = {
			vus: 5,
			iterations: 10,
		};

		if (__VU == 2) {
			throw new Error('oops in ' + __VU);
		}

		export default function () {};
	`

	ts := getSimpleCloudOutputTestState(
		t, script, nil, cloudapi.RunStatusAbortedScriptError, cloudapi.ResultStatusPassed, exitcodes.ScriptException,
	)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, "execution: local")
	assert.Contains(t, stdout, "output: cloud (https://app.k6.io/runs/111)")
	assert.Contains(t, stdout, `level=error msg="Error: oops in 2\n\tat file:///`)
	assert.Contains(t, stdout, `hint="error while initializing VU #2 (script exception)"`)
	assert.Contains(t, stdout, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdout, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=7 tainted=false`)
}

func TestMetricTagAndSetupDataIsolation(t *testing.T) {
	t.Parallel()
	script := `
		import exec from 'k6/execution';
		import { Counter } from 'k6/metrics';
		import { sleep } from 'k6';

		export const options = {
			scenarios: {
				sc1: {
					executor: 'shared-iterations',
					vus: 2,
					iterations: 20,
					maxDuration: '7s',
					gracefulStop: 0,
					exec: 'sc1',
				},
				sc2: {
					executor: 'per-vu-iterations',
					vus: 1,
					iterations: 1,
					startTime: '7s',
					exec: 'sc2',
				},
			},
			thresholds: {
				'iterations': ['count == 21'],
				'iterations{scenario:sc1}': ['count == 20'],
				'iterations{sc:1}': ['count == 20'],
				'iterations{scenario:sc2}': ['count == 1'],
				'mycounter': ['count == 23'],
				'mycounter{sc:1}': ['count == 20'],
				'mycounter{setup:true}': ['count == 1'],
				'mycounter{myiter:1}': ['count >= 1', 'count <= 2'],
				'mycounter{myiter:2}': ['count >= 1', 'count <= 2'],
				'mycounter{scenario:sc2}': ['count == 1'],
				'mycounter{scenario:sc2,sc:1}': ['count == 0'],
				'vus_max': ['value == 2'],
			},
		};
		let myCounter = new Counter('mycounter');

		export function setup() {
			exec.vu.tags.setup = 'true';
			myCounter.add(1);
			return { v: 0 };
		}

		export function sc1(data) {
			if (data.v !== __ITER) {
				throw new Error('sc1: wrong data for iter ' + __ITER + ': ' + JSON.stringify(data));
			}
			if (__ITER != 0 && data.v != exec.vu.tags.myiter) {
				throw new Error('sc1: wrong vu tags for iter ' + __ITER + ': ' + JSON.stringify(exec.vu.tags));
			}
			data.v += 1;
			exec.vu.tags.myiter = data.v;
			exec.vu.tags.sc = 1;
			myCounter.add(1);
			sleep(0.02); // encourage using both of the VUs
		}

		export function sc2(data) {
			if (data.v === 0) {
				throw new Error('sc2: wrong data, expected VU to have modified setup data locally: ' + data.v);
			}

			if (typeof exec.vu.tags.myiter !== 'undefined') {
				throw new Error(
					'sc2: wrong tags, expected VU to have new tags in new scenario: ' + JSON.stringify(exec.vu.tags),
				);
			}

			myCounter.add(1);
		}

		export function teardown(data) {
			if (data.v !== 0) {
				throw new Error('teardown: wrong data: ' + data.v);
			}
			myCounter.add(1);
		}
	`

	ts := getSimpleCloudOutputTestState(
		t, script, []string{"--quiet", "--log-output", "stdout"},
		cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed, 0,
	)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.NotContains(t, stdout, "execution: local") // because of --quiet
	assert.NotContains(t, stdout, "output: cloud")    // because of --quiet
	assert.Equal(t, 12, strings.Count(stdout, "✓"))
}

func getSampleValues(t *testing.T, jsonOutput []byte, metric string, tags map[string]string) []float64 {
	jsonLines := bytes.Split(jsonOutput, []byte("\n"))
	result := []float64{}

	tagsMatch := func(rawTags interface{}) bool {
		sampleTags, ok := rawTags.(map[string]interface{})
		require.True(t, ok)
		for k, v := range tags {
			rv, sok := sampleTags[k]
			if !sok {
				return false
			}
			rvs, sok := rv.(string)
			require.True(t, sok)
			if v != rvs {
				return false
			}
		}
		return true
	}

	for _, jsonLine := range jsonLines {
		if len(jsonLine) == 0 {
			continue
		}
		var line map[string]interface{}
		require.NoError(t, json.Unmarshal(jsonLine, &line))
		sampleType, ok := line["type"].(string)
		require.True(t, ok)
		if sampleType != "Point" {
			continue
		}
		sampleMetric, ok := line["metric"].(string)
		require.True(t, ok)
		if sampleMetric != metric {
			continue
		}
		sampleData, ok := line["data"].(map[string]interface{})
		require.True(t, ok)

		if !tagsMatch(sampleData["tags"]) {
			continue
		}

		samplValue, ok := sampleData["value"].(float64)
		require.True(t, ok)
		result = append(result, samplValue)
	}

	return result
}

func sum(vals []float64) (sum float64) {
	for _, val := range vals {
		sum += val
	}
	return sum
}

func TestActiveVUsCount(t *testing.T) {
	t.Parallel()

	script := `
		var sleep = require('k6').sleep;

		exports.options = {
			scenarios: {
				carr1: {
					executor: 'constant-arrival-rate',
					rate: 10,
					preAllocatedVUs: 1,
					maxVUs: 10,
					startTime: '0s',
					duration: '3s',
					gracefulStop: '0s',
				},
				carr2: {
					executor: 'constant-arrival-rate',
					rate: 10,
					preAllocatedVUs: 1,
					maxVUs: 10,
					duration: '3s',
					startTime: '3s',
					gracefulStop: '0s',
				},
				rarr: {
					executor: 'ramping-arrival-rate',
					startRate: 5,
					stages: [
						{ target: 10, duration: '2s' },
						{ target: 0, duration: '2s' },
					],
					preAllocatedVUs: 1,
					maxVUs: 10,
					startTime: '6s',
					gracefulStop: '0s',
				},
			}
		}

		exports.default = function () {
			sleep(5);
		}
	`

	ts := getSingleFileTestState(t, script, []string{"--compatibility-mode", "base", "--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)

	jsonResults, err := fsext.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)
	// t.Log(string(jsonResults))
	assert.Equal(t, float64(10), slices.Max(getSampleValues(t, jsonResults, "vus_max", nil)))
	assert.Equal(t, float64(10), slices.Max(getSampleValues(t, jsonResults, "vus", nil)))
	assert.Equal(t, float64(0), sum(getSampleValues(t, jsonResults, "iterations", nil)))

	logEntries := ts.LoggerHook.Drain()
	assert.Len(t, logEntries, 4)
	for i, logEntry := range logEntries {
		assert.Equal(t, logrus.WarnLevel, logEntry.Level)
		if i < 3 {
			assert.Equal(t, "Insufficient VUs, reached 10 active VUs and cannot initialize more", logEntry.Message)
		} else {
			assert.Equal(t, "No script iterations fully finished, consider making the test duration longer", logEntry.Message)
		}
	}
}

func TestMinIterationDuration(t *testing.T) {
	t.Parallel()
	script := `
		import { Counter } from 'k6/metrics';

		export let options = {
			minIterationDuration: '7s',
			setupTimeout: '2s',
			teardownTimeout: '2s',
			thresholds: {
				'test_counter': ['count == 3'],
			},
		};

		var c = new Counter('test_counter');

		export function setup() { c.add(1); };
		export default function () { c.add(1); };
		export function teardown() { c.add(1); };
	`

	ts := getSimpleCloudOutputTestState(t, script, nil, cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed, 0)

	start := time.Now()
	cmd.ExecuteWithGlobalState(ts.GlobalState)
	elapsed := time.Since(start)
	assert.Greater(t, elapsed, 7*time.Second, "expected more time to have passed because of minIterationDuration")
	assert.Less(
		t, elapsed, 15*time.Second,
		"expected less time to have passed because minIterationDuration should not affect setup() and teardown() ",
	)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, "✓ test_counter.........: 3")
}

func TestMetricNameError(t *testing.T) {
	t.Parallel()
	script := `
		import { Counter } from 'k6/metrics';

		export let options = {
			vus: 2,
			iterations: 2,
			thresholds: {
				'test counter': ['count == 4'],
			},
		};

		var c = new Counter('test counter');
		new Counter('test_counter_#'); // this is also bad but we error on the one above

		export function setup() { c.add(1); };
		export default function () { c.add(1); };
		export function teardown() { c.add(1); };
	`

	ts := getSingleFileTestState(t, script, nil, exitcodes.ScriptException)

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)

	logEntries := ts.LoggerHook.Drain()
	expectedMsg := `Metric names must only include up to 128 ASCII letters, numbers, or underscores`
	filteredEntries := testutils.FilterEntries(logEntries, logrus.ErrorLevel, expectedMsg)
	require.Len(t, filteredEntries, 1)
	require.Contains(t, filteredEntries[0].Message, "'test counter'")
}

func TestRunTags(t *testing.T) {
	t.Parallel()

	tb := httpmultibin.NewHTTPMultiBin(t)
	script := tb.Replacer.Replace(`
		import http from 'k6/http';
		import ws from 'k6/ws';
		import { Counter } from 'k6/metrics';
		import { group, check, fail } from 'k6';

		let customTags =  { 'over': 'the rainbow' };
		let params = { 'tags': customTags};
		let statusCheck = { 'status is 200': (r) => r.status === 200 }

		let myCounter = new Counter('mycounter');

		export const options = {
			hosts: {
				"HTTPBIN_DOMAIN": "HTTPBIN_IP",
				"HTTPSBIN_DOMAIN": "HTTPSBIN_IP",
			}
		}

		export default function() {
			group('http', function() {
				check(http.get('HTTPSBIN_URL', params), statusCheck, customTags);
				check(http.get('HTTPBIN_URL/status/418', params), statusCheck, customTags);
			})

			group('websockets', function() {
				var response = ws.connect('WSBIN_URL/ws-echo', params, function (socket) {
					socket.on('open', function open() {
						console.log('ws open and say hello');
						socket.send('hello');
					});

					socket.on('message', function (message) {
						console.log('ws got message ' + message);
						if (message != 'hello') {
							fail('Expected to receive "hello" but got "' + message + '" instead !');
						}
						console.log('ws closing socket...');
						socket.close();
					});

					socket.on('close', function () {
						console.log('ws close');
					});

					socket.on('error', function (e) {
						console.log('ws error: ' + e.error());
					});
				});
				console.log('connect returned');
				check(response, { 'status is 101': (r) => r && r.status === 101 }, customTags);
			})

			myCounter.add(1, customTags);
		}
	`)

	ts := getSingleFileTestState(t, script, []string{
		"-u", "2", "--log-output=stdout", "--out", "json=results.json",
		"--tag", "foo=bar", "--tag", "test=mest", "--tag", "over=written, something",
	}, 0)
	ts.Env["K6_ITERATIONS"] = "3"
	ts.Env["K6_INSECURE_SKIP_TLS_VERIFY"] = "true"
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)

	jsonResults, err := fsext.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)

	expTags := map[string]string{"foo": "bar", "test": "mest", "over": "written, something", "scenario": "default"}
	assert.Equal(t, float64(3), sum(getSampleValues(t, jsonResults, "iterations", expTags)))
	assert.Equal(t, 3, len(getSampleValues(t, jsonResults, "iteration_duration", expTags)))
	assert.Less(t, float64(0), sum(getSampleValues(t, jsonResults, "data_received", expTags)))
	assert.Less(t, float64(0), sum(getSampleValues(t, jsonResults, "data_sent", expTags)))

	expTags["over"] = "the rainbow" // we overwrite this in most with custom tags in the script
	assert.Equal(t, float64(6), sum(getSampleValues(t, jsonResults, "checks", expTags)))
	assert.Equal(t, float64(3), sum(getSampleValues(t, jsonResults, "mycounter", expTags)))

	expTags["group"] = "::http"
	assert.Equal(t, float64(3), sum(getSampleValues(t, jsonResults, "checks", expTags)))
	assert.Equal(t, float64(6), sum(getSampleValues(t, jsonResults, "http_reqs", expTags)))
	assert.Equal(t, 6, len(getSampleValues(t, jsonResults, "http_req_duration", expTags)))
	expTags["expected_response"] = "true"
	assert.Equal(t, float64(3), sum(getSampleValues(t, jsonResults, "http_reqs", expTags)))
	assert.Equal(t, 3, len(getSampleValues(t, jsonResults, "http_req_duration", expTags)))
	expTags["expected_response"] = "false"
	assert.Equal(t, float64(3), sum(getSampleValues(t, jsonResults, "http_reqs", expTags)))
	assert.Equal(t, 3, len(getSampleValues(t, jsonResults, "http_req_duration", expTags)))
	delete(expTags, "expected_response")

	expTags["group"] = "::websockets"
	assert.Equal(t, float64(3), sum(getSampleValues(t, jsonResults, "checks", expTags)))
	assert.Equal(t, float64(3), sum(getSampleValues(t, jsonResults, "ws_sessions", expTags)))
	assert.Equal(t, float64(3), sum(getSampleValues(t, jsonResults, "ws_msgs_sent", expTags)))
	assert.Equal(t, float64(3), sum(getSampleValues(t, jsonResults, "ws_msgs_received", expTags)))
	assert.Equal(t, 3, len(getSampleValues(t, jsonResults, "ws_session_duration", expTags)))
	assert.Equal(t, 0, len(getSampleValues(t, jsonResults, "http_req_duration", expTags)))
	expTags["check"] = "status is 101"
	assert.Equal(t, float64(3), sum(getSampleValues(t, jsonResults, "checks", expTags)))
}

func TestRunWithCloudOutputOverrides(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(
		t, "export default function () {};",
		[]string{"-v", "--log-output=stdout", "--out=cloud", "--out", "json=results.json"}, 0,
	)

	configOverride := http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		resp.WriteHeader(http.StatusOK)
		_, err := fmt.Fprint(resp, `{"reference_id": "132", "config": {"webAppURL": "https://bogus.url"}}`)
		assert.NoError(t, err)
	})
	srv := getCloudTestEndChecker(t, 132, configOverride, cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed)
	ts.Env["K6_CLOUD_HOST"] = srv.URL

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, "execution: local")
	assert.Contains(t, stdout, "output: cloud (https://bogus.url/runs/132), json (results.json)")
	assert.Contains(t, stdout, "iterations...........: 1")
}

func TestRunWithCloudOutputCustomConfigAndOverridesLegacyCloudOption(t *testing.T) {
	t.Parallel()

	script := `
export const options = {
  ext: {
    loadimpact: {
      name: 'Hello k6 Cloud!',
      projectID: 123456,
    },
  },
};

export default function() {};`

	ts := getSingleFileTestState(t, script, []string{"-v", "--log-output=stdout", "--out=cloud"}, 0)

	configOverride := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		b, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		bjs := string(b)
		assert.Contains(t, bjs, `"name":"Hello k6 Cloud!"`)
		assert.Contains(t, bjs, `"project_id":123456`)

		resp.WriteHeader(http.StatusOK)
		_, err = fmt.Fprint(resp, `{
			"reference_id": "1337",
			"config": {
				"webAppURL": "https://bogus.url",
				"testRunDetails": "https://some.other.url/foo/tests/org/1337?bar=baz"
			},
			"logs": [
				{"level": "debug", "message": "test debug message"},
				{"level": "info", "message": "test message"}
			]
		}`)
		assert.NoError(t, err)
	})
	srv := getCloudTestEndChecker(t, 1337, configOverride, cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed)
	ts.Env["K6_CLOUD_HOST"] = srv.URL

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, "execution: local")
	assert.Contains(t, stdout, "output: cloud (https://some.other.url/foo/tests/org/1337?bar=baz)")
	assert.Contains(t, stdout, `level=debug msg="test debug message" output=cloud source=grafana-k6-cloud`)
	assert.Contains(t, stdout, `level=info msg="test message" output=cloud source=grafana-k6-cloud`)
}

func TestRunWithCloudOutputCustomConfigAndOverrides(t *testing.T) {
	t.Parallel()

	script := `
export const options = {
  cloud: {
    name: 'Hello k6 Cloud!',
    projectID: 123456,
  },
};

export default function() {};`

	ts := getSingleFileTestState(t, script, []string{"-v", "--log-output=stdout", "--out=cloud"}, 0)

	configOverride := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		b, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		bjs := string(b)
		assert.Contains(t, bjs, `"name":"Hello k6 Cloud!"`)
		assert.Contains(t, bjs, `"project_id":123456`)

		resp.WriteHeader(http.StatusOK)
		_, err = fmt.Fprint(resp, `{
			"reference_id": "1337",
			"config": {
				"webAppURL": "https://bogus.url",
				"testRunDetails": "https://some.other.url/foo/tests/org/1337?bar=baz"
			},
			"logs": [
				{"level": "debug", "message": "test debug message"},
				{"level": "info", "message": "test message"}
			]
		}`)
		assert.NoError(t, err)
	})
	srv := getCloudTestEndChecker(t, 1337, configOverride, cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed)
	ts.Env["K6_CLOUD_HOST"] = srv.URL

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, "execution: local")
	assert.Contains(t, stdout, "output: cloud (https://some.other.url/foo/tests/org/1337?bar=baz)")
	assert.Contains(t, stdout, `level=debug msg="test debug message" output=cloud source=grafana-k6-cloud`)
	assert.Contains(t, stdout, `level=info msg="test message" output=cloud source=grafana-k6-cloud`)
}

func TestPrometheusRemoteWriteOutput(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)
	ts.Env["K6_PROMETHEUS_RW_SERVER_URL"] = "http://a-fake-url-for-fail"
	ts.CmdArgs = []string{"k6", "run", "--out", "experimental-prometheus-rw", "-"}
	ts.Stdin = bytes.NewBufferString(`export default function () {};`)

	cmd.ExecuteWithGlobalState(ts.GlobalState)
	ts.OutMutex.Lock()
	stdout := ts.Stdout.String()
	ts.OutMutex.Unlock()

	assert.Contains(t, stdout, "output: Prometheus remote write")
}

func BenchmarkReadResponseBody(b *testing.B) {
	httpSrv := httpmultibin.NewHTTPMultiBin(b)

	script := httpSrv.Replacer.Replace(`
		import http from "k6/http";
		import { check, sleep } from "k6";

		let statusCheck = { "status is 200": (r) => r.status === 200 }

		export let options = {
			duration: '10s',
			vus: 10
		};

		export default function () {
			let bytes = randomIntBetween(100 * 1024, 5 * 1024 * 1024)

			let response = http.get(http.url` + "`HTTPBIN_IP_URL/bytes/${bytes}`" + `)
			check(response, statusCheck)

			let responses = http.batch([
										["GET", http.url` + "`HTTPBIN_IP_URL/stream-bytes/${bytes}`" + `],
										["GET", http.url` + "`HTTPBIN_IP_URL/stream-bytes/${bytes}`" + `],
										["GET", http.url` + "`HTTPBIN_IP_URL/bytes/${bytes}`" + `],
										["GET", http.url` + "`HTTPBIN_IP_URL/bytes/${bytes}`" + `],
										["GET", http.url` + "`HTTPBIN_IP_URL/gzip`" + `],
										["GET", http.url` + "`HTTPBIN_IP_URL/deflate`" + `],
										["GET", http.url` + "`HTTPBIN_IP_URL/image/jpeg`" + `],
									]);
			responses.forEach(res => check(res, statusCheck))
			sleep(0.1)
		};

		function randomIntBetween(min, max) {
			return Math.floor(Math.random() * (max - min + 1) + min);
		}
	`)

	ts := getSimpleCloudOutputTestState(b, script, nil, cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)
}

func TestUIRenderOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		outputs   []string
		expRender string
	}{
		{outputs: []string{}, expRender: "output: -\n"},
		{outputs: []string{"json"}, expRender: "output: json(stdout)\n\n"},
		{outputs: []string{"json", "csv"}, expRender: "output: json(stdout), csv (file.csv)\n\n"},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.expRender, func(t *testing.T) {
			t.Parallel()

			ts := NewGlobalTestState(t)
			ts.CmdArgs = []string{"k6", "run"}
			for _, o := range tc.outputs {
				ts.CmdArgs = append(ts.CmdArgs, "-o")
				ts.CmdArgs = append(ts.CmdArgs, o)
			}
			ts.CmdArgs = append(ts.CmdArgs, "-")
			ts.Stdin = bytes.NewBufferString(`export default function() {};`)
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			stdout := ts.Stdout.String()
			assert.Contains(t, stdout, tc.expRender)
		})
	}
}

func TestUIRenderWebDashboard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		env       string
		active    bool
		expRender string
	}{
		{expRender: "web dashboard:"},
		{env: "false", expRender: "web dashboard:"},
		{env: "true", active: true, expRender: "web dashboard: http://127.0.0.1:"},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.expRender, func(t *testing.T) {
			t.Parallel()

			ts := NewGlobalTestState(t)
			if tc.env != "" {
				ts.Env["K6_WEB_DASHBOARD"] = tc.env
			}
			ts.Env["K6_WEB_DASHBOARD_PORT"] = "0"
			ts.CmdArgs = []string{"k6", "run", "--log-output=stdout"}
			ts.CmdArgs = append(ts.CmdArgs, "-")
			ts.Stdin = bytes.NewBufferString(`export default function() {};`)
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			if tc.active {
				assert.Contains(t, ts.Stdout.String(), tc.expRender)
			} else {
				assert.NotContains(t, ts.Stdout.String(), tc.expRender)
			}
		})
	}
}

// TestRunStaticArchives tests that the static archives are working as expected.
// each archive contains the following files/catalogs:
// ├── a.js
// ├── foo
// │   └── bar.js
// ├── sample
// │   └── data.json
// └── script.js
// archive was made using binary & platform from the test name
func TestRunStaticArchives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		archive string
	}{
		{archive: "archive_v0.42.0_linux.tar"},
		{archive: "archive_v0.42.0_windows.tar"},
	}

	for _, tc := range tests {
		tc := tc

		t.Run("Using "+tc.archive, func(t *testing.T) {
			t.Parallel()

			ts := NewGlobalTestState(t)

			data, err := os.ReadFile(filepath.Join("testdata/archives", tc.archive)) //nolint:forbidigo // it's a test
			require.NoError(t, err)

			require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), data, 0o644))

			ts.CmdArgs = []string{"k6", "run", "--log-output=stdout", "archive.tar"}

			cmd.ExecuteWithGlobalState(ts.GlobalState)
			stdout := ts.Stdout.String()
			assert.Contains(t, stdout, "called default() from script.js")
			assert.Contains(t, stdout, "called Bar() from foo/bar.js")
			assert.Contains(t, stdout, "called A() from a.js")
			assert.Contains(t, stdout, "extracted john doe from sample/data.json")
		})
	}
}

func TestBadLogOutput(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"NotExist":      "badout",
		"FileBadConfig": "file=,levels=bad",
		"LokiBadConfig": "loki=,levels=bad",
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts := NewGlobalTestState(t)
			ts.CmdArgs = []string{"k6", "run", "--log-output", tc, "-"}
			ts.Stdin = bytes.NewBufferString(`export default function () {};`)
			ts.ExpectedExitCode = -1
			cmd.ExecuteWithGlobalState(ts.GlobalState)
		})
	}
}

// HACK: We need this so multiple tests can register differently named modules.
var uniqueModuleNumber uint64 //nolint:gochecknoglobals

// Tests that the appropriate events are emitted in the correct order.
func TestEventSystemOK(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)

	moduleName := fmt.Sprintf("k6/x/testevents-%d", atomic.AddUint64(&uniqueModuleNumber, 1))
	mod := events.New(event.GlobalEvents, event.VUEvents)
	modules.Register(moduleName, mod)

	ts.CmdArgs = []string{"k6", "--quiet", "run", "-"}
	ts.Stdin = bytes.NewBuffer([]byte(fmt.Sprintf(`
		import events from '%s';
		import { sleep } from 'k6';

		export let options = {
			vus: 1,
			iterations: 5,
		}

		export default function () { sleep(1); }
	`, moduleName)))

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	doneCh := make(chan struct{})
	go func() {
		mod.WG.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	expLog := []string{
		`got event Init with data '<nil>'`,
		`got event TestStart with data '<nil>'`,
		`got event IterStart with data '{Iteration:0 VUID:1 ScenarioName:default Error:<nil>}'`,
		`got event IterEnd with data '{Iteration:0 VUID:1 ScenarioName:default Error:<nil>}'`,
		`got event IterStart with data '{Iteration:1 VUID:1 ScenarioName:default Error:<nil>}'`,
		`got event IterEnd with data '{Iteration:1 VUID:1 ScenarioName:default Error:<nil>}'`,
		`got event IterStart with data '{Iteration:2 VUID:1 ScenarioName:default Error:<nil>}'`,
		`got event IterEnd with data '{Iteration:2 VUID:1 ScenarioName:default Error:<nil>}'`,
		`got event IterStart with data '{Iteration:3 VUID:1 ScenarioName:default Error:<nil>}'`,
		`got event IterEnd with data '{Iteration:3 VUID:1 ScenarioName:default Error:<nil>}'`,
		`got event IterStart with data '{Iteration:4 VUID:1 ScenarioName:default Error:<nil>}'`,
		`got event IterEnd with data '{Iteration:4 VUID:1 ScenarioName:default Error:<nil>}'`,
		`got event TestEnd with data '<nil>'`,
		`got event Exit with data '&{Error:<nil>}'`,
	}
	log := ts.LoggerHook.Lines()
	assert.Equal(t, expLog, log)
}

// Check emitted events in the case of a script error.
func TestEventSystemError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name, script string
		expLog       []string
		expExitCode  exitcodes.ExitCode
	}{
		{
			name: "abort",
			script: `
				import { test } from 'k6/execution';

				export let options = {
					vus: 1,
					iterations: 5,
				}

				export default function () {
					test.abort('oops!');
				}
		`, expLog: []string{
				"got event Init with data '<nil>'",
				"got event TestStart with data '<nil>'",
				"got event IterStart with data '{Iteration:0 VUID:1 ScenarioName:default Error:<nil>}'",
				"got event IterEnd with data '{Iteration:0 VUID:1 ScenarioName:default Error:test aborted: oops! at default (file:///-:11:16(5))}'",
				"got event TestEnd with data '<nil>'",
				"got event Exit with data '&{Error:test aborted: oops! at default (file:///-:11:16(5))}'",
				"test aborted: oops! at default (file:///-:11:16(5))",
			},
			expExitCode: exitcodes.ScriptAborted,
		},
		{
			name:   "init",
			script: "undefinedVar",
			expLog: []string{
				"got event Exit with data '&{Error:could not initialize '-': could not load JS test " +
					"'file:///-': ReferenceError: undefinedVar is not defined\n\tat file:///-:2:1(8)\n}'",
				"ReferenceError: undefinedVar is not defined\n\tat file:///-:2:1(8)\n",
			},
			expExitCode: exitcodes.ScriptException,
		},
		{
			name: "throw",
			script: `
				export let options = {
					vus: 1,
					iterations: 2,
				}

				export default function () {
					throw new Error('oops!');
				}
		`, expLog: []string{
				"got event Init with data '<nil>'",
				"got event TestStart with data '<nil>'",
				"got event IterStart with data '{Iteration:0 VUID:1 ScenarioName:default Error:<nil>}'",
				"got event IterEnd with data '{Iteration:0 VUID:1 ScenarioName:default Error:Error: oops!\n\tat default (file:///-:9:12(3))\n}'",
				"Error: oops!\n\tat default (file:///-:9:12(3))\n",
				"got event IterStart with data '{Iteration:1 VUID:1 ScenarioName:default Error:<nil>}'",
				"got event IterEnd with data '{Iteration:1 VUID:1 ScenarioName:default Error:Error: oops!\n\tat default (file:///-:9:12(3))\n}'",
				"Error: oops!\n\tat default (file:///-:9:12(3))\n",
				"got event TestEnd with data '<nil>'",
				"got event Exit with data '&{Error:<nil>}'",
			},
			expExitCode: 0,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts := NewGlobalTestState(t)

			moduleName := fmt.Sprintf("k6/x/testevents-%d", atomic.AddUint64(&uniqueModuleNumber, 1))
			mod := events.New(event.GlobalEvents, event.VUEvents)
			modules.Register(moduleName, mod)

			ts.CmdArgs = []string{"k6", "--quiet", "run", "-"}
			ts.ExpectedExitCode = int(tc.expExitCode)
			ts.Stdin = bytes.NewBuffer([]byte(fmt.Sprintf("import events from '%s';\n%s", moduleName, tc.script)))

			cmd.ExecuteWithGlobalState(ts.GlobalState)

			doneCh := make(chan struct{})
			go func() {
				mod.WG.Wait()
				close(doneCh)
			}()

			select {
			case <-doneCh:
			case <-time.After(time.Second):
				t.Fatal("timed out")
			}

			log := ts.LoggerHook.Lines()
			assert.Equal(t, tc.expLog, log)
		})
	}
}

func BenchmarkRun(b *testing.B) {
	b.StopTimer()

	for i := 0; i < b.N; i++ {
		ts := NewGlobalTestState(b)

		ts.CmdArgs = []string{"k6", "--quiet", "run", "-"}
		ts.Stdin = bytes.NewBuffer([]byte(`
		export let options = {
			vus: 10,
			iterations: 100,
		}

		export default function () {}
		`))
		ts.ExpectedExitCode = 0

		b.StartTimer()
		cmd.ExecuteWithGlobalState(ts.GlobalState)
		b.StopTimer()
	}
}

func BenchmarkRunEvents(b *testing.B) {
	b.StopTimer()

	for i := 0; i < b.N; i++ {
		ts := NewGlobalTestState(b)

		moduleName := fmt.Sprintf("k6/x/testevents-%d", atomic.AddUint64(&uniqueModuleNumber, 1))
		mod := events.New(event.GlobalEvents, event.VUEvents)
		modules.Register(moduleName, mod)

		ts.CmdArgs = []string{"k6", "--quiet", "run", "-"}
		ts.Stdin = bytes.NewBuffer([]byte(fmt.Sprintf(`
		import events from '%s';
		export let options = {
			vus: 10,
			iterations: 100,
		}

		export default function () {}
		`, moduleName)))
		ts.ExpectedExitCode = 0

		b.StartTimer()
		cmd.ExecuteWithGlobalState(ts.GlobalState)
		b.StopTimer()

		doneCh := make(chan struct{})
		go func() {
			mod.WG.Wait()
			close(doneCh)
		}()

		select {
		case <-doneCh:
		case <-time.After(time.Second):
			b.Fatal("timed out")
		}
	}
}

func TestBrowserPermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		options          string
		expectedExitCode exitcodes.ExitCode
		expectedError    string
	}{
		// When we do not supply the correct browser options,
		// we expect that the test iteration will stop and not
		// attempt to start a chrome instance.
		{
			name:             "browser option not set",
			options:          "",
			expectedExitCode: 0,
			expectedError:    "browser not found in registry. make sure to set browser type option in scenario definition in order to use the browser module",
		},
		// When we do supply the correct browser options,
		// we expect that the browser module will start
		// the a chrome instance and work with the browser
		// APIs defined in the test script.
		//
		// We do not actually want to rely on having chrome
		// installed on dev/ci machines, and all we need to
		// verify is that the browser module does try to exec
		// a command when the correct browser options are supplied.
		// This test will try to run a "fake" command and we
		// expect that the test will fail when attempting to
		// run that command.
		{
			name: "browser option set",
			options: `export const options = {
				scenarios: {
						browser: {
						executor: 'shared-iterations',
						options: {
							browser: {
								type: 'chromium',
							},
						},
					},
				},
			}`,
			expectedExitCode: 108,
			expectedError:    "k6-browser-fake-cmd",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			script := fmt.Sprintf(`
			import { browser } from 'k6/browser';

			%s

			export default function() {
			  browser.isConnected()
			};
			`, tt.options)

			ts := getSingleFileTestState(t, script, []string{}, tt.expectedExitCode)
			ts.Env["K6_BROWSER_EXECUTABLE_PATH"] = "k6-browser-fake-cmd"
			cmd.ExecuteWithGlobalState(ts.GlobalState)
			loglines := ts.LoggerHook.Drain()

			assert.Contains(t, loglines[0].Message, tt.expectedError)
		})
	}
}

func TestBrowserExperimentalImport(t *testing.T) {
	t.Parallel()

	const script = `
		import { browser } from 'k6/experimental/browser';

		export const options = {
			scenarios: {
					browser: {
					executor: 'shared-iterations',
					options: {
						browser: {
							type: 'chromium',
						},
					},
				},
			},
		}

		export default function() {
		};
	`

	const wantExitCode = 107
	ts := getSingleFileTestState(t, script, []string{}, wantExitCode)
	ts.Env["K6_BROWSER_EXECUTABLE_PATH"] = "k6-browser-fake-cmd"
	cmd.ExecuteWithGlobalState(ts.GlobalState)
	loglines := ts.LoggerHook.Drain()

	assert.Contains(t, loglines[0].Message, "use k6/browser instead of k6/experimental/browser")
}

func TestSetupTimeout(t *testing.T) {
	t.Parallel()
	ts := NewGlobalTestState(t)
	ts.ExpectedExitCode = int(exitcodes.SetupTimeout)
	ts.CmdArgs = []string{"k6", "run", "-"}
	ts.Stdin = bytes.NewBufferString(`
		import { sleep } from 'k6';

		export const options = {
			setupTimeout: '1s',
		};

		export function setup() { sleep(100000); };
		export default function() {}
	`)

	start := time.Now()
	cmd.ExecuteWithGlobalState(ts.GlobalState)
	elapsed := time.Since(start)
	assert.Greater(t, elapsed, 1*time.Second, "expected more time to have passed because of setupTimeout")
	assert.Less(
		t, elapsed, 5*time.Second,
		"expected less time to have passed because setupTimeout ",
	)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	stderr := ts.Stderr.String()
	t.Log(stderr)
	assert.Contains(t, stderr, "setup() execution timed out after 1 seconds")
}

func TestTypeScriptSupport(t *testing.T) {
	t.Parallel()
	depScript := `
		export default function(): number {
			let p: number = 42;
			return p;
		}
	`
	mainScript := `
		import bar from "./bar.ts";
		let s: string = "something";
		export default function() {
			console.log(s, bar());
		};
	`

	ts := NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.ts"), []byte(mainScript), 0o644))
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "bar.ts"), []byte(depScript), 0o644))

	ts.CmdArgs = []string{"k6", "run", "--quiet", "test.ts"}

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stderr := ts.Stderr.String()
	t.Log(stderr)
	assert.Contains(t, stderr, `something 42`)
}
