package kafka

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDurationMarshalUnmarshalJSON(t *testing.T) {
	t.Parallel()
	d := Duration{Duration: time.Hour + 2*time.Minute}
	b, err := json.Marshal(d)
	require.NoError(t, err)

	var out Duration
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, d.Duration, out.Duration)
}

func TestDurationUnmarshalJSONErrors(t *testing.T) {
	t.Parallel()
	var d Duration
	err := d.UnmarshalJSON([]byte(`not valid json`))
	require.Error(t, err)

	err = d.UnmarshalJSON([]byte(`"not-a-valid-duration"`))
	require.Error(t, err)

	err = d.UnmarshalJSON([]byte(`42`))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidDuration)
}
