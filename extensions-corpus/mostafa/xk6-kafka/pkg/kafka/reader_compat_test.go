package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errNotTimeout is a stable sentinel for timeout checks (err113).
var errNotTimeout = errors.New("not a timeout")

func TestConfluentConsumerMaxWait(t *testing.T) {
	t.Parallel()
	assert.Equal(t, defaultConfluentTimeout, confluentConsumerMaxWait(nil))

	c := &Consumer{config: ckafka.ConfigMap{}}
	assert.Equal(t, defaultConfluentTimeout, confluentConsumerMaxWait(c))

	c.config["fetch.wait.max.ms"] = 500
	assert.Equal(t, 500*time.Millisecond, confluentConsumerMaxWait(c))

	c.config["fetch.wait.max.ms"] = int32(300)
	assert.Equal(t, 300*time.Millisecond, confluentConsumerMaxWait(c))

	c.config["fetch.wait.max.ms"] = int64(200)
	assert.Equal(t, 200*time.Millisecond, confluentConsumerMaxWait(c))

	c.config["fetch.wait.max.ms"] = float64(150)
	assert.Equal(t, 150*time.Millisecond, confluentConsumerMaxWait(c))

	c.config["fetch.wait.max.ms"] = "not-a-number"
	assert.Equal(t, defaultConfluentTimeout, confluentConsumerMaxWait(c))
}

func TestConsumeConfigEffectiveLimit(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 1, (*ConsumeConfig)(nil).effectiveLimit())
	assert.Equal(t, 1, (&ConsumeConfig{}).effectiveLimit())
	assert.Equal(t, 10, (&ConsumeConfig{MaxMessages: 10}).effectiveLimit())
	assert.Equal(t, 7, (&ConsumeConfig{Limit: 7}).effectiveLimit())
	assert.Equal(t, 3, (&ConsumeConfig{MaxMessages: 3, Limit: 99}).effectiveLimit())
}

func TestMessagesToJS(t *testing.T) {
	t.Parallel()
	msgs := []Message{{
		Topic: "t", Partition: 1, Offset: 2, HighWaterMark: 9,
		Key: []byte("k"), Value: []byte("v"),
		Headers: map[string]any{"h": "x"},
		Time:    time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC),
	}}
	out := messagesToJS(msgs, false)
	require.Len(t, out, 1)
	assert.Contains(t, out[0]["time"], "2020-01-02")
	outNano := messagesToJS(msgs, true)
	require.Len(t, outNano, 1)
	assert.Contains(t, outNano[0]["time"], "2020-01-02T03:04:05")
}

func TestIsConsumerCompatibilityTimeout(t *testing.T) {
	t.Parallel()
	assert.False(t, isConsumerCompatibilityTimeout(context.Background(), context.Background(), nil))

	err := context.DeadlineExceeded
	assert.True(t, isConsumerCompatibilityTimeout(context.Background(), context.Background(), err))

	parent, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)
	consumeCtx := context.Background()
	assert.True(t, isConsumerCompatibilityTimeout(parent, consumeCtx, errNotTimeout))
}
