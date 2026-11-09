package provisioning

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	token := "test-token"
	host := "https://api.k6.io"
	version := "0.1.0"
	stackID := int64(123)
	timeout := 30 * time.Second

	c, err := NewClient(logger, token, host, version, stackID, timeout)
	require.NoError(t, err)

	assert.Equal(t, logger, c.logger)
	assert.Equal(t, token, c.token)
	assert.Equal(t, stackID, c.stackID)

	// The underlying k6cloud.APIClient should be configured correctly.
	require.NotNil(t, c.apiClient)
	cfg := c.apiClient.GetConfig()
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, host, cfg.Servers[0].URL)
	assert.Equal(t, timeout, cfg.HTTPClient.Timeout)

	// The internal v6 client should be constructed with the same auth params.
	assert.NotNil(t, c.v6Client)
}
