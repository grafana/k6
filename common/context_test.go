package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestContextWithDoneChan(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	ctx := contextWithDoneChan(context.Background(), done)
	close(done)
	select {
	case <-ctx.Done():
	case <-time.After(time.Millisecond * 100):
		require.FailNow(t, "should cancel the context after closing the done chan")
	}
}
