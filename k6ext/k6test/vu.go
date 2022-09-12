package k6test

import (
	"testing"

	"github.com/grafana/xk6-browser/k6ext"

	k6common "go.k6.io/k6/js/common"
	k6eventloop "go.k6.io/k6/js/eventloop"
	k6modulestest "go.k6.io/k6/js/modulestest"
	k6lib "go.k6.io/k6/lib"
	k6testutils "go.k6.io/k6/lib/testutils"
	k6metrics "go.k6.io/k6/metrics"

	"github.com/dop251/goja"
	"github.com/oxtoacart/bpool"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

// VU is a k6 VU instance.
// TODO: Do we still need this VU wrapper?
// ToGojaValue can be a helper function that takes a goja.Runtime (although it's
// not much of a helper from calling ToValue(i) directly...), and we can access
// EventLoop from modulestest.Runtime.EventLoop. I guess we still need the
// RunLoop() override to call WaitOnRegistered()?
type VU struct {
	*k6modulestest.VU
	Loop      *k6eventloop.EventLoop
	toBeState *k6lib.State
}

// ToGojaValue is a convenience method for converting any value to a goja value.
func (v *VU) ToGojaValue(i interface{}) goja.Value { return v.Runtime().ToValue(i) }

// RunLoop is a convenience method for running fn in the event loop.
func (v *VU) RunLoop(fn func() error) error {
	v.Loop.WaitOnRegistered()
	return v.Loop.Start(fn)
}

// MoveToVUContext moves the VU to VU context, adding a predefined k6 lib State and nilling the InitEnv
// to simulate how that is done in the real k6.
func (v *VU) MoveToVUContext() {
	v.VU.StateField = v.toBeState
	v.VU.InitEnvField = nil
}

// NewVU returns a mock k6 VU.
func NewVU(tb testing.TB) *VU {
	tb.Helper()

	rt := goja.New()
	rt.SetFieldNameMapper(k6common.FieldNameMapper{})

	samples := make(chan k6metrics.SampleContainer, 1000)

	root, err := k6lib.NewGroup("", nil)
	require.NoError(tb, err)

	state := &k6lib.State{
		Options: k6lib.Options{
			MaxRedirects: null.IntFrom(10),
			UserAgent:    null.StringFrom("TestUserAgent"),
			Throw:        null.BoolFrom(true),
			SystemTags:   &k6metrics.DefaultSystemTagSet,
			Batch:        null.IntFrom(20),
			BatchPerHost: null.IntFrom(20),
			// HTTPDebug:    null.StringFrom("full"),
		},
		Logger:         k6testutils.NewLogger(tb),
		Group:          root,
		BPool:          bpool.NewBufferPool(1),
		Samples:        samples,
		Tags:           k6lib.NewTagMap(map[string]string{"group": root.Path}),
		BuiltinMetrics: k6metrics.RegisterBuiltinMetrics(k6metrics.NewRegistry()),
	}

	testRT := k6modulestest.NewRuntime(tb)
	ctx := k6ext.WithVU(testRT.VU.CtxField, testRT.VU)
	testRT.VU.CtxField = ctx

	return &VU{VU: testRT.VU, Loop: testRT.EventLoop, toBeState: state}
}
