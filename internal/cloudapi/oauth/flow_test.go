package oauth

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGrafana stands in for the browser and the assistant backend: it parses the
// auth URL k6 would open, then calls k6's callback server the way the real
// login page does.
type fakeGrafana struct {
	// exchange serves the token exchange. Defaults to a successful response.
	exchange http.HandlerFunc

	// callbackQuery rewrites the query k6 is called back with, to simulate a
	// hostile or broken login response.
	callbackQuery func(url.Values)

	server *httptest.Server

	// seen records what the auth page was asked for.
	seenChallenge, seenState, seenScopes string
	seenVerifier                         string
}

func newFakeGrafana(t *testing.T) *fakeGrafana {
	t.Helper()
	f := &fakeGrafana{}
	mux := http.NewServeMux()
	mux.HandleFunc(exchangePath, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Code         string `json:"code"`
			CodeVerifier string `json:"code_verifier"`
		}
		// assert, not require: this runs on the server's goroutine, where
		// FailNow is not allowed.
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		f.seenVerifier = body.CodeVerifier

		if f.exchange != nil {
			f.exchange(w, r)
			return
		}
		writeJSON(t, w, map[string]any{"data": map[string]string{
			"token":        "gat_test",
			"email":        "user@example.com",
			"api_endpoint": f.server.URL,
		}})
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

// visit plays the part of the browser: it reads the parameters off the auth URL
// and redirects to k6's callback, as the login page does once the user approves.
//
// It runs on its own goroutine, so it asserts rather than requiring: FailNow
// may only be called from the test goroutine.
func (f *fakeGrafana) visit(t *testing.T, authURL string) {
	t.Helper()

	u, err := url.Parse(authURL)
	if !assert.NoError(t, err) {
		return
	}
	q := u.Query()
	f.seenChallenge = q.Get("code_challenge")
	f.seenState = q.Get("state")
	f.seenScopes = q.Get("scopes")

	callback := url.Values{}
	callback.Set("state", q.Get("state"))
	callback.Set("code", "auth-code")
	callback.Set("endpoint", f.server.URL)
	if f.callbackQuery != nil {
		f.callbackQuery(callback)
	}

	callbackURL := "http://127.0.0.1:" + q.Get("callback_port") + "/callback?" + callback.Encode()
	resp, err := http.Get(callbackURL) //nolint:noctx // short-lived test request
	if !assert.NoError(t, err) {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
}

// writeJSON is called from server handlers, so it asserts rather than requiring.
func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	assert.NoError(t, json.NewEncoder(w).Encode(body))
}

// listenEphemeral binds any free port. Tests must not share the fixed callback
// port range: they run in parallel, and a port a finished flow has released can
// be rebound by another flow while a callback for the first is still in flight,
// delivering it to the wrong server.
func listenEphemeral(ctx context.Context) (net.Listener, int, error) {
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, err
	}
	return listener, listener.Addr().(*net.TCPAddr).Port, nil
}

// runFlow drives a Flow against fake, returning what Run returned.
func runFlow(t *testing.T, fake *fakeGrafana, mutate func(*Flow)) (*Result, error) {
	t.Helper()

	flow := &Flow{
		StackURL:    fake.server.URL,
		Out:         io.Discard,
		OpenBrowser: func(_ context.Context, authURL string) error { go fake.visit(t, authURL); return nil },
		Listen:      listenEphemeral,
	}
	if mutate != nil {
		mutate(flow)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	return flow.Run(ctx)
}

func TestFlowRun(t *testing.T) {
	t.Parallel()

	fake := newFakeGrafana(t)
	result, err := runFlow(t, fake, nil)
	require.NoError(t, err)

	assert.Equal(t, "gat_test", result.AccessToken)
	assert.Equal(t, "user@example.com", result.Email)
	assert.Equal(t, fake.server.URL, result.ProxyEndpoint)
	assert.Equal(t, fake.server.URL+proxyAPIPath, result.APIBase())

	// The verifier must never leave k6 until the exchange, and must be the
	// preimage of the challenge the browser was given.
	assert.NotEmpty(t, fake.seenVerifier)
	assert.NotEqual(t, fake.seenVerifier, fake.seenChallenge)
	assert.Equal(t, fake.seenChallenge, session{verifier: fake.seenVerifier}.challengeFor())
	assert.Equal(t, "grafana-api:read", fake.seenScopes)
}

func TestFlowRunRejectsStateMismatch(t *testing.T) {
	t.Parallel()

	fake := newFakeGrafana(t)
	fake.callbackQuery = func(q url.Values) { q.Set("state", "not-the-state-k6-generated") }

	_, err := runFlow(t, fake, nil)
	require.ErrorContains(t, err, "state mismatch")
}

func TestFlowRunRejectsMissingState(t *testing.T) {
	t.Parallel()

	fake := newFakeGrafana(t)
	fake.callbackQuery = func(q url.Values) { q.Del("state") }

	_, err := runFlow(t, fake, nil)
	require.ErrorContains(t, err, "state mismatch")
}

func TestCallbackRejectsEmptyExpectedState(t *testing.T) {
	t.Parallel()

	// The vacuous-comparison case: were the expected state ever empty, checking
	// it against an absent state parameter would compare "" to "" and pass,
	// disabling the CSRF check without any visible failure.
	flow := &Flow{StackURL: "https://team.grafana.net"}
	resultCh := make(chan *Result, 1)
	errCh := make(chan error, 2)
	recorder := httptest.NewRecorder()

	flow.handleCallback(
		t.Context(),
		recorder,
		httptest.NewRequest(http.MethodGet, "/callback?code=auth-code", nil),
		session{}, // no state, as a failed generation would leave it
		resultCh, errCh,
	)

	require.ErrorContains(t, <-errCh, "state mismatch")
	assert.Empty(t, resultCh, "a login must not succeed without a state check")
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestCallbackRejectsLocalEndpointsFromACloudStack(t *testing.T) {
	t.Parallel()

	// A login against a real stack must not be redirectable to a local address:
	// the exchange carries the PKCE verifier, so a page that induced the login
	// could otherwise collect it from a listener on this machine. The local
	// exemption applies only when the flow itself targets a local stack.
	tests := map[string]string{
		"localhost": "http://localhost:8080",
		"loopback":  "http://127.0.0.1:8080",
		"IPv6":      "http://[::1]:8080",
	}

	for name, endpoint := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			flow := &Flow{StackURL: "https://team.grafana.net"}
			sess, err := newSession()
			require.NoError(t, err)

			resultCh := make(chan *Result, 1)
			errCh := make(chan error, 2)
			target := "/callback?state=" + sess.state + "&code=auth-code&endpoint=" + url.QueryEscape(endpoint)

			flow.handleCallback(
				t.Context(), httptest.NewRecorder(),
				httptest.NewRequest(http.MethodGet, target, nil),
				sess, resultCh, errCh,
			)

			require.ErrorContains(t, <-errCh, "untrusted API endpoint")
			assert.Empty(t, resultCh)
		})
	}
}

