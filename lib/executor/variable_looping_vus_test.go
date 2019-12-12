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
	null "gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
)

func getTestVariableLoopingVUsConfig() VariableLoopingVUsConfig {
	return VariableLoopingVUsConfig{
		BaseConfig: BaseConfig{GracefulStop: types.NullDurationFrom(0)},
		StartVUs:   null.IntFrom(5),
		Stages: []Stage{
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(5),
			},
			{
				Duration: types.NullDurationFrom(0),
				Target:   null.IntFrom(3),
			},
			{
				Duration: types.NullDurationFrom(1 * time.Second),
				Target:   null.IntFrom(3),
			},
		},
		GracefulRampDown: types.NullDurationFrom(0),
	}
}

func TestVariableLoopingVUsRun(t *testing.T) {
	t.Parallel()
	var iterCount int64
	es := lib.NewExecutionState(lib.Options{}, 10, 50)
	var ctx, cancel, executor, _ = setupExecutor(
		t, getTestVariableLoopingVUsConfig(), es,
		simpleRunner(func(ctx context.Context) error {
			time.Sleep(200 * time.Millisecond)
			atomic.AddInt64(&iterCount, 1)
			return nil
		}),
	)
	defer cancel()

	var (
		wg     sync.WaitGroup
		result []int64
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond)
		result = append(result, es.GetCurrentlyActiveVUsCount())
		time.Sleep(1 * time.Second)
		result = append(result, es.GetCurrentlyActiveVUsCount())
		time.Sleep(1 * time.Second)
		result = append(result, es.GetCurrentlyActiveVUsCount())
	}()

	err := executor.Run(ctx, nil)

	wg.Wait()
	require.NoError(t, err)
	assert.Equal(t, []int64{5, 3, 0}, result)
	assert.Equal(t, int64(40), iterCount)
}
