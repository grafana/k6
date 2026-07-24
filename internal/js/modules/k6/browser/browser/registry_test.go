package browser

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"

	k6event "go.k6.io/k6/v2/internal/event"
)

func TestPidRegistry(t *testing.T) {
	t.Parallel()

	p := &pidRegistry{}

	var wg sync.WaitGroup
	iteration := 100
	expected := make([]int, 0, iteration)
	wg.Add(iteration)
	for i := range iteration {
		go func(i int) {
			p.registerPid(i)
			wg.Done()
		}(i)
		expected = append(expected, i)
	}

	wg.Wait()

	got := p.Pids()

	assert.ElementsMatch(t, expected, got)
}

func TestIsRemoteBrowser(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                    string
		envVarName, envVarValue string
		expIsRemote             bool
		expValidWSURLs          []string
		expErr                  error
	}{
		{
			name:        "browser is not remote",
			envVarName:  "FOO",
			envVarValue: "BAR",
			expIsRemote: false,
		},
		{
			name:           "single WS URL",
			envVarName:     env.WebSocketURLs,
			envVarValue:    "WS_URL",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL"},
		},
		{
			name:           "multiple WS URL",
			envVarName:     env.WebSocketURLs,
			envVarValue:    "WS_URL_1,WS_URL_2,WS_URL_3",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2", "WS_URL_3"},
		},
		{
			name:           "ending comma is handled",
			envVarName:     env.WebSocketURLs,
			envVarValue:    "WS_URL_1,WS_URL_2,",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2"},
		},
		{
			name:           "void string does not panic",
			envVarName:     env.WebSocketURLs,
			envVarValue:    "",
			expIsRemote:    true,
			expValidWSURLs: []string{""},
		},
		{
			name:           "comma does not panic",
			envVarName:     env.WebSocketURLs,
			envVarValue:    ",",
			expIsRemote:    true,
			expValidWSURLs: []string{""},
		},
		{
			name:           "read a single scenario with a single ws url",
			envVarName:     env.InstanceScenarios,
			envVarValue:    `[{"id": "one","browsers": [{ "handle": "WS_URL_1" }]}]`,
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1"},
		},
		{
			name:           "read a single scenario with a two ws urls",
			envVarName:     env.InstanceScenarios,
			envVarValue:    `[{"id": "one","browsers": [{"handle": "WS_URL_1"}, {"handle": "WS_URL_2"}]}]`,
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2"},
		},
		{
			name:       "read two scenarios with multiple ws urls",
			envVarName: env.InstanceScenarios,
			envVarValue: `[
				{"id": "one","browsers": [{"handle": "WS_URL_1"}, {"handle": "WS_URL_2"}]},
				{"id": "two","browsers": [{"handle": "WS_URL_3"}, {"handle": "WS_URL_4"}]}
			]`,
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2", "WS_URL_3", "WS_URL_4"},
		},
		{
			name:           "read scenarios without any ws urls",
			envVarName:     env.InstanceScenarios,
			envVarValue:    `[{"id": "one","browsers": [{}]}]`,
			expIsRemote:    false,
			expValidWSURLs: []string{""},
		},
		{
			name:           "read scenarios without any browser objects",
			envVarName:     env.InstanceScenarios,
			envVarValue:    `[{"id": "one"}]`,
			expIsRemote:    false,
			expValidWSURLs: []string{""},
		},
		{
			name:        "read empty scenarios",
			envVarName:  env.InstanceScenarios,
			envVarValue: ``,
			expErr:      errors.New("parsing K6_INSTANCE_SCENARIOS: unexpected end of JSON input"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lookup := func(key string) (string, bool) {
				v := tc.envVarValue
				if tc.envVarName == "K6_INSTANCE_SCENARIOS" {
					v = strconv.Quote(v)
				}
				if key == tc.envVarName {
					return v, true
				}
				return "", false
			}

			rr, err := newRemoteRegistry(lookup)
			if tc.expErr != nil {
				assert.Error(t, tc.expErr, err)
				return
			}
			assert.NoError(t, err)

			wsURL, isRemote := rr.isRemoteBrowser()
			require.Equal(t, tc.expIsRemote, isRemote)
			if isRemote {
				require.Contains(t, tc.expValidWSURLs, wsURL)
			}
		})
	}

	t.Run("K6_INSTANCE_SCENARIOS should override K6_BROWSER_WS_URL", func(t *testing.T) {
		t.Parallel()

		lookup := func(key string) (string, bool) {
			switch key {
			case env.WebSocketURLs:
				return "WS_URL_1", true
			case env.InstanceScenarios:
				return strconv.Quote(`[{"id": "one","browsers": [{ "handle": "WS_URL_2" }]}]`), true
			default:
				return "", false
			}
		}

		rr, err := newRemoteRegistry(lookup)
		assert.NoError(t, err)

		wsURL, isRemote := rr.isRemoteBrowser()

		require.Equal(t, true, isRemote)
		require.Equal(t, "WS_URL_2", wsURL)
	})
}

