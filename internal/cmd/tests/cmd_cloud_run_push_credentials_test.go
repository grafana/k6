package tests

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	provtest "go.k6.io/k6/v2/internal/cloudapi/provisioning/test"
	v6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib/fsext"
)

// Sentinel credentials. Each has a single, distinct source so a test can
// tell exactly which one k6 used to push metrics.
const (
	orgToken         = "org-longlived-token" // K6_CLOUD_TOKEN: legacy / relay push
	scopedToken      = "test-run-token-abc"  // from the provisioning response
	bogusToken       = "bogus-env-token"     // a stray value in the environment
	extToken         = "ext-scoped-token"    // an externally-supplied scoped token
	staleConfigToken = "stale-config-token"  // a stale value in a k6 config file
)

const pushCredScript = `
export const options = { cloud: { name: 'push creds', projectID: 123456 } };
export default function () {};`

// cloudRequestRecorder captures the bearer token of every request the mock
// cloud sees, so a test can assert which token k6 pushed metrics with.
type cloudRequestRecorder struct {
	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct{ path, token string }

func (r *cloudRequestRecorder) handler(w http.ResponseWriter, req *http.Request) {
	_, _ = io.Copy(io.Discard, req.Body)
	r.mu.Lock()
	r.requests = append(r.requests, recordedRequest{req.URL.Path, bearerToken(req)})
	r.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (r *cloudRequestRecorder) tokensForPath(sub string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, req := range r.requests {
		if strings.Contains(req.path, sub) {
			out = append(out, req.token)
		}
	}
	return out
}

func (r *cloudRequestRecorder) allTokens() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.requests))
	for _, req := range r.requests {
		out = append(out, req.token)
	}
	return out
}

// bearerToken extracts the credential from the Authorization header,
// tolerating both schemes k6 uses: "Bearer <tok>" for the provisioning/v6
// push and "Token <tok>" for the legacy v1 push.
func bearerToken(req *http.Request) string {
	auth := req.Header.Get("Authorization")
	auth = strings.TrimPrefix(auth, "Bearer ")
	auth = strings.TrimPrefix(auth, "Token ")
	return auth
}

func failHandler(t *testing.T, msg string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		assert.Fail(t, msg)
		w.WriteHeader(http.StatusInternalServerError)
	})
}

// assertPushedWith asserts that at least one request hit a path containing
// sub, and that every such request carried the expected bearer token.
func assertPushedWith(t *testing.T, rec *cloudRequestRecorder, sub, token string) {
	t.Helper()
	tokens := rec.tokensForPath(sub)
	require.NotEmptyf(t, tokens, "expected a metrics push to a path containing %q", sub)
	for _, tk := range tokens {
		assert.Equalf(t, token, tk, "push to %q used an unexpected bearer token", sub)
	}
}

func assertTokenNeverUsed(t *testing.T, rec *cloudRequestRecorder, token string) {
	t.Helper()
	assert.NotContainsf(t, rec.allTokens(), token, "token %q must never be sent", token)
}

func assertPathNeverHit(t *testing.T, rec *cloudRequestRecorder, sub string) {
	t.Helper()
	assert.Emptyf(t, rec.tokensForPath(sub), "no request should hit a path containing %q", sub)
}

// setupSelfProvisionedRun wires a self-provisioned `k6 cloud run
// --local-execution` against a provisioning mock whose start_local_execution
// returns the scoped token and a push URL that points back at the mock
// (/v1/metrics).
func setupSelfProvisionedRun(
	t *testing.T,
) (*GlobalTestState, *provtest.Server, *cloudRequestRecorder, *atomic.Bool) {
	t.Helper()

	ts := makeTestState(t, pushCredScript, []string{"--local-execution"})
	ts.Env["K6_CLOUD_TOKEN"] = orgToken

	srv := provtest.NewServer(t)
	rec := &cloudRequestRecorder{}
	var notified atomic.Bool

	srv.HandleCreateLoadTest(123456, func(w http.ResponseWriter, _ *http.Request) {
		res := k6cloud.NewLoadTestApiModelWithDefaults()
		res.SetId(provtest.DefaultLoadTestID)
		writeProvJSON(w, http.StatusCreated, res)
	})
	srv.HandleStartLocalExecution(provtest.DefaultLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		resp := provtest.DefaultStartLocalExecutionResponse()
		resp.SetArchiveUploadUrl(srv.PresignedUploadURL())
		resp.SetTestRunDetailsPageUrl(fmt.Sprintf("%s/runs/%d", srv.URL, provtest.DefaultTestRunID))
		rc := resp.GetRuntimeConfig()
		m := rc.GetMetrics()
		m.SetPushUrl(srv.URL + "/v1/metrics")
		rc.SetMetrics(m)
		resp.SetRuntimeConfig(rc)
		writeProvJSON(w, http.StatusOK, resp)
	})
	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{{Status: v6.StatusInitializing}})
	srv.HandleNotify(provtest.DefaultTestRunID, func(w http.ResponseWriter, _ *http.Request) {
		notified.Store(true)
		w.WriteHeader(http.StatusOK)
	})

	// Record the provisioning-mode push (/v1/metrics) and any stray push
	// (e.g. an overridden URL landing on the catch-all).
	srv.Mux.HandleFunc("/v1/metrics", rec.handler)
	srv.Mux.HandleFunc("/", rec.handler)

	ts.Env["K6_CLOUD_HOST"] = srv.URL
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
	return ts, srv, rec, &notified
}

