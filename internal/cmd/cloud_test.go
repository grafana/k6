package cmd

import (
	"encoding/json"
	"testing"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/internal/cmd/tests"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDefaultProjectID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		cloudConfig       *cloudapi.Config
		expectedProjectID int64
		expectedError     string
		logContains       string
	}{
		{
			name: "returns ProjectID when valid and greater than 0",
			cloudConfig: &cloudapi.Config{
				ProjectID: null.IntFrom(123),
			},
			expectedProjectID: 123,
			expectedError:     "",
		},
		{
			name: "returns DefaultProjectID when ProjectID is 0 but StackID and DefaultProjectID are valid",
			cloudConfig: &cloudapi.Config{
				ProjectID:        null.IntFrom(0),
				StackID:          null.IntFrom(456),
				DefaultProjectID: null.IntFrom(789),
				StackURL:         null.StringFrom("test-stack"),
			},
			expectedProjectID: 789,
			expectedError:     "",
			logContains:       "test-stack",
		},
		{
			name: "uses generated stack name when StackURL is not valid",
			cloudConfig: &cloudapi.Config{
				ProjectID:        null.IntFrom(0),
				StackID:          null.IntFrom(456),
				DefaultProjectID: null.IntFrom(789),
				StackURL:         null.String{},
			},
			expectedProjectID: 789,
			expectedError:     "",
			logContains:       "stack-456",
		},
		{
			name: "returns error when StackID is valid but DefaultProjectID is not available",
			cloudConfig: &cloudapi.Config{
				ProjectID:        null.IntFrom(0),
				StackID:          null.IntFrom(456),
				DefaultProjectID: null.Int{},
			},
			expectedProjectID: 0,
			expectedError:     "default stack configured but the default project ID is not available",
		},
		{
			name: "returns 0 when nothing is configured",
			cloudConfig: &cloudapi.Config{
				ProjectID: null.IntFrom(0),
			},
			expectedProjectID: 0,
			expectedError:     "",
		},
		{
			name: "returns 0 when StackID is 0",
			cloudConfig: &cloudapi.Config{
				ProjectID:        null.IntFrom(0),
				StackID:          null.IntFrom(0),
				DefaultProjectID: null.IntFrom(789),
			},
			expectedProjectID: 0,
			expectedError:     "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts := tests.NewGlobalTestState(t)

			projectID, err := resolveDefaultProjectID(ts.GlobalState, tc.cloudConfig)

			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expectedProjectID, projectID)

			// Check log messages if specified
			if tc.logContains != "" {
				logs := ts.LoggerHook.Drain()
				assert.True(t, testutils.LogContains(logs, logrus.WarnLevel, tc.logContains))
			}
		})
	}
}

func TestCheckCloudLogin(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		conf                cloudapi.Config
		expectedAuthError   bool
		expectedTokenReason bool
		expectedStackReason bool
	}{
		{
			name: "valid token and stack passes",
			conf: cloudapi.Config{
				Token:   null.StringFrom("valid-token"),
				StackID: null.IntFrom(1234),
			},
		},
		{
			name: "missing token returns token not configured error",
			conf: cloudapi.Config{
				StackID: null.IntFrom(1234),
			},
			expectedAuthError:   true,
			expectedTokenReason: true,
		},
		{
			name: "empty token string returns token not configured error",
			conf: cloudapi.Config{
				Token:   null.StringFrom(""),
				StackID: null.IntFrom(1234),
			},
			expectedAuthError:   true,
			expectedTokenReason: true,
		},
		{
			name: "missing stack returns stack not configured error",
			conf: cloudapi.Config{
				Token: null.StringFrom("valid-token"),
			},
			expectedAuthError:   true,
			expectedStackReason: true,
		},
		{
			name: "zero stack ID returns stack not configured error",
			conf: cloudapi.Config{
				Token:   null.StringFrom("valid-token"),
				StackID: null.IntFrom(0),
			},
			expectedAuthError:   true,
			expectedStackReason: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := checkCloudLogin(tc.conf)

			if tc.expectedAuthError {
				var authErr *cloudAuthError
				require.ErrorAs(t, err, &authErr)
				switch {
				case tc.expectedTokenReason:
					require.ErrorContains(t, err, "an access token")
				case tc.expectedStackReason:
					require.ErrorContains(t, err, "a stack ID")
				default:
					t.Fatal("if an auth error is expected then an expected reason must be defined")
				}
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestResolveAndSetProjectID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		cloudConfig       *cloudapi.Config
		expectedError     string
		expectedProjectID int64
	}{
		{
			name: "sets projectID in all places when projectID > 0",
			cloudConfig: &cloudapi.Config{
				ProjectID: null.IntFrom(123),
			},
			expectedError:     "",
			expectedProjectID: 123,
		},
		{
			name: "returns 0 when projectID is 0 and no StackID",
			cloudConfig: &cloudapi.Config{
				ProjectID: null.IntFrom(0),
			},
			expectedError:     "",
			expectedProjectID: 0,
		},
		{
			name: "propagates error from resolveDefaultProjectID",
			cloudConfig: &cloudapi.Config{
				ProjectID:        null.IntFrom(0),
				StackID:          null.IntFrom(456),
				DefaultProjectID: null.Int{}, // Invalid - should cause error
			},
			expectedError:     "default stack configured but the default project ID is not available",
			expectedProjectID: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts := tests.NewGlobalTestState(t)

			tmpCloudConfig := map[string]any{}
			arc := &lib.Archive{
				Options: lib.Options{
					External: make(map[string]json.RawMessage),
				},
			}

			err := resolveAndSetProjectID(ts.GlobalState, tc.cloudConfig, tmpCloudConfig, arc)

			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
			}

			if tc.expectedProjectID > 0 {
				assert.Equal(t, tc.expectedProjectID, tmpCloudConfig["projectID"])
				assert.Equal(t, tc.expectedProjectID, tc.cloudConfig.ProjectID.Int64)

				// Verify arc.Options.Cloud contains the projectID
				var cloudData map[string]any
				require.NoError(t, json.Unmarshal(arc.Options.Cloud, &cloudData))
				assert.Equal(t, float64(tc.expectedProjectID), cloudData["projectID"])
			}

			logs := ts.LoggerHook.Drain()
			assert.Len(t, logs, 0)
		})
	}
}