func TestBrowserRegistry(t *testing.T) {
	t.Parallel()

	remoteRegistry, err := newRemoteRegistry(func(key string) (string, bool) {
		// No env vars
		return "", false
	})
	require.NoError(t, err)

	t.Run("init_and_close_browsers_on_iter_events", func(t *testing.T) {
		t.Parallel()

		var (
			vu              = k6test.NewVU(t)
			browserRegistry = newBrowserRegistry(context.Background(), vu, remoteRegistry, &pidRegistry{}, nil)
		)

		vu.ActivateVU()

		// Send a few IterStart events
		vu.StartIteration(t, k6test.WithIteration(0))
		vu.StartIteration(t, k6test.WithIteration(1))
		vu.StartIteration(t, k6test.WithIteration(2))

		// Verify browsers are initialized
		assert.Equal(t, 3, browserRegistry.browserCount())

		// Verify iteration traces are started
		assert.Equal(t, 3, browserRegistry.tr.iterationTracesCount())

		// Send IterEnd events
		vu.EndIteration(t, k6test.WithIteration(0))
		vu.EndIteration(t, k6test.WithIteration(1))
		vu.EndIteration(t, k6test.WithIteration(2))

		// Verify there are no browsers left
		assert.Equal(t, 0, browserRegistry.browserCount())

		// Verify iteration traces have been ended
		assert.Equal(t, 0, browserRegistry.tr.iterationTracesCount())
	})

	t.Run("close_browsers_on_exit_event", func(t *testing.T) {
		t.Parallel()

		var (
			vu              = k6test.NewVU(t)
			browserRegistry = newBrowserRegistry(context.Background(), vu, remoteRegistry, &pidRegistry{}, nil)
		)

		vu.ActivateVU()

		// Send a few IterStart events
		vu.StartIteration(t, k6test.WithIteration(0))
		vu.StartIteration(t, k6test.WithIteration(1))
		vu.StartIteration(t, k6test.WithIteration(2))

		// Verify browsers are initialized
		assert.Equal(t, 3, browserRegistry.browserCount())

		// Send Exit event
		events, ok := vu.EventsField.Global.(*k6event.System)
		require.True(t, ok, "want *k6event.System; got %T", events)
		waitDone := events.Emit(&k6event.Event{
			Type: k6event.Exit,
		})
		require.NoError(t, waitDone(context.Background()), "error waiting on Exit done")

		// Verify there are no browsers left
		assert.Equal(t, 0, browserRegistry.browserCount())
	})

	t.Run("skip_on_non_browser_vu", func(t *testing.T) {
		t.Parallel()

		var (
			vu              = k6test.NewVU(t)
			browserRegistry = newBrowserRegistry(context.Background(), vu, remoteRegistry, &pidRegistry{}, nil)
		)

		vu.ActivateVU()

		// Unset browser type option in scenario options in order to represent that VU is not
		// a browser test VU
		delete(vu.StateField.Options.Scenarios["default"].GetScenarioOptions().Browser, "type")

		vu.StartIteration(t, k6test.WithIteration(0))

		// Verify there are no browsers
		assert.Equal(t, 0, browserRegistry.browserCount())
	})

	// This test ensures that the chromium browser's lifecycle is not controlled
	// by the vu context.
	t.Run("dont_close_browser_on_vu_context_close", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		var cancel context.CancelFunc
		vu.CtxField, cancel = context.WithCancel(vu.CtxField)
		browserRegistry := newBrowserRegistry(context.Background(), vu, remoteRegistry, &pidRegistry{}, nil)

		vu.ActivateVU()

		// Send a few IterStart events
		vu.StartIteration(t, k6test.WithIteration(0))

		// Verify browsers are initialized
		assert.Equal(t, 1, browserRegistry.browserCount())

		// Cancel the "iteration" by closing the context.
		cancel()

		// Verify browsers are still alive
		assert.Equal(t, 1, browserRegistry.browserCount())

		// Do cleanup by sending the Exit event
		events, ok := vu.EventsField.Global.(*k6event.System)
		require.True(t, ok, "want *k6event.System; got %T", events)
		waitDone := events.Emit(&k6event.Event{
			Type: k6event.Exit,
		})
		require.NoError(t, waitDone(context.Background()), "error waiting on Exit done")

		// Verify there are no browsers left
		assert.Equal(t, 0, browserRegistry.browserCount())
	})

	// IterEnd must be a no-op for a non-browser iteration that never called
	// chromium.connectOverCDP: nothing was built (IterStart is skipped), nothing
	// is tracked in userManaged, and the traces registry was never initialized.
	// This locks in the `if r.tr != nil` guard and the unconditional
	// closeUserManaged sweep on IterEnd — without them, IterEnd would panic here.
	t.Run("iterend_noop_on_bare_non_browser_iter", func(t *testing.T) {
		t.Parallel()

		var (
			vu              = k6test.NewVU(t)
			browserRegistry = newBrowserRegistry(context.Background(), vu, remoteRegistry, &pidRegistry{}, nil)
		)

		vu.ActivateVU()

		// Unset the browser type option so this represents a non-browser VU.
		delete(vu.StateField.Options.Scenarios["default"].GetScenarioOptions().Browser, "type")

		vu.StartIteration(t, k6test.WithIteration(0))

		// Nothing built and no traces registry: IterStart is skipped entirely for
		// a non-browser iter, and connectOverCDP (which would init the registry)
		// was never called.
		require.Equal(t, 0, browserRegistry.browserCount())
		require.Nil(t, browserRegistry.tr, "traces registry must not be initialized for a bare non-browser iter")

		// The regression under test: IterEnd reaches closeUserManaged (nothing
		// tracked) and the trace guard (r.tr is nil). Neither may panic.
		require.NotPanics(t, func() {
			vu.EndIteration(t, k6test.WithIteration(0))
		})

		require.Equal(t, 0, browserRegistry.browserCount())
		require.Nil(t, browserRegistry.tr, "traces registry must remain uninitialized after IterEnd")
	})
}

