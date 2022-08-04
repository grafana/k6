package executor

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

func getTestSharedIterationsConfig() SharedIterationsConfig {
	return SharedIterationsConfig{
		VUs:         null.IntFrom(10),
		Iterations:  null.IntFrom(100),
		MaxDuration: types.NullDurationFrom(5 * time.Second),
	}
}

// Baseline test
func TestSharedIterationsRun(t *testing.T) {
	t.Parallel()
	var doneIters uint64

	runner := simpleRunner(func(ctx context.Context, _ *lib.State) error {
		atomic.AddUint64(&doneIters, 1)
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, getTestSharedIterationsConfig())
	defer test.cancel()

	require.NoError(t, test.executor.Run(test.ctx, nil))
	assert.Equal(t, uint64(100), doneIters)
}

// Test that when one VU "slows down", others will pick up the workload.
// This is the reverse behavior of the PerVUIterations executor.
func TestSharedIterationsRunVariableVU(t *testing.T) {
	t.Parallel()
	var (
		result   sync.Map
		slowVUID uint64
	)

	runner := simpleRunner(func(ctx context.Context, state *lib.State) error {
		time.Sleep(10 * time.Millisecond) // small wait to stabilize the test
		// Pick one VU randomly and always slow it down.
		sid := atomic.LoadUint64(&slowVUID)
		if sid == uint64(0) {
			atomic.StoreUint64(&slowVUID, state.VUID)
		}
		if sid == state.VUID {
			time.Sleep(200 * time.Millisecond)
		}
		currIter, _ := result.LoadOrStore(state.VUID, uint64(0))
		result.Store(state.VUID, currIter.(uint64)+1) //nolint:forcetypeassert
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, getTestSharedIterationsConfig())
	defer test.cancel()

	require.NoError(t, test.executor.Run(test.ctx, nil))

	var totalIters uint64
	result.Range(func(key, value interface{}) bool {
		totalIters += value.(uint64)
		return true
	})

	// The slow VU should complete 2 iterations given these timings,
	// while the rest should randomly complete the other 98 iterations.
	val, ok := result.Load(slowVUID)
	assert.True(t, ok)
	assert.Equal(t, uint64(2), val)
	assert.Equal(t, uint64(100), totalIters)
}

func TestSharedIterationsEmitDroppedIterations(t *testing.T) {
	t.Parallel()
	var count int64

	runner := simpleRunner(func(ctx context.Context, _ *lib.State) error {
		atomic.AddInt64(&count, 1)
		<-ctx.Done()
		return nil
	})

	config := &SharedIterationsConfig{
		VUs:         null.IntFrom(5),
		Iterations:  null.IntFrom(100),
		MaxDuration: types.NullDurationFrom(1 * time.Second),
	}

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	engineOut := make(chan metrics.SampleContainer, 1000)
	require.NoError(t, test.executor.Run(test.ctx, engineOut))
	assert.Empty(t, test.logHook.Drain())
	assert.Equal(t, int64(5), count)
	assert.Equal(t, float64(95), sumMetricValues(engineOut, metrics.DroppedIterationsName))
}

func TestSharedIterationsGlobalIters(t *testing.T) {
	t.Parallel()

	config := &SharedIterationsConfig{
		VUs:         null.IntFrom(5),
		Iterations:  null.IntFrom(50),
		MaxDuration: types.NullDurationFrom(1 * time.Second),
	}

	testCases := []struct {
		seq, seg string
		expIters []uint64
	}{
		{"0,1/4,3/4,1", "0:1/4", []uint64{1, 6, 11, 16, 21, 26, 31, 36, 41, 46}},
		{"0,1/4,3/4,1", "1/4:3/4", []uint64{0, 2, 4, 5, 7, 9, 10, 12, 14, 15, 17, 19, 20, 22, 24, 25, 27, 29, 30, 32, 34, 35, 37, 39, 40, 42, 44, 45, 47, 49}},
		{"0,1/4,3/4,1", "3/4:1", []uint64{3, 8, 13, 18, 23, 28, 33, 38, 43, 48}},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%s_%s", tc.seq, tc.seg), func(t *testing.T) {
			t.Parallel()

			gotIters := []uint64{}
			var mx sync.Mutex
			runner := simpleRunner(func(ctx context.Context, state *lib.State) error {
				mx.Lock()
				gotIters = append(gotIters, state.GetScenarioGlobalVUIter())
				mx.Unlock()
				return nil
			})

			test := setupExecutorTest(t, tc.seg, tc.seq, lib.Options{}, runner, config)
			defer test.cancel()

			engineOut := make(chan metrics.SampleContainer, 100)
			require.NoError(t, test.executor.Run(test.ctx, engineOut))
			sort.Slice(gotIters, func(i, j int) bool { return gotIters[i] < gotIters[j] })
			assert.Equal(t, tc.expIters, gotIters)
		})
	}
}
