package grpc_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/metrics"
)

func assertResponse(t *testing.T, cb codeBlock, err error, val sobek.Value, ts testState) {
	if isWindows && cb.windowsErr != "" && err != nil {
		err = errors.New(strings.ReplaceAll(err.Error(), cb.windowsErr, cb.err))
	}
	if cb.err == "" {
		assert.NoError(t, err)
	} else {
		require.Error(t, err)
		assert.Contains(t, err.Error(), cb.err)
	}
	if cb.val != nil {
		require.NotNil(t, val)
		assert.Equal(t, cb.val, val.Export())
	}
	if cb.asserts != nil {
		cb.asserts(t, ts.httpBin, ts.samples, err)
	}
}

func assertMetricEmitted(
	t *testing.T,
	metricName string, //nolint:unparam
	sampleContainers []metrics.SampleContainer,
	url string,
) {
	seenMetric := false

	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			surl, ok := sample.Tags.Get("url")
			assert.True(t, ok)
			if surl == url {
				if sample.Metric.Name == metricName {
					seenMetric = true
				}
			}
		}
	}
	assert.True(t, seenMetric, "url %s didn't emit %s", url, metricName)
}
