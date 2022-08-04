package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/metrics"
	"go.k6.io/k6/ui/pb"
)

// TODO: remove globals and use some type of explicit dependency injection?
//nolint:gochecknoglobals
var (
	executorConfigTypesMutex   sync.RWMutex
	executorConfigConstructors = make(map[string]ExecutorConfigConstructor)
)

// ExecutionStep is used by different executors to specify the planned number of
// VUs they will need at a particular time. The times are relative to their
// StartTime, i.e. they don't take into account the specific starting time of
// the executor, as that will be considered by the external execution executor
// separately.
//
// A slice [{t1, v1}, {t2, v2}, {t3, v3}, ..., {tn, vn}] of execution steps
// means that an executor will need 0 VUs until t1, it will need v1 number of
// VUs from time t1 until t2, need v2 number of VUs from time t2 to t3, and so
// on. t1 is usually 0, tn is usually the same as GetMaxDuration() and vn is
// usually 0.
//
// Keep in mind that t(i) may be exactly equal to t(i+i), when there's an abrupt
// transition in the number of VUs required by an executor. For example, the
// ramping-vus executor may have 0-duration stages, or it may scale up
// VUs in its last stage right until the end. These immediate transitions cannot
// be ignored, since the gracefulStop/gracefulRampDown options potentially allow
// any started iterations to finish.
//
// []ExecutionStep is also used by the ScenarioConfigs, to represent the
// amount of needed VUs among all executors, during the whole execution of a
// test script. In that context, each executor's StartTime is accounted for and
// included in the offsets.
type ExecutionStep struct {
	TimeOffset      time.Duration
	PlannedVUs      uint64
	MaxUnplannedVUs uint64
}

// TODO: make []ExecutionStep or []ExecutorConfig their own type?

// ExecutorConfig is an interface that should be implemented by all executor config types
type ExecutorConfig interface {
	Validate() []error

	GetName() string
	GetType() string
	GetStartTime() time.Duration
	GetGracefulStop() time.Duration

	// This is used to validate whether a particular script can run in the cloud
	// or, in the future, in the native k6 distributed execution. Currently only
	// the externally-controlled executor should return false.
	IsDistributable() bool

	GetEnv() map[string]string
	// Allows us to get the non-default function the executor should run, if it
	// has been specified.
	//
	// TODO: use interface{} so plain http requests can be specified?
	GetExec() string
	GetTags() map[string]string

	// Calculates the VU requirements in different stages of the executor's
	// execution, including any extensions caused by waiting for iterations to
	// finish with graceful stops or ramp-downs.
	GetExecutionRequirements(*ExecutionTuple) []ExecutionStep

	// Return a human-readable description of the executor
	GetDescription(*ExecutionTuple) string

	NewExecutor(*ExecutionState, *logrus.Entry) (Executor, error)

	// HasWork reports whether there is any work for the executor to do with a given segment.
	HasWork(*ExecutionTuple) bool
}

// ScenarioState holds runtime scenario information returned by the k6/execution
// JS module.
type ScenarioState struct {
	Name, Executor string
	StartTime      time.Time
	ProgressFn     func() (float64, []string)
}

// InitVUFunc is just a shorthand so we don't have to type the function
// signature every time.
type InitVUFunc func(context.Context, *logrus.Entry) (InitializedVU, error)

// Executor is the interface all executors should implement
type Executor interface {
	GetConfig() ExecutorConfig
	GetProgress() *pb.ProgressBar
	GetLogger() *logrus.Entry

	Init(ctx context.Context) error
	Run(ctx context.Context, engineOut chan<- metrics.SampleContainer) error
}

// PausableExecutor should be implemented by the executors that can be paused
// and resumed in the middle of the test execution. Currently, only the
// externally controlled executor implements it.
type PausableExecutor interface {
	SetPaused(bool) error
}

