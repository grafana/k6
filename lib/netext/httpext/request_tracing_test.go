package httpext

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/internal/features"
	"go.k6.io/k6/v2/internal/lib/testutils"
	k6trace "go.k6.io/k6/v2/internal/lib/trace"
	"go.k6.io/k6/v2/internal/lib/trace/tracetest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

// tracingFixture makes requests with native tracing enabled, under a parent span.
type tracingFixture struct {
	state   *lib.State
	samples chan metrics.SampleContainer
	rec     *tracetest.Recorder
	ctx     context.Context
	parent  oteltrace.Span
}

func newTracingFixture(
	t *testing.T, srv *httptest.Server, enabled bool, opts ...sdktrace.TracerProviderOption,
) *tracingFixture {
	t.Helper()
	tp, rec := tracetest.NewProvider(opts...)
	registry := metrics.NewRegistry()
	samples := make(chan metrics.SampleContainer, 100)
	state := &lib.State{
		Options:        lib.Options{SystemTags: &metrics.DefaultSystemTagSet},
		Transport:      srv.Client().Transport,
		Samples:        samples,
		Logger:         logrus.New(),
		BufferPool:     lib.NewBufferPool(),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}
	ctx := t.Context()
	parent := oteltrace.SpanFromContext(ctx)
	if enabled {
		tracer := k6trace.Tracer(tp, &features.Flags{NativeTracing: true})
		state.Tracer = tracer
		ctx, parent = tracer.Start(ctx, "parent")
	}
	return &tracingFixture{state: state, samples: samples, rec: rec, ctx: ctx, parent: parent}
}

func (f *tracingFixture) get(t *testing.T, url string, mod func(*ParsedHTTPRequest)) *Response {
	t.Helper()
	res, err := f.do(t, url, mod)
	require.NoError(t, err)
	return res
}

