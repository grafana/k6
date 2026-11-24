// Package clientcfg builds the k6cloud SDK client Configuration shared by
// k6's internal cloud API clients (internal/cloudapi/v6 and
// internal/cloudapi/provisioning). Both need the same User-Agent,
// single-server list, HTTP transport, and retry settings; only the server
// description differs between them, so it is passed in.
package clientcfg

import (
	"net/http"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
)

const (
	// RetryInterval is the default cloud request retry interval.
	RetryInterval = 500 * time.Millisecond
	// MaxRetries specifies max retry attempts.
	MaxRetries = 3
)

// New builds a k6cloud.Configuration for a single-server cloud API client.
// host is the server URL, version sets the k6cloud/<version> User-Agent,
// description labels the server entry, and timeout bounds each HTTP
// request. Retry settings use the shared MaxRetries/RetryInterval.
func New(host, version, description string, timeout time.Duration) *k6cloud.Configuration {
	cfg := k6cloud.NewConfiguration()
	cfg.UserAgent = "k6cloud/" + version
	cfg.Servers = k6cloud.ServerConfigurations{
		{
			URL:         host,
			Description: description,
		},
	}
	cfg.HTTPClient = &http.Client{
		Timeout:   timeout,
		Transport: http.DefaultTransport,
	}
	cfg.MaxRetries = MaxRetries
	cfg.RetryInterval = RetryInterval
	return cfg
}
