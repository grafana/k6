package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewWorkerNilSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("nil interface", func(t *testing.T) {
		t.Parallel()
		_, err := NewWorker(ctx, nil, "tid", "http://example.com")
		require.ErrorContains(t, err, "nil session")
	})

	t.Run("typed nil pointer", func(t *testing.T) {
		t.Parallel()
		var sess *Session
		require.NotPanics(t, func() {
			_, err := NewWorker(ctx, sess, "tid", "http://example.com")
			require.ErrorContains(t, err, "nil session")
		})
	})
}
