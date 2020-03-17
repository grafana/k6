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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
)

type exp struct {
	parseError      bool
	validationError bool
	custom          func(t *testing.T, cm lib.ExecutorConfigMap)
}

type configMapTestCase struct {
	rawJSON  string
	expected exp
}

//nolint:lll,gochecknoglobals
var configMapTestCases = []configMapTestCase{
	{"", exp{parseError: true}},
	{"1234", exp{parseError: true}},
	{"asdf", exp{parseError: true}},
	{"'adsf'", exp{parseError: true}},
	{"[]", exp{parseError: true}},
	{"{}", exp{custom: func(t *testing.T, cm lib.ExecutorConfigMap) {
		assert.Equal(t, cm, lib.ExecutorConfigMap{})
	}}},
	{"{}asdf", exp{parseError: true}},
	{"null", exp{custom: func(t *testing.T, cm lib.ExecutorConfigMap) {
		assert.Nil(t, cm)
	}}},
	{`{"someKey": {}}`, exp{parseError: true}},
	{`{"someKey": {"type": "constant-blah-blah", "vus": 10, "duration": "60s"}}`, exp{parseError: true}},
	{`{"someKey": {"type": "constant-looping-vus", "uknownField": "should_error"}}`, exp{parseError: true}},
	{`{"someKey": {"type": "constant-looping-vus", "vus": 10, "duration": "60s", "env": 123}}`, exp{parseError: true}},

	// Validation errors for constant-looping-vus and the base config
	{`{"someKey": {"type": "constant-looping-vus", "vus": 10, "duration": "60s",
		"gracefulStop": "10s", "startTime": "70s", "env": {"test": "mest"}, "exec": "someFunc"}}`,
		exp{custom: func(t *testing.T, cm lib.ExecutorConfigMap) {
			sched := NewConstantLoopingVUsConfig("someKey")
			sched.VUs = null.IntFrom(10)
			sched.Duration = types.NullDurationFrom(1 * time.Minute)
			sched.GracefulStop = types.NullDurationFrom(10 * time.Second)
			sched.StartTime = types.NullDurationFrom(70 * time.Second)
			sched.Exec = null.StringFrom("someFunc")
			sched.Env = map[string]string{"test": "mest"}
			require.Equal(t, cm, lib.ExecutorConfigMap{"someKey": sched})
			require.Equal(t, sched.BaseConfig.Name, cm["someKey"].GetName())
			require.Equal(t, sched.BaseConfig.Type, cm["someKey"].GetType())
			require.Equal(t, sched.BaseConfig.GetGracefulStop(), cm["someKey"].GetGracefulStop())
			require.Equal(t,
				sched.BaseConfig.StartTime.Duration,
				types.Duration(cm["someKey"].GetStartTime()),
			)
			require.Equal(t, sched.BaseConfig.Env, cm["someKey"].GetEnv())

			assert.Empty(t, cm["someKey"].Validate())
			assert.Empty(t, cm.Validate())

			assert.Equal(t, "10 looping VUs for 1m0s (exec: someFunc, startTime: 1m10s, gracefulStop: 10s)", cm["someKey"].GetDescription(nil))

			schedReqs := cm["someKey"].GetExecutionRequirements(nil)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 70*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(10), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(10), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(nil)
			endOffset, isFinal = lib.GetEndOffset(totalReqs)
			assert.Equal(t, 140*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(10), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(10), lib.GetMaxPossibleVUs(schedReqs))

		}},
	},
	{`{"aname": {"type": "constant-looping-vus", "duration": "60s"}}`, exp{}},
	{`{"": {"type": "constant-looping-vus", "vus": 10, "duration": "60s"}}`, exp{validationError: true}},
	{`{"aname": {"type": "constant-looping-vus"}}`, exp{validationError: true}},
	{`{"aname": {"type": "constant-looping-vus", "vus": 0.5}}`, exp{parseError: true}},
	{`{"aname": {"type": "constant-looping-vus", "vus": 10}}`, exp{validationError: true}},
	{`{"aname": {"type": "constant-looping-vus", "vus": 0, "duration": "60s"}}`, exp{validationError: true}},
	{`{"aname": {"type": "constant-looping-vus", "vus": -1, "duration": "60s"}}`, exp{validationError: true}},
	{`{"aname": {"type": "constant-looping-vus", "vus": 10, "duration": "0s"}}`, exp{validationError: true}},
	{`{"aname": {"type": "constant-looping-vus", "vus": 10, "duration": "10s", "startTime": "-10s"}}`, exp{validationError: true}},
	{`{"aname": {"type": "constant-looping-vus", "vus": 10, "duration": "10s", "exec": ""}}`, exp{validationError: true}},
	{`{"aname": {"type": "constant-looping-vus", "vus": 10, "duration": "10s", "gracefulStop": "-2s"}}`, exp{validationError: true}},
	// variable-looping-vus
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": 20, "gracefulStop": "15s", "gracefulRampDown": "10s",
		    "startTime": "23s", "stages": [{"duration": "60s", "target": 30}, {"duration": "130s", "target": 10}]}}`,
		exp{custom: func(t *testing.T, cm lib.ExecutorConfigMap) {
			sched := NewVariableLoopingVUsConfig("varloops")
			sched.GracefulStop = types.NullDurationFrom(15 * time.Second)
			sched.GracefulRampDown = types.NullDurationFrom(10 * time.Second)
			sched.StartVUs = null.IntFrom(20)
			sched.StartTime = types.NullDurationFrom(23 * time.Second)
			sched.Stages = []Stage{
				{Target: null.IntFrom(30), Duration: types.NullDurationFrom(60 * time.Second)},
				{Target: null.IntFrom(10), Duration: types.NullDurationFrom(130 * time.Second)},
			}
			require.Equal(t, cm, lib.ExecutorConfigMap{"varloops": sched})

			assert.Empty(t, cm["varloops"].Validate())
			assert.Empty(t, cm.Validate())

			assert.Equal(t, "Up to 30 looping VUs for 3m10s over 2 stages (gracefulRampDown: 10s, startTime: 23s, gracefulStop: 15s)", cm["varloops"].GetDescription(nil))

			schedReqs := cm["varloops"].GetExecutionRequirements(nil)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 205*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(30), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(30), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(nil)
			endOffset, isFinal = lib.GetEndOffset(totalReqs)
			assert.Equal(t, 228*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(30), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(30), lib.GetMaxPossibleVUs(schedReqs))
		}},
	},
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": 1, "gracefulStop": "0s", "gracefulRampDown": "10s",
			"stages": [{"duration": "10s", "target": 10}]}}`,
		exp{custom: func(t *testing.T, cm lib.ExecutorConfigMap) {
			assert.Empty(t, cm["varloops"].Validate())
			assert.Empty(t, cm.Validate())

			assert.Equal(t, "Up to 10 looping VUs for 10s over 1 stages (gracefulRampDown: 10s)", cm["varloops"].GetDescription(nil))

			schedReqs := cm["varloops"].GetExecutionRequirements(nil)
			assert.Equal(t, uint64(10), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(10), lib.GetMaxPossibleVUs(schedReqs))
		}},
	},
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": 1, "gracefulStop": "0s", "gracefulRampDown": "0s",
			"stages": [{"duration": "10s", "target": 10}, {"duration": "0s", "target": 1}, {"duration": "10s", "target": 5}]}}`,
		exp{custom: func(t *testing.T, cm lib.ExecutorConfigMap) {
			assert.Empty(t, cm["varloops"].Validate())
			assert.Empty(t, cm.Validate())

			assert.Equal(t, "Up to 10 looping VUs for 20s over 3 stages (gracefulRampDown: 0s)", cm["varloops"].GetDescription(nil))

			schedReqs := cm.GetFullExecutionRequirements(nil)
			assert.Equal(t, uint64(10), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(10), lib.GetMaxPossibleVUs(schedReqs))
		}},
	},
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": 1, "gracefulStop": "0s", "gracefulRampDown": "0s",
			"stages": [{"duration": "10s", "target": 10}, {"duration": "0s", "target": 11},{"duration": "0s", "target": 1}, {"duration": "10s", "target": 5}]}}`,
		exp{custom: func(t *testing.T, cm lib.ExecutorConfigMap) {
			assert.Empty(t, cm["varloops"].Validate())
			assert.Empty(t, cm.Validate())

			assert.Equal(t, "Up to 11 looping VUs for 20s over 4 stages (gracefulRampDown: 0s)", cm["varloops"].GetDescription(nil))

			schedReqs := cm.GetFullExecutionRequirements(nil)
			assert.Equal(t, uint64(11), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(11), lib.GetMaxPossibleVUs(schedReqs))
		}},
	},
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": 0, "stages": [{"duration": "60s", "target": 0}]}}`, exp{}},
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": -1, "stages": [{"duration": "60s", "target": 30}]}}`, exp{validationError: true}},
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": 2, "stages": [{"duration": "-60s", "target": 30}]}}`, exp{validationError: true}},
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": 2, "stages": [{"duration": "60s", "target": -30}]}}`, exp{validationError: true}},
	{`{"varloops": {"type": "variable-looping-vus", "stages": [{"duration": "60s"}]}}`, exp{validationError: true}},
	{`{"varloops": {"type": "variable-looping-vus", "stages": [{"target": 30}]}}`, exp{validationError: true}},
	{`{"varloops": {"type": "variable-looping-vus", "stages": []}}`, exp{validationError: true}},
	{`{"varloops": {"type": "variable-looping-vus"}}`, exp{validationError: true}},
	// shared-iterations
	{`{"ishared": {"type": "shared-iterations", "iterations": 22, "vus": 12, "maxDuration": "100s"}}`,
		exp{custom: func(t *testing.T, cm lib.ExecutorConfigMap) {
			sched := NewSharedIterationsConfig("ishared")
			sched.Iterations = null.IntFrom(22)
			sched.MaxDuration = types.NullDurationFrom(100 * time.Second)
			sched.VUs = null.IntFrom(12)

			assert.Empty(t, cm["ishared"].Validate())
			assert.Empty(t, cm.Validate())

			assert.Equal(t, "22 iterations shared among 12 VUs (maxDuration: 1m40s, gracefulStop: 30s)", cm["ishared"].GetDescription(nil))

			schedReqs := cm["ishared"].GetExecutionRequirements(nil)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 130*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(12), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(12), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(nil)
			assert.Equal(t, schedReqs, totalReqs)
		}},
	},
	{`{"ishared": {"type": "shared-iterations"}}`, exp{}}, // Has 1 VU & 1 iter default values
	{`{"ishared": {"type": "shared-iterations", "iterations": 20}}`, exp{}},
	{`{"ishared": {"type": "shared-iterations", "vus": 10}}`, exp{validationError: true}}, // error because VUs are more than iters
	{`{"ishared": {"type": "shared-iterations", "iterations": 20, "vus": 10, "maxDuration": "30m"}}`, exp{}},
	{`{"ishared": {"type": "shared-iterations", "iterations": 20, "vus": 10, "maxDuration": "-3m"}}`, exp{validationError: true}},
	{`{"ishared": {"type": "shared-iterations", "iterations": 20, "vus": 10, "maxDuration": "0s"}}`, exp{validationError: true}},
	{`{"ishared": {"type": "shared-iterations", "iterations": 20, "vus": -10}}`, exp{validationError: true}},
	{`{"ishared": {"type": "shared-iterations", "iterations": -1, "vus": 1}}`, exp{validationError: true}},
	{`{"ishared": {"type": "shared-iterations", "iterations": 20, "vus": 30}}`, exp{validationError: true}},
	// per-vu-iterations
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 23, "vus": 13, "gracefulStop": 0}}`,
		exp{custom: func(t *testing.T, cm lib.ExecutorConfigMap) {
			sched := NewPerVUIterationsConfig("ipervu")
			sched.Iterations = null.IntFrom(23)
			sched.GracefulStop = types.NullDurationFrom(0)
			sched.VUs = null.IntFrom(13)

			assert.Empty(t, cm["ipervu"].Validate())
			assert.Empty(t, cm.Validate())

			assert.Equal(t, "23 iterations for each of 13 VUs (maxDuration: 10m0s)", cm["ipervu"].GetDescription(nil))

			schedReqs := cm["ipervu"].GetExecutionRequirements(nil)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 600*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(13), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(13), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(nil)
			assert.Equal(t, schedReqs, totalReqs)
		}},
	},
	{`{"ipervu": {"type": "per-vu-iterations"}}`, exp{}}, // Has 1 VU & 1 iter default values
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 20}}`, exp{}},
	{`{"ipervu": {"type": "per-vu-iterations", "vus": 10}}`, exp{}},
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 20, "vus": 10}}`, exp{}},
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 20, "vus": 10, "maxDuration": "-3m"}}`, exp{validationError: true}},
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 20, "vus": 10, "maxDuration": "0s"}}`, exp{validationError: true}},
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 20, "vus": -10}}`, exp{validationError: true}},
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": -1, "vus": 1}}`, exp{validationError: true}},

	// constant-arrival-rate
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 30, "timeUnit": "1m", "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`,
		exp{custom: func(t *testing.T, cm lib.ExecutorConfigMap) {
			sched := NewConstantArrivalRateConfig("carrival")
			sched.Rate = null.IntFrom(30)
			sched.Duration = types.NullDurationFrom(10 * time.Minute)
			sched.TimeUnit = types.NullDurationFrom(1 * time.Minute)
			sched.PreAllocatedVUs = null.IntFrom(20)
			sched.MaxVUs = null.IntFrom(30)

			assert.Empty(t, cm["carrival"].Validate())
			assert.Empty(t, cm.Validate())

			assert.Equal(t, "0.50 iterations/s for 10m0s (maxVUs: 20-30, gracefulStop: 30s)", cm["carrival"].GetDescription(nil))

			schedReqs := cm["carrival"].GetExecutionRequirements(nil)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 630*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(20), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(30), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(nil)
			assert.Equal(t, schedReqs, totalReqs)
		}},
	},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`, exp{}},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30, "timeUnit": "-1s"}}`, exp{validationError: true}},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20}}`, exp{validationError: true}},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "maxVUs": 30}}`, exp{validationError: true}},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "preAllocatedVUs": 20, "maxVUs": 30}}`, exp{validationError: true}},
	{`{"carrival": {"type": "constant-arrival-rate", "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`, exp{validationError: true}},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "0m", "preAllocatedVUs": 20, "maxVUs": 30}}`, exp{validationError: true}},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 0, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`, exp{validationError: true}},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 15}}`, exp{validationError: true}},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "0s", "preAllocatedVUs": 20, "maxVUs": 25}}`, exp{validationError: true}},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": -2, "maxVUs": 25}}`, exp{validationError: true}},
	// variable-arrival-rate
	{`{"varrival": {"type": "variable-arrival-rate", "startRate": 10, "timeUnit": "30s", "preAllocatedVUs": 20,
		"maxVUs": 50, "stages": [{"duration": "3m", "target": 30}, {"duration": "5m", "target": 10}]}}`,
		exp{custom: func(t *testing.T, cm lib.ExecutorConfigMap) {
			sched := NewVariableArrivalRateConfig("varrival")
			sched.StartRate = null.IntFrom(10)
			sched.Stages = []Stage{
				{Target: null.IntFrom(30), Duration: types.NullDurationFrom(180 * time.Second)},
				{Target: null.IntFrom(10), Duration: types.NullDurationFrom(300 * time.Second)},
			}
			sched.TimeUnit = types.NullDurationFrom(30 * time.Second)
			sched.PreAllocatedVUs = null.IntFrom(20)
			sched.MaxVUs = null.IntFrom(50)
			require.Equal(t, cm, lib.ExecutorConfigMap{"varrival": sched})

			assert.Empty(t, cm["varrival"].Validate())
			assert.Empty(t, cm.Validate())

			assert.Equal(t, "Up to 1.00 iterations/s for 8m0s over 2 stages (maxVUs: 20-50, gracefulStop: 30s)", cm["varrival"].GetDescription(nil))

			schedReqs := cm["varrival"].GetExecutionRequirements(nil)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 510*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(20), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(50), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(nil)
			assert.Equal(t, schedReqs, totalReqs)
		}},
	},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, exp{}},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": -20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, exp{validationError: true}},
	{`{"varrival": {"type": "variable-arrival-rate", "startRate": -1, "preAllocatedVUs": 20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, exp{validationError: true}},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 20, "stages": [{"duration": "5m", "target": 10}]}}`, exp{validationError: true}},
	{`{"varrival": {"type": "variable-arrival-rate", "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, exp{validationError: true}},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50}}`, exp{validationError: true}},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50, "stages": []}}`, exp{validationError: true}},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}], "timeUnit": "-1s"}}`, exp{validationError: true}},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 30, "maxVUs": 20, "stages": [{"duration": "5m", "target": 10}]}}`, exp{validationError: true}},
	//TODO: more tests of mixed executors and execution plans
}

func TestConfigMapParsingAndValidation(t *testing.T) {
	t.Parallel()
	for i, tc := range configMapTestCases {
		tc := tc
		t.Run(fmt.Sprintf("TestCase#%d", i), func(t *testing.T) {
			t.Logf(tc.rawJSON)
			var result lib.ExecutorConfigMap
			err := json.Unmarshal([]byte(tc.rawJSON), &result)
			if tc.expected.parseError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			parseErrors := result.Validate()
			if tc.expected.validationError {
				assert.NotEmpty(t, parseErrors)
			} else {
				assert.Empty(t, parseErrors)
			}
			if tc.expected.custom != nil {
				tc.expected.custom(t, result)
			}
		})
	}
}

func TestVariableLoopingVUsConfigExecutionPlanExample(t *testing.T) {
	t.Parallel()
	conf := NewVariableLoopingVUsConfig("test")
	conf.StartVUs = null.IntFrom(4)
	conf.Stages = []Stage{
		{Target: null.IntFrom(6), Duration: types.NullDurationFrom(2 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(5 * time.Second)},
		{Target: null.IntFrom(5), Duration: types.NullDurationFrom(4 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(4 * time.Second)},
		{Target: null.IntFrom(4), Duration: types.NullDurationFrom(3 * time.Second)},
		{Target: null.IntFrom(4), Duration: types.NullDurationFrom(2 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(0 * time.Second)},
		{Target: null.IntFrom(1), Duration: types.NullDurationFrom(3 * time.Second)},
	}

	expRawStepsNoZeroEnd := []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 4},
		{TimeOffset: 1 * time.Second, PlannedVUs: 5},
		{TimeOffset: 2 * time.Second, PlannedVUs: 6},
		{TimeOffset: 3 * time.Second, PlannedVUs: 5},
		{TimeOffset: 4 * time.Second, PlannedVUs: 4},
		{TimeOffset: 5 * time.Second, PlannedVUs: 3},
		{TimeOffset: 6 * time.Second, PlannedVUs: 2},
		{TimeOffset: 7 * time.Second, PlannedVUs: 1},
		{TimeOffset: 8 * time.Second, PlannedVUs: 2},
		{TimeOffset: 9 * time.Second, PlannedVUs: 3},
		{TimeOffset: 10 * time.Second, PlannedVUs: 4},
		{TimeOffset: 11 * time.Second, PlannedVUs: 5},
		{TimeOffset: 12 * time.Second, PlannedVUs: 4},
		{TimeOffset: 13 * time.Second, PlannedVUs: 3},
		{TimeOffset: 14 * time.Second, PlannedVUs: 2},
		{TimeOffset: 15 * time.Second, PlannedVUs: 1},
		{TimeOffset: 16 * time.Second, PlannedVUs: 2},
		{TimeOffset: 17 * time.Second, PlannedVUs: 3},
		{TimeOffset: 18 * time.Second, PlannedVUs: 4},
		{TimeOffset: 20 * time.Second, PlannedVUs: 1},
	}
	rawStepsNoZeroEnd := conf.getRawExecutionSteps(nil, false)
	assert.Equal(t, expRawStepsNoZeroEnd, rawStepsNoZeroEnd)
	endOffset, isFinal := lib.GetEndOffset(rawStepsNoZeroEnd)
	assert.Equal(t, 20*time.Second, endOffset)
	assert.Equal(t, false, isFinal)

	rawStepsZeroEnd := conf.getRawExecutionSteps(nil, true)
	assert.Equal(t,
		append(expRawStepsNoZeroEnd, lib.ExecutionStep{TimeOffset: 23 * time.Second, PlannedVUs: 0}),
		rawStepsZeroEnd,
	)
	endOffset, isFinal = lib.GetEndOffset(rawStepsZeroEnd)
	assert.Equal(t, 23*time.Second, endOffset)
	assert.Equal(t, true, isFinal)

	// GracefulStop and GracefulRampDown equal to the default 30 sec
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 4},
		{TimeOffset: 1 * time.Second, PlannedVUs: 5},
		{TimeOffset: 2 * time.Second, PlannedVUs: 6},
		{TimeOffset: 33 * time.Second, PlannedVUs: 5},
		{TimeOffset: 42 * time.Second, PlannedVUs: 4},
		{TimeOffset: 50 * time.Second, PlannedVUs: 1},
		{TimeOffset: 53 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(nil))

	// Try a longer GracefulStop than the GracefulRampDown
	conf.GracefulStop = types.NullDurationFrom(80 * time.Second)
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 4},
		{TimeOffset: 1 * time.Second, PlannedVUs: 5},
		{TimeOffset: 2 * time.Second, PlannedVUs: 6},
		{TimeOffset: 33 * time.Second, PlannedVUs: 5},
		{TimeOffset: 42 * time.Second, PlannedVUs: 4},
		{TimeOffset: 50 * time.Second, PlannedVUs: 1},
		{TimeOffset: 103 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(nil))

	// Try a much shorter GracefulStop than the GracefulRampDown
	conf.GracefulStop = types.NullDurationFrom(3 * time.Second)
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 4},
		{TimeOffset: 1 * time.Second, PlannedVUs: 5},
		{TimeOffset: 2 * time.Second, PlannedVUs: 6},
		{TimeOffset: 26 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(nil))

	// Try a zero GracefulStop
	conf.GracefulStop = types.NullDurationFrom(0 * time.Second)
	assert.Equal(t, []lib.ExecutionStep{
		{TimeOffset: 0 * time.Second, PlannedVUs: 4},
		{TimeOffset: 1 * time.Second, PlannedVUs: 5},
		{TimeOffset: 2 * time.Second, PlannedVUs: 6},
		{TimeOffset: 23 * time.Second, PlannedVUs: 0},
	}, conf.GetExecutionRequirements(nil))

	// Try a zero GracefulStop and GracefulRampDown, i.e. raw steps with 0 end cap
	conf.GracefulRampDown = types.NullDurationFrom(0 * time.Second)
	assert.Equal(t, rawStepsZeroEnd, conf.GetExecutionRequirements(nil))
}
