package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/cmd"
)

const validToken = "valid-token"
const validStackID = 1234
const validStack = "valid-stack"
const validStackURL = "https://valid-stack.grafana.net"
const defaultProjectID = 5678

func TestCloudLoginWithArgs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		token              string
		stack              string
		wantErr            bool
		wantStdoutContains []string
	}{
		{
			name:    "valid token",
			token:   validToken,
			wantErr: false,
			wantStdoutContains: []string{
				"Logged in successfully",
				fmt.Sprintf("token: %s", validToken),
			},
		},
		{
			name:    "valid token and valid stack",
			token:   validToken,
			stack:   validStack,
			wantErr: false,
			wantStdoutContains: []string{
				"Logged in successfully",
				fmt.Sprintf("token: %s", validToken),
				fmt.Sprintf("stack-id: %d", validStackID),
				fmt.Sprintf("stack-url: %s", validStackURL),
				fmt.Sprintf("default-project-id: %d", defaultProjectID),
			},
		},
		{
			name:    "valid token and 'None' stack",
			token:   validToken,
			stack:   "None",
			wantErr: false,
			wantStdoutContains: []string{
				"Logged in successfully",
				fmt.Sprintf("token: %s", validToken),
			},
		},
		{
			name:               "invalid token",
			token:              "invalid-token",
			wantErr:            true,
			wantStdoutContains: []string{"your API token is invalid"},
		},
		{
			name:               "valid token and invalid stack",
			token:              validToken,
			stack:              "invalid-stack",
			wantErr:            true,
			wantStdoutContains: []string{"your stack is invalid"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := mockValidateTokenServer(t)
			defer srv.Close()

			ts := NewGlobalTestState(t)

			ts.CmdArgs = []string{"k6", "cloud", "login"}
			if tc.token != "" {
				ts.CmdArgs = append(ts.CmdArgs, "--token", tc.token)
			}
			if tc.stack != "" {
				ts.CmdArgs = append(ts.CmdArgs, "--stack", tc.stack)
			}
			ts.Env["K6_CLOUD_HOST"] = srv.URL
			ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

			if tc.wantErr {
				ts.ExpectedExitCode = -1
			} else {
				ts.ExpectedExitCode = 0
			}

			cmd.ExecuteWithGlobalState(ts.GlobalState)

			stdout := ts.Stdout.String()
			stderr := ts.Stderr.String()

			for _, substr := range tc.wantStdoutContains {
				if tc.wantErr {
					assert.Contains(t, stderr, substr)
				} else {
					assert.Contains(t, stdout, substr)
					assert.Contains(t, stderr, "")
				}
			}
		})
	}
}

func mockValidateTokenServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		// v1 path to validate token only
		case "/v1/validate-token":
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)

			var payload map[string]interface{}
			err = json.Unmarshal(body, &payload)
			require.NoError(t, err)

			assert.Contains(t, payload, "token")
			if payload["token"] == validToken {
				_, err = fmt.Fprintf(w, `{"is_valid": true, "message": "Token is valid"}`)
				require.NoError(t, err)
				return
			}
			_, err = fmt.Fprintf(w, `{"is_valid": false, "message": "Token is invalid"}`)
			require.NoError(t, err)

		// v6 path to validate token and stack
		case "/cloud/v6/auth":
			authHeader := req.Header.Get("Authorization")
			stackHeader := req.Header.Get("X-Stack-Url")
			if authHeader == fmt.Sprintf("Bearer %s", validToken) && stackHeader == validStackURL {
				w.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprintf(w, `{"stack_id": %d, "default_project_id": %d}`, validStackID, defaultProjectID)
				require.NoError(t, err)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}
