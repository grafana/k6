package remotewrite

import (
	"context"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

func BenchmarkCompileTemplatesSimple(b *testing.B) {
	for range b.N {
		_, err := compileTemplate("something ${series_id} else")
		require.NoError(b, err)
	}
}

func BenchmarkCompileTemplatesComplex(b *testing.B) {
	for range b.N {
		_, err := compileTemplate("something ${series_id/1000} else")
		require.NoError(b, err)
	}
}

func BenchmarkEvaluateTemplatesSimple(b *testing.B) {
	t, err := compileTemplate("something ${series_id} else")
	require.NoError(b, err)
	b.ResetTimer()

	var buf []byte

	for i := range b.N {
		buf = t.AppendByte(buf[:0], i)
	}
}

func BenchmarkEvaluateTemplatesComplex(b *testing.B) {
	t, err := compileTemplate("something ${series_id/1000} else")
	require.NoError(b, err)
	b.ResetTimer()

	var buf []byte

	for i := range b.N {
		buf = t.AppendByte(buf[:0], i)
	}
}

//nolint:gochecknoglobals // benchmark test constants
var benchmarkLabels = map[string]string{
	"__name__":        "k6_generated_metric_${series_id/1000}",
	"series_id":       "${series_id}",
	"cardinality_1e1": "${series_id/10}",
	"cardinality_1e2": "${series_id/100}",
	"cardinality_1e3": "${series_id/1000}",
	"cardinality_1e4": "${series_id/10000}",
	"cardinality_1e5": "${series_id/100000}",
	"cardinality_1e6": "${series_id/1000000}",
	"cardinality_1e7": "${series_id/10000000}",
	"cardinality_2":   "${series_id%2}",
	"cardinality_50":  "${series_id%50}",
}

type testServer struct {
	server *httptest.Server
	vu     *modulestest.VU
	count  *int64
}

func newTestServer(tb testing.TB) *testServer {
	tb.Helper()

	ts := &testServer{
		count: new(int64),
	}

	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)

		w.WriteHeader(http.StatusOK)
		atomic.AddInt64(ts.count, 1)
	}))
	registry := metrics.NewRegistry()
	ch := make(chan metrics.SampleContainer)

	tb.Cleanup(func() {
		ts.server.Close()
		close(ch) // this might need to be elsewhere
	})

	ts.vu = new(modulestest.VU)
	ts.vu.CtxField = context.Background()

	ts.vu.StateField = new(lib.State)
	ts.vu.StateField.Transport = ts.server.Client().Transport
	ts.vu.StateField.BufferPool = lib.NewBufferPool()
	ts.vu.StateField.Samples = ch
	ts.vu.StateField.BuiltinMetrics = metrics.RegisterBuiltinMetrics(registry)
	ts.vu.StateField.Tags = lib.NewVUStateTags(registry.RootTagSet())

	go func() {
		for range ch { //nolint:revive // we just need to drain the channel
		}
	}()

	return ts
}

func BenchmarkStoreFromPrecompiledTemplates(b *testing.B) {
	s := newTestServer(b)
	c := &Client{
		cfg: &Config{
			Url:     s.server.URL,
			Timeout: "100s",
		},
		vu: s.vu,
	}
	template, err := compileLabelTemplates(benchmarkLabels)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		_, err := c.StoreFromPrecompiledTemplates(i, i+10, int64(i), 0, 100000, template)
		require.NoError(b, err)
	}

	require.LessOrEqual(b, int64(1), *s.count) // this might need an atomic
}

func BenchmarkStoreFromTemplates(b *testing.B) {
	s := newTestServer(b)
	c := &Client{
		cfg: &Config{
			Url:     s.server.URL,
			Timeout: "100s",
		},
		vu: s.vu,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		_, err := c.StoreFromTemplates(i, i+10, int64(i), 0, 100000, benchmarkLabels)
		require.NoError(b, err)
	}

	require.LessOrEqual(b, int64(1), *s.count) // this might need an atomic
}

func BenchmarkGenerateFromPrecompiledTemplates(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		// #nosec G404 -- This is test data generation for load testing, not cryptographic use
		r := rand.New(rand.NewSource(time.Now().Unix()))
		i := 0
		template, err := compileLabelTemplates(benchmarkLabels)
		require.NoError(b, err)

		for pb.Next() {
			i++
			_, _ = generateFromPrecompiledTemplates(r, i, i+10, int64(i), 0, 100000, template)
		}
	})
}
