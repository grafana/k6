package cmd

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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/httpmultibin"
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
	assert.Contains(t, stdOut, "k6/x/alarmist")

	assert.Empty(t, ts.stdErr.Bytes())
	assert.Empty(t, ts.loggerHook.Drain())
}

func TestSimpleTestStdin(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "run", "-"}
	ts.console.Stdin = &testOSFileR{bytes.NewBufferString(`export default function() {};`)}
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
	ts.console.Stdin = &testOSFileR{bytes.NewBufferString(`
		export default function() {};
		export function handleSummary(data) {
			return {}; // silence the end of test summary
		};
	`)}
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
	ts.console.Stdin = &testOSFileR{bytes.NewBufferString(`
		console.log('init');
		export default function() { console.log('foo'); };
	`)}
	newRootCommand(ts.globalState).execute()

	// The test state hook still catches this message
	assert.True(t, testutils.LogContains(ts.loggerHook.Drain(), logrus.InfoLevel, `foo`))

	// But it's not shown on stderr or stdout
	assert.Empty(t, ts.stdErr.Bytes())
	assert.Empty(t, ts.stdOut.Bytes())

	// Instead it should be in the log file
	logContents, err := afero.ReadFile(ts.fs, logFilePath)
	require.NoError(t, err)
	assert.Equal(t, "init\ninit\nfoo\n", string(logContents)) //nolint:dupword
}

func TestRelativeLogPathWithSetupAndTeardown(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)

	ts.args = []string{"k6", "--log-output", "file=test.log", "--log-format", "raw", "run", "-i", "2", "-"}
	ts.console.Stdin = &testOSFileR{bytes.NewBufferString(`
		console.log('init');
		export default function() { console.log('foo'); };
		export function setup() { console.log('bar'); };
		export function teardown() { console.log('baz'); };
	`)}
	newRootCommand(ts.globalState).execute()

	// The test state hook still catches these messages
	logEntries := ts.loggerHook.Drain()
	assert.True(t, testutils.LogContains(logEntries, logrus.InfoLevel, `foo`))
	assert.True(t, testutils.LogContains(logEntries, logrus.InfoLevel, `bar`))
	assert.True(t, testutils.LogContains(logEntries, logrus.InfoLevel, `baz`))

	// And check that the log file also contains everything
	logContents, err := afero.ReadFile(ts.fs, filepath.Join(ts.cwd, "test.log"))
	require.NoError(t, err)
	assert.Equal(t, "init\ninit\ninit\nbar\nfoo\nfoo\ninit\nbaz\ninit\n", string(logContents)) //nolint:dupword
}

func TestWrongCliFlagIterations(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "run", "--iterations", "foo", "-"}
	ts.console.Stdin = &testOSFileR{bytes.NewBufferString(`export default function() {};`)}
	// TODO: check for exitcodes.InvalidConfig after https://github.com/loadimpact/k6/issues/883 is done...
	ts.expectedExitCode = -1
	newRootCommand(ts.globalState).execute()
	assert.True(t, testutils.LogContains(ts.loggerHook.Drain(), logrus.ErrorLevel, `invalid argument "foo"`))
}

func TestWrongEnvVarIterations(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "run", "--vus", "2", "-"}
	ts.envVars["K6_ITERATIONS"] = "4"
	ts.console.Stdin = &testOSFileR{bytes.NewBufferString(`export default function() {};`)}

	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()
	t.Log(stdOut)
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

func TestSSLKEYLOGFILEAbsolute(t *testing.T) {
	t.Parallel()
	ts := newGlobalTestState(t)
	testSSLKEYLOGFILE(t, ts, filepath.Join(ts.cwd, "ssl.log"))
}

func TestSSLKEYLOGFILEARelative(t *testing.T) {
	t.Parallel()
	ts := newGlobalTestState(t)
	testSSLKEYLOGFILE(t, ts, "./ssl.log")
}

