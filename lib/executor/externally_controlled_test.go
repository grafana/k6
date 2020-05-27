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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
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
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)
	es := lib.NewExecutionState(lib.Options{}, et, 10, 50)
	var ctx, cancel, executor, _ = setupExecutor(
		t, getTestExternallyControlledConfig(), es,
		simpleRunner(func(ctx context.Context) error {
			time.Sleep(200 * time.Millisecond)
			atomic.AddUint64(doneIters, 1)
			return nil
		}),
	)
	defer cancel()

	var (
		wg            sync.WaitGroup
		errCh         = make(chan error, 1)
		doneCh        = make(chan struct{})
		resultVUCount []int64
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- executor.Run(ctx, nil)
		close(doneCh)
	}()

	updateConfig := func(vus int) {
		newConfig := ExternallyControlledConfigParams{
			VUs:      null.IntFrom(int64(vus)),
			MaxVUs:   null.IntFrom(10),
			Duration: types.NullDurationFrom(2 * time.Second),
		}
		err := executor.(*ExternallyControlled).UpdateConfig(ctx, newConfig)
		assert.NoError(t, err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		es.MarkStarted()
		time.Sleep(150 * time.Millisecond) // wait for startup
		resultVUCount = append(resultVUCount, es.GetCurrentlyActiveVUsCount())
		time.Sleep(500 * time.Millisecond)
		updateConfig(4)
		time.Sleep(100 * time.Millisecond)
		resultVUCount = append(resultVUCount, es.GetCurrentlyActiveVUsCount())
		time.Sleep(500 * time.Millisecond)
		updateConfig(8)
		time.Sleep(100 * time.Millisecond)
		resultVUCount = append(resultVUCount, es.GetCurrentlyActiveVUsCount())
		time.Sleep(1 * time.Second)
		resultVUCount = append(resultVUCount, es.GetCurrentlyActiveVUsCount())
		es.MarkEnded()
	}()

	<-doneCh
	wg.Wait()
	require.NoError(t, <-errCh)
	assert.Equal(t, uint64(50), atomic.LoadUint64(doneIters))
	assert.Equal(t, []int64{2, 4, 8, 0}, resultVUCount)
}
