package oauth

import (
	"context"
	"encoding/json"
	"errors"
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

// FetchK6Token reads the authenticated user's existing k6 personal API token,
// trying each host in order and returning the first that answers. It reads the
// token rather than regenerating one, so logging in on a new machine does not
// invalidate the token already in use elsewhere.
//
// It returns the token and the host that served it.
func FetchK6Token(ctx context.Context, hosts []string, accessToken string) (string, string, error) {
	var errs []error
	for _, host := range hosts {
		if host == "" {
			continue
		}
		token, err := fetchK6TokenFrom(ctx, host, accessToken)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		return token, host, nil
	}
	if len(errs) == 0 {
		return "", "", errors.New("no host to read the k6 API token from")
	}
	return "", "", errors.Join(errs...)
}

func fetchK6TokenFrom(ctx context.Context, host, accessToken string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	requestURL := strings.TrimSuffix(host, "/") + accountPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := trustedClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach %s: %w", host, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("could not read the response from %s: %w", host, err)
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
		return "", fmt.Errorf("could not parse the account response from %s: %w", host, err)
	}
	if account.Token.Key == "" {
		return "", fmt.Errorf("%s returned no k6 API token", requestURL)
	}
	return account.Token.Key, nil
}
