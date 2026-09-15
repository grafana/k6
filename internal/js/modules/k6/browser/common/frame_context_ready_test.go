package common

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

func TestFrameWaitForExecutionContextReady(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		world executionWorld
		ready bool
		later bool
		want  time.Duration
	}{
		{name: "ready main", world: mainWorld, ready: true},
		{name: "ready utility", world: utilityWorld, ready: true},
		{name: "context arrives", world: mainWorld, later: true, want: 50 * time.Millisecond},
		{name: "missing context canceled", world: mainWorld, want: 75 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 75*time.Millisecond)
				defer cancel()
				f := &Frame{ctx: ctx, log: log.NewNullLogger(), executionContexts: make(map[executionWorld]frameExecutionContext)}
				if tc.ready {
					f.setContext(tc.world, &executionContextTestStub{})
				}
				if tc.later {
					go func() {
						time.Sleep(25 * time.Millisecond)
						f.setContext(tc.world, &executionContextTestStub{})
					}()
				}
				start := time.Now()
				f.waitForExecutionContext(tc.world)
				require.Equal(t, tc.want, time.Since(start))
				require.Equal(t, tc.ready || tc.later, f.hasContext(tc.world))
			})
		})
	}
}
