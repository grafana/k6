package cmd

import (
	"encoding/json"
	"testing"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib"
	"gopkg.in/guregu/null.v3"

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

func TestResolveAndSetProjectID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		cloudConfig       *cloudapi.Config
		expectedError     string
		expectedProjectID int64
		logContains       string
	}{
		{
			name: "sets projectID in all places when projectID > 0",
			cloudConfig: &cloudapi.Config{
				ProjectID: null.IntFrom(123),
			},
			expectedError:     "",
			expectedProjectID: 123,
			logContains:       "No stack specified",
		},
		{
			name: "logs warnings when projectID is 0 and no StackID",
			cloudConfig: &cloudapi.Config{
				ProjectID: null.IntFrom(0),
			},
			expectedError:     "",
			expectedProjectID: 0,
			logContains:       "No stack specified",
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

			tmpCloudConfig := map[string]interface{}{}
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
				var cloudData map[string]interface{}
				require.NoError(t, json.Unmarshal(arc.Options.Cloud, &cloudData))
				assert.Equal(t, float64(tc.expectedProjectID), cloudData["projectID"])

				// Verify arc.Options.External contains the projectID
				var externalData map[string]interface{}
				require.NoError(t, json.Unmarshal(arc.Options.External[cloudapi.LegacyCloudConfigKey], &externalData))
				assert.Equal(t, float64(tc.expectedProjectID), externalData["projectID"])
			}

			logs := ts.LoggerHook.Drain()
			if tc.logContains != "" {
				assert.True(t, testutils.LogContains(logs, logrus.WarnLevel, tc.logContains))
			} else {
				assert.Len(t, logs, 0)
			}
		})
	}
}
