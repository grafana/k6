package sigv4

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTripper_request_includes_required_headers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if required headers are present
		authorization := r.Header.Get(authorizationHeaderKey)
		amzDate := r.Header.Get(amzDateKey)
		contentSHA256 := r.Header.Get(contentSHAKey)

		// Respond to the request
		w.WriteHeader(http.StatusOK)

		assert.NotEmptyf(t, authorization, "%s header should be present", authorizationHeaderKey)
		assert.NotEmptyf(t, amzDate, "%s header should be present", amzDateKey)
		assert.NotEmpty(t, contentSHA256, "%s header should be present", contentSHAKey)
	}))
	defer server.Close()

	client := http.Client{}
	tripper, err := NewRoundTripper(&Config{
		Region:             "us-east1",
		AwsSecretAccessKey: "xyz",
		AwsAccessKeyID:     "abc",
	}, http.DefaultTransport)
	if err != nil {
		t.Fatal(err)
	}
	client.Transport = tripper

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	response, _ := client.Do(req)
	_ = response.Body.Close()
}

func TestConfig_Validation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		shouldError bool
		arg         *Config
	}{
		{
			shouldError: false,
			arg: &Config{
				Region:             "us-east1",
				AwsAccessKeyID:     "someAccessKey",
				AwsSecretAccessKey: "someSecretKey",
			},
		},
		{
			shouldError: true,
			arg:         nil,
		},
		{
			shouldError: true,
			arg: &Config{
				Region: "us-east1",
			},
		},
		{
			shouldError: true,
			arg: &Config{
				Region:         "us-east1",
				AwsAccessKeyID: "someAccessKeyId",
			},
		},
		{
			shouldError: true,
			arg: &Config{
				AwsAccessKeyID:     "SomeAccessKey",
				AwsSecretAccessKey: "SomeSecretKey",
			},
		},
	}

	for _, tc := range testCases {
		got := tc.arg.validate()
		if tc.shouldError {
			assert.Error(t, got)
			continue
		}
		assert.NoError(t, got)
	}
}
