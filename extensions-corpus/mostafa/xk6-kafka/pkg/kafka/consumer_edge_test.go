package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsumerNilReceiverMethods(t *testing.T) {
	t.Parallel()
	var c *Consumer
	ctx := context.Background()

	_, err := c.Consume(ctx, 1)
	require.Error(t, err)

	err = c.Seek(0, 0)
	require.Error(t, err)

	_, err = c.Position(0)
	require.Error(t, err)

	err = c.CommitOffsets(ctx)
	require.Error(t, err)

	err = c.Close()
	require.NoError(t, err)

	assert.Equal(t, ConsumerStats{}, c.Stats())
}