func testSSLKEYLOGFILE(t *testing.T, ts *globalTestState, filePath string) {
	t.Helper()

	// TODO don't use insecureSkipTLSVerify when/if tlsConfig is given to the runner from outside
	tb := httpmultibin.NewHTTPMultiBin(t)
	ts.args = []string{"k6", "run", "-"}
	ts.envVars["SSLKEYLOGFILE"] = filePath
	ts.console.Stdin = &testOSFileR{bytes.NewReader([]byte(tb.Replacer.Replace(`
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
  `)))}

	newRootCommand(ts.globalState).execute()

	assert.True(t,
		testutils.LogContains(ts.loggerHook.Drain(), logrus.WarnLevel, "SSLKEYLOGFILE was specified"))
	sslloglines, err := afero.ReadFile(ts.fs, filepath.Join(ts.cwd, "ssl.log"))
	require.NoError(t, err)
	// TODO maybe have multiple depending on the ciphers used as that seems to change it
	assert.Regexp(t, "^CLIENT_[A-Z_]+ [0-9a-f]+ [0-9a-f]+\n", string(sslloglines))
}

func TestThresholdDeprecationWarnings(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "run", "--system-tags", "url,error,vu,iter,scenario", "-"}
	ts.console.Stdin = &testOSFileR{bytes.NewReader([]byte(`
		export const options = {
			thresholds: {
				'http_req_duration{url:https://test.k6.io}': ['p(95)<500', 'p(99)<1000'],
				'http_req_duration{error:foo}': ['p(99)<1000'],
				'iterations{scenario:default}': ['count == 1'],
				'iterations{vu:1,iter:0}': ['count == 0'], // iter and vu are now unindexable
			},
		};

		export default function () { }`,
	))}

	newRootCommand(ts.globalState).execute()

	logs := ts.loggerHook.Drain()

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

	assert.Len(t, ts.loggerHook.Drain(), 0)
	assert.Contains(t, ts.stdOut.String(), `
     one..................: 0   0/s
       { tag:xyz }........: 0   0/s
     two..................: 42`)
}

func getTestServer(t *testing.T, routes map[string]http.Handler) *httptest.Server {
	mux := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		for methodAndRoute, handler := range routes {
			methodRouteTuple := strings.SplitN(methodAndRoute, " ", 2)
			regex, err := regexp.Compile(methodRouteTuple[1])
			require.NoError(t, err)

			if req.Method == methodRouteTuple[0] && regex.Match([]byte(req.URL.String())) {
				handler.ServeHTTP(resp, req)
				return
			}
		}

		// By default, respond with 200 OK to all unmatched requests
		resp.WriteHeader(http.StatusOK)
	})
	return httptest.NewServer(mux)
}

func getCloudTestEndChecker(t *testing.T, expRunStatus lib.RunStatus, expResultStatus cloudapi.ResultStatus) *httptest.Server {
	testFinished := false
	srv := getTestServer(t, map[string]http.Handler{
		"POST ^/v1/tests$": http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			resp.WriteHeader(http.StatusOK)
			_, err := fmt.Fprintf(resp, `{"reference_id": "111"}`)
			assert.NoError(t, err)
		}),
		"POST ^/v1/tests/111$": http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			require.NotNil(t, req.Body)
			buf := &bytes.Buffer{}
			_, err := io.Copy(buf, req.Body)
			require.NoError(t, err)
			require.NoError(t, req.Body.Close())

			body := buf.Bytes()
			require.True(t, gjson.ValidBytes(body))

			runStatus := gjson.GetBytes(body, "run_status")
			require.True(t, runStatus.Exists()) // important to check, since run_status can be 0
			assert.Equalf(
				t, expRunStatus, lib.RunStatus(runStatus.Int()),
				"received wrong run_status value",
			)

			resultStatus := gjson.GetBytes(body, "result_status")
			require.True(t, resultStatus.Exists())
			assert.Equalf(
				t, expResultStatus, cloudapi.ResultStatus(resultStatus.Int()),
				"received wrong result_status value",
			)
			testFinished = true
		}),
	})

	t.Cleanup(func() {
		assert.Truef(t, testFinished, "expected test to have called the cloud API endpoint to finish the test")
		srv.Close()
	})

	return srv
}

