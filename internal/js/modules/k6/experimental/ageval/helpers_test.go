package ageval

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

// testSetup wires a module instance into a VU-context runtime and exposes the
// module's exports (Agent, judge) as runtime globals so tests can drive them
// from JS, exactly as a real script would.
type testSetup struct {
	rt      *modulestest.Runtime
	mi      *ModuleInstance
	samples chan metrics.SampleContainer
}

func newTestSetup(t testing.TB) *testSetup {
	t.Helper()
	rt := modulestest.NewRuntime(t)

	// Build the instance while the init environment (and registry) is present.
	registry := rt.VU.InitEnvField.Registry
	mi, ok := New().NewModuleInstance(rt.VU).(*ModuleInstance)
	require.True(t, ok)

	for name, export := range mi.Exports().Named {
		require.NoError(t, rt.VU.Runtime().Set(name, export))
	}

	samples := make(chan metrics.SampleContainer, 1000)
	state := &lib.State{
		Options:        lib.Options{},
		BuiltinMetrics: rt.BuiltinMetrics,
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
		Samples:        samples,
	}
	rt.MoveToVUContext(state)

	return &testSetup{rt: rt, mi: mi, samples: samples}
}

// drainSamples reads all currently buffered samples and indexes their values by
// metric name.
func drainSamples(ch chan metrics.SampleContainer) map[string][]float64 {
	out := map[string][]float64{}
	for {
		select {
		case sc := <-ch:
			for _, s := range sc.GetSamples() {
				out[s.Metric.Name] = append(out[s.Metric.Name], s.Value)
			}
		default:
			return out
		}
	}
}

// cannedServer returns an httptest server that replies with the given response
// bodies in order, one per request. The last body repeats if exhausted.
func cannedServer(t testing.TB, bodies ...string) *httptest.Server {
	t.Helper()
	var i int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body := bodies[len(bodies)-1]
		if i < len(bodies) {
			body = bodies[i]
		}
		i++
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const (
	toolUseResponse = `{"content":[{"type":"text","text":"working"},` +
		`{"type":"tool_use","id":"tu_1","name":"echo","input":{"msg":"hi"}}],` +
		`"stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":5}}`
	endTurnResponse = `{"content":[{"type":"text","text":"all done, invoice paid"}],` +
		`"stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":6}}`
	judgeResponse = `{"content":[{"type":"text","text":"{\"score\": 0.9, \"reason\": \"good\"}"}],` +
		`"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":4}}`
)
