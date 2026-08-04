// Package oauth implements the browser-based OAuth PKCE login flow used by
// `k6 cloud login --oauth`.
//
// The flow is served by the Grafana Assistant app plugin, which exposes a CLI
// auth page on every Grafana Cloud stack. k6 opens that page, waits for a
// redirect to a short-lived callback server on localhost, and exchanges the
// authorization code for a Grafana access token. The access token is only a
// means to an end: it is used once to read the user's k6 personal API token
// (see FetchK6Token) and is then discarded, so k6 needs no token refresh or
// revocation machinery.
//
// The design mirrors github.com/grafana/gcx (internal/auth), which pioneered
// this flow.
package oauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// authPagePath is the CLI auth page served by the Grafana Assistant app.
	authPagePath = "/a/grafana-assistant-app/cli/auth"

	// exchangePath swaps the authorization code for an access token.
	exchangePath = "/api/cli/v1/auth/exchange"

	// proxyAPIPath is the prefix under which the assistant backend forwards
	// requests on to the stack's Grafana API.
	proxyAPIPath = "/api/cli/v1/proxy"

	// callback server port range. The first free port wins.
	callbackPortFirst = 54321
	callbackPortLast  = 54399

	maxResponseBytes = 1 << 20 // 1 MiB is plenty for these JSON payloads
)

// DefaultScopes are the token scopes k6 requests. Reading the k6 personal API
// token is a GET through the k6 app plugin's resource API, so read access to
// the Grafana API is all that is needed.
func DefaultScopes() []string {
	return []string{"grafana-api:read"}
}

// Result is the outcome of a successful login.
type Result struct {
	// AccessToken is the Grafana access token (gat_*). Short-lived, and only
	// used to read the k6 personal API token. Never serialized: k6 persists the
	// k6 API token, not this one.
	AccessToken string `json:"-"`

	// Email identifies the authenticated user, for display.
	Email string

	// ProxyEndpoint is the assistant backend that forwards API requests to the
	// stack with the token's identity attached. Non-empty on success. Use
	// APIBase to address it.
	ProxyEndpoint string

	// StackURL is the Grafana stack the user authenticated against, as
	// reported by the browser (e.g. https://my-team.grafana.net).
	StackURL string
}

// APIBase returns the base URL for Grafana API calls made with AccessToken.
// The proxy endpoint serves the stack's API under a prefix of its own, so a
// Grafana path is appended to this rather than to ProxyEndpoint directly.
func (r *Result) APIBase() string {
	if r.ProxyEndpoint == "" {
		return ""
	}
	return strings.TrimSuffix(r.ProxyEndpoint, "/") + proxyAPIPath
}

// Flow runs the browser login.
type Flow struct {
	// StackURL is the Grafana stack to authenticate against, e.g.
	// https://my-team.grafana.net. It serves the CLI auth page. Required.
	StackURL string

	// Scopes to request. Defaults to DefaultScopes().
	Scopes []string

	// DeviceName labels the token in the user's Grafana device list, so they
	// can tell later which login created it. Omitted when empty.
	DeviceName string

	// Out receives the user-facing progress messages, including the URL to open
	// if the browser does not. Messages are dropped when nil.
	Out io.Writer

	// OpenBrowser launches a URL in the user's browser. Defaults to
	// OpenInBrowser. A non-nil error only downgrades the UX: the URL is
	// printed for the user to open manually.
	OpenBrowser func(context.Context, string) error
}

// Run opens the browser and blocks until the user completes the login, ctx is
// cancelled, or the callback reports a failure.
func (f *Flow) Run(ctx context.Context) (*Result, error) {
	if err := validateGrafanaURL(f.StackURL); err != nil {
		return nil, fmt.Errorf("cannot log in to %q: %w", f.StackURL, err)
	}

	out := f.Out
	if out == nil {
		out = io.Discard
	}
	openBrowser := f.OpenBrowser
	if openBrowser == nil {
		openBrowser = OpenInBrowser
	}

	listener, port, err := listenOnCallbackPort(ctx)
	if err != nil {
		return nil, err
	}
	// The server takes ownership of the listener and closes it on shutdown.

	sess, err := newSession()
	if err != nil {
		_ = listener.Close()
		return nil, err
	}

	resultCh := make(chan *Result, 1)
	// Room for both senders — the callback handler and the serving goroutine —
	// so neither can block forever on a full channel.
	errCh := make(chan error, 2)
	server := f.serveCallback(ctx, listener, sess, resultCh, errCh)
	defer shutdown(ctx, server)

	authURL := f.authURL(port, sess)

	// Progress messages are advisory. A terminal that cannot be written to is
	// not a reason to fail a login, so the write errors are dropped.
	_, _ = fmt.Fprintln(out, "Opening your browser to authenticate with Grafana Cloud.")
	_, _ = fmt.Fprintf(out, "If it doesn't open, visit:\n  %s\n\n", authURL)
	_, _ = fmt.Fprintf(out, "Verification code: %s\n", sess.verificationCode())
	_, _ = fmt.Fprint(out, "Check that it matches the code shown in the browser before approving.\n\n")

	if err := openBrowser(ctx, authURL); err != nil {
		_, _ = fmt.Fprintf(out, "(Could not open a browser automatically: %v)\n", err)
	}
	_, _ = fmt.Fprintln(out, "Waiting for you to complete the login...")

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *Flow) authURL(port int, sess session) string {
	base := strings.TrimSuffix(f.StackURL, "/")

	q := url.Values{}
	q.Set("callback_port", fmt.Sprint(port))
	q.Set("state", sess.state)
	q.Set("code_challenge", sess.challenge)
	q.Set("code_challenge_method", "S256")
	if f.DeviceName != "" {
		q.Set("device_name", f.DeviceName)
	}
	scopes := f.Scopes
	if len(scopes) == 0 {
		scopes = DefaultScopes()
	}
	q.Set("scopes", strings.Join(scopes, ","))

	return base + authPagePath + "?" + q.Encode()
}