func (f *tracingFixture) do(t *testing.T, url string, mod func(*ParsedHTTPRequest)) (*Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(f.ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	preq := &ParsedHTTPRequest{
		Req:              req,
		URL:              &URL{u: req.URL, URL: url},
		Body:             new(bytes.Buffer),
		Timeout:          5 * time.Second,
		Redirects:        null.IntFrom(10),
		ResponseCallback: func(status int) bool { return status < 400 },
		TagsAndMeta:      f.state.Tags.GetCurrentValues(),
	}
	if mod != nil {
		mod(preq)
	}
	return MakeRequest(f.ctx, f.state, preq)
}

// traceIDMetadata returns the trace ID metadata of the metric samples of the request.
func (f *tracingFixture) traceIDMetadata(t *testing.T) (string, bool) {
	t.Helper()
	require.Len(t, f.samples, 1)
	trail, ok := (<-f.samples).(*Trail)
	require.True(t, ok)
	traceID, ok := trail.Metadata["trace_id"]
	return traceID, ok
}

func spanAttr(s sdktrace.ReadOnlySpan, key string) (attribute.Value, bool) {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func TestMakeRequestTracing(t *testing.T) {
	t.Parallel()

	t.Run("span and propagation", func(t *testing.T) {
		t.Parallel()
		var mu sync.Mutex
		var traceparent string
		srv := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			traceparent = r.Header.Get("traceparent")
		}))
		t.Cleanup(srv.Close)
		f := newTracingFixture(t, srv, true)

		res := f.get(t, srv.URL+"/path/123?q=1", func(p *ParsedHTTPRequest) {
			p.TagsAndMeta.SetSystemTagOrMeta(metrics.TagName, "/path/:id")
		})
		f.parent.End()

		spans := f.rec.ByName("GET")
		require.Len(t, spans, 1)
		span := spans[0]
		assert.Equal(t, oteltrace.SpanKindClient, span.SpanKind())
		assert.Equal(t, f.parent.SpanContext().SpanID(), span.Parent().SpanID())
		assert.Equal(t, f.parent.SpanContext().TraceID(), span.SpanContext().TraceID())
		assert.Equal(t, codes.Unset, span.Status().Code)

		srvURL, err := url.Parse(srv.URL)
		require.NoError(t, err)
		wantPort, err := strconv.Atoi(srvURL.Port())
		require.NoError(t, err)
		status, _ := spanAttr(span, "http.response.status_code")
		assert.Equal(t, int64(res.Status), status.AsInt64())
		urlFull, _ := spanAttr(span, "url.full")
		assert.Equal(t, srv.URL+"/path/123?q=1", urlFull.AsString())
		name, _ := spanAttr(span, "k6.http.request.name")
		assert.Equal(t, "/path/:id", name.AsString())
		method, _ := spanAttr(span, "http.request.method")
		assert.Equal(t, "GET", method.AsString())
		address, _ := spanAttr(span, "server.address")
		assert.Equal(t, srvURL.Hostname(), address.AsString())
		port, _ := spanAttr(span, "server.port")
		assert.Equal(t, int64(wantPort), port.AsInt64())

		traceID, _ := f.traceIDMetadata(t)
		assert.Equal(t, span.SpanContext().TraceID().String(), traceID, "the samples link to the trace")

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t,
			"00-"+span.SpanContext().TraceID().String()+"-"+span.SpanContext().SpanID().String()+"-01",
			traceparent, "the server must be the child of the request span")
	})

	t.Run("redirects share the request span", func(t *testing.T) {
		t.Parallel()
		var mu sync.Mutex
		var traceparents []string
		mux := http.NewServeMux()
		mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			traceparents = append(traceparents, r.Header.Get("traceparent"))
			mu.Unlock()
			http.Redirect(w, r, "/end", http.StatusFound)
		})
		mux.HandleFunc("/end", func(_ http.ResponseWriter, r *http.Request) {
			mu.Lock()
			traceparents = append(traceparents, r.Header.Get("traceparent"))
			mu.Unlock()
		})
		srv := httptest.NewTLSServer(mux)
		t.Cleanup(srv.Close)
		f := newTracingFixture(t, srv, true)

		f.get(t, srv.URL+"/start", nil)

		spans := f.rec.ByName("GET")
		require.Len(t, spans, 1)
		sc := spans[0].SpanContext()
		want := "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() + "-01"
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, []string{want, want}, traceparents)
	})

	t.Run("unexpected response is an error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		t.Cleanup(srv.Close)
		f := newTracingFixture(t, srv, true)

		f.get(t, srv.URL, nil)
		f.get(t, srv.URL, func(p *ParsedHTTPRequest) { p.ResponseCallback = nil })
		f.get(t, srv.URL, func(p *ParsedHTTPRequest) {
			p.ResponseCallback = func(status int) bool { return status == http.StatusNotFound }
		})

		spans := f.rec.ByName("GET")
		require.Len(t, spans, 3)
		for i, name := range []string{"unexpected by the callback", "without callback"} {
			assert.Equal(t, sdktrace.Status{Code: codes.Error}, spans[i].Status(), name)
			errType, _ := spanAttr(spans[i], "error.type")
			assert.Equal(t, "404", errType.AsString(), name)
		}
		assert.Equal(t, codes.Unset, spans[2].Status().Code, "a response expected by the script isn't an error")
		_, hasType := spanAttr(spans[2], "error.type")
		assert.False(t, hasType)
	})

	t.Run("failed request", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		f := newTracingFixture(t, srv, true)
		addr := srv.URL
		srv.Close()

		f.get(t, addr, nil)
		_, err := f.do(t, addr, func(p *ParsedHTTPRequest) { p.Throw = true })
		require.Error(t, err)

		spans := f.rec.ByName("GET")
		require.Len(t, spans, 2)
		for i, name := range []string{"without throw", "with throw"} {
			assert.Equal(t, codes.Error, spans[i].Status().Code, name)
			_, hasStatus := spanAttr(spans[i], "http.response.status_code")
			assert.False(t, hasStatus, name)
			errType, _ := spanAttr(spans[i], "error.type")
			assert.Equal(t, strconv.Itoa(int(tcpDialRefusedErrorCode)), errType.AsString(), name)
		}
	})

	t.Run("http-debug dumps the sent trace context", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		t.Cleanup(srv.Close)
		f := newTracingFixture(t, srv, true)
		f.state.Options.HTTPDebug = null.StringFrom("headers")
		logHook := testutils.NewLogHook(logrus.InfoLevel)
		logger := logrus.New()
		logger.AddHook(logHook)
		logger.SetOutput(io.Discard)
		f.state.Logger = logger

		f.get(t, srv.URL, nil)

		spans := f.rec.ByName("GET")
		require.Len(t, spans, 1)
		assert.True(t, testutils.LogContains(logHook.Drain(), logrus.InfoLevel,
			"Traceparent: 00-"+spans[0].SpanContext().TraceID().String()))
	})

	t.Run("unsupported 101 response ends the span", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Add("Connection", "Upgrade")
			w.Header().Add("Upgrade", "h2c")
			w.WriteHeader(http.StatusSwitchingProtocols)
		}))
		t.Cleanup(srv.Close)
		f := newTracingFixture(t, srv, true)

		_, err := f.do(t, srv.URL, nil)
		require.Error(t, err)

		spans := f.rec.ByName("GET")
		require.Len(t, spans, 1)
		assert.Equal(t, codes.Error, spans[0].Status().Code)
		assert.Empty(t, f.samples, "no metrics are emitted for the request, as before")
	})

	t.Run("script traceparent is kept", func(t *testing.T) {
		t.Parallel()
		const own = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
		var mu sync.Mutex
		var got []string
		srv := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			got = r.Header.Values("traceparent")
		}))
		t.Cleanup(srv.Close)
		f := newTracingFixture(t, srv, true)

		f.get(t, srv.URL, func(p *ParsedHTTPRequest) { p.Req.Header.Set("traceparent", own) })

		_, hasTraceID := f.traceIDMetadata(t)
		assert.False(t, hasTraceID, "the samples aren't linked to a trace the server isn't part of")
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, []string{own}, got)
	})

	t.Run("not sampled", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		t.Cleanup(srv.Close)
		f := newTracingFixture(t, srv, true, sdktrace.WithSampler(sdktrace.NeverSample()))

		f.get(t, srv.URL, nil)

		_, hasTraceID := f.traceIDMetadata(t)
		assert.False(t, hasTraceID, "the samples aren't linked to a trace that isn't exported")
	})

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()
		var mu sync.Mutex
		var headers http.Header
		srv := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			headers = r.Header.Clone()
		}))
		t.Cleanup(srv.Close)
		f := newTracingFixture(t, srv, false)

		f.get(t, srv.URL, nil)

		assert.Empty(t, f.rec.Ended())
		_, hasTraceID := f.traceIDMetadata(t)
		assert.False(t, hasTraceID)
		mu.Lock()
		defer mu.Unlock()
		assert.Empty(t, headers.Values("traceparent"))
		assert.Empty(t, headers.Values("tracestate"))
	})
}

