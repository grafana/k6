package cloudapi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/types"
)

func TestConfigApply(t *testing.T) {
	t.Parallel()
	empty := Config{}
	defaults := NewConfig()

	assert.Equal(t, empty, empty.Apply(empty))
	assert.Equal(t, empty, empty.Apply(defaults))
	assert.Equal(t, defaults, defaults.Apply(defaults))
	assert.Equal(t, defaults, defaults.Apply(empty))
	assert.Equal(t, defaults, defaults.Apply(empty).Apply(empty))

	full := Config{
		Token:                 null.NewString("Token", true),
		ProjectID:             null.NewInt(1, true),
		Name:                  null.NewString("Name", true),
		Host:                  null.NewString("Host", true),
		Timeout:               types.NewNullDuration(5*time.Second, true),
		LogsTailURL:           null.NewString("LogsTailURL", true),
		PushRefID:             null.NewString("PushRefID", true),
		WebAppURL:             null.NewString("foo", true),
		NoCompress:            null.NewBool(true, true),
		StopOnError:           null.NewBool(true, true),
		APIVersion:            null.NewInt(2, true),
		AggregationPeriod:     types.NewNullDuration(2*time.Second, true),
		AggregationWaitPeriod: types.NewNullDuration(4*time.Second, true),
		MaxTimeSeriesInBatch:  null.NewInt(3, true),
		MetricPushInterval:    types.NewNullDuration(1*time.Second, true),
		MetricPushConcurrency: null.NewInt(3, true),
		TracesEnabled:         null.NewBool(true, true),
		TracesHost:            null.NewString("TracesHost", true),
		TracesPushInterval:    types.NewNullDuration(10*time.Second, true),
		TracesPushConcurrency: null.NewInt(6, true),
	}

	assert.Equal(t, full, full.Apply(empty))
	assert.Equal(t, full, full.Apply(defaults))
	assert.Equal(t, full, full.Apply(full))
	assert.Equal(t, full, empty.Apply(full))
	assert.Equal(t, full, defaults.Apply(full))
}

func TestGetConsolidatedConfig(t *testing.T) {
	t.Parallel()
	config, warn, err := GetConsolidatedConfig(json.RawMessage(`{"token":"jsonraw"}`), nil, "", nil, nil)
	require.NoError(t, err)
	require.Equal(t, config.Token.String, "jsonraw")
	require.Empty(t, warn)

	config, warn, err = GetConsolidatedConfig(
		json.RawMessage(`{"token":"jsonraw"}`),
		nil,
		"",
		json.RawMessage(`{"token":"ext"}`),
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, config.Token.String, "ext")
	require.Empty(t, warn)

	config, warn, err = GetConsolidatedConfig(
		json.RawMessage(`{"token":"jsonraw"}`),
		map[string]string{"K6_CLOUD_TOKEN": "envvalue"},
		"",
		json.RawMessage(`{"token":"ext"}`),
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, config.Token.String, "envvalue")
	require.Empty(t, warn)
}

func TestGetConsolidatedConfig_WithLegacyOnly(t *testing.T) {
	t.Parallel()

	config, warn, err := GetConsolidatedConfig(json.RawMessage(`{"token":"jsonraw"}`), nil, "", nil,
		map[string]json.RawMessage{"loadimpact": json.RawMessage(`{"token":"ext"}`)})
	require.NoError(t, err)
	require.Equal(t, config.Token.String, "ext")
	require.NotEmpty(t, warn)

	config, warn, err = GetConsolidatedConfig(json.RawMessage(`{"token":"jsonraw"}`), map[string]string{"K6_CLOUD_TOKEN": "envvalue"}, "", nil,
		map[string]json.RawMessage{"loadimpact": json.RawMessage(`{"token":"ext"}`)})
	require.NoError(t, err)
	require.Equal(t, config.Token.String, "envvalue")
	require.NotEmpty(t, warn)
}

func TestGetConsolidatedConfig_LegacyHasLowerPriority(t *testing.T) {
	t.Parallel()

	config, warn, err := GetConsolidatedConfig(
		json.RawMessage(`{"token":"jsonraw"}`),
		nil,
		"",
		json.RawMessage(`{"token":"cloud"}`),
		map[string]json.RawMessage{"loadimpact": json.RawMessage(`{"token":"ext"}`)},
	)

	require.NoError(t, err)
	require.Equal(t, config.Token.String, "cloud")
	require.Empty(t, warn)
}

func TestGetConsolidatedConfig_EnvHasHigherPriority(t *testing.T) {
	t.Parallel()

	config, warn, err := GetConsolidatedConfig(
		json.RawMessage(`{"token":"jsonraw"}`),
		map[string]string{"K6_CLOUD_TOKEN": "envvalue"},
		"",
		json.RawMessage(`{"token":"cloud"}`),
		map[string]json.RawMessage{"loadimpact": json.RawMessage(`{"token":"ext"}`)},
	)
	require.NoError(t, err)

	require.Equal(t, config.Token.String, "envvalue")
	require.Empty(t, warn)
}
