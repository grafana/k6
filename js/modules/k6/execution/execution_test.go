/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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

package execution

import (
	"context"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
)

func TestScenarioStage(t *testing.T) {
	t.Parallel()

	rt := goja.New()
	ctx := common.WithRuntime(context.Background(), rt)
	ctx = lib.WithScenarioState(ctx, &lib.ScenarioState{
		Stages: []lib.ScenarioStage{
			{
				Index:    0,
				Name:     "ramp up",
				Duration: 10 * time.Second,
			},
			{
				Index:    1,
				Name:     "ramp down",
				Duration: 10 * time.Second,
			},
		},
		StartTime: time.Now().Add(-11 * time.Second),
	})
	m, ok := New().NewModuleInstance(
		&modulestest.InstanceCore{
			Runtime: rt,
			InitEnv: &common.InitEnvironment{},
			State:   &lib.State{},
			Ctx:     ctx,
		},
	).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("exec", m.GetExports().Default))

	num, err := rt.RunString(`exec.scenario.stage.number`)
	require.NoError(t, err)
	assert.Equal(t, int64(1), num.ToInteger())

	stage, err := rt.RunString(`exec.scenario.stage.name`)
	require.NoError(t, err)
	assert.Equal(t, "ramp down", stage.String())
}