func TestStartConnectTraceAttributes(t *testing.T) {
	t.Parallel()

	rec := &traceRecorder{}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))

	vu := k6test.NewVU(t, k6test.WithTracerProvider(tp))
	vu.ActivateVU()
	vu.StartIteration(t)
	vu.State().VUID = 42 // non-zero so the test.vu assertion is meaningful

	r := &browserRegistry{
		vu:          vu,
		m:           make(map[int64]*common.Browser),
		userManaged: make(map[int64][]*common.Browser),
	}
	r.startConnectTrace(vu.Context(), vu.State().Iteration)

	span, ok := rec.find("iteration")
	require.True(t, ok, "expected an 'iteration' root span")
	require.Equal(t, int64(42), spanAttrInt64(t, span, "test.vu"))
	require.Equal(t, "default", spanAttrString(t, span, "test.scenario"))
}

// traceRecorder is a minimal sdktrace.SpanProcessor that captures started spans
// so tests can inspect their names and attributes.
type traceRecorder struct {
	mu    sync.Mutex
	spans []recordedSpan
}

type recordedSpan struct {
	name  string
	attrs []attribute.KeyValue
}

func (r *traceRecorder) OnStart(_ context.Context, s sdktrace.ReadWriteSpan) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spans = append(r.spans, recordedSpan{name: s.Name(), attrs: s.Attributes()})
}

func (r *traceRecorder) OnEnd(sdktrace.ReadOnlySpan)      {}
func (r *traceRecorder) Shutdown(context.Context) error   { return nil }
func (r *traceRecorder) ForceFlush(context.Context) error { return nil }

func (r *traceRecorder) find(name string) (recordedSpan, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.spans {
		if s.name == name {
			return s, true
		}
	}
	return recordedSpan{}, false
}

func spanAttrInt64(t *testing.T, s recordedSpan, key string) int64 {
	t.Helper()
	for _, kv := range s.attrs {
		if string(kv.Key) == key {
			return kv.Value.AsInt64()
		}
	}
	t.Fatalf("attribute %q not found on span %q", key, s.name)
	return 0
}

func spanAttrString(t *testing.T, s recordedSpan, key string) string {
	t.Helper()
	for _, kv := range s.attrs {
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}
	t.Fatalf("attribute %q not found on span %q", key, s.name)
	return ""
}

func TestParseTracesMetadata(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		env         map[string]string
		expMetadata map[string]string
		expErrMssg  string
	}{
		{
			name:        "no metadata",
			env:         make(map[string]string),
			expMetadata: make(map[string]string),
		},
		{
			name: "one metadata field",
			env: map[string]string{
				"K6_BROWSER_TRACES_METADATA": "meta=value",
			},
			expMetadata: map[string]string{
				"meta": "value",
			},
		},
		{
			name: "one metadata field finishing in comma",
			env: map[string]string{
				"K6_BROWSER_TRACES_METADATA": "meta=value,",
			},
			expMetadata: map[string]string{
				"meta": "value",
			},
		},
		{
			name: "multiple metadata fields",
			env: map[string]string{
				"K6_BROWSER_TRACES_METADATA": "meta1=value1,meta2=value2",
			},
			expMetadata: map[string]string{
				"meta1": "value1",
				"meta2": "value2",
			},
		},
		{
			name: "multiple metadata fields finishing in comma",
			env: map[string]string{
				"K6_BROWSER_TRACES_METADATA": "meta1=value1,meta2=value2,",
			},
			expMetadata: map[string]string{
				"meta1": "value1",
				"meta2": "value2",
			},
		},
		{
			name: "invalid metadata",
			env: map[string]string{
				"K6_BROWSER_TRACES_METADATA": "thisIsInvalid",
			},
			expErrMssg: "is not a valid key=value metadata",
		},
		{
			name: "invalid metadata void",
			env: map[string]string{
				"K6_BROWSER_TRACES_METADATA": "",
			},
			expErrMssg: "is not a valid key=value metadata",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lookup := func(key string) (string, bool) {
				v, ok := tc.env[key]
				return v, ok
			}
			metadata, err := parseTracesMetadata(lookup)
			if err != nil {
				assert.ErrorContains(t, err, tc.expErrMssg)
				return
			}
			assert.Equal(t, tc.expMetadata, metadata)
		})
	}
}
