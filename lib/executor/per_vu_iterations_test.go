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
	"go.k6.io/k6/metrics"
)

func getTestPerVUIterationsConfig() PerVUIterationsConfig {
	return PerVUIterationsConfig{
		BaseConfig:  BaseConfig{GracefulStop: types.NullDurationFrom(1 * time.Second)},
		VUs:         null.IntFrom(10),
		Iterations:  null.IntFrom(100),
		MaxDuration: types.NullDurationFrom(3 * time.Second),
	}
}

// Baseline test
func TestPerVUIterationsRun(t *testing.T) {
	t.Parallel()
	var result sync.Map

	runner := simpleRunner(func(ctx context.Context, state *lib.State) error {
		currIter, _ := result.LoadOrStore(state.VUID, uint64(0))
		result.Store(state.VUID, currIter.(uint64)+1) //nolint:forcetypeassert
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, getTestPerVUIterationsConfig())
	defer test.cancel()

	engineOut := make(chan metrics.SampleContainer, 1000)
	require.NoError(t, test.executor.Run(test.ctx, engineOut))

	var totalIters uint64
	result.Range(func(key, value interface{}) bool {
		vuIters := value.(uint64)
		assert.Equal(t, uint64(100), vuIters)
		totalIters += vuIters
		return true
	})
	assert.Equal(t, uint64(1000), totalIters)
}

// Test that when one VU "slows down", others will *not* pick up the workload.
// This is the reverse behavior of the SharedIterations executor.
func TestPerVUIterationsRunVariableVU(t *testing.T) {
	t.Parallel()
	var (
		result   sync.Map
		slowVUID = uint64(1)
	)

	runner := simpleRunner(func(ctx context.Context, state *lib.State) error {
		if state.VUID == slowVUID {
			time.Sleep(200 * time.Millisecond)
		}
		currIter, _ := result.LoadOrStore(state.VUID, uint64(0))
		result.Store(state.VUID, currIter.(uint64)+1) //nolint:forcetypeassert
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, getTestPerVUIterationsConfig())
	defer test.cancel()

	engineOut := make(chan metrics.SampleContainer, 1000)
	require.NoError(t, test.executor.Run(test.ctx, engineOut))

	val, ok := result.Load(slowVUID)
	assert.True(t, ok)

	var totalIters uint64
	result.Range(func(key, value interface{}) bool {
		vuIters := value.(uint64)
		if key != slowVUID {
			assert.Equal(t, uint64(100), vuIters)
		}
		totalIters += vuIters
		return true
	})

	// The slow VU should complete 15 iterations given these timings,
	// while the rest should equally complete their assigned 100 iterations.
	assert.Equal(t, uint64(15), val)
	assert.Equal(t, uint64(915), totalIters)
}

func TestPerVuIterationsEmitDroppedIterations(t *testing.T) {
	t.Parallel()
	var count int64

	config := PerVUIterationsConfig{
		VUs:         null.IntFrom(5),
		Iterations:  null.IntFrom(20),
		MaxDuration: types.NullDurationFrom(1 * time.Second),
	}

	runner := simpleRunner(func(ctx context.Context, _ *lib.State) error {
		atomic.AddInt64(&count, 1)
		<-ctx.Done()
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	engineOut := make(chan metrics.SampleContainer, 1000)
	require.NoError(t, test.executor.Run(test.ctx, engineOut))
	assert.Empty(t, test.logHook.Drain())
	assert.Equal(t, int64(5), count)
	assert.Equal(t, float64(95), sumMetricValues(engineOut, metrics.DroppedIterationsName))
}
