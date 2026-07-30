package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// accountPath is the k6 app plugin's resource route for the authenticated
// user's k6 Cloud account. The plugin forwards the call to the k6 Cloud API
// with stack-scoped credentials attached, so a Grafana token is enough to
// reach it.
const accountPath = "/api/plugins/k6-app/resources/cloud/v3/account/me"

// FetchK6Token reads the authenticated user's existing k6 personal API token
// from apiBase, which must be a Result's APIBase. It reads the token rather
// than regenerating one, so logging in on a new machine does not invalidate the
// token already in use elsewhere.
func FetchK6Token(ctx context.Context, apiBase, accessToken string) (string, error) {
	if err := validateGrafanaURL(apiBase); err != nil {
		return "", fmt.Errorf("cannot read the k6 API token from %q: %w", apiBase, err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	requestURL := strings.TrimSuffix(apiBase, "/") + accountPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	// apiBase derives from the browser's login response, so it is tainted — but
	// validateGrafanaURL above has confined it to Grafana Cloud, and
	// trustedClient re-checks every redirect target.
	resp, err := trustedClient().Do(req) //nolint:gosec // G704: apiBase is allowlisted above
	if err != nil {
		return "", fmt.Errorf("could not reach %s: %w", apiBase, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("could not read the response from %s: %w", apiBase, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", accountError(requestURL, resp.StatusCode, raw)
	}

	var account struct {
		Token struct {
			Key string `json:"key"`
		} `json:"token"`
	}
	if err := json.Unmarshal(raw, &account); err != nil {
		return "", fmt.Errorf("could not parse the account response from %s: %w", apiBase, err)
	}
	if account.Token.Key == "" {
		return "", fmt.Errorf(
			"%s returned no k6 API token; open the k6 app on your stack once to have one created",
			requestURL)
	}
	return account.Token.Key, nil
}

// accountError explains a failed account read. The two most likely causes — the
// app not being installed, and the user lacking permission to mint CLI tokens —
// are indistinguishable from the status code alone, so the server's own message
// is kept alongside the hint rather than replaced by it.
func accountError(requestURL string, status int, body []byte) error {
	detail := strings.TrimSpace(stripControlChars(string(body)))
	if detail == "" {
		detail = "no details given"
	}

	var hint string
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		hint = "; the login may lack permission to read k6 tokens" +
			" — check that your Grafana user has the gcx User role"
	case http.StatusNotFound:
		hint = "; the k6 app may not be installed on this stack"
	}

	return fmt.Errorf("%s returned status %d%s: %s", requestURL, status, hint, detail)
}
