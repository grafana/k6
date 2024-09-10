// Code created by gotmpl. DO NOT MODIFY.
// source: internal/shared/otlp/otlpmetric/oconf/optiontypes.go.tmpl

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oconf // import "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/oconf"

import "time"

const (
	// DefaultCollectorGRPCPort is the default gRPC port of the collector.
	DefaultCollectorGRPCPort uint16 = 4317
	// DefaultCollectorHTTPPort is the default HTTP port of the collector.
	DefaultCollectorHTTPPort uint16 = 4318
	// DefaultCollectorHost is the host address the Exporter will attempt
	// connect to if no collector address is provided.
	DefaultCollectorHost string = "localhost"
)

// Compression describes the compression used for payloads sent to the
// collector.
type Compression int

const (
	// NoCompression tells the driver to send payloads without
	// compression.
	NoCompression Compression = iota
	// GzipCompression tells the driver to send payloads after
	// compressing them with gzip.
	GzipCompression
)

// RetrySettings defines configuration for retrying batches in case of export failure
// using an exponential backoff.
type RetrySettings struct {
	// Enabled indicates whether to not retry sending batches in case of export failure.
	Enabled bool
	// InitialInterval the time to wait after the first failure before retrying.
	InitialInterval time.Duration
	// MaxInterval is the upper bound on backoff interval. Once this value is reached the delay between
	// consecutive retries will always be `MaxInterval`.
	MaxInterval time.Duration
	// MaxElapsedTime is the maximum amount of time (including retries) spent trying to send a request/batch.
	// Once this value is reached, the data is discarded.
	MaxElapsedTime time.Duration
}
