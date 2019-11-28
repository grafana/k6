package executor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
)

func getTestPerVUIterationsConfig() PerVUIterationsConfig {
	return PerVUIterationsConfig{
		VUs:         null.IntFrom(10),
		Iterations:  null.IntFrom(100),
		MaxDuration: types.NullDurationFrom(5 * time.Second),
	}
}

func TestPerVUIterations(t *testing.T) {
	t.Parallel()
	var result sync.Map
	var ctx, cancel, executor, _ = setupExecutor(
		t, getTestPerVUIterationsConfig(),
		func(ctx context.Context, out chan<- stats.SampleContainer) error {
			state := lib.GetState(ctx)
			currIter, _ := result.LoadOrStore(state.Vu, uint64(0))
			result.Store(state.Vu, currIter.(uint64)+1)
			return nil
		},
	)
	defer cancel()
	err := executor.Run(ctx, nil)
	require.NoError(t, err)

	var totalIters uint64
	result.Range(func(key, value interface{}) bool {
		vuIters := value.(uint64)
		assert.Equal(t, uint64(100), vuIters)
		totalIters += vuIters
		return true
	})
	assert.Equal(t, uint64(1000), totalIters)
}