// LiveUpdatableExecutor should be implemented for the executors whose
// configuration can be modified in the middle of the test execution. Currently,
// only the manual execution executor implements it.
type LiveUpdatableExecutor interface {
	UpdateConfig(ctx context.Context, newConfig interface{}) error
}

// ExecutorConfigConstructor is a simple function that returns a concrete
// Config instance with the specified name and all default values correctly
// initialized
type ExecutorConfigConstructor func(name string, rawJSON []byte) (ExecutorConfig, error)

// RegisterExecutorConfigType adds the supplied ExecutorConfigConstructor as
// the constructor for its type in the configConstructors map, in a thread-safe
// manner
func RegisterExecutorConfigType(configType string, constructor ExecutorConfigConstructor) {
	executorConfigTypesMutex.Lock()
	defer executorConfigTypesMutex.Unlock()

	if constructor == nil {
		panic("executor configs: constructor is nil")
	}
	if _, configTypeExists := executorConfigConstructors[configType]; configTypeExists {
		panic("executor configs: lib.RegisterExecutorConfigType called twice for  " + configType)
	}

	executorConfigConstructors[configType] = constructor
}

// ScenarioConfigs can contain mixed executor config types
type ScenarioConfigs map[string]ExecutorConfig

// UnmarshalJSON implements the json.Unmarshaler interface in a two-step manner,
// creating the correct type of configs based on the `type` property.
func (scs *ScenarioConfigs) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if len(data) == 4 && string(data) == "null" {
		return nil
	}

	// TODO: use a more sophisticated combination of dec.Token() and dec.More(),
	// which would allow us to support both arrays and maps for this config?
	var protoConfigs map[string]protoExecutorConfig
	if err := StrictJSONUnmarshal(data, &protoConfigs); err != nil {
		return err
	}

	result := make(ScenarioConfigs, len(protoConfigs))
	for k, v := range protoConfigs {
		if v.executorType == "" {
			return fmt.Errorf("scenario '%s' doesn't have a specified executor type", k)
		}
		config, err := GetParsedExecutorConfig(k, v.executorType, v.rawJSON)
		if err != nil {
			return err
		}
		result[k] = config
	}

	*scs = result

	return nil
}

// Validate checks if all of the specified executor options make sense
func (scs ScenarioConfigs) Validate() (errors []error) {
	for name, exec := range scs {
		if execErr := exec.Validate(); len(execErr) != 0 {
			errors = append(errors,
				fmt.Errorf("scenario %s has configuration errors: %s", name, ConcatErrors(execErr, ", ")))
		}
	}
	return errors
}

// GetSortedConfigs returns a slice with the executor configurations,
// sorted in a consistent and predictable manner. It is useful when we want or
// have to avoid using maps with string keys (and tons of string lookups in
// them) and avoid the unpredictable iterations over Go maps. Slices allow us
// constant-time lookups and ordered iterations.
//
// The configs in the returned slice will be sorted by their start times in an
// ascending order, and alphabetically by their names (which are unique) if
// there are ties.
func (scs ScenarioConfigs) GetSortedConfigs() []ExecutorConfig {
	configs := make([]ExecutorConfig, len(scs))

	// Populate the configs slice with sorted executor configs
	i := 0
	for _, config := range scs {
		configs[i] = config // populate the slice in an unordered manner
		i++
	}
	sort.Slice(configs, func(a, b int) bool { // sort by (start time, name)
		switch {
		case configs[a].GetStartTime() < configs[b].GetStartTime():
			return true
		case configs[a].GetStartTime() == configs[b].GetStartTime():
			return strings.Compare(configs[a].GetName(), configs[b].GetName()) < 0
		default:
			return false
		}
	})

	return configs
}

