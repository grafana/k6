package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuiltinOutputString(t *testing.T) {
	t.Parallel()

	exp := []string{
		"cloud", "csv", "datadog", "experimental-prometheus-rw",
		"influxdb", "json", "kafka", "statsd",
		"experimental-opentelemetry", "opentelemetry",
		"summary",
	}
	values := builtinOutputValues()
	outputs := make([]string, 0, len(values))
	for _, value := range values {
		outputs = append(outputs, value.String())
	}

	assert.Equal(t, exp, outputs)
}

func TestBuiltinOutputStringIsCaseSensitive(t *testing.T) {
	t.Parallel()

	const mixedCaseOutput = "CsV"

	_, err := builtinOutputString(mixedCaseOutput)
	assert.Error(t, err)
}
