package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFakeK6Plugin serves the k6 app plugin's account route, recording the
// Authorization header it was called with.
func newFakeK6Plugin(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *string) {
	t.Helper()

	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc(proxyAPIPath+accountPath, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		handler(w, r)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, &gotAuth
}

func TestFetchK6Token(t *testing.T) {
	t.Parallel()

	server, gotAuth := newFakeK6Plugin(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{"token": map[string]string{"key": "k6-api-token"}})
	})

	token, err := FetchK6Token(context.Background(), server.URL+proxyAPIPath, "gat_test")
	require.NoError(t, err)
	assert.Equal(t, "k6-api-token", token)
	assert.Equal(t, "Bearer gat_test", *gotAuth)
}

func TestFetchK6TokenErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		handler http.HandlerFunc
		wantErr string
	}{
		"unauthorized": {
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"nope"}`))
			},
			wantErr: "status 401",
		},
		"no token in response": {
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, map[string]any{"token": map[string]string{"key": ""}})
			},
			wantErr: "returned no k6 API token",
		},
		"unparseable response": {
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("not json"))
			},
			wantErr: "could not parse the account response",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server, _ := newFakeK6Plugin(t, tc.handler)
			_, err := FetchK6Token(context.Background(), server.URL+proxyAPIPath, "gat_test")
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestFetchK6TokenRefusesUntrustedHost(t *testing.T) {
	t.Parallel()

	// The access token must never be sent to a host outside Grafana Cloud, even
	// though the endpoint it came from was supplied by the browser.
	_, err := FetchK6Token(context.Background(), "https://evil.example.com", "gat_test")
	require.ErrorContains(t, err, "not a Grafana Cloud domain")
}

func TestFetchK6TokenErrorOmitsTheAccessToken(t *testing.T) {
	t.Parallel()

	server, _ := newFakeK6Plugin(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := FetchK6Token(context.Background(), server.URL+proxyAPIPath, "gat_secret_value")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "gat_secret_value", "errors are logged, so must not carry the token")
}

func TestResultAPIBase(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://a.grafana.net/assistant"+proxyAPIPath,
		(&Result{ProxyEndpoint: "https://a.grafana.net/assistant"}).APIBase())
	assert.Equal(t, "https://a.grafana.net/assistant"+proxyAPIPath,
		(&Result{ProxyEndpoint: "https://a.grafana.net/assistant/"}).APIBase(),
		"a trailing slash must not produce a doubled separator")
	assert.Empty(t, (&Result{}).APIBase())
}
