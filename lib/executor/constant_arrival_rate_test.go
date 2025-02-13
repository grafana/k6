package executor

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

func newExecutionSegmentFromString(str string) *lib.ExecutionSegment {
	r, err := lib.NewExecutionSegmentFromString(str)
	if err != nil {
		panic(err)
	}
	return r
}

func newExecutionSegmentSequenceFromString(str string) *lib.ExecutionSegmentSequence {
	r, err := lib.NewExecutionSegmentSequenceFromString(str)
	if err != nil {
		panic(err)
	}
	return &r
}

func getTestConstantArrivalRateConfig() *ConstantArrivalRateConfig {
	return &ConstantArrivalRateConfig{
		BaseConfig:      BaseConfig{GracefulStop: types.NullDurationFrom(1 * time.Second)},
		TimeUnit:        types.NullDurationFrom(time.Second),
		Rate:            null.IntFrom(50),
		Duration:        types.NullDurationFrom(5 * time.Second),
		PreAllocatedVUs: null.IntFrom(10),
		MaxVUs:          null.IntFrom(20),
	}
}

func TestConstantArrivalRateRunNotEnoughAllocatedVUsWarn(t *testing.T) {
	t.Parallel()

	runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
		time.Sleep(time.Second)
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, getTestConstantArrivalRateConfig())
	defer test.cancel()

	engineOut := make(chan metrics.SampleContainer, 1000)
	require.NoError(t, test.executor.Run(test.ctx, engineOut))
	entries := test.logHook.Drain()
	require.NotEmpty(t, entries)
	for _, entry := range entries {
		require.Equal(t,
			"Insufficient VUs, reached 20 active VUs and cannot initialize more",
			entry.Message)
		require.Equal(t, logrus.WarnLevel, entry.Level)
	}
}

func TestConstantArrivalRateRunCorrectRate(t *testing.T) {
	t.Parallel()

	var count int64
	runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
		atomic.AddInt64(&count, 1)
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, getTestConstantArrivalRateConfig())
	defer test.cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// check that we got around the amount of VU iterations as we would expect
		var totalCount int64

		i := 5
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			i--
			if i == 0 {
				break
			}
			currentCount := atomic.SwapInt64(&count, 0)
			totalCount += currentCount
			// We have a relatively relaxed constraint here, but we also check
			// the final iteration count exactly below:
			assert.InDelta(t, 50, currentCount, 5)
		}

		time.Sleep(200 * time.Millisecond) // just in case

		assert.InDelta(t, 250, totalCount+atomic.LoadInt64(&count), 2)
	}()
	engineOut := make(chan metrics.SampleContainer, 1000)
	require.NoError(t, test.executor.Run(test.ctx, engineOut))
	wg.Wait()
	require.Empty(t, test.logHook.Drain())
}

//nolint:paralleltest // this is flaky if ran with other tests
func TestConstantArrivalRateRunCorrectTiming(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("this test is very flaky on the Windows GitHub Action runners...")
	}
	tests := []struct {
		segment  string
		sequence string
		start    time.Duration
		steps    []int64
	}{
		{
			segment: "0:1/3",
			start:   time.Millisecond * 20,
			steps:   []int64{40, 60, 60, 60, 60, 60, 60},
		},
		{
			segment: "1/3:2/3",
			start:   time.Millisecond * 20,
			steps:   []int64{60, 60, 60, 60, 60, 60, 40},
		},
		{
			segment: "2/3:1",
			start:   time.Millisecond * 20,
			steps:   []int64{40, 60, 60, 60, 60, 60, 60},
		},
		{
			segment: "1/6:3/6",
			start:   time.Millisecond * 20,
			steps:   []int64{40, 80, 40, 80, 40, 80, 40},
		},
		{
			segment:  "1/6:3/6",
			sequence: "1/6,3/6",
			start:    time.Millisecond * 20,
			steps:    []int64{40, 80, 40, 80, 40, 80, 40},
		},
		// sequences
		{
			segment:  "0:1/3",
			sequence: "0,1/3,2/3,1",
			start:    time.Millisecond * 0,
			steps:    []int64{60, 60, 60, 60, 60, 60, 40},
		},
		{
			segment:  "1/3:2/3",
			sequence: "0,1/3,2/3,1",
			start:    time.Millisecond * 20,
			steps:    []int64{60, 60, 60, 60, 60, 60, 40},
		},
		{
			segment:  "2/3:1",
			sequence: "0,1/3,2/3,1",
			start:    time.Millisecond * 40,
			steps:    []int64{60, 60, 60, 60, 60, 100},
		},
	}
	for _, test := range tests {
		test := test

		t.Run(fmt.Sprintf("segment %s sequence %s", test.segment, test.sequence), func(t *testing.T) {
			var count int64
			startTime := time.Now()
			expectedTimeInt64 := int64(test.start)
			runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
				current := atomic.AddInt64(&count, 1)

				expectedTime := test.start
				if current != 1 {
					expectedTime = time.Duration(atomic.AddInt64(&expectedTimeInt64,
						int64(time.Millisecond)*test.steps[(current-2)%int64(len(test.steps))]))
				}

				// FIXME: replace this check with a unit test asserting that the scheduling is correct,
				// without depending on the execution time itself
				assert.WithinDuration(t,
					startTime.Add(expectedTime),
					time.Now(),
					time.Millisecond*24,
					"%d expectedTime %s", current, expectedTime,
				)

				return nil
			})

			config := getTestConstantArrivalRateConfig()
			seconds := 2
			config.Duration.Duration = types.Duration(time.Second * time.Duration(seconds))
			execTest := setupExecutorTest(
				t, test.segment, test.sequence, lib.Options{}, runner, config,
			)
			defer execTest.cancel()

			newET, err := execTest.state.ExecutionTuple.GetNewExecutionTupleFromValue(config.MaxVUs.Int64)
			require.NoError(t, err)
			rateScaled := newET.ScaleInt64(config.Rate.Int64)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				// check that we got around the amount of VU iterations as we would expect
				var currentCount int64

				for i := 0; i < seconds; i++ {
					time.Sleep(time.Second)
					currentCount = atomic.LoadInt64(&count)
					assert.InDelta(t, int64(i+1)*rateScaled, currentCount, 3)
				}
			}()
			startTime = time.Now()
			engineOut := make(chan metrics.SampleContainer, 1000)
			err = execTest.executor.Run(execTest.ctx, engineOut)
			wg.Wait()
			require.NoError(t, err)
			require.Empty(t, execTest.logHook.Drain())
		})
	}
}