func TestCallbackAllowsLocalEndpointsFromALocalStack(t *testing.T) {
	t.Parallel()

	// The counterpart: pointing the flow at a local stack is how development
	// setups and these tests work, so local endpoints stay acceptable there.
	// The full local path is exercised by the httptest-backed flow tests; this
	// pins the decision itself.
	local := &Flow{StackURL: "http://127.0.0.1:3000"}
	require.NoError(t, local.validateCallbackURL("http://127.0.0.1:8080"))

	cloud := &Flow{StackURL: "https://team.grafana.net"}
	require.Error(t, cloud.validateCallbackURL("http://127.0.0.1:8080"))
	require.NoError(t, cloud.validateCallbackURL("https://assistant.grafana.net/assistant"))
}

func TestFlowRunRejectsUntrustedEndpoints(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		param, value, wantErr string
	}{
		"endpoint off-domain": {
			param: "endpoint", value: "https://evil.example.com", wantErr: "untrusted API endpoint",
		},
		"endpoint plain HTTP": {
			param: "endpoint", value: "http://stack.grafana.net", wantErr: "untrusted API endpoint",
		},
		"endpoint lookalike domain": {
			param: "endpoint", value: "https://grafana.net.evil.example.com", wantErr: "untrusted API endpoint",
		},
		"stack off-domain": {
			param: "instanceEndpoint", value: "https://evil.example.com", wantErr: "untrusted stack URL",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fake := newFakeGrafana(t)
			fake.callbackQuery = func(q url.Values) { q.Set(tc.param, tc.value) }

			_, err := runFlow(t, fake, nil)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestFlowRunReportsDeniedLogin(t *testing.T) {
	t.Parallel()

	fake := newFakeGrafana(t)
	fake.callbackQuery = func(q url.Values) { q.Set("error", "user denied\x07 access") }

	_, err := runFlow(t, fake, nil)
	require.ErrorContains(t, err, "login was denied")
	// Control characters from the server must not reach the terminal.
	assert.NotContains(t, err.Error(), "\x07")
}

func TestFlowRunReportsFailedExchange(t *testing.T) {
	t.Parallel()

	fake := newFakeGrafana(t)
	fake.exchange = func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}

	_, err := runFlow(t, fake, nil)
	require.ErrorContains(t, err, "token exchange failed with status 401")
}

func TestFlowRunRejectsExchangeWithUntrustedAPIEndpoint(t *testing.T) {
	t.Parallel()

	fake := newFakeGrafana(t)
	fake.exchange = func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{"data": map[string]string{
			"token":        "gat_test",
			"api_endpoint": "https://evil.example.com",
		}})
	}

	_, err := runFlow(t, fake, nil)
	require.ErrorContains(t, err, "untrusted api_endpoint")
}

func TestFlowRunRejectsUntrustedStackURL(t *testing.T) {
	t.Parallel()

	_, err := runFlow(t, newFakeGrafana(t), func(f *Flow) {
		f.StackURL = "https://evil.example.com"
	})
	require.ErrorContains(t, err, "not a Grafana Cloud domain")
}

