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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
)

func getTestConstantVUsConfig() ConstantVUsConfig {
	return ConstantVUsConfig{
		BaseConfig: BaseConfig{GracefulStop: types.NullDurationFrom(100 * time.Millisecond)},
		VUs:        null.IntFrom(10),
		Duration:   types.NullDurationFrom(1 * time.Second),
	}
}

func TestConstantVUsRun(t *testing.T) {
	t.Parallel()
	var result sync.Map

	runner := simpleRunner(func(ctx context.Context, state *lib.State) error {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		currIter, _ := result.LoadOrStore(state.VUID, uint64(0))
		result.Store(state.VUID, currIter.(uint64)+1) //nolint:forcetypeassert
		time.Sleep(210 * time.Millisecond)
		return nil
	})

	test := setupExecutorTest(t, "", "", lib.Options{}, runner, getTestConstantVUsConfig())
	defer test.cancel()

	require.NoError(t, test.executor.Run(test.ctx, nil))

	var totalIters uint64
	result.Range(func(key, value interface{}) bool {
		vuIters := value.(uint64)
		assert.Equal(t, uint64(5), vuIters)
		totalIters += vuIters
		return true
	})
	assert.Equal(t, uint64(50), totalIters)
}
