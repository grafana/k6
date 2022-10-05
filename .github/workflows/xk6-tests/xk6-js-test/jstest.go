package jstest

import (
	"fmt"
	"time"

	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

func init() {
	modules.Register("k6/x/jsexttest", New())
}

type (
	RootModule struct{}

	// JSTest is meant to test xk6 and the JS extension sub-system of k6.
	JSTest struct {
		vu modules.VU

		foos *metrics.Metric
	}
)

// Ensure the interfaces are implemented correctly.
var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &JSTest{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface and returns
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &JSTest{
		vu:   vu,
		foos: vu.InitEnv().Registry.MustNewMetric("foos", metrics.Counter),
	}
}

// Exports implements the modules.Instance interface and returns the exports
// of the JS module.
func (j *JSTest) Exports() modules.Exports {
	return modules.Exports{Default: j}
}

// Foo emits a foo metric
func (j *JSTest) Foo(arg float64) (bool, error) {
	state := j.vu.State()
	if state == nil {
		return false, fmt.Errorf("the VU State is not available in the init context")
	}

	ctx := j.vu.Context()

	tags := state.Tags.GetCurrentValues().Tags.With("foo", "bar")
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		Time:       time.Now(),
		TimeSeries: metrics.TimeSeries{Metric: j.foos, Tags: tags},
		Value:      arg,
	})

	return true, nil
}
