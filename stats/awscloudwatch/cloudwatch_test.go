package awscloudwatch

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCloudWatchClient(t *testing.T) {
	s := &sample{
		Tags: map[string]string{
			"proto":       "http",
			"subproto":    "http2",
			"status":      "200",
			"method":      "GET",
			"url":         "https://github.com/loadimpact/k6",
			"name":        "https://github.com/loadimpact/k6",
			"group":       "",
			"check":       "must be 200",
			"error":       "Some error",
			"error_code":  "abcd3",
			"tls_version": "1.2",
			"foo":         "bar",
		},
	}

	require.Equal(
		t, 10, len(toMetricDatum(s).Dimensions),
	)
}
