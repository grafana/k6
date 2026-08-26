package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errTestBrokerDisconnect     = errors.New("broker disconnect")
	errTestConnectionClosedPeer = errors.New("connection closed by peer")
)

func TestConsumerContextErrorWrapsDeadlineExceeded(t *testing.T) {
	err := consumerContextError(context.DeadlineExceeded)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.EqualError(t, err, "Consumer context cancelled., OriginalError: context deadline exceeded")
}

func TestConsumerReadErrorWrapsOriginalError(t *testing.T) {
	err := consumerReadError(errTestBrokerDisconnect)
	require.Error(t, err)
	assert.ErrorIs(t, err, errTestBrokerDisconnect)
	assert.EqualError(t, err, "Failed to consume message., OriginalError: broker disconnect")
}

func TestConsumerReadTimeoutNormalizationPrefersExpiredContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	time.Sleep(time.Millisecond)

	normalizedErr := normalizeConsumerReadError(ctx, errTestConnectionClosedPeer)
	require.Error(t, normalizedErr)
	assert.ErrorIs(t, normalizedErr, context.DeadlineExceeded)
}

func TestConsumerReadTimeoutNormalizationPreservesBrokerErrorWhileContextActive(t *testing.T) {
	normalizedErr := normalizeConsumerReadError(context.Background(), errTestConnectionClosedPeer)
	require.Error(t, normalizedErr)
	assert.ErrorIs(t, normalizedErr, errTestConnectionClosedPeer)
	assert.NotErrorIs(t, normalizedErr, context.DeadlineExceeded)
}

func TestConsumerCompatibilityTimeoutDetectsExpiredChildDeadline(t *testing.T) {
	parentCtx := context.Background()
	consumeCtx, cancel := context.WithTimeout(parentCtx, time.Nanosecond)
	defer cancel()

	time.Sleep(time.Millisecond)

	assert.True(t, isConsumerCompatibilityTimeout(
		parentCtx,
		consumeCtx,
		consumerContextError(context.Canceled),
	))
}

func TestConsumerCompatibilityTimeoutIgnoresExternalCancellation(t *testing.T) {
	parentCtx, cancelParent := context.WithCancel(context.Background())
	consumeCtx, cancelConsume := context.WithTimeout(parentCtx, time.Second)
	defer cancelConsume()

	cancelParent()

	assert.False(t, isConsumerCompatibilityTimeout(
		parentCtx,
		consumeCtx,
		consumerContextError(context.Canceled),
	))
}

func TestConsumerContextCauseUsesExpiredDeadlineWhenErrIsNil(t *testing.T) {
	ctx := deadlineOnlyContext{deadline: time.Now().Add(-time.Millisecond)}
	assert.ErrorIs(t, consumerContextCause(ctx), context.DeadlineExceeded)
}

func TestConsumerReadTimeoutNormalizationPrefersExpiredDeadlineWhenErrIsNil(t *testing.T) {
	ctx := deadlineOnlyContext{deadline: time.Now().Add(-time.Millisecond)}
	normalizedErr := normalizeConsumerReadError(ctx, errTestConnectionClosedPeer)
	require.Error(t, normalizedErr)
	assert.ErrorIs(t, normalizedErr, context.DeadlineExceeded)
}

type deadlineOnlyContext struct {
	deadline time.Time
}

func (c deadlineOnlyContext) Deadline() (time.Time, bool) {
	return c.deadline, true
}

func (deadlineOnlyContext) Done() <-chan struct{} {
	return nil
}

func (deadlineOnlyContext) Err() error {
	return nil
}

func (deadlineOnlyContext) Value(any) any {
	return nil
}