// TestCloudMetricsPushCredentials specifies how the cloud output resolves
// the endpoint and bearer token it pushes metrics with, across the three
// ways a run can be provisioned:
//
//   - Self-provisioned (k6 cloud run --local-execution): the push URL and
//     scoped token come from the provisioning API's start_local_execution
//     response. They are set programmatically and are never overridden by
//     the environment or a config file.
//   - Externally-provisioned (--local-execution + K6_CLOUD_PUSH_REF_ID): an
//     orchestrator that already created the run supplies the scoped push URL
//     and token via K6_CLOUD_METRICS_PUSH_URL and K6_CLOUD_TEST_RUN_TOKEN,
//     which are required together.
//   - Legacy (k6 run --out cloud, or a PushRefID relay without scoped
//     creds): metrics go to the endpoint derived from the host using the
//     long-lived org token.
//
// Each case runs the real command against an in-process mock cloud
// (httptest, no network) and asserts which bearer token k6 pushes with,
// using distinct sentinel tokens so the source is unambiguous.
func TestCloudMetricsPushCredentials(t *testing.T) {
	t.Parallel()

	// Self-provisioned: creds come from the provisioning response and must
	// not be overridden by stray env vars or config-file values.

	t.Run("self-provisioned run pushes with the scoped token from provisioning", func(t *testing.T) {
		t.Parallel()
		ts, _, rec, notified := setupSelfProvisionedRun(t)
		cmd.ExecuteWithGlobalState(ts.GlobalState)
		assertPushedWith(t, rec, "/v1/metrics", scopedToken)
		assert.True(t, notified.Load(), "a self-provisioned run notifies at the end")
	})

	t.Run("self-provisioned run ignores a stray token env var", func(t *testing.T) {
		t.Parallel()
		ts, _, rec, notified := setupSelfProvisionedRun(t)
		ts.Env["K6_CLOUD_TEST_RUN_TOKEN"] = bogusToken
		cmd.ExecuteWithGlobalState(ts.GlobalState)
		assertPushedWith(t, rec, "/v1/metrics", scopedToken)
		assertTokenNeverUsed(t, rec, bogusToken)
		assert.True(t, notified.Load())
	})

	t.Run("self-provisioned run ignores a stray metrics-push-URL env var", func(t *testing.T) {
		t.Parallel()
		ts, srv, rec, _ := setupSelfProvisionedRun(t)
		ts.Env["K6_CLOUD_METRICS_PUSH_URL"] = srv.URL + "/overridden-metrics"
		cmd.ExecuteWithGlobalState(ts.GlobalState)
		assertPushedWith(t, rec, "/v1/metrics", scopedToken)
		assertPathNeverHit(t, rec, "/overridden-metrics")
	})

	t.Run("self-provisioned run ignores stray scoped-cred env vars", func(t *testing.T) {
		t.Parallel()
		ts, srv, rec, _ := setupSelfProvisionedRun(t)
		ts.Env["K6_CLOUD_TEST_RUN_TOKEN"] = bogusToken
		ts.Env["K6_CLOUD_METRICS_PUSH_URL"] = srv.URL + "/overridden-metrics"
		cmd.ExecuteWithGlobalState(ts.GlobalState)
		assertPushedWith(t, rec, "/v1/metrics", scopedToken)
		assertTokenNeverUsed(t, rec, bogusToken)
		assertPathNeverHit(t, rec, "/overridden-metrics")
	})

	t.Run("self-provisioned run ignores a stale token in the config file", func(t *testing.T) {
		t.Parallel()
		ts, _, rec, _ := setupSelfProvisionedRun(t)
		cfg := []byte(`{"collectors":{"cloud":{"testRunToken":"` + staleConfigToken + `"}}}`)
		require.NoError(t, ts.FS.MkdirAll(filepath.Dir(ts.Flags.ConfigFilePath), 0o755))
		require.NoError(t, fsext.WriteFile(ts.FS, ts.Flags.ConfigFilePath, cfg, 0o644))
		cmd.ExecuteWithGlobalState(ts.GlobalState)
		assertPushedWith(t, rec, "/v1/metrics", scopedToken)
		assertTokenNeverUsed(t, rec, staleConfigToken)
	})

	// Legacy: `k6 run --out cloud` and PushRefID relays push to the
	// host-derived endpoint with the long-lived org token, unaffected by the
	// scoped-cred env vars.

	t.Run("k6 run --out cloud ignores a stray token env var", func(t *testing.T) {
		t.Parallel()
		ts := getSingleFileTestState(t, pushCredScript, []string{"-v", "--log-output=stdout", "--out=cloud"}, 0)
		ts.Env["K6_CLOUD_TOKEN"] = orgToken
		ts.Env["K6_CLOUD_TEST_RUN_TOKEN"] = bogusToken

		rec := &cloudRequestRecorder{}
		const refID = 1337
		srv := getTestServer(t, map[string]http.Handler{
			"POST ^/v1/tests$": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, `{"reference_id": "%d", "config": {}}`, refID)
			}),
			fmt.Sprintf("POST ^/v1/tests/%d$", refID): http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
			"POST ^/v2/metrics/": http.HandlerFunc(rec.handler),
		})
		t.Cleanup(srv.Close)
		ts.Env["K6_CLOUD_HOST"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)
		assertPushedWith(t, rec, "/v2/metrics/", orgToken)
		assertTokenNeverUsed(t, rec, bogusToken)
	})

	t.Run("a PushRefID relay without scoped creds pushes with the org token", func(t *testing.T) {
		t.Parallel()
		ts := makeTestState(t, pushCredScript, []string{"--local-execution"})
		ts.Env["K6_CLOUD_TOKEN"] = orgToken
		ts.Env["K6_CLOUD_PUSH_REF_ID"] = "99999"

		rec := &cloudRequestRecorder{}
		srv := getTestServer(t, map[string]http.Handler{
			"POST ^/v1/tests$":        failHandler(t, "CreateTestRun must not be called with PushRefID"),
			"POST ^/provisioning/v1/": failHandler(t, "provisioning API must not be called with PushRefID"),
			"POST ^/cloud/v6/":        failHandler(t, "v6 API must not be called with PushRefID"),
			"POST ^/v2/metrics/":      http.HandlerFunc(rec.handler),
		})
		t.Cleanup(srv.Close)
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)
		assertPushedWith(t, rec, "/v2/metrics/", orgToken)
	})

	t.Run("k6 run --out cloud with PushRefID ignores a lone scoped-cred env var", func(t *testing.T) {
		t.Parallel()
		ts := getSingleFileTestState(t, pushCredScript, []string{"-v", "--log-output=stdout", "--out=cloud"}, 0)
		ts.Env["K6_CLOUD_TOKEN"] = orgToken
		ts.Env["K6_CLOUD_PUSH_REF_ID"] = "1337"
		ts.Env["K6_CLOUD_TEST_RUN_TOKEN"] = extToken // lone scoped cred, no push URL

		rec := &cloudRequestRecorder{}
		srv := getTestServer(t, map[string]http.Handler{
			"POST ^/v1/tests$":   failHandler(t, "CreateTestRun must not be called with PushRefID"),
			"POST ^/v2/metrics/": http.HandlerFunc(rec.handler),
		})
		t.Cleanup(srv.Close)
		ts.Env["K6_CLOUD_HOST"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)
		assertPushedWith(t, rec, "/v2/metrics/", orgToken)
	})

	// Externally-provisioned: an orchestrator supplies the scoped creds via
	// env; both are required together.

	t.Run("externally-provisioned run pushes with the scoped creds from env", func(t *testing.T) {
		t.Parallel()
		ts := makeTestState(t, pushCredScript, []string{"--local-execution"})
		ts.Env["K6_CLOUD_TOKEN"] = orgToken
		ts.Env["K6_CLOUD_PUSH_REF_ID"] = "99999"

		rec := &cloudRequestRecorder{}
		srv := getTestServer(t, map[string]http.Handler{
			"POST ^/provisioning/v1/": failHandler(t, "provisioning API must not be called with PushRefID"),
			"POST ^/cloud/v6/":        failHandler(t, "v6 API must not be called with PushRefID"),
			"POST ^/v1/metrics":       http.HandlerFunc(rec.handler),
			"POST ^/v2/metrics/":      http.HandlerFunc(rec.handler),
		})
		t.Cleanup(srv.Close)
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
		ts.Env["K6_CLOUD_METRICS_PUSH_URL"] = srv.URL + "/v1/metrics"
		ts.Env["K6_CLOUD_TEST_RUN_TOKEN"] = extToken

		cmd.ExecuteWithGlobalState(ts.GlobalState)
		assertPushedWith(t, rec, "/v1/metrics", extToken)
	})

	t.Run("externally-provisioned run requires both scoped creds", func(t *testing.T) {
		t.Parallel()
		ts := makeTestState(t, pushCredScript, []string{"--local-execution", "--log-output=stdout"})
		ts.Env["K6_CLOUD_TOKEN"] = orgToken
		ts.Env["K6_CLOUD_PUSH_REF_ID"] = "99999"
		ts.Env["K6_CLOUD_TEST_RUN_TOKEN"] = extToken // only one of the pair
		ts.ExpectedExitCode = -1

		srv := getTestServer(t, map[string]http.Handler{})
		t.Cleanup(srv.Close)
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)
		out := ts.Stdout.String() + ts.Stderr.String()
		assert.Contains(t, out, "must be set together",
			"a partial scoped-cred pair must fail with a clear error")
	})
}