func TestServerPort(t *testing.T) {
	t.Parallel()

	for rawURL, want := range map[string]int{
		"https://example.com/":      443,
		"http://example.com/":       80,
		"https://example.com:8443/": 8443,
	} {
		u, err := url.Parse(rawURL)
		require.NoError(t, err)
		port, ok := serverPort(u)
		assert.True(t, ok, rawURL)
		assert.Equal(t, want, port, rawURL)
	}

	_, ok := serverPort(&url.URL{Scheme: "ftp", Host: "example.com"})
	assert.False(t, ok, "no default port for an unknown scheme")
}

func TestRedactURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ in, want string }{
		"no query":     {"https://a.com/p", "https://a.com/p"},
		"no sensitive": {"https://a.com/p?q=1&r=2", "https://a.com/p?q=1&r=2"},
		"aws": {
			"https://a.com/p?X-Amz-Signature=abc&q=1&X-Amz-Security-Token=tok&X-Amz-Credential=c",
			"https://a.com/p?X-Amz-Signature=REDACTED&q=1&X-Amz-Security-Token=REDACTED&X-Amz-Credential=REDACTED",
		},
		"sig and goog":   {"https://a.com/p?sig=x&X-Goog-Signature=y", "https://a.com/p?sig=REDACTED&X-Goog-Signature=REDACTED"},
		"case-sensitive": {"https://a.com/p?SIG=x", "https://a.com/p?SIG=x"},
		"fragment":       {"https://a.com/p?sig=x#f", "https://a.com/p?sig=REDACTED#f"},
		"encoded key":    {"https://a.com/p?X%2DAmz%2DSignature=x", "https://a.com/p?X%2DAmz%2DSignature=REDACTED"},
		"no value":       {"https://a.com/p?sig", "https://a.com/p?sig"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, redactURL(tc.in))
		})
	}
}
