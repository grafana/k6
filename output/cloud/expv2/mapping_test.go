package expv2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimestampAsProto(t *testing.T) {
	t.Parallel()

	date := time.Unix(10, 0).UTC()
	timestamp := timestampAsProto(date.UnixNano())
	assert.Equal(t, date, timestamp.AsTime())

	// sub-second precision is not supported
	date = time.Unix(10, 282).UTC()
	timestamp = timestampAsProto(date.UnixNano())
	assert.Equal(t, time.Unix(10, 0).UTC(), timestamp.AsTime())
}
