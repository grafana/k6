package executor

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/lib/testutils/minirunner"
	"go.k6.io/k6/lib"
)

func TestExecutionStateVUIDs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		seq, seg string
	}{
		{},
		{seq: "0,1/4,3/4,1", seg: "0:1/4"},
		{seq: "0,0.3,0.5,0.6,0.7,0.8,0.9,1", seg: "0.5:0.6"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("seq:%s;segment:%s", tc.seq, tc.seg), func(t *testing.T) {
			t.Parallel()
			ess, err := lib.NewExecutionSegmentSequenceFromString(tc.seq)
			require.NoError(t, err)
			segment, err := lib.NewExecutionSegmentFromString(tc.seg)
			require.NoError(t, err)
			et, err := lib.NewExecutionTuple(segment, &ess)
			require.NoError(t, err)

			start, offsets, _ := et.GetStripedOffsets()
			es := lib.NewExecutionState(nil, et, 0, 0)

			idl, idg := es.GetUniqueVUIdentifiers()
			assert.EqualValues(t, 1, idl)
			expGlobal := start + 1
			assert.EqualValues(t, expGlobal, idg)

			idl, idg = es.GetUniqueVUIdentifiers()
			assert.EqualValues(t, 2, idl)
			expGlobal += offsets[0]
			assert.EqualValues(t, expGlobal, idg)

			idl, idg = es.GetUniqueVUIdentifiers()
			assert.EqualValues(t, 3, idl)
			expGlobal += offsets[0]
			assert.EqualValues(t, expGlobal, idg)

			seed := time.Now().UnixNano()
			r := rand.New(rand.NewSource(seed)) //nolint:gosec
			t.Logf("Random source seeded with %d\n", seed)
			count := 100 + r.Intn(50)
			wg := sync.WaitGroup{}
			wg.Add(count)
			for i := 0; i < count; i++ {
				go func() {
					es.GetUniqueVUIdentifiers()
					wg.Done()
				}()
			}
			wg.Wait()
			idl, idg = es.GetUniqueVUIdentifiers()
			assert.EqualValues(t, 4+count, idl)
			assert.EqualValues(t, (3+count)*int(offsets[0])+int(start+1), idg)
		})
	}
}

func TestExecutionStateGettingVUsWhenNonAreAvailable(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(nil, et, 0, 0)
	logHook := testutils.NewLogHook(logrus.WarnLevel)
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(io.Discard)
	vu, err := es.GetPlannedVU(logrus.NewEntry(testLog), true)
	require.Nil(t, vu)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not get a VU from the buffer in")
	entries := logHook.Drain()
	require.Equal(t, lib.MaxRetriesGetPlannedVU, len(entries))
	for _, entry := range entries {
		require.Contains(t, entry.Message, "Could not get a VU from the buffer for ")
	}
}

func TestExecutionStateGettingVUs(t *testing.T) {
	t.Parallel()
	logHook := testutils.NewLogHook(logrus.WarnLevel, logrus.DebugLevel)
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(io.Discard)
	logEntry := logrus.NewEntry(testLog)

	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(nil, et, 10, 20)
	es.SetInitVUFunc(func(_ context.Context, _ *logrus.Entry) (lib.InitializedVU, error) {
		return &minirunner.VU{}, nil
	})

	var vu lib.InitializedVU
	for i := 0; i < 10; i++ {
		require.EqualValues(t, i, es.GetInitializedVUsCount())
		vu, err = es.InitializeNewVU(context.Background(), logEntry)
		require.NoError(t, err)
		require.EqualValues(t, i+1, es.GetInitializedVUsCount())
		es.ReturnVU(vu, false)
		require.EqualValues(t, 0, es.GetCurrentlyActiveVUsCount())
		require.EqualValues(t, i+1, es.GetInitializedVUsCount())
	}

	// Test getting initialized VUs is okay :)
	for i := 0; i < 10; i++ {
		require.EqualValues(t, i, es.GetCurrentlyActiveVUsCount())
		vu, err = es.GetPlannedVU(logEntry, true)
		require.NoError(t, err)
		require.Empty(t, logHook.Drain())
		require.NotNil(t, vu)
		require.EqualValues(t, i+1, es.GetCurrentlyActiveVUsCount())
		require.EqualValues(t, 10, es.GetInitializedVUsCount())
	}

	// Check that getting 1 more planned VU will error out
	vu, err = es.GetPlannedVU(logEntry, true)
	require.Nil(t, vu)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not get a VU from the buffer in")
	entries := logHook.Drain()
	require.Equal(t, lib.MaxRetriesGetPlannedVU, len(entries))
	for _, entry := range entries {
		require.Contains(t, entry.Message, "Could not get a VU from the buffer for ")
	}

	// Test getting uninitialized vus will work
	for i := 0; i < 10; i++ {
		require.EqualValues(t, 10+i, es.GetInitializedVUsCount())
		vu, err = es.GetUnplannedVU(context.Background(), logEntry)
		require.NoError(t, err)
		require.Empty(t, logHook.Drain())
		require.NotNil(t, vu)
		require.EqualValues(t, 10+i+1, es.GetInitializedVUsCount())
		require.EqualValues(t, 10, es.GetCurrentlyActiveVUsCount())
	}

	// Check that getting 1 more unplanned VU will error out
	vu, err = es.GetUnplannedVU(context.Background(), logEntry)
	require.Nil(t, vu)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not get a VU from the buffer in")
	entries = logHook.Drain()
	require.Equal(t, lib.MaxRetriesGetPlannedVU, len(entries))
	for _, entry := range entries {
		require.Contains(t, entry.Message, "Could not get a VU from the buffer for ")
	}
}

func TestMarkStartedPanicsOnSecondRun(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(nil, et, 0, 0)
	require.False(t, es.HasStarted())
	es.MarkStarted()
	require.True(t, es.HasStarted())
	require.Panics(t, es.MarkStarted)
}

func TestMarkEnded(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(nil, et, 0, 0)
	require.False(t, es.HasEnded())
	es.MarkEnded()
	require.True(t, es.HasEnded())
	require.Panics(t, es.MarkEnded)
}
