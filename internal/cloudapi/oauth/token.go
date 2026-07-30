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

	resp, err := trustedClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach %s: %w", apiBase, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("could not read the response from %s: %w", apiBase, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s returned status %d: %s",
			requestURL, resp.StatusCode, stripControlChars(string(raw)))
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
		return "", fmt.Errorf("%s returned no k6 API token", requestURL)
	}
	return account.Token.Key, nil
}
