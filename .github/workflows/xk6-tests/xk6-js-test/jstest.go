package jstest

import (
	"fmt"
	"time"

	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/stats"
)

func init() {
	modules.Register("k6/x/jsexttest", New())
}

type (
	RootModule struct{}

	// JSTest is meant to test xk6 and the JS extension sub-system of k6.
	JSTest struct {
		vu modules.VU
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
	return &JSTest{vu: vu}
}

// Exports implements the modules.Instance interface and returns the exports
// of the JS module.
func (j *JSTest) Exports() modules.Exports {
	return modules.Exports{Default: j}
}

// Foo emits a foo metric
func (j JSTest) Foo(arg float64) (bool, error) {
	state := j.vu.State()
	if state == nil {
		return false, fmt.Errorf("the VU State is not avaialble in the init context")
	}

	ctx := j.vu.Context()

	allTheFoos := stats.New("foos", stats.Counter)
	tags := state.CloneTags()
	tags["foo"] = "bar"
	stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
		Time:   time.Now(),
		Metric: allTheFoos, Tags: stats.IntoSampleTags(&tags),
		Value: arg,
	})

	return true, nil
}
