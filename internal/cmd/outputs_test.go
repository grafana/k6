package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuiltinOutputString(t *testing.T) {
	t.Parallel()
	exp := []string{
		"cloud", "csv", "datadog", "experimental-prometheus-rw",
		"influxdb", "json", "kafka", "statsd", "experimental-opentelemetry",
	}
	assert.Equal(t, exp, builtinOutputStrings())
}
