package k6test

import (
	"testing"

	"github.com/grafana/xk6-browser/k6ext"

	k6eventloop "go.k6.io/k6/js/eventloop"
	k6modulestest "go.k6.io/k6/js/modulestest"
	k6lib "go.k6.io/k6/lib"
	k6testutils "go.k6.io/k6/lib/testutils"
	k6metrics "go.k6.io/k6/metrics"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

// VU is a k6 VU instance.
// TODO: Do we still need this VU wrapper?
// ToGojaValue can be a helper function that takes a goja.Runtime (although it's
// not much of a helper from calling ToValue(i) directly...), and we can access
// EventLoop from modulestest.Runtime.EventLoop.
type VU struct {
	*k6modulestest.VU
	Loop      *k6eventloop.EventLoop
	toBeState *k6lib.State
	samples   chan k6metrics.SampleContainer
}

// ToGojaValue is a convenience method for converting any value to a goja value.
func (v *VU) ToGojaValue(i any) goja.Value { return v.Runtime().ToValue(i) }

// MoveToVUContext moves the VU to VU context, adding a predefined k6 lib State and nilling the InitEnv
// to simulate how that is done in the real k6.
func (v *VU) MoveToVUContext() {
	v.VU.StateField = v.toBeState
	v.VU.InitEnvField = nil
}

// AssertSamples asserts each sample VU received since AssertSamples
// is last called, then it returns the number of received samples.
func (v *VU) AssertSamples(assertSample func(s k6metrics.Sample)) int {
	var n int
	for _, bs := range k6metrics.GetBufferedSamples(v.samples) {
		for _, s := range bs.GetSamples() {
			assertSample(s)
			n++
		}
	}
	return n
}

// WithSamplesListener is used to indicate we want to use a bidirectional channel
// so that the test can read the metrics being emitted to the channel.
type WithSamplesListener chan k6metrics.SampleContainer

// NewVU returns a mock k6 VU.
func NewVU(tb testing.TB, opts ...any) *VU {
	tb.Helper()

	samples := make(chan k6metrics.SampleContainer, 1000)
	for _, opt := range opts {
		switch opt := opt.(type) { //nolint:gocritic
		case WithSamplesListener:
			samples = opt
		}
	}

	root, err := k6lib.NewGroup("", nil)
	require.NoError(tb, err)
	testRT := k6modulestest.NewRuntime(tb)
	tags := testRT.VU.InitEnvField.Registry.RootTagSet()

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
		BufferPool:     k6lib.NewBufferPool(),
		Samples:        samples,
		Tags:           k6lib.NewVUStateTags(tags.With("group", root.Path)),
		BuiltinMetrics: k6metrics.RegisterBuiltinMetrics(k6metrics.NewRegistry()),
	}

	ctx := k6ext.WithVU(testRT.VU.CtxField, testRT.VU)
	testRT.VU.CtxField = ctx

	return &VU{VU: testRT.VU, Loop: testRT.EventLoop, toBeState: state, samples: samples}
}
