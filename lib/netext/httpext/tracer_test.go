package httpext

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mccutchen/go-httpbin/httpbin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

const traceDelay = 100 * time.Millisecond

func getTestTracer(t *testing.T) (*Tracer, *httptrace.ClientTrace) {
	tracer := &Tracer{}
	ct := tracer.Trace()
	if runtime.GOOS == "windows" {
		// HACK: Time resolution is not as accurate on Windows, see:
		//  https://github.com/golang/go/issues/8687
		//  https://github.com/golang/go/issues/41087
		// Which seems to be causing some metrics to have a value of 0,
		// since e.g. ConnectStart and ConnectDone could register the same time.
		// So we force delays in the ClientTrace event handlers
		// to hopefully reduce the chances of this happening.
		ct = &httptrace.ClientTrace{
			ConnectStart: func(a, n string) {
				t.Logf("called ConnectStart at\t\t%v\n", now())
				time.Sleep(traceDelay)
				tracer.ConnectStart(a, n)
			},
			ConnectDone: func(a, n string, e error) {
				t.Logf("called ConnectDone at\t\t%v\n", now())
				time.Sleep(traceDelay)
				tracer.ConnectDone(a, n, e)
			},
			GetConn: func(h string) {
				t.Logf("called GetConn at\t\t%v\n", now())
				time.Sleep(traceDelay)
				tracer.GetConn(h)
			},
			GotConn: func(i httptrace.GotConnInfo) {
				t.Logf("called GotConn at\t\t%v\n", now())
				time.Sleep(traceDelay)
				tracer.GotConn(i)
			},
			TLSHandshakeStart: func() {
				t.Logf("called TLSHandshakeStart at\t\t%v\n", now())
				time.Sleep(traceDelay)
				tracer.TLSHandshakeStart()
			},
			TLSHandshakeDone: func(s tls.ConnectionState, e error) {
				t.Logf("called TLSHandshakeDone at\t\t%v\n", now())
				time.Sleep(traceDelay)
				tracer.TLSHandshakeDone(s, e)
			},
			WroteRequest: func(i httptrace.WroteRequestInfo) {
				t.Logf("called WroteRequest at\t\t%v\n", now())
				time.Sleep(traceDelay)
				tracer.WroteRequest(i)
			},
			GotFirstResponseByte: func() {
				t.Logf("called GotFirstResponseByte at\t%v\n", now())
				time.Sleep(traceDelay)
				tracer.GotFirstResponseByte()
			},
		}
	}

	return tracer, ct
}

func TestTracer(t *testing.T) { //nolint:tparallel
	t.Parallel()
	srv := httptest.NewTLSServer(httpbin.New().Handler())
	defer srv.Close()

	transport, ok := srv.Client().Transport.(*http.Transport)
	assert.True(t, ok)
	transport.DialContext = netext.NewDialer(
		net.Dialer{},
		netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4),
	).DialContext

	var prev int64
	assertLaterOrZero := func(t *testing.T, val int64, canBeZero bool) {
		if canBeZero && val == 0 {
			return
		}
		if prev > val {
			_, file, line, _ := runtime.Caller(1)
			t.Errorf("Expected %d to be greater or equal to %d (from %s:%d)", val, prev, file, line)
			return
		}
		prev = val
	}
	builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())

	for tnum, isReuse := range []bool{false, true, true} { //nolint:paralleltest
		t.Run(fmt.Sprintf("Test #%d", tnum), func(t *testing.T) {
			// Do not enable parallel testing, test relies on sequential execution
			req, err := http.NewRequest("GET", srv.URL+"/get", nil)
			require.NoError(t, err)

			tracer, ct := getTestTracer(t)
			res, err := transport.RoundTrip(req.WithContext(httptrace.WithClientTrace(context.Background(), ct)))
			require.NoError(t, err)

			_, err = io.Copy(ioutil.Discard, res.Body)
			assert.NoError(t, err)
			assert.NoError(t, res.Body.Close())
			if runtime.GOOS == "windows" {
				time.Sleep(traceDelay)
			}
			trail := tracer.Done()
			trail.SaveSamples(builtinMetrics, metrics.IntoSampleTags(&map[string]string{"tag": "value"}))
			samples := trail.GetSamples()

			assertLaterOrZero(t, tracer.getConn, isReuse)
			assertLaterOrZero(t, tracer.connectStart, isReuse)
			assertLaterOrZero(t, tracer.connectDone, isReuse)
			assertLaterOrZero(t, tracer.tlsHandshakeStart, isReuse)
			assertLaterOrZero(t, tracer.tlsHandshakeDone, isReuse)
			assertLaterOrZero(t, tracer.gotConn, false)
			assertLaterOrZero(t, tracer.wroteRequest, false)
			assertLaterOrZero(t, tracer.gotFirstResponseByte, false)
			assertLaterOrZero(t, now(), false)

			assert.Equal(t, strings.TrimPrefix(srv.URL, "https://"), trail.ConnRemoteAddr.String())

			assert.Len(t, samples, 8)
			seenMetrics := map[*metrics.Metric]bool{}
			for i, s := range samples {
				assert.NotContains(t, seenMetrics, s.Metric)
				seenMetrics[s.Metric] = true

				assert.False(t, s.Time.IsZero())
				assert.Equal(t, map[string]string{"tag": "value"}, s.Tags.CloneTags())

				switch s.Metric {
				case builtinMetrics.HTTPReqs:
					assert.Equal(t, 1.0, s.Value)
					assert.Equal(t, 0, i, "`HTTPReqs` is reported before the other HTTP builtinMetrics")
				case builtinMetrics.HTTPReqConnecting, builtinMetrics.HTTPReqTLSHandshaking:
					if isReuse {
						assert.Equal(t, 0.0, s.Value)
						break
					}
					fallthrough
				case builtinMetrics.HTTPReqDuration, builtinMetrics.HTTPReqBlocked, builtinMetrics.HTTPReqSending, builtinMetrics.HTTPReqWaiting, builtinMetrics.HTTPReqReceiving:
					assert.True(t, s.Value > 0.0, "%s is <= 0", s.Metric.Name)
				default:
					t.Errorf("unexpected metric: %s", s.Metric.Name)
				}
			}
		})
	}
}

