package executor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
)

func getTestConstantArrivalRateConfig() ConstantArrivalRateConfig {
	return ConstantArrivalRateConfig{
		TimeUnit:        types.NullDurationFrom(time.Second),
		Rate:            null.IntFrom(50),
		Duration:        types.NullDurationFrom(5 * time.Second),
		PreAllocatedVUs: null.IntFrom(10),
		MaxVUs:          null.IntFrom(20),
	}
}

func TestConstantArrivalRateRunNotEnoughAllocatedVUsWarn(t *testing.T) {
	t.Parallel()
	var ctx, cancel, executor, logHook = setupExecutor(
		t, getTestConstantArrivalRateConfig(),
		func(ctx context.Context, out chan<- stats.SampleContainer) error {
			time.Sleep(time.Second)
			return nil
		},
		[]logrus.Level{logrus.WarnLevel},
	)
	defer cancel()
	var engineOut = make(chan stats.SampleContainer, 1000)
	err := executor.Run(ctx, engineOut)
	require.NoError(t, err)
	entries := logHook.Drain()
	require.NotEmpty(t, entries)
	for _, entry := range entries {
		require.Equal(t,
			"Insufficient VUs, reached 20 active VUs and cannot allocate more",
			entry.Message)
		require.Equal(t, logrus.WarnLevel, entry.Level)
	}
}

func TestConstantArrivalRateRunCorrectRate(t *testing.T) {
	t.Parallel()
	var count int64
	var ctx, cancel, executor, logHook = setupExecutor(
		t, getTestConstantArrivalRateConfig(),
		func(ctx context.Context, out chan<- stats.SampleContainer) error {
			atomic.AddInt64(&count, 1)
			return nil
		},
		[]logrus.Level{logrus.WarnLevel},
	)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// check that we got around the amount of VU iterations as we would expect
		var currentCount int64

		for i := 0; i < 5; i++ {
			time.Sleep(time.Second)
			currentCount = atomic.SwapInt64(&count, 0)
			require.InDelta(t, 50, currentCount, 1)
		}
	}()
	var engineOut = make(chan stats.SampleContainer, 1000)
	err := executor.Run(ctx, engineOut)
	wg.Wait()
	require.NoError(t, err)
	require.Empty(t, logHook.Drain())
}

func TestArrivalRateCancel(t *testing.T) {
	t.Parallel()
	var mat = map[string]func(
		*testing.T,
		func(context.Context, chan<- stats.SampleContainer) error,
	) (context.Context, context.CancelFunc, lib.Executor, *testutils.SimpleLogrusHook){
		"constant": func(t *testing.T, vuFn func(ctx context.Context, out chan<- stats.SampleContainer) error) (
			context.Context, context.CancelFunc, lib.Executor, *testutils.SimpleLogrusHook) {
			return setupExecutor(t, getTestConstantArrivalRateConfig(), vuFn, []logrus.Level{logrus.WarnLevel})
		},
		"variable": func(t *testing.T, vuFn func(ctx context.Context, out chan<- stats.SampleContainer) error) (
			context.Context, context.CancelFunc, lib.Executor, *testutils.SimpleLogrusHook) {
			return setupExecutor(t, getTestVariableArrivalRateConfig(), vuFn, []logrus.Level{logrus.WarnLevel})
		},
	}
	for name, fn := range mat {
		fn := fn
		t.Run(name, func(t *testing.T) {
			var ch = make(chan struct{})
			var errCh = make(chan error, 1)
			var weAreDoneCh = make(chan struct{})
			var ctx, cancel, executor, logHook = fn(
				t, func(ctx context.Context, out chan<- stats.SampleContainer) error {
					select {
					case <-ch:
						<-ch
					default:
					}
					return nil
				})
			defer cancel()
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()

				var engineOut = make(chan stats.SampleContainer, 1000)
				errCh <- executor.Run(ctx, engineOut)
				close(weAreDoneCh)
			}()

			time.Sleep(time.Second)
			ch <- struct{}{}
			cancel()
			time.Sleep(time.Second)
			select {
			case <-weAreDoneCh:
				t.Fatal("Run returned before all VU iterations were finished")
			default:
			}
			close(ch)
			<-weAreDoneCh
			wg.Wait()
			require.NoError(t, <-errCh)
			require.Empty(t, logHook.Drain())
		})
	}
}
