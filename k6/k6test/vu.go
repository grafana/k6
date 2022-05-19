package k6test

import (
	"context"
	"testing"

	"github.com/grafana/xk6-browser/k6"

	k6common "go.k6.io/k6/js/common"
	k6eventloop "go.k6.io/k6/js/eventloop"
	k6modulestest "go.k6.io/k6/js/modulestest"
	k6lib "go.k6.io/k6/lib"
	k6metrics "go.k6.io/k6/metrics"

	"github.com/dop251/goja"
	"github.com/oxtoacart/bpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

// VU is a k6 VU instance.
type VU struct {
	*k6modulestest.VU
	Loop *k6eventloop.EventLoop
}

// ToGojaValue is a convenient method for converting any value to a goja value.
func (v *VU) ToGojaValue(i interface{}) goja.Value { return v.Runtime().ToValue(i) }

// NewVU returns a mock VU.
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
		Logger:         logrus.StandardLogger(),
		Group:          root,
		BPool:          bpool.NewBufferPool(1),
		Samples:        samples,
		Tags:           k6lib.NewTagMap(map[string]string{"group": root.Path}),
		BuiltinMetrics: k6metrics.RegisterBuiltinMetrics(k6metrics.NewRegistry()),
	}
	vu := &VU{
		VU: &k6modulestest.VU{
			RuntimeField: rt,
			InitEnvField: &k6common.InitEnvironment{
				Registry: k6metrics.NewRegistry(),
			},
			StateField: state,
		},
	}
	ctx := k6.WithVU(context.Background(), vu)
	vu.CtxField = ctx

	loop := k6eventloop.New(vu)
	vu.RegisterCallbackField = loop.RegisterCallback
	vu.Loop = loop

	return vu
}