func getSimpleCloudOutputTestState(
	t *testing.T, script []byte, cliFlags []string,
	expRunStatus lib.RunStatus, expResultStatus cloudapi.ResultStatus, expExitCode int,
) *globalTestState {
	srv := getCloudTestEndChecker(t, expRunStatus, expResultStatus)

	if cliFlags == nil {
		cliFlags = []string{"-v", "--log-output=stdout"}
	}

	ts := newGlobalTestState(t)
	require.NoError(t, afero.WriteFile(ts.fs, filepath.Join(ts.cwd, "test.js"), script, 0o644))
	ts.envVars["K6_CLOUD_HOST"] = srv.URL
	ts.args = append([]string{"k6", "run", "--out", "cloud", "test.js"}, cliFlags...)
	ts.expectedExitCode = expExitCode

	return ts
}

func TestSetupTeardownThresholds(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	script := []byte(tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import { Counter } from "k6/metrics";

		let statusCheck = { "status is 200": (r) => r.status === 200 }
		let myCounter = new Counter("setup_teardown");

		export let options = {
			iterations: 5,
			thresholds: {
				"setup_teardown": ["count == 2"],
				"iterations": ["count == 5"],
				"http_reqs": ["count == 7"],
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
	`))

	ts := getSimpleCloudOutputTestState(t, script, nil, lib.RunStatusFinished, cloudapi.ResultStatusPassed, 0)
	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()
	assert.Contains(t, stdOut, `✓ http_reqs......................: 7`)
	assert.Contains(t, stdOut, `✓ iterations.....................: 5`)
	assert.Contains(t, stdOut, `✓ setup_teardown.................: 2`)

	logMsgs := ts.loggerHook.Drain()
	for _, msg := range logMsgs {
		if msg.Level != logrus.DebugLevel {
			assert.Failf(t, "unexpected log message", "level %s, msg '%s'", msg.Level, msg.Message)
		}
	}
	assert.True(t, testutils.LogContains(logMsgs, logrus.DebugLevel, "Metrics emission of VUs and VUsMax metrics stopped"))
	assert.True(t, testutils.LogContains(logMsgs, logrus.DebugLevel, "Metrics processing finished!"))
}

func TestThresholdsFailed(t *testing.T) {
	t.Parallel()
	script := []byte(`
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
	`)

	// Since these thresholds don't have an abortOnFail property, the run_status
	// in the cloud will still be Finished, even if the test itself failed.
	ts := getSimpleCloudOutputTestState(
		t, script, nil, lib.RunStatusFinished, cloudapi.ResultStatusFailed, int(exitcodes.ThresholdsHaveFailed),
	)
	newRootCommand(ts.globalState).execute()

	assert.True(t, testutils.LogContains(ts.loggerHook.Drain(), logrus.ErrorLevel, `some thresholds have failed`))
	stdOut := ts.stdOut.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, `   ✓ iterations...........: 3`)
	assert.Contains(t, stdOut, `     ✗ { scenario:sc1 }...: 1`)
	assert.Contains(t, stdOut, `     ✗ { scenario:sc2 }...: 2`)
	assert.Contains(t, stdOut, `     ✓ { scenario:sc3 }...: 0   0/s`)
}

func TestAbortedByThreshold(t *testing.T) {
	t.Parallel()
	script := []byte(`
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
	`)

	ts := getSimpleCloudOutputTestState(
		t, script, nil, lib.RunStatusAbortedThreshold, cloudapi.ResultStatusFailed, int(exitcodes.ThresholdsHaveFailed),
	)
	newRootCommand(ts.globalState).execute()

	assert.True(t, testutils.LogContains(ts.loggerHook.Drain(), logrus.ErrorLevel, `some thresholds have failed`))
	stdOut := ts.stdOut.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, `✗ iterations`)
	assert.Contains(t, stdOut, `teardown() called`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics processing finished!"`)
	assert.Contains(t, stdOut, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=8 tainted=true`)
}

func TestAbortedByUserWithGoodThresholds(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import { Counter } from 'k6/metrics';

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

		export default function () {};
	`)

	ts := getSimpleCloudOutputTestState(t, script, nil, lib.RunStatusAbortedUser, cloudapi.ResultStatusPassed, 0)
	ts.globalState.signalNotify = func(c chan<- os.Signal, s ...os.Signal) {
		go func() {
			// simulate a Ctrl+C after 3 seconds
			time.Sleep(3 * time.Second)
			c <- os.Interrupt
		}()
	}
	ts.globalState.signalStop = func(c chan<- os.Signal) { /* noop */ }

	newRootCommand(ts.globalState).execute()

	assert.False(t, testutils.LogContains(ts.loggerHook.Drain(), logrus.ErrorLevel, `some thresholds have failed`))
	stdOut := ts.stdOut.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, `✓ iterations`)
	assert.Contains(t, stdOut, `✓ tc`)
	assert.Contains(t, stdOut, `✓ { group:::teardown }`)
	assert.Contains(t, stdOut, `Stopping k6 in response to signal`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics processing finished!"`)
	assert.Contains(t, stdOut, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=5 tainted=false`)
}

func TestAbortedByUserWithRestAPI(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import { sleep } from 'k6';
		export default function () {
			console.log('a simple iteration')
			sleep(1);
		};

		export function teardown() {
			console.log('teardown() called');
		}
	`)

	ts := getSimpleCloudOutputTestState(
		t, script, []string{"-v", "--log-output=stdout", "--iterations", "20"},
		lib.RunStatusAbortedUser, cloudapi.ResultStatusPassed, 0,
	)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		newRootCommand(ts.globalState).execute()
	}()

	reachedIteration := false
	for i := 0; i <= 10 && reachedIteration == false; i++ {
		time.Sleep(1 * time.Second)
		stdOut := ts.stdOut.String()

		if !strings.Contains(stdOut, "a simple iteration") {
			t.Logf("did not see an iteration on try %d at t=%s", i, time.Now())
			continue
		}

		reachedIteration = true
		req, err := http.NewRequestWithContext(
			ts.ctx, http.MethodPatch, fmt.Sprintf("http://%s/v1/status", ts.flags.address),
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
	}

	assert.True(t, reachedIteration)

	wg.Wait()
	stdOut := ts.stdOut.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, `a simple iteration`)
	assert.Contains(t, stdOut, `teardown() called`)
	assert.Contains(t, stdOut, `PATCH /v1/status`)
	assert.Contains(t, stdOut, `run: stopped by user; exiting...`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics processing finished!"`)
	assert.Contains(t, stdOut, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=5 tainted=false`)
}

func TestAbortedByScriptSetupErrorWithDependency(t *testing.T) {
	t.Parallel()
	depScript := []byte(`
		export default function () {
			baz();
		}
		function baz() {
			throw new Error("baz");
		}
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`)
	mainScript := []byte(`
		import bar from "./bar.js";
		export function setup() {
			console.log('wonky setup');
			bar();
		};
		export default function() {};

		export { handleSummary } from "./bar.js";
	`)

	srv := getCloudTestEndChecker(t, lib.RunStatusAbortedScriptError, cloudapi.ResultStatusPassed)

	ts := newGlobalTestState(t)
	require.NoError(t, afero.WriteFile(ts.fs, filepath.Join(ts.cwd, "test.js"), mainScript, 0o644))
	require.NoError(t, afero.WriteFile(ts.fs, filepath.Join(ts.cwd, "bar.js"), depScript, 0o644))

	ts.envVars["K6_CLOUD_HOST"] = srv.URL
	ts.args = []string{"k6", "run", "-v", "--out", "cloud", "--log-output=stdout", "test.js"}
	ts.expectedExitCode = int(exitcodes.ScriptException)

	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, `wonky setup`)

	rootPath := "file:///"
	if runtime.GOOS == "windows" {
		rootPath += "c:/"
	}
	assert.Contains(t, stdOut, `level=error msg="Error: baz\n\tat baz (`+rootPath+`test/bar.js:6:9(3))\n\tat `+
		rootPath+`test/bar.js:3:3(3)\n\tat setup (`+rootPath+`test/test.js:5:3(9))\n" hint="script exception"`)
	assert.Contains(t, stdOut, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=7 tainted=false`)
	assert.Contains(t, stdOut, "bogus summary")
}

func runTestWithNoLinger(t *testing.T, ts *globalTestState) {
	newRootCommand(ts.globalState).execute()
}

func runTestWithLinger(t *testing.T, ts *globalTestState) {
	ts.args = append(ts.args, "--linger")

	sendSignal := make(chan struct{})
	ts.globalState.signalNotify = func(c chan<- os.Signal, s ...os.Signal) {
		go func() {
			<-sendSignal
			c <- os.Interrupt
		}()
	}
	ts.globalState.signalStop = func(c chan<- os.Signal) { /* noop */ }

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		newRootCommand(ts.globalState).execute()
	}()

	testFinished := false
	for i := 0; i <= 15 && testFinished == false; i++ {
		time.Sleep(1 * time.Second)
		stdOut := ts.stdOut.String()

		if !strings.Contains(stdOut, "Linger set; waiting for Ctrl+C") {
			t.Logf("test wasn't finished on try %d at t=%s", i, time.Now())
			continue
		}
		testFinished = true
		close(sendSignal)
	}

	require.True(t, testFinished)
	wg.Wait()
}

func TestAbortedByScriptSetupError(t *testing.T) {
	t.Parallel()
	script := []byte(`
		export function setup() {
			console.log('wonky setup');
			throw new Error('foo');
		}

		export function teardown() {
			console.log('nice teardown');
		}

		export default function () {};

		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`)

	doChecks := func(t *testing.T, ts *globalTestState) {
		stdOut := ts.stdOut.String()
		assert.Contains(t, stdOut, "Error: foo")
		assert.Contains(t, stdOut, "wonky setup")
		assert.NotContains(t, stdOut, "nice teardown") // do not execute teardown if setup failed
		assert.Contains(t, stdOut, "bogus summary")
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
	script := []byte(`
		export function setup() {
			console.log('nice setup');
		}

		export function teardown() {
			console.log('wonky teardown');
			throw new Error('foo');
		}

		export default function () {};

		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`)

	doChecks := func(t *testing.T, ts *globalTestState) {
		stdOut := ts.stdOut.String()
		assert.Contains(t, stdOut, "Error: foo")
		assert.Contains(t, stdOut, "nice setup")
		assert.Contains(t, stdOut, "wonky teardown")
		assert.Contains(t, stdOut, "bogus summary")
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

func testAbortedByScriptError(t *testing.T, script []byte, runTest func(*testing.T, *globalTestState)) *globalTestState {
	ts := getSimpleCloudOutputTestState(
		t, script, nil, lib.RunStatusAbortedScriptError, cloudapi.ResultStatusPassed, int(exitcodes.ScriptException),
	)
	runTest(t, ts)

	stdOut := ts.stdOut.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics processing finished!"`)
	assert.Contains(t, stdOut, `level=debug msg="Everything has finished, exiting k6!"`)
	assert.Contains(t, stdOut, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=7 tainted=false`)
	return ts
}

func TestAbortedByTestAbortFirstInitCode(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import exec from 'k6/execution';
		exec.test.abort('foo');
		export default function () {};

		// Should not be called, since error is in the init context
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`)

	ts := newGlobalTestState(t)
	require.NoError(t, afero.WriteFile(ts.fs, filepath.Join(ts.cwd, "test.js"), script, 0o644))
	ts.args = []string{"k6", "run", "-v", "--log-output=stdout", "test.js"}
	ts.expectedExitCode = int(exitcodes.ScriptAborted)

	newRootCommand(ts.globalState).execute()
	stdOut := ts.stdOut.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, "test aborted: foo")
	assert.NotContains(t, stdOut, "bogus summary")
}

func TestAbortedByTestAbortInNonFirstInitCode(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import exec from 'k6/execution';

		export const options = {vus: 3, duration: '5s'};

		if (__VU > 1) {
			exec.test.abort('foo');
		}

		export default function () {};

		// Should not be called, since error is in the init context
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`)

	// FIXME: when VU initialization is properly synchronized, replace the
	// following lines with this line only:
	//
	//   ts := testAbortedByScriptTestAbort(t, false, script, runTestWithNoLinger)
	//
	// See https://github.com/grafana/k6/issues/2790 for details. Right now we
	// need the stdOut locking because VU initialization is not properly synchronized:
	// when a test is aborted during the init phase, some logs might be emitted
	// after the root command returns...

	ts := getSimpleCloudOutputTestState(
		t, script, nil, lib.RunStatusAbortedUser, cloudapi.ResultStatusPassed, int(exitcodes.ScriptAborted),
	)
	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, "test aborted: foo")
	assert.Contains(t, stdOut, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=5 tainted=false`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.NotContains(t, stdOut, "bogus summary")
}

func TestAbortedByScriptAbortInVUCode(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import exec from 'k6/execution';
		export default function () {
			exec.test.abort('foo');
		};
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`)

	t.Run("noLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, true, script, runTestWithNoLinger)
	})

	t.Run("withLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, true, script, runTestWithLinger)
	})
}

func TestAbortedByScriptAbortInSetup(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import exec from 'k6/execution';
		export function setup() {
			exec.test.abort('foo');
		}
		export default function () {};
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`)

	t.Run("noLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, true, script, runTestWithNoLinger)
	})

	t.Run("withLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, true, script, runTestWithLinger)
	})
}

