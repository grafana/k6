package cloudapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

func TestCreateCloudTest(t *testing.T) {
	t.Parallel()

	t.Run("successful test creation", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))

			formData := parseFormData(t, r)
			testName := formData["name"]

			assert.Contains(t, r.URL.Path, "789")

			w.Header().Add("Content-Type", "application/json")
			fprint(t, w, fmt.Sprintf(`{
				"id": 456,
				"name": "%s",
				"project_id": 789,
				"baseline_test_run_id": null,
				"created": "2024-01-01T00:00:00Z",
				"updated": "2024-01-01T00:00:00Z"
			}`, testName))
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		arc := createTestArchive(t)
		result, err := client.CreateCloudTest("test-name", 789, arc)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int32(456), result.Id)
		assert.Equal(t, "test-name", result.Name)
		assert.Equal(t, int32(789), result.ProjectId)
	})

	t.Run("empty name should fail test creation", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		arc := createTestArchive(t)
		_, err = client.CreateCloudTest("", 789, arc)
		require.Error(t, err)
	})

	t.Run("empty project ID should fail test creation", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		arc := createTestArchive(t)
		_, err = client.CreateCloudTest("test-name", 0, arc)
		require.Error(t, err)
	})

	t.Run("empty stackID should fail test creation", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)

		arc := createTestArchive(t)
		_, err = client.CreateCloudTest("test-name", 789, arc)
		require.Error(t, err)
	})
}

func TestUpdateCloudTest(t *testing.T) {
	t.Parallel()

	t.Run("successful test update", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))
			assert.Contains(t, r.URL.Path, "789")

			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		arc := createTestArchive(t)
		err = client.updateCloudTest(789, arc)
		require.NoError(t, err)
	})

	t.Run("empty testID should fail test update", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		arc := createTestArchive(t)
		err = client.updateCloudTest(0, arc)
		require.Error(t, err)
	})

	t.Run("empty stackID should fail test update", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)

		arc := createTestArchive(t)
		err = client.updateCloudTest(789, arc)
		require.Error(t, err)
	})
}

func TestGetCloudTestByName(t *testing.T) {
	t.Parallel()

	t.Run("test found", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))
			assert.Contains(t, r.URL.Path, "789")

			assert.Equal(t, "test-name", r.URL.Query().Get("name"))

			w.Header().Set("Content-Type", "application/json")
			fprint(t, w, `{
				"value": [
					{
						"id": 456,
						"name": "test-name",
						"project_id": 789,
						"baseline_test_run_id": null,
						"created": "2024-01-01T00:00:00Z",
						"updated": "2024-01-01T00:00:00Z"
					}
				]
			}`)
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		result, err := client.GetCloudTestByName("test-name", 789)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int32(456), result.Id)
		assert.Equal(t, "test-name", result.Name)
	})

	t.Run("empty name should fail test creation", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		_, err = client.GetCloudTestByName("", 789)
		require.Error(t, err)
	})

	t.Run("empty stackID should fail test creation", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)

		_, err = client.GetCloudTestByName("test-name", 789)
		require.Error(t, err)
	})
}

func TestCreateOrUpdateCloudTest(t *testing.T) {
	t.Parallel()

	t.Run("creates new test when it doesn't exist", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))

			formData := parseFormData(t, r)
			testName := formData["name"]

			assert.Contains(t, r.URL.Path, "789")

			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fprint(t, w, fmt.Sprintf(`{
				"id": 456,
				"name": "%s",
				"project_id": 789,
				"baseline_test_run_id": null,
				"created": "2024-01-01T00:00:00Z",
				"updated": "2024-01-01T00:00:00Z"
			}`, testName))
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		arc := createTestArchive(t)
		result, err := client.CreateOrUpdateCloudTest("test-name", 789, arc)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int32(456), result.Id)
		assert.Equal(t, "test-name", result.Name)
		assert.Equal(t, int32(789), result.ProjectId)
	})

	t.Run("updates test when it already exists", func(t *testing.T) {
		t.Parallel()
		getCalled := false
		updateCalled := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch r.Method {
			case http.MethodPost:
				assert.Contains(t, r.URL.Path, "789")
				w.WriteHeader(http.StatusConflict)
				fprint(t, w, `{
					"error": {
						"code": "error",
						"message": "There is already a LoadTest with the indicated name."
					}
				}`)
			case http.MethodGet:
				getCalled = true
				assert.Contains(t, r.URL.Path, "789")
				assert.Equal(t, "test-name", r.URL.Query().Get("name"))
				w.WriteHeader(http.StatusOK)
				fprint(t, w, `{
					"value": [
						{
							"id": 456,
							"name": "test-name",
							"project_id": 789,
							"baseline_test_run_id": null,
							"created": "2024-01-01T00:00:00Z",
							"updated": "2024-01-01T00:00:00Z"
						}
					]
				}`)
			case http.MethodPut:
				assert.Contains(t, r.URL.Path, "456")
				updateCalled = true
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		arc := createTestArchive(t)
		result, err := client.CreateOrUpdateCloudTest("test-name", 789, arc)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int32(456), result.Id)
		assert.Equal(t, "test-name", result.Name)
		assert.True(t, getCalled, "get endpoint should be called when test exists")
		assert.True(t, updateCalled, "update endpoint should be called when test exists")
	})

	t.Run("non-conflict errors are propagated", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fprint(t, w, `{
				"error": {
					"code": "error",
					"message": "Invalid request"
				}
			}`)
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		arc := createTestArchive(t)
		_, err = client.CreateOrUpdateCloudTest("test-name", 789, arc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid request")
	})
}

