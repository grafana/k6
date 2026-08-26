package kafka

import (
	"os"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func TestRegisterMetricsReturnsConcreteErrorOnDuplicateRegistration(t *testing.T) {
	runtime := sobek.New()
	runtime.SetFieldNameMapper(common.FieldNameMapper{})

	registry := metrics.NewRegistry()
	_, err := registry.NewMetric("kafka_reader_dial_count", metrics.Gauge)
	require.NoError(t, err)

	mockVU := &modulestest.VU{
		RuntimeField: runtime,
		InitEnvField: &common.InitEnvironment{
			TestPreInitState: &lib.TestPreInitState{
				Registry: registry,
			},
		},
	}

	_, err = registerMetrics(mockVU)
	require.Error(t, err)
}

func TestMetricCompatibilityDocsCoverRegisteredMetrics(t *testing.T) {
	readme, err := os.ReadFile("../../README.md")
	require.NoError(t, err)

	migrationGuide, err := os.ReadFile("../../MIGRATION.md")
	require.NoError(t, err)

	combinedDocs := string(readme) + string(migrationGuide)
	for _, name := range registeredKafkaMetricNames() {
		assert.Contains(t, combinedDocs, name)
	}

	assert.Contains(t, string(migrationGuide), "## Metric Compatibility Appendix")
	assert.Contains(t, string(migrationGuide), "### Renamed Metrics")
	assert.Contains(t, string(migrationGuide), "### Removed Metrics")
}