func TestAbortedByScriptAbortInTeardown(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import exec from 'k6/execution';
		export function teardown() {
			exec.test.abort('foo');
		}
		export default function () {};
		export function handleSummary() { return {stdout: '\n\n\nbogus summary\n\n\n'};}
	`)

	t.Run("noLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, true, script, runTestWithNoLinger)
	})

	t.Run("withLinger", func(t *testing.T) {
		t.Parallel()
		testAbortedByScriptTestAbort(t, true, script, runTestWithLinger)
	})
}

func testAbortedByScriptTestAbort(
	t *testing.T, shouldHaveMetrics bool, script []byte, runTest func(*testing.T, *globalTestState),
) *globalTestState {
	ts := getSimpleCloudOutputTestState(
		t, script, nil, lib.RunStatusAbortedUser, cloudapi.ResultStatusPassed, int(exitcodes.ScriptAborted),
	)
	runTest(t, ts)

	stdOut := ts.stdOut.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, "test aborted: foo")
	assert.Contains(t, stdOut, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=5 tainted=false`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	if shouldHaveMetrics {
		assert.Contains(t, stdOut, `level=debug msg="Metrics processing finished!"`)
		assert.Contains(t, stdOut, "bogus summary")
	} else {
		assert.NotContains(t, stdOut, "bogus summary")
	}
	return ts
}

