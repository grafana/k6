/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package executor

import (
	"context"
	"io/ioutil"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/testutils/minirunner"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionStateVUIDs(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 0, 0)
	assert.Equal(t, uint64(1), es.GetUniqueVUIdentifier())
	assert.Equal(t, uint64(2), es.GetUniqueVUIdentifier())
	assert.Equal(t, uint64(3), es.GetUniqueVUIdentifier())
	wg := sync.WaitGroup{}
	rand.Seed(time.Now().UnixNano())
	count := 100 + rand.Intn(50)
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			es.GetUniqueVUIdentifier()
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(t, uint64(4+count), es.GetUniqueVUIdentifier())
}

func TestExecutionStateGettingVUsWhenNonAreAvailable(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 0, 0)
	logHook := &testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.WarnLevel}}
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(ioutil.Discard)
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
	logHook := &testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.WarnLevel, logrus.DebugLevel}}
	testLog := logrus.New()
	testLog.AddHook(logHook)
	testLog.SetOutput(ioutil.Discard)
	logEntry := logrus.NewEntry(testLog)

	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 20)
	es.SetInitVUFunc(func(_ context.Context, _ *logrus.Entry) (lib.VU, error) {
		return &minirunner.VU{}, nil
	})

	for i := 0; i < 10; i++ {
		require.EqualValues(t, i, es.GetInitializedVUsCount())
		vu, err := es.InitializeNewVU(context.Background(), logEntry)
		require.NoError(t, err)
		require.EqualValues(t, i+1, es.GetInitializedVUsCount())
		es.ReturnVU(vu, false)
		require.EqualValues(t, 0, es.GetCurrentlyActiveVUsCount())
		require.EqualValues(t, i+1, es.GetInitializedVUsCount())
	}

	// Test getting initialized VUs is okay :)
	for i := 0; i < 10; i++ {
		require.EqualValues(t, i, es.GetCurrentlyActiveVUsCount())
		vu, err := es.GetPlannedVU(logEntry, true)
		require.NoError(t, err)
		require.Empty(t, logHook.Drain())
		require.NotNil(t, vu)
		require.EqualValues(t, i+1, es.GetCurrentlyActiveVUsCount())
		require.EqualValues(t, 10, es.GetInitializedVUsCount())
	}

	// Check that getting 1 more planned VU will error out
	vu, err := es.GetPlannedVU(logEntry, true)
	require.Nil(t, vu)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not get a VU from the buffer in")
	entries := logHook.Drain()
	require.Equal(t, lib.MaxRetriesGetPlannedVU, len(entries))
	for _, entry := range entries {
		require.Contains(t, entry.Message, "Could not get a VU from the buffer for ")
	}

	// Test getting uninitiazed vus will work
	for i := 0; i < 10; i++ {
		require.EqualValues(t, 10+i, es.GetCurrentlyActiveVUsCount())
		vu, err = es.GetUnplannedVU(context.Background(), logEntry)
		require.NoError(t, err)
		require.Empty(t, logHook.Drain())
		require.NotNil(t, vu)
		require.EqualValues(t, 10+i+1, es.GetCurrentlyActiveVUsCount())
		require.EqualValues(t, 10+i+1, es.GetInitializedVUsCount())
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
	es := lib.NewExecutionState(lib.Options{}, et, 0, 0)
	require.False(t, es.HasStarted())
	es.MarkStarted()
	require.True(t, es.HasStarted())
	require.Panics(t, es.MarkStarted)
}

func TestMarkEnded(t *testing.T) {
	t.Parallel()
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 0, 0)
	require.False(t, es.HasEnded())
	es.MarkEnded()
	require.True(t, es.HasEnded())
	require.Panics(t, es.MarkEnded)
}
