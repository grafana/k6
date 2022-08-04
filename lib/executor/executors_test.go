package executor

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
)

type exp struct {
	parseError      bool
	validationError bool
	custom          func(t *testing.T, cm lib.ScenarioConfigs)
}

type configMapTestCase struct {
	rawJSON  string
	expected exp
}

//nolint:gochecknoglobals
var configMapTestCases = []configMapTestCase{
	{"", exp{parseError: true}},
	{"1234", exp{parseError: true}},
	{"asdf", exp{parseError: true}},
	{"'adsf'", exp{parseError: true}},
	{"[]", exp{parseError: true}},
	{"{}", exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
		assert.Equal(t, cm, lib.ScenarioConfigs{})
	}}},
	{"{}asdf", exp{parseError: true}},
	{"null", exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
		assert.Nil(t, cm)
	}}},
	{`{"someKey": {}}`, exp{parseError: true}},
	{`{"someKey": {"executor": "constant-blah-blah", "vus": 10, "duration": "60s"}}`, exp{parseError: true}},
	{`{"someKey": {"executor": "constant-vus", "uknownField": "should_error"}}`, exp{parseError: true}},
	{`{"someKey": {"executor": "constant-vus", "vus": 10, "duration": "60s", "env": 123}}`, exp{parseError: true}},

	// Validation errors for constant-vus and the base config
	{
		`{"someKey": {"executor": "constant-vus", "vus": 10, "duration": "60s",
		"gracefulStop": "10s", "startTime": "70s", "env": {"test": "mest"}, "exec": "someFunc"}}`,
		exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
			sched := NewConstantVUsConfig("someKey")
			sched.VUs = null.IntFrom(10)
			sched.Duration = types.NullDurationFrom(1 * time.Minute)
			sched.GracefulStop = types.NullDurationFrom(10 * time.Second)
			sched.StartTime = types.NullDurationFrom(70 * time.Second)
			sched.Exec = null.StringFrom("someFunc")
			sched.Env = map[string]string{"test": "mest"}
			require.Equal(t, cm, lib.ScenarioConfigs{"someKey": sched})
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

			et, err := lib.NewExecutionTuple(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, "10 looping VUs for 1m0s (exec: someFunc, startTime: 1m10s, gracefulStop: 10s)", cm["someKey"].GetDescription(et))

			schedReqs := cm["someKey"].GetExecutionRequirements(et)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 70*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(10), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(10), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(et)
			endOffset, isFinal = lib.GetEndOffset(totalReqs)
			assert.Equal(t, 140*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(10), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(10), lib.GetMaxPossibleVUs(schedReqs))
		}},
	},
	{`{"aname": {"executor": "constant-vus", "duration": "60s"}}`, exp{}},
	{`{"": {"executor": "constant-vus", "vus": 10, "duration": "60s"}}`, exp{validationError: true}},
	{`{"aname": {"executor": "constant-vus"}}`, exp{validationError: true}},
	{`{"aname": {"executor": "constant-vus", "vus": 0.5}}`, exp{parseError: true}},
	{`{"aname": {"executor": "constant-vus", "vus": 10}}`, exp{validationError: true}},
	{`{"aname": {"executor": "constant-vus", "vus": 0, "duration": "60s"}}`, exp{validationError: true}},
	{`{"aname": {"executor": "constant-vus", "vus": -1, "duration": "60s"}}`, exp{validationError: true}},
	{`{"aname": {"executor": "constant-vus", "vus": 10, "duration": "0s"}}`, exp{validationError: true}},
	{`{"aname": {"executor": "constant-vus", "vus": 10, "duration": "10s", "startTime": "-10s"}}`, exp{validationError: true}},
	{`{"aname": {"executor": "constant-vus", "vus": 10, "duration": "10s", "exec": ""}}`, exp{validationError: true}},
	{`{"aname": {"executor": "constant-vus", "vus": 10, "duration": "10s", "gracefulStop": "-2s"}}`, exp{validationError: true}},
	// ramping-vus
	{
		`{"varloops": {"executor": "ramping-vus", "startVUs": 20, "gracefulStop": "15s", "gracefulRampDown": "10s",
		    "startTime": "23s", "stages": [{"duration": "60s", "target": 30}, {"duration": "130s", "target": 10}]}}`,
		exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
			sched := NewRampingVUsConfig("varloops")
			sched.GracefulStop = types.NullDurationFrom(15 * time.Second)
			sched.GracefulRampDown = types.NullDurationFrom(10 * time.Second)
			sched.StartVUs = null.IntFrom(20)
			sched.StartTime = types.NullDurationFrom(23 * time.Second)
			sched.Stages = []Stage{
				{Target: null.IntFrom(30), Duration: types.NullDurationFrom(60 * time.Second)},
				{Target: null.IntFrom(10), Duration: types.NullDurationFrom(130 * time.Second)},
			}
			require.Equal(t, cm, lib.ScenarioConfigs{"varloops": sched})

			assert.Empty(t, cm["varloops"].Validate())
			assert.Empty(t, cm.Validate())

			et, err := lib.NewExecutionTuple(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, "Up to 30 looping VUs for 3m10s over 2 stages (gracefulRampDown: 10s, startTime: 23s, gracefulStop: 15s)", cm["varloops"].GetDescription(et))

			schedReqs := cm["varloops"].GetExecutionRequirements(et)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 205*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(30), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(30), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(et)
			endOffset, isFinal = lib.GetEndOffset(totalReqs)
			assert.Equal(t, 228*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(30), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(30), lib.GetMaxPossibleVUs(schedReqs))
		}},
	},
	{
		`{"varloops": {"executor": "ramping-vus", "startVUs": 1, "gracefulStop": "0s", "gracefulRampDown": "10s",
			"stages": [{"duration": "10s", "target": 10}]}}`,
		exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
			assert.Empty(t, cm["varloops"].Validate())
			assert.Empty(t, cm.Validate())

			et, err := lib.NewExecutionTuple(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, "Up to 10 looping VUs for 10s over 1 stages (gracefulRampDown: 10s)", cm["varloops"].GetDescription(et))

			schedReqs := cm["varloops"].GetExecutionRequirements(et)
			assert.Equal(t, uint64(10), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(10), lib.GetMaxPossibleVUs(schedReqs))
		}},
	},
	{
		`{"varloops": {"executor": "ramping-vus", "startVUs": 1, "gracefulStop": "0s", "gracefulRampDown": "0s",
			"stages": [{"duration": "10s", "target": 10}, {"duration": "0s", "target": 1}, {"duration": "10s", "target": 5}]}}`,
		exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
			assert.Empty(t, cm["varloops"].Validate())
			assert.Empty(t, cm.Validate())

			et, err := lib.NewExecutionTuple(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, "Up to 10 looping VUs for 20s over 3 stages (gracefulRampDown: 0s)", cm["varloops"].GetDescription(et))

			schedReqs := cm.GetFullExecutionRequirements(et)
			assert.Equal(t, uint64(10), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(10), lib.GetMaxPossibleVUs(schedReqs))
		}},
	},
	{
		`{"varloops": {"executor": "ramping-vus", "startVUs": 1, "gracefulStop": "0s", "gracefulRampDown": "0s",
			"stages": [{"duration": "10s", "target": 10}, {"duration": "0s", "target": 11},{"duration": "0s", "target": 1}, {"duration": "10s", "target": 5}]}}`,
		exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
			assert.Empty(t, cm["varloops"].Validate())
			assert.Empty(t, cm.Validate())

			et, err := lib.NewExecutionTuple(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, "Up to 11 looping VUs for 20s over 4 stages (gracefulRampDown: 0s)", cm["varloops"].GetDescription(et))

			schedReqs := cm.GetFullExecutionRequirements(et)
			assert.Equal(t, uint64(11), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(11), lib.GetMaxPossibleVUs(schedReqs))
		}},
	},
	{`{"varloops": {"executor": "ramping-vus", "startVUs": 0, "stages": [{"duration": "60s", "target": 0}]}}`, exp{validationError: true}},
	{`{"varloops": {"executor": "ramping-vus", "startVUs": -1, "stages": [{"duration": "60s", "target": 30}]}}`, exp{validationError: true}},
	{`{"varloops": {"executor": "ramping-vus", "startVUs": 2, "stages": [{"duration": "-60s", "target": 30}]}}`, exp{validationError: true}},
	{`{"varloops": {"executor": "ramping-vus", "startVUs": 2, "stages": [{"duration": "60s", "target": -30}]}}`, exp{validationError: true}},
	{`{"varloops": {"executor": "ramping-vus", "stages": [{"duration": "60s"}]}}`, exp{validationError: true}},
	{`{"varloops": {"executor": "ramping-vus", "stages": [{"target": 30}]}}`, exp{validationError: true}},
	{`{"varloops": {"executor": "ramping-vus", "stages": []}}`, exp{validationError: true}},
	{`{"varloops": {"executor": "ramping-vus"}}`, exp{validationError: true}},
	// shared-iterations
	{
		`{"ishared": {"executor": "shared-iterations", "iterations": 22, "vus": 12, "maxDuration": "100s"}}`,
		exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
			sched := NewSharedIterationsConfig("ishared")
			sched.Iterations = null.IntFrom(22)
			sched.MaxDuration = types.NullDurationFrom(100 * time.Second)
			sched.VUs = null.IntFrom(12)

			assert.Empty(t, cm["ishared"].Validate())
			assert.Empty(t, cm.Validate())

			et, err := lib.NewExecutionTuple(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, "22 iterations shared among 12 VUs (maxDuration: 1m40s, gracefulStop: 30s)", cm["ishared"].GetDescription(et))

			schedReqs := cm["ishared"].GetExecutionRequirements(et)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 130*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(12), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(12), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(et)
			assert.Equal(t, schedReqs, totalReqs)

			et = mustNewExecutionTuple(newExecutionSegmentFromString("0:1/3"), newExecutionSegmentSequenceFromString("0,1/3,2/3,1"))
			assert.Equal(t, "8 iterations shared among 4 VUs (maxDuration: 1m40s, gracefulStop: 30s)", cm["ishared"].GetDescription(et))

			schedReqs = cm["ishared"].GetExecutionRequirements(et)
			endOffset, isFinal = lib.GetEndOffset(schedReqs)
			assert.Equal(t, 130*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(4), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(4), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs = cm.GetFullExecutionRequirements(et)
			assert.Equal(t, schedReqs, totalReqs)

			et = mustNewExecutionTuple(newExecutionSegmentFromString("1/3:2/3"), newExecutionSegmentSequenceFromString("0,1/3,2/3,1"))
			assert.Equal(t, "7 iterations shared among 4 VUs (maxDuration: 1m40s, gracefulStop: 30s)", cm["ishared"].GetDescription(et))

			schedReqs = cm["ishared"].GetExecutionRequirements(et)
			endOffset, isFinal = lib.GetEndOffset(schedReqs)
			assert.Equal(t, 130*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(4), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(4), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs = cm.GetFullExecutionRequirements(et)
			assert.Equal(t, schedReqs, totalReqs)

			et = mustNewExecutionTuple(newExecutionSegmentFromString("12/13:1"),
				newExecutionSegmentSequenceFromString("0,1/13,2/13,3/13,4/13,5/13,6/13,7/13,8/13,9/13,10/13,11/13,12/13,1"))
			assert.Equal(t, "0 iterations shared among 0 VUs (maxDuration: 1m40s, gracefulStop: 30s)", cm["ishared"].GetDescription(et))

			schedReqs = cm["ishared"].GetExecutionRequirements(et)
			endOffset, isFinal = lib.GetEndOffset(schedReqs)
			assert.Equal(t, time.Duration(0), endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(0), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(0), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs = cm.GetFullExecutionRequirements(et)
			assert.Equal(t, schedReqs, totalReqs)
		}},
	},
	{`{"ishared": {"executor": "shared-iterations"}}`, exp{}}, // Has 1 VU & 1 iter default values
	{`{"ishared": {"executor": "shared-iterations", "iterations": 20}}`, exp{}},
	{`{"ishared": {"executor": "shared-iterations", "vus": 10}}`, exp{validationError: true}}, // error because VUs are more than iters
	{`{"ishared": {"executor": "shared-iterations", "iterations": 20, "vus": 10, "maxDuration": "30m"}}`, exp{}},
	{`{"ishared": {"executor": "shared-iterations", "iterations": 20, "vus": 10, "maxDuration": "-3m"}}`, exp{validationError: true}},
	{`{"ishared": {"executor": "shared-iterations", "iterations": 20, "vus": 10, "maxDuration": "0s"}}`, exp{validationError: true}},
	{`{"ishared": {"executor": "shared-iterations", "iterations": 20, "vus": -10}}`, exp{validationError: true}},
	{`{"ishared": {"executor": "shared-iterations", "iterations": -1, "vus": 1}}`, exp{validationError: true}},
	{`{"ishared": {"executor": "shared-iterations", "iterations": 20, "vus": 30}}`, exp{validationError: true}},
	// per-vu-iterations
	{
		`{"ipervu": {"executor": "per-vu-iterations", "iterations": 23, "vus": 13, "gracefulStop": 0}}`,
		exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
			sched := NewPerVUIterationsConfig("ipervu")
			sched.Iterations = null.IntFrom(23)
			sched.GracefulStop = types.NullDurationFrom(0)
			sched.VUs = null.IntFrom(13)

			assert.Empty(t, cm["ipervu"].Validate())
			assert.Empty(t, cm.Validate())

			et, err := lib.NewExecutionTuple(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, "23 iterations for each of 13 VUs (maxDuration: 10m0s)", cm["ipervu"].GetDescription(et))

			schedReqs := cm["ipervu"].GetExecutionRequirements(et)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 600*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(13), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(13), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(et)
			assert.Equal(t, schedReqs, totalReqs)
		}},
	},
	{`{"ipervu": {"executor": "per-vu-iterations"}}`, exp{}}, // Has 1 VU & 1 iter default values
	{`{"ipervu": {"executor": "per-vu-iterations", "iterations": 20}}`, exp{}},
	{`{"ipervu": {"executor": "per-vu-iterations", "vus": 10}}`, exp{}},
	{`{"ipervu": {"executor": "per-vu-iterations", "iterations": 20, "vus": 10}}`, exp{}},
	{`{"ipervu": {"executor": "per-vu-iterations", "iterations": 20, "vus": 10, "maxDuration": "-3m"}}`, exp{validationError: true}},
	{`{"ipervu": {"executor": "per-vu-iterations", "iterations": 20, "vus": 10, "maxDuration": "0s"}}`, exp{validationError: true}},
	{`{"ipervu": {"executor": "per-vu-iterations", "iterations": 20, "vus": -10}}`, exp{validationError: true}},
	{`{"ipervu": {"executor": "per-vu-iterations", "iterations": -1, "vus": 1}}`, exp{validationError: true}},

	// constant-arrival-rate
	{
		`{"carrival": {"executor": "constant-arrival-rate", "rate": 30, "timeUnit": "1m", "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`,
		exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
			et, err := lib.NewExecutionTuple(nil, nil)
			require.NoError(t, err)
			sched := NewConstantArrivalRateConfig("carrival")
			sched.Rate = null.IntFrom(30)
			sched.Duration = types.NullDurationFrom(10 * time.Minute)
			sched.TimeUnit = types.NullDurationFrom(1 * time.Minute)
			sched.PreAllocatedVUs = null.IntFrom(20)
			sched.MaxVUs = null.IntFrom(30)

			assert.Empty(t, cm["carrival"].Validate())
			assert.Empty(t, cm.Validate())

			assert.Equal(t, "0.50 iterations/s for 10m0s (maxVUs: 20-30, gracefulStop: 30s)", cm["carrival"].GetDescription(et))

			schedReqs := cm["carrival"].GetExecutionRequirements(et)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 630*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(20), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(30), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(et)
			assert.Equal(t, schedReqs, totalReqs)
		}},
	},
	{`{"carrival": {"executor": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`, exp{}},
	{`{"carrival": {"executor": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30, "timeUnit": "-1s"}}`, exp{validationError: true}},
	{
		`{"carrival": {"executor": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20}}`,
		exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
			assert.Empty(t, cm["carrival"].Validate())
			require.EqualValues(t, 20, cm["carrival"].(*ConstantArrivalRateConfig).MaxVUs.Int64)
		}},
	},
	{`{"carrival": {"executor": "constant-arrival-rate", "rate": 10, "duration": "10m", "maxVUs": 30}}`, exp{validationError: true}},
	{`{"carrival": {"executor": "constant-arrival-rate", "rate": 10, "preAllocatedVUs": 20, "maxVUs": 30}}`, exp{validationError: true}},
	{`{"carrival": {"executor": "constant-arrival-rate", "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`, exp{validationError: true}},
	{`{"carrival": {"executor": "constant-arrival-rate", "rate": 10, "duration": "0m", "preAllocatedVUs": 20, "maxVUs": 30}}`, exp{validationError: true}},
	{`{"carrival": {"executor": "constant-arrival-rate", "rate": 0, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`, exp{validationError: true}},
	{`{"carrival": {"executor": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 15}}`, exp{validationError: true}},
	{`{"carrival": {"executor": "constant-arrival-rate", "rate": 10, "duration": "0s", "preAllocatedVUs": 20, "maxVUs": 25}}`, exp{validationError: true}},
	{`{"carrival": {"executor": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": -2, "maxVUs": 25}}`, exp{validationError: true}},
	// ramping-arrival-rate
	{
		`{"varrival": {"executor": "ramping-arrival-rate", "startRate": 10, "timeUnit": "30s", "preAllocatedVUs": 20,
		"maxVUs": 50, "stages": [{"duration": "3m", "target": 30}, {"duration": "5m", "target": 10}]}}`,
		exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
			sched := NewRampingArrivalRateConfig("varrival")
			sched.StartRate = null.IntFrom(10)
			sched.Stages = []Stage{
				{Target: null.IntFrom(30), Duration: types.NullDurationFrom(180 * time.Second)},
				{Target: null.IntFrom(10), Duration: types.NullDurationFrom(300 * time.Second)},
			}
			sched.TimeUnit = types.NullDurationFrom(30 * time.Second)
			sched.PreAllocatedVUs = null.IntFrom(20)
			sched.MaxVUs = null.IntFrom(50)
			require.Equal(t, cm, lib.ScenarioConfigs{"varrival": sched})

			assert.Empty(t, cm["varrival"].Validate())
			assert.Empty(t, cm.Validate())

			et, err := lib.NewExecutionTuple(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, "Up to 1.00 iterations/s for 8m0s over 2 stages (maxVUs: 20-50, gracefulStop: 30s)", cm["varrival"].GetDescription(et))

			schedReqs := cm["varrival"].GetExecutionRequirements(et)
			endOffset, isFinal := lib.GetEndOffset(schedReqs)
			assert.Equal(t, 510*time.Second, endOffset)
			assert.Equal(t, true, isFinal)
			assert.Equal(t, uint64(20), lib.GetMaxPlannedVUs(schedReqs))
			assert.Equal(t, uint64(50), lib.GetMaxPossibleVUs(schedReqs))

			totalReqs := cm.GetFullExecutionRequirements(et)
			assert.Equal(t, schedReqs, totalReqs)
		}},
	},
	{`{"varrival": {"executor": "ramping-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, exp{}},
	{`{"varrival": {"executor": "ramping-arrival-rate", "preAllocatedVUs": -20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, exp{validationError: true}},
	{`{"varrival": {"executor": "ramping-arrival-rate", "startRate": -1, "preAllocatedVUs": 20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, exp{validationError: true}},
	{
		`{"varrival": {"executor": "ramping-arrival-rate", "preAllocatedVUs": 20, "stages": [{"duration": "5m", "target": 10}]}}`,
		exp{custom: func(t *testing.T, cm lib.ScenarioConfigs) {
			assert.Empty(t, cm["varrival"].Validate())
			require.EqualValues(t, 20, cm["varrival"].(*RampingArrivalRateConfig).MaxVUs.Int64)
		}},
	},
	{`{"varrival": {"executor": "ramping-arrival-rate", "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, exp{validationError: true}},
	{`{"varrival": {"executor": "ramping-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50}}`, exp{validationError: true}},
	{`{"varrival": {"executor": "ramping-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50, "stages": []}}`, exp{validationError: true}},
	{`{"varrival": {"executor": "ramping-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}], "timeUnit": "-1s"}}`, exp{validationError: true}},
	{`{"varrival": {"executor": "ramping-arrival-rate", "preAllocatedVUs": 30, "maxVUs": 20, "stages": [{"duration": "5m", "target": 10}]}}`, exp{validationError: true}},
	// TODO: more tests of mixed executors and execution plans
}

func TestConfigMapParsingAndValidation(t *testing.T) {
	t.Parallel()
	for i, tc := range configMapTestCases {
		tc := tc
		t.Run(fmt.Sprintf("TestCase#%d", i), func(t *testing.T) {
			t.Parallel()
			t.Logf(tc.rawJSON)
			var result lib.ScenarioConfigs
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