func TestAbortedByScriptInitError(t *testing.T) {
	t.Parallel()
	script := []byte(`
		export const options = {
			vus: 5,
			iterations: 10,
		};

		if (__VU == 2) {
			throw new Error('oops in ' + __VU);
		}

		export default function () {};
	`)

	ts := getSimpleCloudOutputTestState(
		t, script, nil, lib.RunStatusAbortedScriptError, cloudapi.ResultStatusPassed, int(exitcodes.ScriptException),
	)
	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()

	t.Log(stdOut)
	assert.Contains(t, stdOut, `level=error msg="Error: oops in 2\n\tat file:///`)
	assert.Contains(t, stdOut, `hint="error while initializing VU #2 (script exception)"`)
	assert.Contains(t, stdOut, `level=debug msg="Metrics emission of VUs and VUsMax metrics stopped"`)
	assert.Contains(t, stdOut, `level=debug msg="Sending test finished" output=cloud ref=111 run_status=7 tainted=false`)
}

func TestMetricTagAndSetupDataIsolation(t *testing.T) {
	t.Parallel()
	script := []byte(`
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
	`)

	ts := getSimpleCloudOutputTestState(
		t, script, []string{"--quiet", "--log-output", "stdout"},
		lib.RunStatusFinished, cloudapi.ResultStatusPassed, 0,
	)
	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()
	t.Log(stdOut)
	assert.Equal(t, 12, strings.Count(stdOut, "✓"))
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

func max(vals []float64) float64 {
	max := vals[0]
	for _, val := range vals {
		if max < val {
			max = val
		}
	}
	return max
}

func TestActiveVUsCount(t *testing.T) {
	t.Parallel()

	script := []byte(`
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
	`)

	ts := newGlobalTestState(t)
	require.NoError(t, afero.WriteFile(ts.fs, filepath.Join(ts.cwd, "test.js"), script, 0o644))
	ts.args = []string{"k6", "run", "--compatibility-mode", "base", "--out", "json=results.json", "test.js"}
	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()
	t.Log(stdOut)

	jsonResults, err := afero.ReadFile(ts.fs, "results.json")
	require.NoError(t, err)
	// t.Log(string(jsonResults))
	assert.Equal(t, float64(10), max(getSampleValues(t, jsonResults, "vus_max", nil)))
	assert.Equal(t, float64(10), max(getSampleValues(t, jsonResults, "vus", nil)))
	assert.Equal(t, float64(0), sum(getSampleValues(t, jsonResults, "iterations", nil)))

	logEntries := ts.loggerHook.Drain()
	assert.Len(t, logEntries, 4)
	for i, logEntry := range logEntries {
		assert.Equal(t, logrus.WarnLevel, logEntry.Level)
		if i < 3 {
			assert.Equal(t, "Insufficient VUs, reached 10 active VUs and cannot initialize more", logEntry.Message)
		} else {
			assert.Equal(t, "No script iterations finished, consider making the test duration longer", logEntry.Message)
		}
	}
}

func TestMinIterationDuration(t *testing.T) {
	t.Parallel()
	script := []byte(`
		import { Counter } from 'k6/metrics';

		export let options = {
			minIterationDuration: '5s',
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
	`)

	ts := getSimpleCloudOutputTestState(t, script, nil, lib.RunStatusFinished, cloudapi.ResultStatusPassed, 0)

	start := time.Now()
	newRootCommand(ts.globalState).execute()
	elapsed := time.Since(start)
	assert.Greater(t, elapsed, 5*time.Second, "expected more time to have passed because of minIterationDuration")
	assert.Less(
		t, elapsed, 10*time.Second,
		"expected less time to have passed because minIterationDuration should not affect setup() and teardown() ",
	)

	stdOut := ts.stdOut.String()
	t.Log(stdOut)
	assert.Contains(t, stdOut, "✓ test_counter.........: 3")
}

func TestRunTags(t *testing.T) {
	t.Parallel()

	tb := httpmultibin.NewHTTPMultiBin(t)
	script := []byte(tb.Replacer.Replace(`
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
	`))

	ts := newGlobalTestState(t)
	require.NoError(t, afero.WriteFile(ts.fs, filepath.Join(ts.cwd, "test.js"), script, 0o644))
	ts.args = []string{
		"k6", "run", "-u", "2", "--tag", "foo=bar", "--tag", "test=mest", "--tag", "over=written",
		"--log-output=stdout", "--out", "json=results.json", "test.js",
	}
	ts.envVars["K6_ITERATIONS"] = "3"
	ts.envVars["K6_INSECURE_SKIP_TLS_VERIFY"] = "true"
	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()
	t.Log(stdOut)

	jsonResults, err := afero.ReadFile(ts.fs, "results.json")
	require.NoError(t, err)

	expTags := map[string]string{"foo": "bar", "test": "mest", "over": "written", "scenario": "default"}
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

func TestPrometheusRemoteWriteOutput(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "run", "--out", "experimental-prometheus-rw", "-"}
	ts.console.Stdin = &testOSFileR{bytes.NewBufferString(`
		import exec from 'k6/execution';
		export default function () {};
	`)}

	newRootCommand(ts.globalState).execute()

	stdOut := ts.stdOut.String()
	assert.Contains(t, stdOut, "output: Prometheus remote write")
}
