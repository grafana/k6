package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stretchr/testify/assert"
)

var errTestTimeout = errors.New("timeout")

func TestCompatibilityProducerMessageBytesIncludesTopic(t *testing.T) {
	msg := Message{
		Key:   []byte("key1"),
		Value: []byte("value1"),
	}

	assert.Equal(t, 33, compatibilityProducerMessageBytes(msg))
}

func TestCompatibilityConsumerFetchesAddsTrailingFetch(t *testing.T) {
	assert.Equal(t, 2.0, compatibilityConsumerFetches(1, 1, nil))
	assert.Equal(t, 2.0, compatibilityConsumerFetches(1, 1, errTestTimeout))
	assert.Equal(t, 1.0, compatibilityConsumerFetches(0, 1, errTestTimeout))
	assert.Equal(t, 0.0, compatibilityConsumerFetches(0, 1, nil))
}

func TestShouldRetryConsumerCompatibilityRead(t *testing.T) {
	parentCtx := context.Background()
	timeoutCtx, cancelTimeout := context.WithCancel(parentCtx)
	cancelTimeout()

	assert.True(t, shouldRetryConsumerCompatibilityRead(
		parentCtx,
		timeoutCtx,
		context.DeadlineExceeded,
		0,
		true,
	))

	activeParent, cancelParent := context.WithCancel(context.Background())
	canceledChild, cancelChild := context.WithCancel(activeParent)
	cancelChild()

	assert.True(t, shouldRetryConsumerCompatibilityRead(
		activeParent,
		canceledChild,
		context.Canceled,
		0,
		true,
	))

	cancelParent()

	assert.False(t, shouldRetryConsumerCompatibilityRead(
		activeParent,
		canceledChild,
		context.Canceled,
		0,
		true,
	))
	assert.False(t, shouldRetryConsumerCompatibilityRead(
		parentCtx,
		context.Background(),
		nil,
		0,
		true,
	))
	assert.False(t, shouldRetryConsumerCompatibilityRead(
		parentCtx,
		context.Background(),
		context.DeadlineExceeded,
		1,
		true,
	))
	assert.False(t, shouldRetryConsumerCompatibilityRead(
		parentCtx,
		context.Background(),
		context.DeadlineExceeded,
		0,
		false,
	))
}

func TestCompatibilityConfigFloat(t *testing.T) {
	t.Parallel()
	cfg := ckafka.ConfigMap{}
	assert.Equal(t, 0.0, compatibilityConfigFloat(cfg, "missing"))

	cfg["k"] = int(42)
	assert.Equal(t, 42.0, compatibilityConfigFloat(cfg, "k"))

	cfg["k"] = int32(3)
	assert.Equal(t, 3.0, compatibilityConfigFloat(cfg, "k"))

	cfg["k"] = int64(9)
	assert.Equal(t, 9.0, compatibilityConfigFloat(cfg, "k"))

	cfg["k"] = float32(1.25)
	assert.InDelta(t, 1.25, compatibilityConfigFloat(cfg, "k"), 0.001)

	cfg["k"] = float64(2.5)
	assert.Equal(t, 2.5, compatibilityConfigFloat(cfg, "k"))

	cfg["k"] = true
	assert.Equal(t, 1.0, compatibilityConfigFloat(cfg, "k"))

	cfg["k"] = false
	assert.Equal(t, 0.0, compatibilityConfigFloat(cfg, "k"))

	cfg["k"] = "3.14"
	assert.InDelta(t, 3.14, compatibilityConfigFloat(cfg, "k"), 0.001)

	cfg["k"] = "not-a-float"
	assert.Equal(t, 0.0, compatibilityConfigFloat(cfg, "k"))

	cfg["k"] = struct{}{}
	assert.Equal(t, 0.0, compatibilityConfigFloat(cfg, "k"))
}

func TestCompatibilityRequiredAcks(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0.0, compatibilityRequiredAcks(ckafka.ConfigMap{}))
	assert.Equal(t, -1.0, compatibilityRequiredAcks(ckafka.ConfigMap{"acks": "all"}))
	assert.Equal(t, 1.0, compatibilityRequiredAcks(ckafka.ConfigMap{"acks": "1"}))
	assert.Equal(t, 1.0, compatibilityRequiredAcks(ckafka.ConfigMap{"acks": int(1)}))
	assert.Equal(t, 0.0, compatibilityRequiredAcks(ckafka.ConfigMap{"acks": "not-int"}))
}

func TestCompatibilityConfigDuration(t *testing.T) {
	t.Parallel()
	cfg := ckafka.ConfigMap{}
	assert.Equal(t, time.Duration(0), compatibilityConfigDuration(cfg, "session.timeout.ms"))

	cfg["session.timeout.ms"] = int32(1500)
	assert.Equal(t, 1500*time.Millisecond, compatibilityConfigDuration(cfg, "session.timeout.ms"))

	cfg["session.timeout.ms"] = "bad"
	assert.Equal(t, time.Duration(0), compatibilityConfigDuration(cfg, "session.timeout.ms"))

	cfg["session.timeout.ms"] = int64(2000)
	assert.Equal(t, 2000*time.Millisecond, compatibilityConfigDuration(cfg, "session.timeout.ms"))

	cfg["session.timeout.ms"] = int(333)
	assert.Equal(t, 333*time.Millisecond, compatibilityConfigDuration(cfg, "session.timeout.ms"))

	cfg["session.timeout.ms"] = float64(444)
	assert.Equal(t, 444*time.Millisecond, compatibilityConfigDuration(cfg, "session.timeout.ms"))
}

func TestCompatibilityConfigString(t *testing.T) {
	t.Parallel()
	cfg := ckafka.ConfigMap{}
	assert.Equal(t, "", compatibilityConfigString(cfg, "missing"))

	cfg["k"] = "plain"
	assert.Equal(t, "plain", compatibilityConfigString(cfg, "k"))

	cfg["k"] = int(99)
	assert.Equal(t, "99", compatibilityConfigString(cfg, "k"))
}
