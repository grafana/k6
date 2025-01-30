// Package stale handles the staleness process.
//
// TODO: migrate here more logic dedicated to this topic
// from the remote write package.
package stale

import "math"

// Marker is the Prometheus Remote Write special value for marking
// a time series as stale.
//
// Check https://www.robustperception.io/staleness-and-promql and
// https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness
// for details about the Prometheus staleness markers.
//
// The value is the same used by the Prometheus package.
// https://pkg.go.dev/github.com/prometheus/prometheus/pkg/value#pkg-constants
//
// It isn't imported directly to avoid the direct dependency
// from the big Prometheus project that would bring more
// dependencies.
//
//nolint:gochecknoglobals
var Marker = math.Float64frombits(0x7ff0000000000002)