type failingConn struct {
	net.Conn
}

var failOnConnWrite = false

func (c failingConn) Write(b []byte) (int, error) {
	if failOnConnWrite {
		failOnConnWrite = false
		return 0, errors.New("write error")
	}

	return c.Conn.Write(b)
}

func TestTracerNegativeHttpSendingValues(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(httpbin.New().Handler())
	defer srv.Close()

	transport, ok := srv.Client().Transport.(*http.Transport)
	assert.True(t, ok)

	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, proto, addr string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, proto, addr)
		return failingConn{conn}, err
	}

	req, err := http.NewRequest("GET", srv.URL+"/get", nil)
	require.NoError(t, err)

	{
		tracer := &Tracer{}
		res, err := transport.RoundTrip(req.WithContext(httptrace.WithClientTrace(context.Background(), tracer.Trace())))
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, res.Body)
		assert.NoError(t, err)
		assert.NoError(t, res.Body.Close())
		tracer.Done()
	}

	// make the next connection write fail
	failOnConnWrite = true

	{
		tracer := &Tracer{}
		res, err := transport.RoundTrip(req.WithContext(httptrace.WithClientTrace(context.Background(), tracer.Trace())))
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, res.Body)
		assert.NoError(t, err)
		assert.NoError(t, res.Body.Close())
		trail := tracer.Done()
		builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())
		trail.SaveSamples(builtinMetrics, nil)

		require.True(t, trail.Sending > 0)
	}
}

func TestTracerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(httpbin.New().Handler())
	defer srv.Close()

	tracer := &Tracer{}
	req, err := http.NewRequest("GET", srv.URL+"/get", nil)
	require.NoError(t, err)

	_, err = http.DefaultTransport.RoundTrip(
		req.WithContext(
			httptrace.WithClientTrace(
				context.Background(),
				tracer.Trace())))

	assert.Error(t, err)
}

func TestCancelledRequest(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(httpbin.New().Handler())
	t.Cleanup(srv.Close)

	cancelTest := func(t *testing.T) {
		tracer := &Tracer{}
		req, err := http.NewRequestWithContext(context.Background(), "GET", srv.URL+"/delay/1", nil)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(httptrace.WithClientTrace(req.Context(), tracer.Trace()))
		req = req.WithContext(ctx)
		go func() {
			time.Sleep(time.Duration(rand.Int31n(50)) * time.Millisecond) //nolint:gosec
			cancel()
		}()

		resp, err := srv.Client().Transport.RoundTrip(req) //nolint:bodyclose
		_ = tracer.Done()
		if resp == nil && err == nil {
			t.Errorf("Expected either a RoundTrip response or error but got %#v and %#v", resp, err)
		}
	}

	// This Run will not return until the parallel subtests complete.
	t.Run("group", func(t *testing.T) {
		t.Parallel()
		for i := 0; i < 200; i++ {
			t.Run(fmt.Sprintf("TestCancelledRequest_%d", i),
				func(t *testing.T) {
					t.Parallel()
					cancelTest(t)
				})
		}
	})
}
