package scheduler

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
)

type configMapTestCase struct {
	rawJSON               string
	expectParseError      bool
	expectValidationError bool
	customValidator       func(t *testing.T, cm ConfigMap)
}

var testCases = []configMapTestCase{
	{"", true, false, nil},
	{"1234", true, false, nil},
	{"asdf", true, false, nil},
	{"'adsf'", true, false, nil},
	{"[]", true, false, nil},
	{"{}", false, false, func(t *testing.T, cm ConfigMap) {
		assert.Equal(t, cm, ConfigMap{})
	}},
	{"{}asdf", true, false, nil},
	{"null", false, false, func(t *testing.T, cm ConfigMap) {
		assert.Nil(t, cm)
	}},
	{`{"someKey": {}}`, true, false, nil},
	{`{"someKey": {"type": "constant-blah-blah", "vus": 10, "duration": "60s"}}`, true, false, nil},
	{`{"someKey": {"type": "constant-looping-vus", "uknownField": "should_error"}}`, true, false, nil},
	{`{"someKey": {"type": "constant-looping-vus", "vus": 10, "duration": "60s", "env": 123}}`, true, false, nil},

	// Validation errors for constant-looping-vus and the base config
	{`{"someKey": {"type": "constant-looping-vus", "vus": 10, "duration": "60s", "interruptible": false,
		"iterationTimeout": "10s", "startTime": "70s", "env": {"test": "mest"}, "exec": "someFunc"}}`,
		false, false, func(t *testing.T, cm ConfigMap) {
			sched := NewConstantLoopingVUsConfig("someKey")
			sched.VUs = null.IntFrom(10)
			sched.Duration = types.NullDurationFrom(1 * time.Minute)
			sched.Interruptible = null.BoolFrom(false)
			sched.IterationTimeout = types.NullDurationFrom(10 * time.Second)
			sched.StartTime = types.NullDurationFrom(70 * time.Second)
			sched.Exec = null.StringFrom("someFunc")
			sched.Env = map[string]string{"test": "mest"}
			require.Equal(t, cm, ConfigMap{"someKey": sched})
			require.Equal(t, sched.BaseConfig, cm["someKey"].GetBaseConfig())
			assert.Equal(t, 70*time.Second, cm["someKey"].GetMaxDuration())
			assert.Equal(t, int64(10), cm["someKey"].GetMaxVUs())
			assert.Empty(t, cm["someKey"].Validate())
		}},
	{`{"": {"type": "constant-looping-vus", "vus": 10, "duration": "60s"}}`, false, true, nil},
	{`{"aname": {"type": "constant-looping-vus"}}`, false, true, nil},
	{`{"aname": {"type": "constant-looping-vus", "vus": 10}}`, false, true, nil},
	{`{"aname": {"type": "constant-looping-vus", "duration": "60s"}}`, false, true, nil},
	{`{"aname": {"type": "constant-looping-vus", "vus": -1, "duration": "60s"}}`, false, true, nil},
	{`{"aname": {"type": "constant-looping-vus", "vus": 10, "duration": "0s"}}`, false, true, nil},
	{`{"aname": {"type": "constant-looping-vus", "vus": 10, "duration": "10s", "startTime": "-10s"}}`, false, true, nil},
	{`{"aname": {"type": "constant-looping-vus", "vus": 10, "duration": "10s", "exec": ""}}`, false, true, nil},
	{`{"aname": {"type": "constant-looping-vus", "vus": 10, "duration": "10s", "iterationTimeout": "-2s"}}`, false, true, nil},

	// variable-looping-vus
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": 20, "iterationTimeout": "15s",
		"stages": [{"duration": "60s", "target": 30}, {"duration": "120s", "target": 10}]}}`,
		false, false, func(t *testing.T, cm ConfigMap) {
			sched := NewVariableLoopingVUsConfig("varloops")
			sched.IterationTimeout = types.NullDurationFrom(15 * time.Second)
			sched.StartVUs = null.IntFrom(20)
			sched.Stages = []Stage{
				{Target: null.IntFrom(30), Duration: types.NullDurationFrom(60 * time.Second)},
				{Target: null.IntFrom(10), Duration: types.NullDurationFrom(120 * time.Second)},
			}
			require.Equal(t, cm, ConfigMap{"varloops": sched})
			assert.Equal(t, int64(30), cm["varloops"].GetMaxVUs())
			assert.Equal(t, 195*time.Second, cm["varloops"].GetMaxDuration())
			assert.Empty(t, cm["varloops"].Validate())
		}},
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": 0, "stages": [{"duration": "60s", "target": 0}]}}`, false, false, nil},
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": -1, "stages": [{"duration": "60s", "target": 30}]}}`, false, true, nil},
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": 2, "stages": [{"duration": "-60s", "target": 30}]}}`, false, true, nil},
	{`{"varloops": {"type": "variable-looping-vus", "startVUs": 2, "stages": [{"duration": "60s", "target": -30}]}}`, false, true, nil},
	{`{"varloops": {"type": "variable-looping-vus", "stages": [{"duration": "60s"}]}}`, false, true, nil},
	{`{"varloops": {"type": "variable-looping-vus", "stages": [{"target": 30}]}}`, false, true, nil},
	{`{"varloops": {"type": "variable-looping-vus", "stages": []}}`, false, true, nil},
	{`{"varloops": {"type": "variable-looping-vus"}}`, false, true, nil},

	// shared-iterations
	{`{"ishared": {"type": "shared-iterations", "iterations": 20, "vus": 10}}`,
		false, false, func(t *testing.T, cm ConfigMap) {
			sched := NewSharedIterationsConfig("ishared")
			sched.Iterations = null.IntFrom(20)
			sched.VUs = null.IntFrom(10)
			require.Equal(t, cm, ConfigMap{"ishared": sched})
			assert.Equal(t, int64(10), cm["ishared"].GetMaxVUs())
			assert.Equal(t, 3630*time.Second, cm["ishared"].GetMaxDuration())
			assert.Empty(t, cm["ishared"].Validate())
		}},
	{`{"ishared": {"type": "shared-iterations", "iterations": 20, "vus": 10, "maxDuration": "30m"}}`, false, false, nil},
	{`{"ishared": {"type": "shared-iterations", "iterations": 20, "vus": 10, "maxDuration": "-3m"}}`, false, true, nil},
	{`{"ishared": {"type": "shared-iterations", "iterations": 20, "vus": 10, "maxDuration": "0s"}}`, false, true, nil},
	{`{"ishared": {"type": "shared-iterations", "iterations": 20, "vus": -10}}`, false, true, nil},
	{`{"ishared": {"type": "shared-iterations", "iterations": -1, "vus": 1}}`, false, true, nil},
	{`{"ishared": {"type": "shared-iterations", "iterations": 20, "vus": 30}}`, false, true, nil},

	// per-vu-iterations
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 20, "vus": 10}}`,
		false, false, func(t *testing.T, cm ConfigMap) {
			sched := NewPerVUIterationsConfig("ipervu")
			sched.Iterations = null.IntFrom(20)
			sched.VUs = null.IntFrom(10)
			require.Equal(t, cm, ConfigMap{"ipervu": sched})
			assert.Equal(t, int64(10), cm["ipervu"].GetMaxVUs())
			assert.Equal(t, 3630*time.Second, cm["ipervu"].GetMaxDuration())
			assert.Empty(t, cm["ipervu"].Validate())
		}},
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 20, "vus": 10}}`, false, false, nil},
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 20}}`, false, true, nil},
	{`{"ipervu": {"type": "per-vu-iterations", "vus": 10}}`, false, true, nil},
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 20, "vus": 10, "maxDuration": "-3m"}}`, false, true, nil},
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 20, "vus": 10, "maxDuration": "0s"}}`, false, true, nil},
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": 20, "vus": -10}}`, false, true, nil},
	{`{"ipervu": {"type": "per-vu-iterations", "iterations": -1, "vus": 1}}`, false, true, nil},

	// constant-arrival-rate
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "timeUnit": "1m", "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`,
		false, false, func(t *testing.T, cm ConfigMap) {
			sched := NewConstantArrivalRateConfig("carrival")
			sched.Rate = null.IntFrom(10)
			sched.Duration = types.NullDurationFrom(10 * time.Minute)
			sched.TimeUnit = types.NullDurationFrom(1 * time.Minute)
			sched.PreAllocatedVUs = null.IntFrom(20)
			sched.MaxVUs = null.IntFrom(30)
			require.Equal(t, cm, ConfigMap{"carrival": sched})
			assert.Equal(t, int64(30), cm["carrival"].GetMaxVUs())
			assert.Equal(t, 630*time.Second, cm["carrival"].GetMaxDuration())
			assert.Empty(t, cm["carrival"].Validate())
		}},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`, false, false, nil},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30, "timeUnit": "-1s"}}`, false, true, nil},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20}}`, false, true, nil},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "maxVUs": 30}}`, false, true, nil},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "preAllocatedVUs": 20, "maxVUs": 30}}`, false, true, nil},
	{`{"carrival": {"type": "constant-arrival-rate", "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`, false, true, nil},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "0m", "preAllocatedVUs": 20, "maxVUs": 30}}`, false, true, nil},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 0, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 30}}`, false, true, nil},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": 20, "maxVUs": 15}}`, false, true, nil},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "0s", "preAllocatedVUs": 20, "maxVUs": 25}}`, false, true, nil},
	{`{"carrival": {"type": "constant-arrival-rate", "rate": 10, "duration": "10m", "preAllocatedVUs": -2, "maxVUs": 25}}`, false, true, nil},

	// variable-arrival-rate
	{`{"varrival": {"type": "variable-arrival-rate", "startRate": 10, "timeUnit": "30s", "preAllocatedVUs": 20, "maxVUs": 50,
		"stages": [{"duration": "3m", "target": 30}, {"duration": "5m", "target": 10}]}}`,
		false, false, func(t *testing.T, cm ConfigMap) {
			sched := NewVariableArrivalRateConfig("varrival")
			sched.StartRate = null.IntFrom(10)
			sched.Stages = []Stage{
				{Target: null.IntFrom(30), Duration: types.NullDurationFrom(180 * time.Second)},
				{Target: null.IntFrom(10), Duration: types.NullDurationFrom(300 * time.Second)},
			}
			sched.TimeUnit = types.NullDurationFrom(30 * time.Second)
			sched.PreAllocatedVUs = null.IntFrom(20)
			sched.MaxVUs = null.IntFrom(50)
			require.Equal(t, cm, ConfigMap{"varrival": sched})
			assert.Equal(t, int64(50), cm["varrival"].GetMaxVUs())
			assert.Equal(t, 510*time.Second, cm["varrival"].GetMaxDuration())
			assert.Empty(t, cm["varrival"].Validate())
		}},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, false, false, nil},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": -20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, false, true, nil},
	{`{"varrival": {"type": "variable-arrival-rate", "startRate": -1, "preAllocatedVUs": 20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, false, true, nil},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 20, "stages": [{"duration": "5m", "target": 10}]}}`, false, true, nil},
	{`{"varrival": {"type": "variable-arrival-rate", "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}]}}`, false, true, nil},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50}}`, false, true, nil},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50, "stages": []}}`, false, true, nil},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 20, "maxVUs": 50, "stages": [{"duration": "5m", "target": 10}], "timeUnit": "-1s"}}`, false, true, nil},
	{`{"varrival": {"type": "variable-arrival-rate", "preAllocatedVUs": 30, "maxVUs": 20, "stages": [{"duration": "5m", "target": 10}]}}`, false, true, nil},
}

func TestConfigMapParsingAndValidation(t *testing.T) {
	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("TestCase#%d", i), func(t *testing.T) {
			var result ConfigMap
			err := json.Unmarshal([]byte(tc.rawJSON), &result)
			if tc.expectParseError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			validationErrors := result.Validate()
			if tc.expectValidationError {
				assert.NotEmpty(t, validationErrors)
			} else {
				assert.Empty(t, validationErrors)
			}
			if tc.customValidator != nil {
				tc.customValidator(t, result)
			}
		})
	}
}

//TODO: check percentage split calculations