// shutdown stops the callback server and releases its port.
//
// The login's context is usually already cancelled by the time this runs — a
// timeout is one of the ways a login ends — so cancellation is stripped from it
// and the shutdown gets a deadline of its own.
func shutdown(ctx context.Context, server *http.Server) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

// serveCallback starts a single-use callback server. Replayed callbacks get a
// 410, so an authorization code cannot be redeemed twice.
func (f *Flow) serveCallback(
	ctx context.Context, listener net.Listener, sess session,
	resultCh chan<- *Result, errCh chan<- error,
) *http.Server {
	var once sync.Once

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		handled := false
		once.Do(func() {
			handled = true
			f.handleCallback(ctx, w, r, sess, resultCh, errCh)
		})
		if !handled {
			http.Error(w, "This login has already been completed.", http.StatusGone)
		}
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("callback server failed: %w", err)
		}
	}()
	return server
}

func (f *Flow) handleCallback(
	ctx context.Context, w http.ResponseWriter, r *http.Request, sess session,
	resultCh chan<- *Result, errCh chan<- error,
) {
	fail := func(err error, userMessage string) {
		errCh <- err
		writeErrorPage(w, userMessage)
	}

	q := r.URL.Query()

	// An empty expected state cannot happen (newSession fails instead), but
	// comparing it would make the check vacuous, so it is rejected explicitly.
	if got := q.Get("state"); sess.state == "" || got != sess.state {
		fail(errors.New("state mismatch: the login response did not come from the request k6 started"),
			"Invalid state parameter.")
		return
	}
	if errMsg := stripControlChars(q.Get("error")); errMsg != "" {
		fail(fmt.Errorf("login was denied: %s", errMsg), errMsg)
		return
	}

	code := q.Get("code")
	if code == "" {
		fail(errors.New("no authorization code in the login response"), "No authorization code received.")
		return
	}

	proxyEndpoint := q.Get("endpoint")
	if err := f.validateCallbackURL(proxyEndpoint); err != nil {
		fail(fmt.Errorf("untrusted API endpoint in the login response: %w", err), "Invalid API endpoint.")
		return
	}

	// Which stack the user picked in the browser. Absent when logging in
	// against a stack URL directly, in which case the caller already knows it.
	stackURL := q.Get("instanceEndpoint")
	if stackURL != "" {
		if err := f.validateCallbackURL(stackURL); err != nil {
			fail(fmt.Errorf("untrusted stack URL in the login response: %w", err), "Invalid stack URL.")
			return
		}
	}

	exchanged, err := exchangeCode(ctx, proxyEndpoint, code, sess.verifier, f.validateCallbackURL)
	if err != nil {
		fail(err, "Could not exchange the authorization code for a token.")
		return
	}

	resultCh <- &Result{
		AccessToken:   exchanged.Data.Token,
		Email:         exchanged.Data.Email,
		ProxyEndpoint: exchanged.Data.APIEndpoint,
		StackURL:      stackURL,
	}
	writeSuccessPage(w)
}

type exchangeResponse struct {
	Data struct {
		Token       string `json:"token"`
		Email       string `json:"email"`
		APIEndpoint string `json:"api_endpoint"`
	} `json:"data"`
}

// validateCallbackURL checks a URL the browser handed back. A flow pointed at a
// local stack is a development or test setup, so local endpoints are acceptable
// there; a flow pointed at Grafana Cloud must stay within Grafana Cloud. Without
// that distinction, a page that induced a login could redirect the code exchange
// — and with it the PKCE verifier — to a process listening on this machine.
func (f *Flow) validateCallbackURL(raw string) error {
	if isLocalURL(f.StackURL) {
		return validateGrafanaURL(raw)
	}
	return validateGrafanaHost(raw)
}

