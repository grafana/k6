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
		Token:                           null.NewString("Token", true),
		ProjectID:                       null.NewInt(1, true),
		Name:                            null.NewString("Name", true),
		Host:                            null.NewString("Host", true),
		LogsTailURL:                     null.NewString("LogsTailURL", true),
		PushRefID:                       null.NewString("PushRefID", true),
		WebAppURL:                       null.NewString("foo", true),
		NoCompress:                      null.NewBool(true, true),
		StopOnError:                     null.NewBool(true, true),
		Timeout:                         types.NewNullDuration(5*time.Second, true),
		MaxMetricSamplesPerPackage:      null.NewInt(2, true),
		MetricPushInterval:              types.NewNullDuration(1*time.Second, true),
		MetricPushConcurrency:           null.NewInt(3, true),
		AggregationPeriod:               types.NewNullDuration(2*time.Second, true),
		AggregationCalcInterval:         types.NewNullDuration(3*time.Second, true),
		AggregationWaitPeriod:           types.NewNullDuration(4*time.Second, true),
		AggregationMinSamples:           null.NewInt(4, true),
		AggregationSkipOutlierDetection: null.NewBool(true, true),
		AggregationOutlierAlgoThreshold: null.NewInt(5, true),
		AggregationOutlierIqrRadius:     null.NewFloat(6, true),
		AggregationOutlierIqrCoefLower:  null.NewFloat(7, true),
		AggregationOutlierIqrCoefUpper:  null.NewFloat(8, true),
	}

	assert.Equal(t, full, full.Apply(empty))
	assert.Equal(t, full, full.Apply(defaults))
	assert.Equal(t, full, full.Apply(full))
	assert.Equal(t, full, empty.Apply(full))
	assert.Equal(t, full, defaults.Apply(full))
}

func TestGetConsolidatedConfig(t *testing.T) {
	t.Parallel()
	config, err := GetConsolidatedConfig(json.RawMessage(`{"token":"jsonraw"}`), nil, "", nil)
	require.NoError(t, err)
	require.Equal(t, config.Token.String, "jsonraw")

	config, err = GetConsolidatedConfig(json.RawMessage(`{"token":"jsonraw"}`), nil, "",
		map[string]json.RawMessage{"loadimpact": json.RawMessage(`{"token":"ext"}`)})
	require.NoError(t, err)
	require.Equal(t, config.Token.String, "ext")

	config, err = GetConsolidatedConfig(json.RawMessage(`{"token":"jsonraw"}`), map[string]string{"K6_CLOUD_TOKEN": "envvalue"}, "",
		map[string]json.RawMessage{"loadimpact": json.RawMessage(`{"token":"ext"}`)})
	require.NoError(t, err)
	require.Equal(t, config.Token.String, "envvalue")
}