func TestStartCloudTestRun(t *testing.T) {
	t.Parallel()

	t.Run("successful test run start", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))
			assert.NotEmpty(t, r.Header.Get("K6-Idempotency-Key"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fprint(t, w, `{
				"id": 999,
				"test_id": 456,
				"project_id": 789,
				"started_by": "user@example.com",
				"created": "2024-06-01T19:00:00Z",
				"ended": null,
				"cost": null,
				"k6_dependencies": {
					"k6": ">=v0.52",
					"k6/x/faker": ">=0.4.0"
				},
				"k6_versions": {
					"k6": "v0.56.0",
					"k6/x/faker": "v0.4.1"
				},
				"note": "",
				"retention_expiry": "2024-06-07T19:00:00Z",
				"distribution": null,
				"options": null,
				"result": null,
				"result_details": null,
				"status": "created",
				"status_details": {
					"type": "created",
					"entered": "2024-06-01T19:00:00Z"
				},
				"status_history": [
				{
					"type": "created",
					"entered": "2024-06-01T19:00:00Z"
				}
				]
			}`)
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		result, err := client.StartCloudTestRun(456)
		require.NoError(t, err)
		assert.Equal(t, int32(999), result.Id)
		assert.Equal(t, int32(456), result.TestId)
	})
}

func TestStopCloudTestRun(t *testing.T) {
	t.Parallel()

	t.Run("successful test run stop", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "999")
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))

			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		err = client.StopCloudTestRun(999)
		require.NoError(t, err)
	})
}

func TestValidateOptions(t *testing.T) {
	t.Parallel()

	t.Run("successful options validation", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))

			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var validateOptions k6cloud.ValidateOptionsRequest
			err = json.Unmarshal(b, &validateOptions)
			require.NoError(t, err)

			duration := validateOptions.Options.AdditionalProperties["duration"]
			assert.Equal(t, "1m0s", duration)

			w.Header().Set("Content-Type", "application/json")
			fprint(t, w, `{}`)
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		opts := lib.Options{
			Duration: types.NullDurationFrom(60 * time.Second),
		}
		err = client.ValidateOptions(789, opts)
		require.NoError(t, err)
	})

	t.Run("validation error", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fprint(t, w, `{
				"error": {
					"code": "error",
					"message": "Invalid VUs number"
				}
			}`)
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)
		err = client.SetStackID(123)
		require.NoError(t, err)

		opts := lib.Options{
			VUs: null.IntFrom(-1), // Invalid option
		}
		err = client.ValidateOptions(789, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid VUs number")
	})
}

func TestValidateToken(t *testing.T) {
	t.Parallel()

	t.Run("successful token validation", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the authorization header
			authHeader := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer test-token", authHeader)

			// Verify the stack URL
			stackURL := r.Header.Get("X-Stack-Url")
			assert.Equal(t, stackURL, "https://stack.grafana.net")

			w.Header().Add("Content-Type", "application/json")
			fprint(t, w, `{
				"stack_id": 123,
				"default_project_id": 456
			}`)
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken("https://stack.grafana.net")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, int32(123), resp.StackId)
		assert.Equal(t, int32(456), resp.DefaultProjectId)
	})

	t.Run("unauthorized token should fail", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fprint(t, w, `{
				"error": {
					"code": "error",
					"message": "Invalid token"
				}
			}`)
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "invalid-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken("https://stack.grafana.net")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "(401/error) Invalid token")
	})

	t.Run("network error should fail", func(t *testing.T) {
		t.Parallel()
		// Use an invalid URL to simulate network error
		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://invalid-url-that-does-not-exist", "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken("https://stack.grafana.net")
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("missing stack URL should fail", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken("")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, "stack URL is required to validate token", err.Error())
	})

	t.Run("invalid stack URL should fail", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken("://invalid-url")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "invalid stack URL")
	})
}

func fprint(t *testing.T, w io.Writer, format string) int {
	n, err := fmt.Fprint(w, format)
	require.NoError(t, err)
	return n
}

// createTestArchive creates a minimal valid archive for testing
func createTestArchive(t *testing.T) *lib.Archive {
	t.Helper()
	fs := fsext.NewMemMapFs()
	err := fsext.WriteFile(fs, "/path/to/a.js", []byte(`// a contents`), 0o644)
	require.NoError(t, err)

	return &lib.Archive{
		Type:        "js",
		K6Version:   build.Version,
		Options:     lib.Options{},
		FilenameURL: &url.URL{Scheme: "file", Path: "/path/to/a.js"},
		Data:        []byte(`// a contents`),
		PwdURL:      &url.URL{Scheme: "file", Path: "/path/to"},
		Filesystems: map[string]fsext.Fs{
			"file": fs,
		},
	}
}

func parseFormData(t *testing.T, r *http.Request) map[string]string {
	t.Helper()

	// Parse the multipart form data
	reader, err := r.MultipartReader()
	require.NoError(t, err)

	// Initialize a map to store the parsed form data
	formData := make(map[string]string)

	// Iterate through the parts
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		require.NoError(t, nextErr)

		// Read the part content
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, part)
		require.NoError(t, err)

		// Store the part content in the map
		formData[part.FormName()] = buf.String()
	}

	return formData
}
