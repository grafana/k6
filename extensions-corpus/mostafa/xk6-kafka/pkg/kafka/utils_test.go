package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFreezeNilObject(t *testing.T) {
	t.Parallel()
	err := freeze(nil)
	require.Error(t, err)
	var xk *Xk6KafkaError
	require.ErrorAs(t, err, &xk)
}

func TestBase64ToBytesInvalid(t *testing.T) {
	t.Parallel()
	_, err := base64ToBytes("%%%")
	require.Error(t, err)
}

func TestToJSONBytesAndIsJSON(t *testing.T) {
	b, xkErr := toJSONBytes(map[string]any{"a": 1})
	require.Nil(t, xkErr)
	assert.JSONEq(t, `{"a":1}`, string(b))

	_, xkErr = toJSONBytes("not-map")
	require.Equal(t, ErrInvalidDataType, xkErr)
	assert.False(t, isJSON([]byte(`not json`)))
	assert.False(t, isJSON([]byte(`"string"`)))
}
