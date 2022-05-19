package tests

import (
	"context"
	"testing"

	"github.com/grafana/xk6-browser/common"

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

type mockVU struct {
	*k6modulestest.VU
	loop *k6eventloop.EventLoop
}

func newMockVU(tb testing.TB) *mockVU {
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
	mockVU := &mockVU{
		VU: &k6modulestest.VU{
			RuntimeField: rt,
			InitEnvField: &k6common.InitEnvironment{
				Registry: k6metrics.NewRegistry(),
			},
			StateField: state,
		},
	}
	ctx := context.Background()
	ctx = common.WithVU(ctx, mockVU)
	mockVU.CtxField = ctx

	loop := k6eventloop.New(mockVU)
	mockVU.RegisterCallbackField = loop.RegisterCallback
	mockVU.loop = loop

	return mockVU
}
