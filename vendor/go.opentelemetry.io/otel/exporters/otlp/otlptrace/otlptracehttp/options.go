// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlptracehttp // import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"

import (
	"crypto/tls"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp/internal/otlpconfig"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp/internal/retry"
)

// Compression describes the compression used for payloads sent to the
// collector.
type Compression otlpconfig.Compression

const (
	// NoCompression tells the driver to send payloads without
	// compression.
	NoCompression = Compression(otlpconfig.NoCompression)
	// GzipCompression tells the driver to send payloads after
	// compressing them with gzip.
	GzipCompression = Compression(otlpconfig.GzipCompression)
)

// Option applies an option to the HTTP client.
type Option interface {
	applyHTTPOption(otlpconfig.Config) otlpconfig.Config
}

func asHTTPOptions(opts []Option) []otlpconfig.HTTPOption {
	converted := make([]otlpconfig.HTTPOption, len(opts))
	for i, o := range opts {
		converted[i] = otlpconfig.NewHTTPOption(o.applyHTTPOption)
	}
	return converted
}

// RetryConfig defines configuration for retrying batches in case of export
// failure using an exponential backoff.
type RetryConfig retry.Config

type wrappedOption struct {
	otlpconfig.HTTPOption
}

func (w wrappedOption) applyHTTPOption(cfg otlpconfig.Config) otlpconfig.Config {
	return w.ApplyHTTPOption(cfg)
}

// WithEndpoint sets the target endpoint the Exporter will connect to.
//
// If the OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
// environment variable is set, and this option is not passed, that variable
// value will be used. If both are set, OTEL_EXPORTER_OTLP_TRACES_ENDPOINT
// will take precedence.
//
// If both this option and WithEndpointURL are used, the last used option will
// take precedence.
//
// By default, if an environment variable is not set, and this option is not
// passed, "localhost:4317" will be used.
//
// This option has no effect if WithGRPCConn is used.
func WithEndpoint(endpoint string) Option {
	return wrappedOption{otlpconfig.WithEndpoint(endpoint)}
}

// WithEndpointURL sets the target endpoint URL the Exporter will connect to.
//
// If the OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
// environment variable is set, and this option is not passed, that variable
// value will be used. If both are set, OTEL_EXPORTER_OTLP_TRACES_ENDPOINT
// will take precedence.
//
// If both this option and WithEndpoint are used, the last used option will
// take precedence.
//
// If an invalid URL is provided, the default value will be kept.
//
// By default, if an environment variable is not set, and this option is not
// passed, "localhost:4317" will be used.
//
// This option has no effect if WithGRPCConn is used.
func WithEndpointURL(u string) Option {
	return wrappedOption{otlpconfig.WithEndpointURL(u)}
}

// WithCompression tells the driver to compress the sent data.
func WithCompression(compression Compression) Option {
	return wrappedOption{otlpconfig.WithCompression(otlpconfig.Compression(compression))}
}

// WithURLPath allows one to override the default URL path used
// for sending traces. If unset, default ("/v1/traces") will be used.
func WithURLPath(urlPath string) Option {
	return wrappedOption{otlpconfig.WithURLPath(urlPath)}
}

// WithTLSClientConfig can be used to set up a custom TLS
// configuration for the client used to send payloads to the
// collector. Use it if you want to use a custom certificate.
func WithTLSClientConfig(tlsCfg *tls.Config) Option {
	return wrappedOption{otlpconfig.WithTLSClientConfig(tlsCfg)}
}

// WithInsecure tells the driver to connect to the collector using the
// HTTP scheme, instead of HTTPS.
func WithInsecure() Option {
	return wrappedOption{otlpconfig.WithInsecure()}
}

// WithHeaders allows one to tell the driver to send additional HTTP
// headers with the payloads. Specifying headers like Content-Length,
// Content-Encoding and Content-Type may result in a broken driver.
func WithHeaders(headers map[string]string) Option {
	return wrappedOption{otlpconfig.WithHeaders(headers)}
}

// WithTimeout tells the driver the max waiting time for the backend to process
// each spans batch.  If unset, the default will be 10 seconds.
func WithTimeout(duration time.Duration) Option {
	return wrappedOption{otlpconfig.WithTimeout(duration)}
}

// WithRetry configures the retry policy for transient errors that may occurs
// when exporting traces. An exponential back-off algorithm is used to ensure
// endpoints are not overwhelmed with retries. If unset, the default retry
// policy will retry after 5 seconds and increase exponentially after each
// error for a total of 1 minute.
func WithRetry(rc RetryConfig) Option {
	return wrappedOption{otlpconfig.WithRetry(retry.Config(rc))}
}
