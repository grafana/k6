package webcrypto

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/eventloop"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"
)

// testSetup is a helper struct holding components
// necessary to test the redis client, in the context
// of the execution of a k6 script.
type testSetup struct {
	rt      *goja.Runtime
	state   *lib.State
	samples chan metrics.SampleContainer
	ev      *eventloop.EventLoop
}

// newTestSetup initializes a new test setup.
// It prepares a test setup with a mocked redis server and a goja runtime,
// and event loop, ready to execute scripts as if being executed in the
// main context of k6.
func newTestSetup(t testing.TB) testSetup {
	tb := httpmultibin.NewHTTPMultiBin(t)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	root, err := lib.NewGroup("", nil)
	require.NoError(t, err)

	samples := make(chan metrics.SampleContainer, 1000)

	state := &lib.State{
		Group:  root,
		Dialer: tb.Dialer,
		Options: lib.Options{
			SystemTags: metrics.NewSystemTagSet(
				metrics.TagURL,
				metrics.TagProto,
				metrics.TagStatus,
				metrics.TagSubproto,
			),
			UserAgent: null.StringFrom("TestUserAgent"),
		},
		Samples:        samples,
		TLSConfig:      tb.TLSClientConfig,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(metrics.NewRegistry()),
		Tags:           lib.NewTagMap(nil),
	}

	vu := &modulestest.VU{
		CtxField:     tb.Context,
		InitEnvField: &common.InitEnvironment{},
		RuntimeField: rt,
		StateField:   state,
	}

	m := new(RootModule).NewModuleInstance(vu)
	require.NoError(t, rt.Set("crypto", m.Exports().Named["crypto"]))

	ev := eventloop.New(vu)
	vu.RegisterCallbackField = ev.RegisterCallback

	return testSetup{
		rt:      rt,
		state:   state,
		samples: samples,
		ev:      ev,
	}
}

// newInitContextTestSetup initializes a new test setup.
// It prepares a test setup with a mocked redis server and a goja runtime,
// and event loop, ready to execute scripts as if being executed in the
// main context of k6.
func newInitContextTestSetup(t testing.TB) testSetup {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	samples := make(chan metrics.SampleContainer, 1000)

	var state *lib.State

	vu := &modulestest.VU{
		CtxField:     context.Background(),
		InitEnvField: &common.InitEnvironment{},
		RuntimeField: rt,
		StateField:   state,
	}

	m := new(RootModule).NewModuleInstance(vu)
	require.NoError(t, rt.Set("crypto", m.Exports().Named["crypto"]))

	ev := eventloop.New(vu)
	vu.RegisterCallbackField = ev.RegisterCallback

	return testSetup{
		rt:      rt,
		state:   state,
		samples: samples,
		ev:      ev,
	}
}
