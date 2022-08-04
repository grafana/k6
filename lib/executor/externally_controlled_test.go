package executor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
)

func getTestExternallyControlledConfig() ExternallyControlledConfig {
	return ExternallyControlledConfig{
		ExternallyControlledConfigParams: ExternallyControlledConfigParams{
			VUs:      null.IntFrom(2),
			MaxVUs:   null.IntFrom(10),
			Duration: types.NullDurationFrom(2 * time.Second),
		},
	}
}

func TestExternallyControlledRun(t *testing.T) {
	t.Parallel()

	doneIters := new(uint64)
	runner := simpleRunner(func(ctx context.Context, _ *lib.State) error {
		time.Sleep(200 * time.Millisecond)
		atomic.AddUint64(doneIters, 1)
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, getTestExternallyControlledConfig())
	defer test.cancel()

	var (
		wg     sync.WaitGroup
		errCh  = make(chan error, 1)
		doneCh = make(chan struct{})
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		test.state.MarkStarted()
		errCh <- test.executor.Run(test.ctx, nil)
		test.state.MarkEnded()
		close(doneCh)
	}()

	updateConfig := func(vus, maxVUs int64, errMsg string) {
		newConfig := ExternallyControlledConfigParams{
			VUs:      null.IntFrom(vus),
			MaxVUs:   null.IntFrom(maxVUs),
			Duration: types.NullDurationFrom(2 * time.Second),
		}
		err := test.executor.(*ExternallyControlled).UpdateConfig(test.ctx, newConfig) //nolint:forcetypeassert
		if errMsg != "" {
			assert.EqualError(t, err, errMsg)
		} else {
			assert.NoError(t, err)
		}
	}

	var resultVUCount [][]int64
	snapshot := func() {
		resultVUCount = append(resultVUCount,
			[]int64{test.state.GetCurrentlyActiveVUsCount(), test.state.GetInitializedVUsCount()})
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		snapshotTicker := time.NewTicker(500 * time.Millisecond)
		ticks := 0
		for {
			select {
			case <-snapshotTicker.C:
				snapshot()
				switch ticks {
				case 0, 2:
					updateConfig(4, 10, "")
				case 1:
					updateConfig(8, 20, "")
				case 3:
					updateConfig(15, 10,
						"invalid configuration supplied: the number of active VUs (15)"+
							" must be less than or equal to the number of maxVUs (10)")
					updateConfig(-1, 10,
						"invalid configuration supplied: the number of VUs can't be negative")
				}
				ticks++
			case <-doneCh:
				snapshotTicker.Stop()
				snapshot()
				return
			}
		}
	}()

	wg.Wait()
	require.NoError(t, <-errCh)
	assert.InDelta(t, 48, int(atomic.LoadUint64(doneIters)), 2)
	assert.Equal(t, [][]int64{{2, 10}, {4, 10}, {8, 20}, {4, 10}, {0, 10}}, resultVUCount)
}