func TestArrivalRateCancel(t *testing.T) {
	t.Parallel()

	testCases := map[string]lib.ExecutorConfig{
		"constant": getTestConstantArrivalRateConfig(),
		"ramping":  getTestRampingArrivalRateConfig(),
	}
	for name, config := range testCases {
		config := config
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := make(chan struct{})
			errCh := make(chan error, 1)
			weAreDoneCh := make(chan struct{})

			runner := simpleRunner(func(_ context.Context, _ *lib.State) error {
				select {
				case <-ch:
					<-ch
				default:
				}
				return nil
			})
			test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
			defer test.cancel()

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()

				engineOut := make(chan metrics.SampleContainer, 1000)
				errCh <- test.executor.Run(test.ctx, engineOut)
				close(weAreDoneCh)
			}()

			time.Sleep(time.Second)
			ch <- struct{}{}
			test.cancel()
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
			require.Empty(t, test.logHook.Drain())
		})
	}
}

func TestConstantArrivalRateDroppedIterations(t *testing.T) {
	t.Parallel()
	var count int64

	config := &ConstantArrivalRateConfig{
		BaseConfig:      BaseConfig{GracefulStop: types.NullDurationFrom(0 * time.Second)},
		TimeUnit:        types.NullDurationFrom(time.Second),
		Rate:            null.IntFrom(10),
		Duration:        types.NullDurationFrom(950 * time.Millisecond),
		PreAllocatedVUs: null.IntFrom(5),
		MaxVUs:          null.IntFrom(5),
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
	logs := test.logHook.Drain()
	require.Len(t, logs, 1)
	assert.Contains(t, logs[0].Message, "cannot initialize more")
	assert.Equal(t, int64(5), count)
	assert.Equal(t, float64(5), sumMetricValues(engineOut, metrics.DroppedIterationsName))
}

func TestConstantArrivalRateGlobalIters(t *testing.T) {
	t.Parallel()

	config := &ConstantArrivalRateConfig{
		BaseConfig:      BaseConfig{GracefulStop: types.NullDurationFrom(100 * time.Millisecond)},
		TimeUnit:        types.NullDurationFrom(950 * time.Millisecond),
		Rate:            null.IntFrom(20),
		Duration:        types.NullDurationFrom(1 * time.Second),
		PreAllocatedVUs: null.IntFrom(5),
		MaxVUs:          null.IntFrom(5),
	}

	testCases := []struct {
		seq, seg string
		expIters []uint64
	}{
		{"0,1/4,3/4,1", "0:1/4", []uint64{1, 6, 11, 16, 21}},
		{"0,1/4,3/4,1", "1/4:3/4", []uint64{0, 2, 4, 5, 7, 9, 10, 12, 14, 15, 17, 19, 20}},
		{"0,1/4,3/4,1", "3/4:1", []uint64{3, 8, 13, 18}},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%s_%s", tc.seq, tc.seg), func(t *testing.T) {
			t.Parallel()

			gotIters := []uint64{}
			var mx sync.Mutex
			runner := simpleRunner(func(_ context.Context, state *lib.State) error {
				mx.Lock()
				gotIters = append(gotIters, state.GetScenarioGlobalVUIter())
				mx.Unlock()
				return nil
			})
			test := setupExecutorTest(t, tc.seg, tc.seq, lib.Options{}, runner, config)
			defer test.cancel()

			engineOut := make(chan metrics.SampleContainer, 100)
			require.NoError(t, test.executor.Run(test.ctx, engineOut))
			assert.Equal(t, tc.expIters, gotIters)
		})
	}
}

func TestConstantArrivalRateActiveVUs(t *testing.T) {
	t.Parallel()

	config := &ConstantArrivalRateConfig{
		BaseConfig:      BaseConfig{GracefulStop: types.NullDurationFrom(0 * time.Second)},
		TimeUnit:        types.NullDurationFrom(time.Second),
		Rate:            null.IntFrom(10),
		Duration:        types.NullDurationFrom(950 * time.Millisecond),
		PreAllocatedVUs: null.IntFrom(5),
		MaxVUs:          null.IntFrom(10),
	}

	var (
		running          int64
		getCurrActiveVUs func() int64
		runMx            sync.Mutex
	)

	runner := simpleRunner(func(ctx context.Context, _ *lib.State) error {
		runMx.Lock()
		running++
		assert.Equal(t, running, getCurrActiveVUs())
		runMx.Unlock()
		// Block the VU to cause the executor to schedule more
		<-ctx.Done()
		return nil
	})
	test := setupExecutorTest(t, "", "", lib.Options{}, runner, config)
	defer test.cancel()

	getCurrActiveVUs = test.state.GetCurrentlyActiveVUsCount

	engineOut := make(chan metrics.SampleContainer, 1000)
	require.NoError(t, test.executor.Run(test.ctx, engineOut))

	assert.GreaterOrEqual(t, running, int64(5))
	assert.LessOrEqual(t, running, int64(10))
}