func exchangeCode(
	ctx context.Context, endpoint, code, verifier string, validate func(string) error,
) (*exchangeResponse, error) {
	body, err := json.Marshal(map[string]string{"code": code, "code_verifier": verifier})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	exchangeURL := strings.TrimSuffix(endpoint, "/") + exchangePath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, exchangeURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// The endpoint comes from the browser, so it is tainted — but the caller has
	// already run it through validateGrafanaURL, and trustedClient re-checks
	// every redirect target, so the request cannot leave Grafana Cloud.
	resp, err := trustedClient().Do(req) //nolint:gosec // G704: endpoint is allowlisted above
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("could not read the token exchange response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}

	var result exchangeResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("could not parse the token exchange response: %w", err)
	}
	if result.Data.Token == "" {
		return nil, errors.New("the token exchange response contained no token")
	}
	if err := validate(result.Data.APIEndpoint); err != nil {
		return nil, fmt.Errorf("the token exchange returned an untrusted api_endpoint: %w", err)
	}
	return &result, nil
}

// trustedClient refuses to follow a redirect to a host outside Grafana Cloud,
// so a redirect cannot walk the bearer token off to a third party.
func trustedClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			// Stricter than the endpoint check: a redirect target is entirely
			// server-chosen, so the local-address exemption does not apply.
			// Otherwise a redirect could hand the bearer token to any process
			// listening on this machine.
			if err := validateGrafanaHost(req.URL.Scheme + "://" + req.URL.Host); err != nil {
				return fmt.Errorf("blocked a redirect to an untrusted URL: %w", err)
			}
			return nil
		},
	}
}

// grafanaHostSuffixes are the domains a login response may point k6 at.
func grafanaHostSuffixes() []string {
	return []string{".grafana.net", ".grafana-dev.net", ".grafana-ops.net"}
}

// validateGrafanaURL rejects any URL that is not an HTTPS Grafana Cloud host,
// allowing local addresses so that the flow can be pointed at a development
// server or a test double. Everything the browser hands back is
// attacker-influenced, so it is all checked before k6 sends a token to it.
func validateGrafanaURL(raw string) error {
	if isLocalURL(raw) {
		return nil
	}
	return validateGrafanaHost(raw)
}

// validateGrafanaHost is validateGrafanaURL without the local-address
// exemption, for URLs that no legitimate local setup would produce.
func validateGrafanaHost(raw string) error {
	u, err := parseNonEmptyURL(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return fmt.Errorf("URL must use HTTPS, got %q", u.Scheme)
	}
	hostname := u.Hostname()
	for _, suffix := range grafanaHostSuffixes() {
		if strings.HasSuffix(hostname, suffix) {
			return nil
		}
	}
	return fmt.Errorf("host %q is not a Grafana Cloud domain", hostname)
}

func isLocalURL(raw string) bool {
	u, err := parseNonEmptyURL(raw)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func parseNonEmptyURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, errors.New("URL is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("malformed URL: %w", err)
	}
	if u.Host == "" {
		return nil, errors.New("URL has no host")
	}
	return u, nil
}

func listenOnCallbackPort(ctx context.Context) (net.Listener, int, error) {
	var lc net.ListenConfig
	for port := callbackPortFirst; port <= callbackPortLast; port++ {
		listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return listener, port, nil
		}
	}
	return nil, 0, fmt.Errorf("no free port for the login callback server in range %d-%d",
		callbackPortFirst, callbackPortLast)
}

// session holds the secrets for a single login: the CSRF state, and the PKCE
// verifier with its public challenge.
type session struct {
	state     string
	verifier  string
	challenge string
}

// newSession generates a login's secrets, and fails the login if it cannot.
// Nothing here has a safe fallback: an empty state would make the CSRF check
// vacuous, and a guessable verifier would defeat PKCE.
func newSession() (session, error) {
	state, err := randomBytes()
	if err != nil {
		return session{}, fmt.Errorf("could not generate a login state: %w", err)
	}
	verifier, err := randomBytes()
	if err != nil {
		return session{}, fmt.Errorf("could not generate a PKCE verifier: %w", err)
	}

	sess := session{
		state:    hex.EncodeToString(state),
		verifier: base64.RawURLEncoding.EncodeToString(verifier),
	}
	sess.challenge = sess.challengeFor()
	return sess, nil
}

// challengeFor derives the public PKCE challenge from the verifier. Only the
// challenge is sent to the browser; the verifier is held back until the
// exchange, which is what stops an intercepted authorization code from being
// redeemed by anyone else.
func (s session) challengeFor() string {
	sum := sha256.Sum256([]byte(s.verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomBytes() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// verificationCode is a short, human-comparable digest of the challenge. The
// browser shows the same code, so the user can confirm they are approving the
// login this terminal started and not one an attacker induced.
func (s session) verificationCode() string {
	raw, err := base64.RawURLEncoding.DecodeString(s.challenge)
	if err != nil || len(raw) < 4 {
		return s.challenge
	}
	h := hex.EncodeToString(raw[:4])
	return h[:4] + "-" + h[4:]
}

// stripControlChars keeps server-supplied text from injecting control
// sequences into the terminal or the callback page.
func stripControlChars(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}