func TestFlowRunHonoursCancellation(t *testing.T) {
	t.Parallel()

	fake := newFakeGrafana(t)
	flow := &Flow{
		StackURL:    fake.server.URL,
		Out:         io.Discard,
		OpenBrowser: func(context.Context, string) error { return nil }, // never completed by the user
		Listen:      listenEphemeral,
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := flow.Run(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestFlowRunReleasesCallbackPort(t *testing.T) {
	t.Parallel()

	// Two logins in a row must both work: the first has to release its port
	// even though its server was never gracefully stopped by a caller.
	fake := newFakeGrafana(t)
	_, err := runFlow(t, fake, nil)
	require.NoError(t, err)

	_, err = runFlow(t, newFakeGrafana(t), nil)
	require.NoError(t, err)
}

func TestCallbackIsSingleUse(t *testing.T) {
	t.Parallel()

	fake := newFakeGrafana(t)
	var callbackURL string
	fake.callbackQuery = func(url.Values) {}

	flow := &Flow{
		StackURL: fake.server.URL,
		Out:      io.Discard,
		Listen:   listenEphemeral,
		OpenBrowser: func(_ context.Context, authURL string) error {
			u, err := url.Parse(authURL)
			require.NoError(t, err)
			q := u.Query()
			callbackURL = "http://127.0.0.1:" + q.Get("callback_port") + "/callback?" +
				url.Values{
					"state":    {q.Get("state")},
					"code":     {"auth-code"},
					"endpoint": {fake.server.URL},
				}.Encode()
			go func() {
				resp, err := http.Get(callbackURL) //nolint:noctx // short-lived test request
				if err == nil {
					_ = resp.Body.Close()
				}
			}()
			return nil
		},
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	_, err := flow.Run(ctx)
	require.NoError(t, err)

	// Replaying the same callback against the still-running server is refused.
	resp, err := http.Get(callbackURL) //nolint:noctx // short-lived test request
	if err != nil {
		return // the server has already shut down, which is equally fine
	}
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusGone, resp.StatusCode)
}

func TestNewSessionIsUnpredictable(t *testing.T) {
	t.Parallel()

	first, err := newSession()
	require.NoError(t, err)
	second, err := newSession()
	require.NoError(t, err)

	assert.NotEmpty(t, first.state)
	assert.NotEmpty(t, first.verifier)
	assert.NotEqual(t, first.state, second.state, "state must not repeat across logins")
	assert.NotEqual(t, first.verifier, second.verifier, "verifier must not repeat across logins")
	assert.Equal(t, first.challenge, first.challengeFor(), "challenge must be S256 of the verifier")
}

func TestValidateGrafanaURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		url               string
		wantURL, wantHost bool
	}{
		"stack":             {url: "https://team.grafana.net", wantURL: true, wantHost: true},
		"dev stack":         {url: "https://team.grafana-dev.net", wantURL: true, wantHost: true},
		"ops stack":         {url: "https://team.grafana-ops.net", wantURL: true, wantHost: true},
		"assistant backend": {url: "https://assistant-prod-us-central-0.grafana.net/assistant", wantURL: true, wantHost: true},
		"localhost":         {url: "http://localhost:3000", wantURL: true, wantHost: false},
		"loopback IP":       {url: "http://127.0.0.1:3000", wantURL: true, wantHost: false},
		"plain HTTP stack":  {url: "http://team.grafana.net", wantURL: false, wantHost: false},
		"off-domain":        {url: "https://evil.example.com", wantURL: false, wantHost: false},
		"suffix lookalike":  {url: "https://grafana.net.evil.com", wantURL: false, wantHost: false},
		"empty":             {url: "", wantURL: false, wantHost: false},
		"no host":           {url: "https://", wantURL: false, wantHost: false},
		"garbage":           {url: "://", wantURL: false, wantHost: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.wantURL {
				assert.NoError(t, validateGrafanaURL(tc.url))
			} else {
				assert.Error(t, validateGrafanaURL(tc.url))
			}
			// validateGrafanaHost is the stricter check used for redirects: no
			// local-address exemption.
			if tc.wantHost {
				assert.NoError(t, validateGrafanaHost(tc.url))
			} else {
				assert.Error(t, validateGrafanaHost(tc.url))
			}
		})
	}
}

func TestStripControlChars(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "clean text", stripControlChars("clean\x00 text"))
	assert.Equal(t, "no escape", stripControlChars("no\x1b[31m escape"[:2]+"\x07 escape"))
	assert.Equal(t, "", stripControlChars("\x00\x1b\x7f"))
}

func TestVerificationCodeIsStableAndShort(t *testing.T) {
	t.Parallel()

	sess, err := newSession()
	require.NoError(t, err)

	code := sess.verificationCode()
	assert.Len(t, code, 9, "the code must stay short enough to compare by eye")
	assert.Equal(t, code, sess.verificationCode(), "the code must not change between reads")
	assert.Contains(t, code, "-")

	other, err := newSession()
	require.NoError(t, err)
	assert.NotEqual(t, code, other.verificationCode(), "the code must be login-specific")
}