// GetFullExecutionRequirements combines the execution requirements from all of
// the configured executors. It takes into account their start times and their
// individual VU requirements and calculates the total VU requirements for each
// moment in the test execution.
func (scs ScenarioConfigs) GetFullExecutionRequirements(et *ExecutionTuple) []ExecutionStep {
	sortedConfigs := scs.GetSortedConfigs()

	// Combine the steps and requirements from all different executors, and
	// sort them by their time offset, counting the executors' startTimes as
	// well.
	type trackedStep struct {
		ExecutionStep
		configID int
	}
	trackedSteps := []trackedStep{}
	for configID, config := range sortedConfigs { // orderly iteration over a slice
		configStartTime := config.GetStartTime()
		configSteps := config.GetExecutionRequirements(et)
		for _, cs := range configSteps {
			cs.TimeOffset += configStartTime // add the executor start time to the step time offset
			trackedSteps = append(trackedSteps, trackedStep{cs, configID})
		}
	}
	// Sort by (time offset, config id). It's important that we use stable
	// sorting algorithm, since there could be steps with the same time from
	// the same executor and their order is important.
	sort.SliceStable(trackedSteps, func(a, b int) bool {
		if trackedSteps[a].TimeOffset == trackedSteps[b].TimeOffset {
			return trackedSteps[a].configID < trackedSteps[b].configID
		}

		return trackedSteps[a].TimeOffset < trackedSteps[b].TimeOffset
	})

	// Go through all of the sorted steps from all of the executors, and
	// build a new list of execution steps that consolidates all of their
	// requirements. If multiple executors have an execution step at exactly
	// the same time offset, they will be combined into a single new execution
	// step with the sum of the values from the previous ones.
	currentTimeOffset := time.Duration(0)
	currentPlannedVUs := make([]uint64, len(scs))
	currentMaxUnplannedVUs := make([]uint64, len(scs))
	sum := func(data []uint64) (result uint64) { // sigh...
		for _, val := range data {
			result += val
		}
		return result
	}
	consolidatedSteps := []ExecutionStep{}
	addCurrentStepIfDifferent := func() {
		newPlannedVUs := sum(currentPlannedVUs)
		newMaxUnplannedVUs := sum(currentMaxUnplannedVUs)
		stepsLen := len(consolidatedSteps)
		if stepsLen == 0 ||
			consolidatedSteps[stepsLen-1].PlannedVUs != newPlannedVUs ||
			consolidatedSteps[stepsLen-1].MaxUnplannedVUs != newMaxUnplannedVUs {
			consolidatedSteps = append(consolidatedSteps, ExecutionStep{
				TimeOffset:      currentTimeOffset,
				PlannedVUs:      newPlannedVUs,
				MaxUnplannedVUs: newMaxUnplannedVUs,
			})
		}
	}
	for _, step := range trackedSteps {
		// TODO: optimize by skipping some steps
		// If the time offset is different, create a new step with the current values

		currentTimeOffset = step.TimeOffset
		currentPlannedVUs[step.configID] = step.PlannedVUs
		currentMaxUnplannedVUs[step.configID] = step.MaxUnplannedVUs
		addCurrentStepIfDifferent()
	}
	return consolidatedSteps
}

// GetParsedExecutorConfig returns a struct instance corresponding to the supplied
// config type. It will be fully initialized - with both the default values of
// the type, as well as with whatever the user had specified in the JSON
func GetParsedExecutorConfig(name, configType string, rawJSON []byte) (result ExecutorConfig, err error) {
	executorConfigTypesMutex.Lock()
	defer executorConfigTypesMutex.Unlock()

	constructor, exists := executorConfigConstructors[configType]
	if !exists {
		return nil, fmt.Errorf("unknown executor type '%s'", configType)
	}
	return constructor(name, rawJSON)
}

type protoExecutorConfig struct {
	executorType string
	rawJSON      json.RawMessage
}

// UnmarshalJSON unmarshals the base config (to get the type), but it also
// stores the unprocessed JSON so we can parse the full config in the next step
func (pc *protoExecutorConfig) UnmarshalJSON(b []byte) error {
	var tmp struct {
		ExecutorType string `json:"executor"`
	}
	err := json.Unmarshal(b, &tmp)
	*pc = protoExecutorConfig{tmp.ExecutorType, b}
	return err
}
