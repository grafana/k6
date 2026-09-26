package oauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

	token, err := FetchK6Token(t.Context(), server.URL+proxyAPIPath, "gat_test")
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
		"unauthorized names the permission": {
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr: "gcx User role",
		},
		"forbidden names the permission": {
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			wantErr: "gcx User role",
		},
		"not found blames the missing app": {
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: "k6 app may not be installed",
		},
		"empty body still reports the status": {
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: "no details given",
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
			_, err := FetchK6Token(t.Context(), server.URL+proxyAPIPath, "gat_test")
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestFetchK6TokenClipsAHugeErrorBody(t *testing.T) {
	t.Parallel()

	// The body is read up to maxResponseBytes; none of that belongs in a
	// terminal, and the server controls its length.
	server, _ := newFakeK6Plugin(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("x", 100_000)))
	})

	_, err := FetchK6Token(t.Context(), server.URL+proxyAPIPath, "gat_test")
	require.Error(t, err)
	assert.Less(t, len(err.Error()), 1_000, "a server must not be able to flood the terminal")
	assert.Contains(t, err.Error(), "…")
}

func TestFetchK6TokenRefusesUntrustedHost(t *testing.T) {
	t.Parallel()

	// The access token must never be sent to a host outside Grafana Cloud, even
	// though the endpoint it came from was supplied by the browser.
	_, err := FetchK6Token(t.Context(), "https://evil.example.com", "gat_test")
	require.ErrorContains(t, err, "not a Grafana Cloud domain")
}

func TestFetchK6TokenErrorOmitsTheAccessToken(t *testing.T) {
	t.Parallel()

	server, _ := newFakeK6Plugin(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := FetchK6Token(t.Context(), server.URL+proxyAPIPath, "gat_secret_value")
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
