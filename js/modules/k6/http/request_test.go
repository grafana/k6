package http

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/dop251/goja"
	"github.com/klauspost/compress/zstd"
	"github.com/mccutchen/go-httpbin/httpbin"
	"github.com/oxtoacart/bpool"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/metrics"
)

// TODO replace this with the Single version
func assertRequestMetricsEmitted(t *testing.T, sampleContainers []metrics.SampleContainer, method, url, name string, status int, group string) {
	if name == "" {
		name = url
	}

	seenDuration := false
	seenBlocked := false
	seenConnecting := false
	seenTLSHandshaking := false
	seenSending := false
	seenWaiting := false
	seenReceiving := false
	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			tags := sample.Tags.CloneTags()
			if tags["url"] == url {
				switch sample.Metric.Name {
				case metrics.HTTPReqDurationName:
					seenDuration = true
				case metrics.HTTPReqBlockedName:
					seenBlocked = true
				case metrics.HTTPReqConnectingName:
					seenConnecting = true
				case metrics.HTTPReqTLSHandshakingName:
					seenTLSHandshaking = true
				case metrics.HTTPReqSendingName:
					seenSending = true
				case metrics.HTTPReqWaitingName:
					seenWaiting = true
				case metrics.HTTPReqReceivingName:
					seenReceiving = true
				}

				assert.Equal(t, strconv.Itoa(status), tags["status"])
				assert.Equal(t, method, tags["method"])
				assert.Equal(t, group, tags["group"])
				assert.Equal(t, name, tags["name"])
			}
		}
	}
	assert.True(t, seenDuration, "url %s didn't emit Duration", url)
	assert.True(t, seenBlocked, "url %s didn't emit Blocked", url)
	assert.True(t, seenConnecting, "url %s didn't emit Connecting", url)
	assert.True(t, seenTLSHandshaking, "url %s didn't emit TLSHandshaking", url)
	assert.True(t, seenSending, "url %s didn't emit Sending", url)
	assert.True(t, seenWaiting, "url %s didn't emit Waiting", url)
	assert.True(t, seenReceiving, "url %s didn't emit Receiving", url)
}

func assertRequestMetricsEmittedSingle(t *testing.T, sampleContainer metrics.SampleContainer, expectedTags map[string]string, metrics []string, callback func(sample metrics.Sample)) {
	t.Helper()

	metricMap := make(map[string]bool, len(metrics))
	for _, m := range metrics {
		metricMap[m] = false
	}
	for _, sample := range sampleContainer.GetSamples() {
		tags := sample.Tags.CloneTags()
		v, ok := metricMap[sample.Metric.Name]
		assert.True(t, ok, "unexpected metric %s", sample.Metric.Name)
		assert.False(t, v, "second metric %s", sample.Metric.Name)
		metricMap[sample.Metric.Name] = true
		assert.EqualValues(t, expectedTags, tags, "%s", tags)
		if callback != nil {
			callback(sample)
		}
	}
	for k, v := range metricMap {
		assert.True(t, v, "didn't emit %s", k)
	}
}

func newRuntime(t testing.TB) (
	*httpmultibin.HTTPMultiBin, *lib.State, chan metrics.SampleContainer, *goja.Runtime, *ModuleInstance,
) {
	tb := httpmultibin.NewHTTPMultiBin(t)

	root, err := lib.NewGroup("", nil)
	require.NoError(t, err)
	registry := metrics.NewRegistry()

	logger := logrus.New()
	logger.Level = logrus.DebugLevel

	options := lib.Options{
		MaxRedirects: null.IntFrom(10),
		UserAgent:    null.StringFrom("TestUserAgent"),
		Throw:        null.BoolFrom(true),
		SystemTags:   &metrics.DefaultSystemTagSet,
		Batch:        null.IntFrom(20),
		BatchPerHost: null.IntFrom(20),
		// HTTPDebug:    null.StringFrom("full"),
	}
	samples := make(chan metrics.SampleContainer, 1000)

	state := &lib.State{
		Options:   options,
		Logger:    logger,
		Group:     root,
		TLSConfig: tb.TLSClientConfig,
		Transport: tb.HTTPTransport,
		BPool:     bpool.NewBufferPool(1),
		Samples:   samples,
		Tags: lib.NewTagMap(map[string]string{
			"group": root.Path,
		}),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
	}

	rt, mi, mockVU := getTestModuleInstance(t)
	mockVU.InitEnvField = nil
	mockVU.StateField = state
	return tb, state, samples, rt, mi
}

func TestRequestAndBatch(t *testing.T) {
	t.Parallel()
	tb, state, samples, rt, _ := newRuntime(t)
	sr := tb.Replacer.Replace

	// Handle paths with custom logic
	tb.Mux.HandleFunc("/digest-auth/failure", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))

	t.Run("Redirects", func(t *testing.T) {
		t.Run("tracing", func(t *testing.T) {
			_, err := rt.RunString(sr(`
			var res = http.get("HTTPBIN_URL/redirect/9");
			`))
			assert.NoError(t, err)
			bufSamples := metrics.GetBufferedSamples(samples)

			reqsCount := 0
			for _, container := range bufSamples {
				for _, sample := range container.GetSamples() {
					if sample.Metric.Name == "http_reqs" {
						reqsCount++
					}
				}
			}

			assert.Equal(t, 10, reqsCount)
			assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/redirect/9"), sr("HTTPBIN_URL/redirect/9"), 302, "")
			for i := 8; i > 0; i-- {
				url := sr(fmt.Sprintf("HTTPBIN_URL/relative-redirect/%d", i))
				assertRequestMetricsEmitted(t, bufSamples, "GET", url, url, 302, "")
			}
			assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get"), sr("HTTPBIN_URL/get"), 200, "")
		})

		t.Run("10", func(t *testing.T) {
			_, err := rt.RunString(sr(`http.get("HTTPBIN_URL/redirect/10")`))
			assert.NoError(t, err)
		})
		t.Run("11", func(t *testing.T) {
			_, err := rt.RunString(sr(`
			var res = http.get("HTTPBIN_URL/redirect/11");
			if (res.status != 302) { throw new Error("wrong status: " + res.status) }
			if (res.url != "HTTPBIN_URL/relative-redirect/1") { throw new Error("incorrect URL: " + res.url) }
			if (res.headers["Location"] != "/get") { throw new Error("incorrect Location header: " + res.headers["Location"]) }
			`))
			assert.NoError(t, err)

			t.Run("Unset Max", func(t *testing.T) {
				hook := logtest.NewLocal(state.Logger)
				defer hook.Reset()

				oldOpts := state.Options
				defer func() { state.Options = oldOpts }()
				state.Options.MaxRedirects = null.NewInt(10, false)

				_, err := rt.RunString(sr(`
				var res = http.get("HTTPBIN_URL/redirect/11");
				if (res.status != 302) { throw new Error("wrong status: " + res.status) }
				if (res.url != "HTTPBIN_URL/relative-redirect/1") { throw new Error("incorrect URL: " + res.url) }
				if (res.headers["Location"] != "/get") { throw new Error("incorrect Location header: " + res.headers["Location"]) }
				`))
				assert.NoError(t, err)

				logEntry := hook.LastEntry()
				if assert.NotNil(t, logEntry) {
					assert.Equal(t, logrus.WarnLevel, logEntry.Level)
					assert.Equal(t, sr("HTTPBIN_URL/redirect/11"), logEntry.Data["url"])
					assert.Equal(t, "Stopped after 11 redirects and returned the redirection; pass { redirects: n } in request params or set global maxRedirects to silence this", logEntry.Message)
				}
			})
		})
		t.Run("requestScopeRedirects", func(t *testing.T) {
			_, err := rt.RunString(sr(`
			var res = http.get("HTTPBIN_URL/redirect/1", {redirects: 3});
			if (res.status != 200) { throw new Error("wrong status: " + res.status) }
			if (res.url != "HTTPBIN_URL/get") { throw new Error("incorrect URL: " + res.url) }
			`))
			assert.NoError(t, err)
		})
		t.Run("requestScopeNoRedirects", func(t *testing.T) {
			_, err := rt.RunString(sr(`
			var res = http.get("HTTPBIN_URL/redirect/1", {redirects: 0});
			if (res.status != 302) { throw new Error("wrong status: " + res.status) }
			if (res.url != "HTTPBIN_URL/redirect/1") { throw new Error("incorrect URL: " + res.url) }
			if (res.headers["Location"] != "/get") { throw new Error("incorrect Location header: " + res.headers["Location"]) }
			`))
			assert.NoError(t, err)
		})

		t.Run("post body", func(t *testing.T) {
			tb.Mux.HandleFunc("/post-redirect", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, r.Method, "POST")
				_, _ = io.Copy(ioutil.Discard, r.Body)
				http.Redirect(w, r, sr("HTTPBIN_URL/post"), http.StatusPermanentRedirect)
			}))
			_, err := rt.RunString(sr(`
			var res = http.post("HTTPBIN_URL/post-redirect", "pesho", {redirects: 1});

			if (res.status != 200) { throw new Error("wrong status: " + res.status) }
			if (res.url != "HTTPBIN_URL/post") { throw new Error("incorrect URL: " + res.url) }
			if (res.json().data != "pesho") { throw new Error("incorrect data : " + res.json().data) }
			`))
			assert.NoError(t, err)
		})
	})
	t.Run("Timeout", func(t *testing.T) {
		t.Run("10s", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				http.get("HTTPBIN_URL/delay/1", {
					timeout: 5*1000,
				})
			`))
			assert.NoError(t, err)
		})
		t.Run("10s", func(t *testing.T) {
			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			startTime := time.Now()
			_, err := rt.RunString(sr(`
				http.get("HTTPBIN_URL/delay/10", {
					timeout: 1*1000,
				})
			`))
			endTime := time.Now()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "request timeout")
			assert.WithinDuration(t, startTime.Add(1*time.Second), endTime, 2*time.Second)

			logEntry := hook.LastEntry()
			assert.Nil(t, logEntry)
		})
		t.Run("10s", func(t *testing.T) {
			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			startTime := time.Now()
			_, err := rt.RunString(sr(`
				http.get("HTTPBIN_URL/delay/10", {
					timeout: "1s",
				})
			`))
			endTime := time.Now()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "request timeout")
			assert.WithinDuration(t, startTime.Add(1*time.Second), endTime, 2*time.Second)

			logEntry := hook.LastEntry()
			assert.Nil(t, logEntry)
		})
	})
	t.Run("UserAgent", func(t *testing.T) {
		_, err := rt.RunString(sr(`
			var res = http.get("HTTPBIN_URL/headers");
			var headers = res.json()["headers"];
			if (headers['User-Agent'] != "TestUserAgent") {
				throw new Error("incorrect user agent: " + headers['User-Agent'])
			}
		`))
		assert.NoError(t, err)

		t.Run("Override", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.get("HTTPBIN_URL/headers", {
					headers: { "User-Agent": "OtherUserAgent" },
				});
				var headers = res.json()["headers"];
				if (headers['User-Agent'] != "OtherUserAgent") {
					throw new Error("incorrect user agent: " + headers['User-Agent'])
				}
			`))
			assert.NoError(t, err)
		})

		t.Run("Override empty", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.get("HTTPBIN_URL/headers", {
					headers: { "User-Agent": "" },
				});
				var headers = res.json()["headers"]
				if (typeof headers['User-Agent'] !== 'undefined') {
					throw new Error("not undefined user agent: " +  headers['User-Agent'])
				}
			`))
			assert.NoError(t, err)
		})

		t.Run("empty", func(t *testing.T) {
			oldUserAgent := state.Options.UserAgent
			defer func() {
				state.Options.UserAgent = oldUserAgent
			}()

			state.Options.UserAgent = null.NewString("", true)
			_, err := rt.RunString(sr(`
				var res = http.get("HTTPBIN_URL/headers");
				var headers = res.json()["headers"]
				if (typeof headers['User-Agent'] !== 'undefined') {
					throw new Error("not undefined user agent: " + headers['User-Agent'])
				}
			`))
			assert.NoError(t, err)
		})

		t.Run("default", func(t *testing.T) {
			oldUserAgent := state.Options.UserAgent
			defer func() {
				state.Options.UserAgent = oldUserAgent
			}()

			state.Options.UserAgent = null.NewString("Default one", false)
			_, err := rt.RunString(sr(`
				var res = http.get("HTTPBIN_URL/headers");
				var headers = res.json()["headers"]
				if (headers['User-Agent'] != "Default one") {
					throw new Error("incorrect user agent: " + headers['User-Agent'])
				}
			`))
			assert.NoError(t, err)
		})
	})
	t.Run("Compression", func(t *testing.T) {
		t.Run("gzip", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.get("HTTPSBIN_IP_URL/gzip");
				if (res.json()['gzipped'] != true) {
					throw new Error("unexpected body data: " + res.json()['gzipped'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("deflate", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.get("HTTPBIN_URL/deflate");
				if (res.json()['deflated'] != true) {
					throw new Error("unexpected body data: " + res.json()['deflated'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("zstd", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.get("HTTPSBIN_IP_URL/zstd");
				if (res.json()['compression'] != 'zstd') {
					throw new Error("unexpected body data: " + res.json()['compression'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("brotli", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.get("HTTPSBIN_IP_URL/brotli");
				if (res.json()['compression'] != 'br') {
					throw new Error("unexpected body data: " + res.json()['compression'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("zstd-br", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.get("HTTPSBIN_IP_URL/zstd-br");
				if (res.json()['compression'] != 'zstd, br') {
					throw new Error("unexpected compression: " + res.json()['compression'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("custom compression", func(t *testing.T) {
			// We should not try to decode it
			tb.Mux.HandleFunc("/customcompression", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Encoding", "custom")
				_, err := w.Write([]byte(`{"custom": true}`))
				assert.NoError(t, err)
			}))

			_, err := rt.RunString(sr(`
				var res = http.get("HTTPBIN_URL/customcompression");
				if (res.json()["custom"] != true) {
					throw new Error("unexpected body data: " + res.body)
				}
			`))
			assert.NoError(t, err)
		})
	})
	t.Run("CompressionWithAcceptEncodingHeader", func(t *testing.T) {
		t.Run("gzip", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var params = { headers: { "Accept-Encoding": "gzip" } };
				var res = http.get("HTTPBIN_URL/gzip", params);
				if (res.json()['gzipped'] != true) {
					throw new Error("unexpected body data: " + res.json()['gzipped'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("deflate", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var params = { headers: { "Accept-Encoding": "deflate" } };
				var res = http.get("HTTPBIN_URL/deflate", params);
				if (res.json()['deflated'] != true) {
					throw new Error("unexpected body data: " + res.json()['deflated'])
				}
			`))
			assert.NoError(t, err)
		})
	})
	t.Run("HTTP/2", func(t *testing.T) {
		metrics.GetBufferedSamples(samples) // Clean up buffered samples from previous tests
		_, err := rt.RunString(sr(`
		var res = http.request("GET", "HTTP2BIN_URL/get");
		if (res.status != 200) { throw new Error("wrong status: " + res.status) }
		if (res.proto != "HTTP/2.0") { throw new Error("wrong proto: " + res.proto) }
		`))
		assert.NoError(t, err)

		bufSamples := metrics.GetBufferedSamples(samples)
		assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTP2BIN_URL/get"), "", 200, "")
		for _, sampleC := range bufSamples {
			for _, sample := range sampleC.GetSamples() {
				proto, ok := sample.Tags.Get("proto")
				assert.True(t, ok)
				assert.Equal(t, "HTTP/2.0", proto)
			}
		}
	})
	t.Run("Invalid", func(t *testing.T) {
		hook := logtest.NewLocal(state.Logger)
		defer hook.Reset()

		_, err := rt.RunString(`http.request("", "");`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported protocol scheme")

		logEntry := hook.LastEntry()
		assert.Nil(t, logEntry)

		t.Run("throw=false", func(t *testing.T) {
			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			_, err := rt.RunString(`
				var res = http.request("GET", "some://example.com", null, { throw: false });
				if (res.error.search('unsupported protocol scheme "some"')  == -1) {
					throw new Error("wrong error:" + res.error);
				}
				throw new Error("another error");
			`)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "another error")

			logEntry := hook.LastEntry()
			if assert.NotNil(t, logEntry) {
				assert.Equal(t, logrus.WarnLevel, logEntry.Level)
				assert.Contains(t, logEntry.Data["error"].(error).Error(), "unsupported protocol scheme")
				assert.Equal(t, "Request Failed", logEntry.Message)
			}
		})
	})
	t.Run("InvalidURL", func(t *testing.T) {
		t.Parallel()

		expErr := `invalid URL: parse "https:// test.k6.io": invalid character " " in host name`
		t.Run("throw=true", func(t *testing.T) {
			js := `
				http.request("GET", "https:// test.k6.io");
				throw new Error("whoops!"); // shouldn't be reached
			`
			_, err := rt.RunString(js)
			require.Error(t, err)
			assert.Contains(t, err.Error(), expErr)
		})

		t.Run("throw=false", func(t *testing.T) {
			state.Options.Throw.Bool = false
			defer func() { state.Options.Throw.Bool = true }()

			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			js := `
				(function(){
					var r = http.request("GET", "https:// test.k6.io");
	                return {error: r.error, error_code: r.error_code};
				})()
			`
			ret, err := rt.RunString(js)
			require.NoError(t, err)
			require.NotNil(t, ret)
			var retobj map[string]interface{}
			var ok bool
			if retobj, ok = ret.Export().(map[string]interface{}); !ok {
				require.Fail(t, "got wrong return object: %#+v", retobj)
			}
			require.Equal(t, int64(1020), retobj["error_code"])
			require.Equal(t, expErr, retobj["error"])

			logEntry := hook.LastEntry()
			require.NotNil(t, logEntry)
			assert.Equal(t, logrus.WarnLevel, logEntry.Level)
			assert.Contains(t, logEntry.Data["error"].(error).Error(), expErr)
			assert.Equal(t, "Request Failed", logEntry.Message)
		})

		t.Run("throw=false,nopanic", func(t *testing.T) {
			state.Options.Throw.Bool = false
			defer func() { state.Options.Throw.Bool = true }()

			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			js := `
				(function(){
					var r = http.request("GET", "https:// test.k6.io");
					r.html();
					r.json();
	                return r.error_code; // not reached because of json()
				})()
			`
			ret, err := rt.RunString(js)
			require.Error(t, err)
			assert.Nil(t, ret)
			assert.Contains(t, err.Error(), "unexpected end of JSON input")

			logEntry := hook.LastEntry()
			require.NotNil(t, logEntry)
			assert.Equal(t, logrus.WarnLevel, logEntry.Level)
			assert.Contains(t, logEntry.Data["error"].(error).Error(), expErr)
			assert.Equal(t, "Request Failed", logEntry.Message)
		})
	})

	t.Run("Unroutable", func(t *testing.T) {
		_, err := rt.RunString(`http.request("GET", "http://sdafsgdhfjg/");`)
		assert.Error(t, err)
	})

	t.Run("Params", func(t *testing.T) {
		for _, literal := range []string{`undefined`, `null`} {
			t.Run(literal, func(t *testing.T) {
				_, err := rt.RunString(fmt.Sprintf(sr(`
				var res = http.request("GET", "HTTPBIN_URL/headers", null, %s);
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`), literal))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
			})
		}

		t.Run("cookies", func(t *testing.T) {
			t.Run("access", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/cookies/set?key=value", null, { redirects: 0 });
				if (res.cookies.key[0].value != "value") { throw new Error("wrong cookie value: " + res.cookies.key[0].value); }
				var props = ["name", "value", "domain", "path", "expires", "max_age", "secure", "http_only"];
				var cookie = res.cookies.key[0];
				for (var i = 0; i < props.length; i++) {
					if (cookie[props[i]] === undefined) {
						throw new Error("cookie property not found: " + props[i]);
					}
				}
				if (Object.keys(cookie).length != props.length) {
					throw new Error("cookie has more properties than expected: " + JSON.stringify(Object.keys(cookie)));
				}
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies/set?key=value"), "", 302, "")
			})

			t.Run("vuJar", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = rt.RunString(sr(`
				var jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value");
				var res = http.request("GET", "HTTPBIN_URL/cookies", null, { cookies: { key2: "value2" } });
				if (res.json().key != "value") { throw new Error("wrong cookie value: " + res.json().key); }
				if (res.json().key2 != "value2") { throw new Error("wrong cookie value: " + res.json().key2); }
				var jarCookies = jar.cookiesForURL("HTTPBIN_URL/cookies");
				if (jarCookies.key[0] != "value") { throw new Error("wrong cookie value in jar"); }
				if (jarCookies.key2 != undefined) { throw new Error("unexpected cookie in jar"); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("requestScope", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/cookies", null, { cookies: { key: "value" } });
				if (res.json().key != "value") { throw new Error("wrong cookie value: " + res.json().key); }
				var jar = http.cookieJar();
				var jarCookies = jar.cookiesForURL("HTTPBIN_URL/cookies");
				if (jarCookies.key != undefined) { throw new Error("unexpected cookie in jar"); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("requestScopeReplace", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = rt.RunString(sr(`
				var jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value");
				var res = http.request("GET", "HTTPBIN_URL/cookies", null, { cookies: { key: { value: "replaced", replace: true } } });
				if (res.json().key != "replaced") { throw new Error("wrong cookie value: " + res.json().key); }
				var jarCookies = jar.cookiesForURL("HTTPBIN_URL/cookies");
				if (jarCookies.key[0] != "value") { throw new Error("wrong cookie value in jar"); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("redirect", func(t *testing.T) {
				t.Run("set cookie after redirect", func(t *testing.T) {
					// TODO figure out a way to remove this ?
					tb.Mux.HandleFunc("/set-cookie-without-redirect", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						cookie := http.Cookie{
							Name:   "key-foo",
							Value:  "value-bar",
							Path:   "/",
							Domain: sr("HTTPSBIN_DOMAIN"),
						}

						http.SetCookie(w, &cookie)
						w.WriteHeader(200)
					}))
					cookieJar, err := cookiejar.New(nil)
					require.NoError(t, err)
					state.CookieJar = cookieJar
					_, err = rt.RunString(sr(`
						var res = http.request("GET", "HTTPBIN_URL/redirect-to?url=HTTPSBIN_URL/set-cookie-without-redirect");
						if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`))
					require.NoError(t, err)

					redirectURL, err := url.Parse(sr("HTTPSBIN_URL"))
					require.NoError(t, err)
					require.Len(t, cookieJar.Cookies(redirectURL), 1)
					require.Equal(t, "key-foo", cookieJar.Cookies(redirectURL)[0].Name)
					require.Equal(t, "value-bar", cookieJar.Cookies(redirectURL)[0].Value)

					assertRequestMetricsEmitted(
						t,
						metrics.GetBufferedSamples(samples),
						"GET",
						sr("HTTPSBIN_URL/set-cookie-without-redirect"),
						sr("HTTPSBIN_URL/set-cookie-without-redirect"),
						200,
						"",
					)
				})
				t.Run("set cookie before redirect", func(t *testing.T) {
					cookieJar, err := cookiejar.New(nil)
					require.NoError(t, err)
					state.CookieJar = cookieJar
					_, err = rt.RunString(sr(`
						var res = http.request("GET", "HTTPSBIN_URL/cookies/set?key=value");
						if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`))
					require.NoError(t, err)

					redirectURL, err := url.Parse(sr("HTTPSBIN_URL/cookies"))
					require.NoError(t, err)

					require.Len(t, cookieJar.Cookies(redirectURL), 1)
					require.Equal(t, "key", cookieJar.Cookies(redirectURL)[0].Name)
					require.Equal(t, "value", cookieJar.Cookies(redirectURL)[0].Value)

					assertRequestMetricsEmitted(
						t,
						metrics.GetBufferedSamples(samples),
						"GET",
						sr("HTTPSBIN_URL/cookies"),
						sr("HTTPSBIN_URL/cookies"),
						200,
						"",
					)
				})
				t.Run("set cookie after redirect and before second redirect", func(t *testing.T) {
					cookieJar, err := cookiejar.New(nil)
					require.NoError(t, err)
					state.CookieJar = cookieJar

					// TODO figure out a way to remove this ?
					tb.Mux.HandleFunc("/set-cookie-and-redirect", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						cookie := http.Cookie{
							Name:   "key-foo",
							Value:  "value-bar",
							Path:   "/set-cookie-and-redirect",
							Domain: sr("HTTPSBIN_DOMAIN"),
						}

						http.SetCookie(w, &cookie)
						http.Redirect(w, r, sr("HTTPBIN_IP_URL/get"), http.StatusMovedPermanently)
					}))

					_, err = rt.RunString(sr(`
						var res = http.request("GET", "HTTPBIN_IP_URL/redirect-to?url=HTTPSBIN_URL/set-cookie-and-redirect");
						if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`))
					require.NoError(t, err)

					redirectURL, err := url.Parse(sr("HTTPSBIN_URL/set-cookie-and-redirect"))
					require.NoError(t, err)

					require.Len(t, cookieJar.Cookies(redirectURL), 1)
					require.Equal(t, "key-foo", cookieJar.Cookies(redirectURL)[0].Name)
					require.Equal(t, "value-bar", cookieJar.Cookies(redirectURL)[0].Value)

					for _, cookieLessURL := range []string{"HTTPSBIN_URL", "HTTPBIN_IP_URL/redirect-to", "HTTPBIN_IP_URL/get"} {
						redirectURL, err = url.Parse(sr(cookieLessURL))
						require.NoError(t, err)
						require.Empty(t, cookieJar.Cookies(redirectURL))
					}

					assertRequestMetricsEmitted(
						t,
						metrics.GetBufferedSamples(samples),
						"GET",
						sr("HTTPBIN_IP_URL/get"),
						sr("HTTPBIN_IP_URL/get"),
						200,
						"",
					)
				})
			})

			t.Run("domain", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = rt.RunString(sr(`
				var jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value", { domain: "HTTPBIN_DOMAIN" });
				var res = http.request("GET", "HTTPBIN_URL/cookies");
				if (res.json().key != "value") {
					throw new Error("wrong cookie value 1: " + res.json().key);
				}
				jar.set("HTTPBIN_URL/cookies", "key2", "value2", { domain: "example.com" });
				res = http.request("GET", "HTTPBIN_URL/cookies");
				if (res.json().key != "value") {
					throw new Error("wrong cookie value 2: " + res.json().key);
				}
				if (res.json().key2 != undefined) {
					throw new Error("cookie 'key2' unexpectedly found");
				}
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("path", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = rt.RunString(sr(`
				var jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value", { path: "/cookies" });
				var res = http.request("GET", "HTTPBIN_URL/cookies");
				if (res.json().key != "value") {
					throw new Error("wrong cookie value: " + res.json().key);
				}
				jar.set("HTTPBIN_URL/cookies", "key2", "value2", { path: "/some-other-path" });
				res = http.request("GET", "HTTPBIN_URL/cookies");
				if (res.json().key != "value") {
					throw new Error("wrong cookie value: " + res.json().key);
				}
				if (res.json().key2 != undefined) {
					throw new Error("cookie 'key2' unexpectedly found");
				}
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("expires", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = rt.RunString(sr(`
				var jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value", { expires: "Sun, 24 Jul 1983 17:01:02 GMT" });
				var res = http.request("GET", "HTTPBIN_URL/cookies");
				if (res.json().key != undefined) {
					throw new Error("cookie 'key' unexpectedly found");
				}
				jar.set("HTTPBIN_URL/cookies", "key", "value", { expires: "Sat, 24 Jul 2083 17:01:02 GMT" });
				res = http.request("GET", "HTTPBIN_URL/cookies");
				if (res.json().key != "value") {
					throw new Error("cookie 'key' not found");
				}
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("secure", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = rt.RunString(sr(`
				var jar = http.cookieJar();
				jar.set("HTTPSBIN_IP_URL/cookies", "key", "value", { secure: true });
				var res = http.request("GET", "HTTPSBIN_IP_URL/cookies");
				if (res.json().key != "value") {
					throw new Error("wrong cookie value: " + res.json().key);
				}
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPSBIN_IP_URL/cookies"), "", 200, "")
			})

			t.Run("localJar", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = rt.RunString(sr(`
				var jar = new http.CookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value");
				var res = http.request("GET", "HTTPBIN_URL/cookies", null, { cookies: { key2: "value2" }, jar: jar });
				if (res.json().key != "value") { throw new Error("wrong cookie value: " + res.json().key); }
				if (res.json().key2 != "value2") { throw new Error("wrong cookie value: " + res.json().key2); }
				var jarCookies = jar.cookiesForURL("HTTPBIN_URL/cookies");
				if (jarCookies.key[0] != "value") { throw new Error("wrong cookie value in jar: " + jarCookies.key[0]); }
				if (jarCookies.key2 != undefined) { throw new Error("unexpected cookie in jar"); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			//nolint:paralleltest
			t.Run("clear", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = rt.RunString(sr(`
				var jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value");
				var res = http.request("GET", "HTTPBIN_URL/cookies");
				if (res.json().key != "value") { throw new Error("cookie 'key' unexpectedly don't found"); }
				jar.clear('HTTPBIN_URL/cookies');
				res = http.request("GET", "HTTPBIN_URL/cookies");
				if (res.json().key == "value") { throw new Error("wrong clean: unexpected cookie in jar"); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			//nolint:paralleltest
			t.Run("delete", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = rt.RunString(sr(`
				var jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key1", "value1");
				jar.set("HTTPBIN_URL/cookies", "key2", "value2");
				var res = http.request("GET", "HTTPBIN_URL/cookies");
				if (res.json().key1 != "value1" || res.json().key2 != "value2") { throw new Error("cookie 'keys' unexpectedly don't found"); }
				jar.delete('HTTPBIN_URL/cookies', "key1");
				res = http.request("GET", "HTTPBIN_URL/cookies");
				if (res.json().key1 == "value1" || res.json().key2 != "value2"  ) { throw new Error("wrong clean: unexpected cookie in jar"); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})
		})

		t.Run("auth", func(t *testing.T) {
			t.Run("basic", func(t *testing.T) {
				url := sr("http://bob:pass@HTTPBIN_IP:HTTPBIN_PORT/basic-auth/bob/pass")
				urlExpected := sr("http://****:****@HTTPBIN_IP:HTTPBIN_PORT/basic-auth/bob/pass")

				_, err := rt.RunString(fmt.Sprintf(`
				var res = http.request("GET", "%s", null, {});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`, url))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", urlExpected, urlExpected, 200, "")
			})
			t.Run("digest", func(t *testing.T) {
				t.Run("success", func(t *testing.T) {
					url := sr("http://bob:pass@HTTPBIN_IP:HTTPBIN_PORT/digest-auth/auth/bob/pass")
					urlRaw := sr("HTTPBIN_IP_URL/digest-auth/auth/bob/pass")

					_, err := rt.RunString(fmt.Sprintf(`
					var res = http.request("GET", "%s", null, { auth: "digest" });
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					if (res.error_code != 0) { throw new Error("wrong error code: " + res.error_code); }
					`, url))
					assert.NoError(t, err)

					sampleContainers := metrics.GetBufferedSamples(samples)
					assertRequestMetricsEmitted(t, sampleContainers[0:1], "GET",
						urlRaw, urlRaw, 401, "")
					assertRequestMetricsEmitted(t, sampleContainers[1:2], "GET",
						urlRaw, urlRaw, 200, "")
				})
				t.Run("failure", func(t *testing.T) {
					url := sr("http://bob:pass@HTTPBIN_IP:HTTPBIN_PORT/digest-auth/failure")

					_, err := rt.RunString(fmt.Sprintf(`
					var res = http.request("GET", "%s", null, { auth: "digest", timeout: 1, throw: false });
					`, url))
					assert.NoError(t, err)
				})
			})
		})

		t.Run("headers", func(t *testing.T) {
			for _, literal := range []string{`null`, `undefined`} {
				t.Run(literal, func(t *testing.T) {
					_, err := rt.RunString(fmt.Sprintf(sr(`
					var res = http.request("GET", "HTTPBIN_URL/headers", null, { headers: %s });
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`), literal))
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
				})
			}

			t.Run("object", func(t *testing.T) {
				_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/headers", null, {
					headers: { "X-My-Header": "value" },
				});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().headers["X-My-Header"] != "value") { throw new Error("wrong X-My-Header: " + res.json().headers["X-My-Header"]); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
			})

			t.Run("Host", func(t *testing.T) {
				_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/headers", null, {
					headers: { "Host": "HTTPBIN_DOMAIN" },
				});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().headers["Host"] != "HTTPBIN_DOMAIN") { throw new Error("wrong Host: " + res.json().headers["Host"]); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
			})

			t.Run("response_request", func(t *testing.T) {
				_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/headers", null, {
					headers: { "host": "HTTPBIN_DOMAIN" },
				});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.request.headers["Host"] != "HTTPBIN_DOMAIN") { throw new Error("wrong Host: " + res.request.headers["Host"]); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
			})

			t.Run("differentHost", func(t *testing.T) {
				_, err := rt.RunString(sr(`
				var custHost = 'k6.io';
				var res = http.request("GET", "HTTPBIN_URL/headers", null, {
					headers: { "host": custHost },
				});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.request.headers["Host"] != custHost) { throw new Error("wrong Host: " + res.request.headers["Host"]); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
			})
		})

		t.Run("tags", func(t *testing.T) {
			for _, literal := range []string{`null`, `undefined`} {
				t.Run(literal, func(t *testing.T) {
					_, err := rt.RunString(fmt.Sprintf(sr(`
					var res = http.request("GET", "HTTPBIN_URL/headers", null, { tags: %s });
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`), literal))
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
				})
			}

			t.Run("name/none", func(t *testing.T) {
				_, err := rt.RunString(sr(`
					var res = http.request("GET", "HTTPBIN_URL/headers");
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET",
					sr("HTTPBIN_URL/headers"), sr("HTTPBIN_URL/headers"), 200, "")
			})

			t.Run("name/request", func(t *testing.T) {
				_, err := rt.RunString(sr(`
					var res = http.request("GET", "HTTPBIN_URL/headers", null, { tags: { name: "myReq" }});
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET",
					sr("HTTPBIN_URL/headers"), "myReq", 200, "")
			})

			t.Run("name/template", func(t *testing.T) {
				_, err := rt.RunString("http.get(http.url`" + sr(`HTTPBIN_URL/anything/${1+1}`) + "`);")
				assert.NoError(t, err)
				// There's no /anything endpoint in the go-httpbin library we're using, hence the 404,
				// but it doesn't matter for this test.
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET",
					sr("HTTPBIN_URL/anything/2"), sr("HTTPBIN_URL/anything/${}"), 404, "")
			})

			t.Run("object", func(t *testing.T) {
				_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/headers", null, { tags: { tag: "value" } });
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`))
				assert.NoError(t, err)
				bufSamples := metrics.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
				for _, sampleC := range bufSamples {
					for _, sample := range sampleC.GetSamples() {
						tagValue, ok := sample.Tags.Get("tag")
						assert.True(t, ok)
						assert.Equal(t, "value", tagValue)
					}
				}
			})

			t.Run("tags-precedence", func(t *testing.T) {
				oldTags := state.Tags
				defer func() { state.Tags = oldTags }()
				state.Tags = lib.NewTagMap(map[string]string{
					"runtag1": "val1",
					"runtag2": "val2",
				})

				_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/headers", null, { tags: { method: "test", name: "myName", runtag1: "fromreq" } });
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`))
				assert.NoError(t, err)

				bufSamples := metrics.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/headers"), "myName", 200, "")
				for _, sampleC := range bufSamples {
					for _, sample := range sampleC.GetSamples() {
						tagValue, ok := sample.Tags.Get("method")
						assert.True(t, ok)
						assert.Equal(t, "GET", tagValue)

						tagValue, ok = sample.Tags.Get("name")
						assert.True(t, ok)
						assert.Equal(t, "myName", tagValue)

						tagValue, ok = sample.Tags.Get("runtag1")
						assert.True(t, ok)
						assert.Equal(t, "fromreq", tagValue)

						tagValue, ok = sample.Tags.Get("runtag2")
						assert.True(t, ok)
						assert.Equal(t, "val2", tagValue)
					}
				}
			})
		})
	})

	t.Run("GET", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var res = http.get("HTTPBIN_URL/get?a=1&b=2", {headers: {"X-We-Want-This": "value"}});
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.json().args.a != "1") { throw new Error("wrong ?a: " + res.json().args.a); }
		if (res.json().args.b != "2") { throw new Error("wrong ?b: " + res.json().args.b); }
		if (res.request.headers["X-We-Want-This"] != "value") { throw new Error("Missing or invalid X-We-Want-This header!"); }
		`))
		require.NoError(t, err)
		assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/get?a=1&b=2"), "", 200, "")

		t.Run("Tagged", func(t *testing.T) {
			_, err := rt.RunString(`
			var a = "1";
			var b = "2";
			var res = http.get(http.url` + "`" + sr(`HTTPBIN_URL/get?a=${a}&b=${b}`) + "`" + `);
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			if (res.json().args.a != a) { throw new Error("wrong ?a: " + res.json().args.a); }
			if (res.json().args.b != b) { throw new Error("wrong ?b: " + res.json().args.b); }
			`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/get?a=1&b=2"), sr("HTTPBIN_URL/get?a=${}&b=${}"), 200, "")
		})
	})
	t.Run("HEAD", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var res = http.head("HTTPBIN_URL/get?a=1&b=2", {headers: {"X-We-Want-This": "value"}});
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.body.length != 0) { throw new Error("HEAD responses shouldn't have a body"); }
		if (!res.headers["Content-Length"]) { throw new Error("Missing or invalid Content-Length header!"); }
		if (res.request.headers["X-We-Want-This"] != "value") { throw new Error("Missing or invalid X-We-Want-This header!"); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "HEAD", sr("HTTPBIN_URL/get?a=1&b=2"), "", 200, "")
	})

	t.Run("OPTIONS", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var res = http.options("HTTPBIN_URL/?a=1&b=2", null, {headers: {"X-We-Want-This": "value"}});
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (!res.headers["Access-Control-Allow-Methods"]) { throw new Error("Missing Access-Control-Allow-Methods header!"); }
		if (res.request.headers["X-We-Want-This"] != "value") { throw new Error("Missing or invalid X-We-Want-This header!"); }
		`))
		require.NoError(t, err)
		assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "OPTIONS", sr("HTTPBIN_URL/?a=1&b=2"), "", 200, "")
	})

	// DELETE HTTP requests shouldn't usually send a request body, they should use url parameters instead; references:
	// https://golang.org/pkg/net/http/#Request.ParseForm
	// https://stackoverflow.com/questions/299628/is-an-entity-body-allowed-for-an-http-delete-request
	// https://tools.ietf.org/html/rfc7231#section-4.3.5
	t.Run("DELETE", func(t *testing.T) {
		_, err := rt.RunString(sr(`
		var res = http.del("HTTPBIN_URL/delete?test=mest", null, {headers: {"X-We-Want-This": "value"}});
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.json().args.test != "mest") { throw new Error("wrong args: " + JSON.stringify(res.json().args)); }
		if (res.request.headers["X-We-Want-This"] != "value") { throw new Error("Missing or invalid X-We-Want-This header!"); }
		`))
		require.NoError(t, err)
		assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "DELETE", sr("HTTPBIN_URL/delete?test=mest"), "", 200, "")
	})

	postMethods := map[string]string{
		"POST":  "post",
		"PUT":   "put",
		"PATCH": "patch",
	}
	for method, fn := range postMethods {
		t.Run(method, func(t *testing.T) {
			_, err := rt.RunString(fmt.Sprintf(sr(`
				var res = http.%s("HTTPBIN_URL/%s", "data", {headers: {"X-We-Want-This": "value"}});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().data != "data") { throw new Error("wrong data: " + res.json().data); }
				if (res.json().headers["Content-Type"]) { throw new Error("content type set: " + res.json().headers["Content-Type"]); }
				if (res.request.headers["X-We-Want-This"] != "value") { throw new Error("Missing or invalid X-We-Want-This header!"); }
				`), fn, strings.ToLower(method)))
			require.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), method, sr("HTTPBIN_URL/")+strings.ToLower(method), "", 200, "")

			t.Run("object", func(t *testing.T) {
				_, err := rt.RunString(fmt.Sprintf(sr(`
				var equalArray = function(a, b) {
					return a.length === b.length && a.every(function(v, i) { return v === b[i]});
				}
				var res = http.%s("HTTPBIN_URL/%s", {a: "a", b: 2, c: ["one", "two"]});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().form.a != "a") { throw new Error("wrong a=: " + res.json().form.a); }
				if (res.json().form.b != "2") { throw new Error("wrong b=: " + res.json().form.b); }
				if (!equalArray(res.json().form.c, ["one", "two"])) { throw new Error("wrong c: " + res.json().form.c); }
				if (res.json().headers["Content-Type"] != "application/x-www-form-urlencoded") { throw new Error("wrong content type: " + res.json().headers["Content-Type"]); }
				`), fn, strings.ToLower(method)))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), method, sr("HTTPBIN_URL/")+strings.ToLower(method), "", 200, "")
				t.Run("Content-Type", func(t *testing.T) {
					_, err := rt.RunString(fmt.Sprintf(sr(`
						var res = http.%s("HTTPBIN_URL/%s", {a: "a", b: 2}, {headers: {"Content-Type": "application/x-www-form-urlencoded; charset=utf-8"}});
						if (res.status != 200) { throw new Error("wrong status: " + res.status); }
						if (res.json().form.a != "a") { throw new Error("wrong a=: " + res.json().form.a); }
						if (res.json().form.b != "2") { throw new Error("wrong b=: " + res.json().form.b); }
						if (res.json().headers["Content-Type"] != "application/x-www-form-urlencoded; charset=utf-8") { throw new Error("wrong content type: " + res.json().headers["Content-Type"]); }
						`), fn, strings.ToLower(method)))
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), method, sr("HTTPBIN_URL/")+strings.ToLower(method), "", 200, "")
				})
			})
		})
	}

	t.Run("Batch", func(t *testing.T) {
		t.Run("error", func(t *testing.T) {
			invalidURLerr := `invalid URL: parse "https:// invalidurl.com": invalid character " " in host name`
			testCases := []struct {
				name, code, expErr string
				throw              bool
			}{
				{
					name: "no arg", code: ``,
					expErr: `no argument was provided to http.batch()`, throw: true,
				},
				{
					name: "invalid null arg", code: `null`,
					expErr: `invalid http.batch() argument type <nil>`, throw: true,
				},
				{
					name: "invalid undefined arg", code: `undefined`,
					expErr: `invalid http.batch() argument type <nil>`, throw: true,
				},
				{
					name: "invalid string arg", code: `"https://somevalidurl.com"`,
					expErr: `invalid http.batch() argument type string`, throw: true,
				},
				{
					name: "invalid URL short", code: `["https:// invalidurl.com"]`,
					expErr: invalidURLerr, throw: true,
				},
				{
					name: "invalid URL short no throw", code: `["https:// invalidurl.com"]`,
					expErr: invalidURLerr, throw: false,
				},
				{
					name: "invalid URL array", code: `[ ["GET", "https:// invalidurl.com"] ]`,
					expErr: invalidURLerr, throw: true,
				},
				{
					name: "invalid URL array no throw", code: `[ ["GET", "https:// invalidurl.com"] ]`,
					expErr: invalidURLerr, throw: false,
				},
				{
					name: "invalid URL object", code: `[ {method: "GET", url: "https:// invalidurl.com"} ]`,
					expErr: invalidURLerr, throw: true,
				},
				{
					name: "invalid object no throw", code: `[ {method: "GET", url: "https:// invalidurl.com"} ]`,
					expErr: invalidURLerr, throw: false,
				},
				{
					name: "object no url key", code: `[ {method: "GET"} ]`,
					expErr: `batch request 0 doesn't have a url key`, throw: true,
				},
				{
					name: "multiple arguments", code: `["GET", "https://test.k6.io"],["GET", "https://test.k6.io"]`,
					expErr: `http.batch() accepts only an array or an object of requests`, throw: true,
				},
			}

			for _, tc := range testCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) { //nolint:paralleltest
					oldThrow := state.Options.Throw.Bool
					state.Options.Throw.Bool = tc.throw
					defer func() { state.Options.Throw.Bool = oldThrow }()

					hook := logtest.NewLocal(state.Logger)
					defer hook.Reset()

					ret, err := rt.RunString(fmt.Sprintf(`
						(function(){
							var r = http.batch(%s);
							if (r.length !== 1) throw new Error('unexpected responses length: '+r.length);
							return {error: r[0].error, error_code: r[0].error_code};
						})()`, tc.code))
					if tc.throw {
						require.Error(t, err)
						assert.Contains(t, err.Error(), tc.expErr)
						require.Nil(t, ret)
					} else {
						require.NoError(t, err)
						require.NotNil(t, ret)
						var retobj map[string]interface{}
						var ok bool
						if retobj, ok = ret.Export().(map[string]interface{}); !ok {
							require.Fail(t, "got wrong return object: %#+v", retobj)
						}
						require.Equal(t, int64(1020), retobj["error_code"])
						require.Equal(t, invalidURLerr, retobj["error"])

						logEntry := hook.LastEntry()
						require.NotNil(t, logEntry)
						assert.Equal(t, logrus.WarnLevel, logEntry.Level)
						assert.Contains(t, logEntry.Data["error"].(error).Error(), tc.expErr)
						assert.Equal(t, "A batch request failed", logEntry.Message)
					}
				})
			}
		})
		t.Run("error,nopanic", func(t *testing.T) { //nolint:paralleltest
			invalidURLerr := `invalid URL: parse "https:// invalidurl.com": invalid character " " in host name`
			testCases := []struct{ name, code string }{
				{
					name: "array", code: `[
						["GET", "https:// invalidurl.com"],
						["GET", "https://somevalidurl.com"],
					]`,
				},
				{
					name: "object", code: `[
						{method: "GET", url: "https:// invalidurl.com"},
						{method: "GET", url: "https://somevalidurl.com"},
					]`,
				},
			}

			for _, tc := range testCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) { //nolint:paralleltest
					oldThrow := state.Options.Throw.Bool
					state.Options.Throw.Bool = false
					defer func() { state.Options.Throw.Bool = oldThrow }()

					hook := logtest.NewLocal(state.Logger)
					defer hook.Reset()

					ret, err := rt.RunString(fmt.Sprintf(`
						(function(){
							var r = http.batch(%s);
							if (r.length !== 2) throw new Error('unexpected responses length: '+r.length);
							if (r[1] !== null) throw new Error('expected response at index 1 to be null');
							r[0].html();
							r[0].json();
	            		    return r[0].error_code; // not reached because of json()
						})()
					`, tc.code))
					require.Error(t, err)
					assert.Nil(t, ret)
					assert.Contains(t, err.Error(), "unexpected end of JSON input")
					logEntry := hook.LastEntry()
					require.NotNil(t, logEntry)
					assert.Equal(t, logrus.WarnLevel, logEntry.Level)
					assert.Contains(t, logEntry.Data["error"].(error).Error(), invalidURLerr)
					assert.Equal(t, "A batch request failed", logEntry.Message)
				})
			}
		})
		t.Run("GET", func(t *testing.T) {
			_, err := rt.RunString(sr(`
		{
			let reqs = [
				["GET", "HTTPBIN_URL/"],
				["GET", "HTTPBIN_IP_URL/"],
			];
			let res = http.batch(reqs);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + res[key].status); }
				if (res[key].url != reqs[key][1]) { throw new Error("wrong url: " + res[key].url); }
			}
		}`))
			require.NoError(t, err)
			bufSamples := metrics.GetBufferedSamples(samples)
			assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/"), "", 200, "")
			assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")

			t.Run("Tagged", func(t *testing.T) {
				_, err := rt.RunString(sr(`
			{
				let fragment = "get";
				let reqs = [
					["GET", http.url` + "`" + `HTTPBIN_URL/${fragment}` + "`" + `],
					["GET", http.url` + "`" + `HTTPBIN_IP_URL/` + "`" + `],
				];
				let res = http.batch(reqs);
				for (var key in res) {
					if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
					if (res[key].url != reqs[key][1].url) { throw new Error("wrong url: " + key + ": " + res[key].url + " != " + reqs[key][1].url); }
				}
			}`))
				assert.NoError(t, err)
				bufSamples := metrics.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get"), sr("HTTPBIN_URL/${}"), 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")
			})

			t.Run("Shorthand", func(t *testing.T) {
				_, err := rt.RunString(sr(`
			{
				let reqs = [
					"HTTPBIN_URL/",
					"HTTPBIN_IP_URL/",
				];
				let res = http.batch(reqs);
				for (var key in res) {
					if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
					if (res[key].url != reqs[key]) { throw new Error("wrong url: " + key + ": " + res[key].url); }
				}
			}`))
				assert.NoError(t, err)
				bufSamples := metrics.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/"), "", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")

				t.Run("Tagged", func(t *testing.T) {
					_, err := rt.RunString(sr(`
				{
					let fragment = "get";
					let reqs = [
						http.url` + "`" + `HTTPBIN_URL/${fragment}` + "`" + `,
						http.url` + "`" + `HTTPBIN_IP_URL/` + "`" + `,
					];
					let res = http.batch(reqs);
					for (var key in res) {
						if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
						if (res[key].url != reqs[key].url) { throw new Error("wrong url: " + key + ": " + res[key].url + " != " + reqs[key].url); }
					}
				}`))
					assert.NoError(t, err)
					bufSamples := metrics.GetBufferedSamples(samples)
					assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get"), sr("HTTPBIN_URL/${}"), 200, "")
					assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")
				})
			})

			t.Run("ObjectForm", func(t *testing.T) {
				_, err := rt.RunString(sr(`
			{
				let reqs = [
					{ method: "GET", url: "HTTPBIN_URL/" },
					{ url: "HTTPBIN_IP_URL/", method: "GET"},
				];
				let res = http.batch(reqs);
				for (var key in res) {
					if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
					if (res[key].url != reqs[key].url) { throw new Error("wrong url: " + key + ": " + res[key].url + " != " + reqs[key].url); }
				}
			}`))
				assert.NoError(t, err)
				bufSamples := metrics.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/"), "", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")
			})

			t.Run("ObjectKeys", func(t *testing.T) {
				_, err := rt.RunString(sr(`
				var reqs = {
					shorthand: "HTTPBIN_URL/get?r=shorthand",
					arr: ["GET", "HTTPBIN_URL/get?r=arr", null, {tags: {name: 'arr'}}],
					obj1: { method: "GET", url: "HTTPBIN_URL/get?r=obj1" },
					obj2: { url: "HTTPBIN_URL/get?r=obj2", params: {tags: {name: 'obj2'}}, method: "GET"},
				};
				var res = http.batch(reqs);
				for (var key in res) {
					if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
					if (res[key].json().args.r != key) { throw new Error("wrong request id: " + key); }
				}`))
				assert.NoError(t, err)
				bufSamples := metrics.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get?r=shorthand"), "", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get?r=arr"), "arr", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get?r=obj1"), "", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get?r=obj2"), "obj2", 200, "")
			})

			t.Run("BodyAndParams", func(t *testing.T) {
				testStr := "testbody"
				rt.Set("someStrFile", testStr)
				rt.Set("someBinFile", []byte(testStr))

				_, err := rt.RunString(sr(`
					var reqs = [
						["POST", "HTTPBIN_URL/post", "testbody"],
						["POST", "HTTPBIN_URL/post", someStrFile],
						["POST", "HTTPBIN_URL/post", someBinFile],
						{
							method: "POST",
							url: "HTTPBIN_URL/post",
							test: "test1",
							body: "testbody",
						}, {
							body: someBinFile,
							url: "HTTPBIN_IP_URL/post",
							params: { tags: { name: "myname" } },
							method: "POST",
						}, {
							method: "POST",
							url: "HTTPBIN_IP_URL/post",
							body: {
								hello: "world!",
							},
							params: {
								tags: { name: "myname" },
								headers: { "Content-Type": "application/x-www-form-urlencoded" },
							},
						},
					];
					var res = http.batch(reqs);
					for (var key in res) {
						if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
						if (res[key].json().data != "testbody" && res[key].json().form.hello != "world!") { throw new Error("wrong response for " + key + ": " + res[key].body); }
					}`))
				assert.NoError(t, err)
				bufSamples := metrics.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "POST", sr("HTTPBIN_URL/post"), "", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "POST", sr("HTTPBIN_IP_URL/post"), "myname", 200, "")
			})
		})
		t.Run("POST", func(t *testing.T) {
			_, err := rt.RunString(sr(`
			var res = http.batch([ ["POST", "HTTPBIN_URL/post", { key: "value" }] ]);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
				if (res[key].json().form.key != "value") { throw new Error("wrong form: " + key + ": " + JSON.stringify(res[key].json().form)); }
			}`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "POST", sr("HTTPBIN_URL/post"), "", 200, "")
		})
		t.Run("PUT", func(t *testing.T) {
			_, err := rt.RunString(sr(`
			var res = http.batch([ ["PUT", "HTTPBIN_URL/put", { key: "value" }] ]);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
				if (res[key].json().form.key != "value") { throw new Error("wrong form: " + key + ": " + JSON.stringify(res[key].json().form)); }
			}`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "PUT", sr("HTTPBIN_URL/put"), "", 200, "")
		})
	})

	t.Run("HTTPRequest", func(t *testing.T) {
		t.Run("EmptyBody", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var reqUrl = "HTTPBIN_URL/cookies"
				var res = http.get(reqUrl);
				var jar = new http.CookieJar();

				jar.set("HTTPBIN_URL/cookies", "key", "value");
				res = http.request("GET", "HTTPBIN_URL/cookies", null, { cookies: { key2: "value2" }, jar: jar });

				if (res.json().key != "value") { throw new Error("wrong cookie value: " + res.json().key); }

				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.request["method"] !== "GET") { throw new Error("http request method was not \"GET\": " + JSON.stringify(res.request)) }
				if (res.request["body"].length != 0) { throw new Error("http request body was not null: " + JSON.stringify(res.request["body"])) }
				if (res.request["url"] != reqUrl) {
					throw new Error("wrong http request url: " + JSON.stringify(res.request))
				}
				if (res.request["cookies"]["key2"][0].name != "key2") { throw new Error("wrong http request cookies: " + JSON.stringify(JSON.stringify(res.request["cookies"]["key2"]))) }
				if (res.request["headers"]["User-Agent"][0] != "TestUserAgent") { throw new Error("wrong http request headers: " + JSON.stringify(res.request)) }
				`))
			assert.NoError(t, err)
		})
		t.Run("NonEmptyBody", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.post("HTTPBIN_URL/post", {a: "a", b: 2}, {headers: {"Content-Type": "application/x-www-form-urlencoded; charset=utf-8"}});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.request["body"] != "a=a&b=2") { throw new Error("http request body was not set properly: " + JSON.stringify(res.request))}
				`))
			assert.NoError(t, err)
		})
	})
}

func TestRequestCancellation(t *testing.T) {
	t.Parallel()
	tb, state, _, rt, mi := newRuntime(t)
	sr := tb.Replacer.Replace

	hook := logtest.NewLocal(state.Logger)
	defer hook.Reset()

	testVU, ok := mi.vu.(*modulestest.VU)
	require.True(t, ok)

	newctx, cancel := context.WithCancel(mi.vu.Context())
	testVU.CtxField = newctx
	cancel()

	_, err := rt.RunString(sr(`http.get("HTTPBIN_URL/get/");`))
	assert.Error(t, err)
	assert.Nil(t, hook.LastEntry())
}

func TestRequestArrayBufferBody(t *testing.T) {
	t.Parallel()
	tb, _, _, rt, _ := newRuntime(t) //nolint:dogsled
	sr := tb.Replacer.Replace

	tb.Mux.HandleFunc("/post-arraybuffer", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		var in bytes.Buffer
		_, err := io.Copy(&in, r.Body)
		require.NoError(t, err)
		_, err = w.Write(in.Bytes())
		require.NoError(t, err)
	}))

	testCases := []struct {
		arr, expected string
	}{
		{"Uint8Array", "104,101,108,108,111"},
		{"Uint16Array", "104,0,101,0,108,0,108,0,111,0"},
		{"Uint32Array", "104,0,0,0,101,0,0,0,108,0,0,0,108,0,0,0,111,0,0,0"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.arr, func(t *testing.T) {
			_, err := rt.RunString(sr(fmt.Sprintf(`
			var arr = new %[1]s([104, 101, 108, 108, 111]); // "hello"
			var res = http.post("HTTPBIN_URL/post-arraybuffer", arr.buffer, { responseType: 'binary' });

			if (res.status != 200) { throw new Error("wrong status: " + res.status) }

			var resTyped = new Uint8Array(res.body);
			var exp = new %[1]s([%[2]s]);
			if (exp.length !== resTyped.length) {
				throw new Error(
					"incorrect data length: expected " + exp.length + ", received " + resTypedLength)
			}
			for (var i = 0; i < exp.length; i++) {
				if (exp[i] !== resTyped[i])	{
					throw new Error(
						"incorrect data at index " + i + ": expected " + exp[i] + ", received " + resTyped[i])
				}
			}
			`, tc.arr, tc.expected)))
			assert.NoError(t, err)
		})
	}
}

func TestRequestCompression(t *testing.T) {
	t.Parallel()
	tb, state, _, rt, _ := newRuntime(t)

	logHook := testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.WarnLevel}}
	state.Logger.AddHook(&logHook)

	// We don't expect any failed requests
	state.Options.Throw = null.BoolFrom(true)

	text := `
	Lorem ipsum dolor sit amet, consectetur adipiscing elit.
	Maecenas sed pharetra sapien. Nunc laoreet molestie ante ac gravida.
	Etiam interdum dui viverra posuere egestas. Pellentesque at dolor tristique,
	mattis turpis eget, commodo purus. Nunc orci aliquam.`

	decompress := func(algo string, input io.Reader) io.Reader {
		switch algo {
		case "br":
			w := brotli.NewReader(input)
			return w
		case "gzip":
			w, err := gzip.NewReader(input)
			if err != nil {
				t.Fatal(err)
			}
			return w
		case "deflate":
			w, err := zlib.NewReader(input)
			if err != nil {
				t.Fatal(err)
			}
			return w
		case "zstd":
			w, err := zstd.NewReader(input)
			if err != nil {
				t.Fatal(err)
			}
			return w
		default:
			t.Fatal("unknown algorithm " + algo)
		}
		return nil // unreachable
	}

	var (
		expectedEncoding string
		actualEncoding   string
	)
	tb.Mux.HandleFunc("/compressed-text", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, expectedEncoding, r.Header.Get("Content-Encoding"))

		expectedLength, err := strconv.Atoi(r.Header.Get("Content-Length"))
		require.NoError(t, err)
		algos := strings.Split(actualEncoding, ", ")
		compressedBuf := new(bytes.Buffer)
		n, err := io.Copy(compressedBuf, r.Body)
		require.Equal(t, int(n), expectedLength)
		require.NoError(t, err)
		var prev io.Reader = compressedBuf

		if expectedEncoding != "" {
			for i := len(algos) - 1; i >= 0; i-- {
				prev = decompress(algos[i], prev)
			}
		}

		var buf bytes.Buffer
		_, err = io.Copy(&buf, prev)
		require.NoError(t, err)
		require.Equal(t, text, buf.String())
	}))

	testCases := []struct {
		name          string
		compression   string
		expectedError string
	}{
		{compression: ""},
		{compression: "  "},
		{compression: "gzip"},
		{compression: "gzip, gzip"},
		{compression: "gzip,   gzip "},
		{compression: "gzip,gzip"},
		{compression: "gzip, gzip, gzip, gzip, gzip, gzip, gzip"},
		{compression: "deflate"},
		{compression: "deflate, gzip"},
		{compression: "gzip,deflate, gzip"},
		{compression: "zstd"},
		{compression: "zstd, gzip, deflate"},
		{compression: "br"},
		{compression: "br, gzip, deflate"},
		{
			compression:   "George",
			expectedError: `unknown compression algorithm George`,
		},
		{
			compression:   "gzip, George",
			expectedError: `unknown compression algorithm George`,
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.compression, func(t *testing.T) {
			algos := strings.Split(testCase.compression, ",")
			for i, algo := range algos {
				algos[i] = strings.TrimSpace(algo)
			}
			expectedEncoding = strings.Join(algos, ", ")
			actualEncoding = expectedEncoding
			_, err := rt.RunString(tb.Replacer.Replace(`
		http.post("HTTPBIN_URL/compressed-text", ` + "`" + text + "`" + `,  {"compression": "` + testCase.compression + `"});
	`))
			if testCase.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), testCase.expectedError)
			}
		})
	}

	t.Run("custom set header", func(t *testing.T) {
		expectedEncoding = "not, valid"
		actualEncoding = "gzip, deflate"

		logHook.Drain()
		t.Run("encoding", func(t *testing.T) {
			_, err := rt.RunString(tb.Replacer.Replace(`
				http.post("HTTPBIN_URL/compressed-text", ` + "`" + text + "`" + `,
					{"compression": "` + actualEncoding + `",
					 "headers": {"Content-Encoding": "` + expectedEncoding + `"}
					}
				);
			`))
			require.NoError(t, err)
			require.NotEmpty(t, logHook.Drain())
		})

		t.Run("encoding and length", func(t *testing.T) {
			_, err := rt.RunString(tb.Replacer.Replace(`
				http.post("HTTPBIN_URL/compressed-text", ` + "`" + text + "`" + `,
					{"compression": "` + actualEncoding + `",
					 "headers": {"Content-Encoding": "` + expectedEncoding + `",
								 "Content-Length": "12"}
					}
				);
			`))
			require.NoError(t, err)
			require.NotEmpty(t, logHook.Drain())
		})

		expectedEncoding = actualEncoding
		t.Run("correct encoding", func(t *testing.T) {
			_, err := rt.RunString(tb.Replacer.Replace(`
				http.post("HTTPBIN_URL/compressed-text", ` + "`" + text + "`" + `,
					{"compression": "` + actualEncoding + `",
					 "headers": {"Content-Encoding": "` + actualEncoding + `"}
					}
				);
			`))
			require.NoError(t, err)
			require.Empty(t, logHook.Drain())
		})

		// TODO: move to some other test?
		t.Run("correct length", func(t *testing.T) {
			_, err := rt.RunString(tb.Replacer.Replace(
				`http.post("HTTPBIN_URL/post", "0123456789", { "headers": {"Content-Length": "10"}});`,
			))
			require.NoError(t, err)
			require.Empty(t, logHook.Drain())
		})

		t.Run("content-length is set", func(t *testing.T) {
			_, err := rt.RunString(tb.Replacer.Replace(`
				var resp = http.post("HTTPBIN_URL/post", "0123456789");
				if (resp.json().headers["Content-Length"][0] != "10") {
					throw new Error("content-length not set: " + JSON.stringify(resp.json().headers));
				}
			`))
			require.NoError(t, err)
			require.Empty(t, logHook.Drain())
		})
	})
}

func TestResponseTypes(t *testing.T) {
	t.Parallel()
	tb, state, _, rt, _ := newRuntime(t)

	// We don't expect any failed requests
	state.Options.Throw = null.BoolFrom(true)

	text := `•?((¯°·._.• ţ€$ţɨɲǥ µɲɨȼ๏ď€ ɨɲ Ќ6 •._.·°¯))؟•`
	textLen := len(text)
	tb.Mux.HandleFunc("/get-text", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := w.Write([]byte(text))
		assert.NoError(t, err)
		assert.Equal(t, textLen, n)
	}))
	tb.Mux.HandleFunc("/compare-text", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, text, string(body))
	}))

	binaryLen := 300
	binary := make([]byte, binaryLen)
	for i := 0; i < binaryLen; i++ {
		binary[i] = byte(i)
	}
	tb.Mux.HandleFunc("/get-bin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := w.Write(binary)
		assert.NoError(t, err)
		assert.Equal(t, binaryLen, n)
	}))
	tb.Mux.HandleFunc("/compare-bin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)
		assert.True(t, bytes.Equal(binary, body))
	}))

	replace := func(s string) string {
		return strings.NewReplacer(
			"EXP_TEXT", text,
			"EXP_BIN_LEN", strconv.Itoa(binaryLen),
		).Replace(tb.Replacer.Replace(s))
	}

	_, err := rt.RunString(replace(`
		var expText = "EXP_TEXT";
		var expBinLength = EXP_BIN_LEN;

		// Check default behaviour with a unicode text
		var respTextImplicit = http.get("HTTPBIN_URL/get-text").body;
		if (respTextImplicit !== expText) {
			throw new Error("default response body should be '" + expText + "' but was '" + respTextImplicit + "'");
		}
		http.post("HTTPBIN_URL/compare-text", respTextImplicit);

		// Check discarding of responses
		var respNone = http.get("HTTPBIN_URL/get-text", { responseType: "none" }).body;
		if (respNone != null) {
			throw new Error("none response body should be null but was " + respNone);
		}

		// Check binary transmission of the text response as well
		var respBin = http.get("HTTPBIN_URL/get-text", { responseType: "binary" });

		// Convert a UTF-8 ArrayBuffer to a JS string
		var respBinText = String.fromCharCode.apply(null, new Uint8Array(respBin.body));
		var strConv = decodeURIComponent(escape(respBinText));
		if (strConv !== expText) {
			throw new Error("converted response body should be '" + expText + "' but was '" + strConv + "'");
		}
		http.post("HTTPBIN_URL/compare-text", respBin.body);

		// Check binary response
		var respBin = http.get("HTTPBIN_URL/get-bin", { responseType: "binary" });
		var respBinTyped = new Uint8Array(respBin.body);
		if (expBinLength !== respBinTyped.length) {
			throw new Error("response body length should be '" + expBinLength
							+ "' but was '" + respBinTyped.length + "'");
		}
		for(var i = 0; i < respBinTyped.length; i++) {
			if (respBinTyped[i] !== i%256) {
				throw new Error("expected value " + (i%256) + " to be at position "
								+ i + " but it was " + respBinTyped[i]);
			}
		}
		http.post("HTTPBIN_URL/compare-bin", respBin.body);

		// Check ArrayBuffer response
		var respBin = http.get("HTTPBIN_URL/get-bin", { responseType: "binary" }).body;
		if (respBin.byteLength !== expBinLength) {
			throw new Error("response body length should be '" + expBinLength + "' but was '" + respBin.byteLength + "'");
		}

		// Check ArrayBuffer responses with http.batch()
		var responses = http.batch([
			["GET", "HTTPBIN_URL/get-bin", null, { responseType: "binary" }],
			["GET", "HTTPBIN_URL/get-bin", null, { responseType: "binary" }],
		]);
		if (responses.length != 2) {
			throw new Error("expected 2 responses, received " + responses.length);
		}
		for (var i = 0; i < responses.length; i++) {
			if (responses[i].body.byteLength !== expBinLength) {
				throw new Error("response body length should be '"
					+ expBinLength + "' but was '" + responses[i].body.byteLength + "'");
			}
		}
	`))
	assert.NoError(t, err)

	// Verify that if we enable discardResponseBodies globally, the default value is none
	state.Options.DiscardResponseBodies = null.BoolFrom(true)

	_, err = rt.RunString(replace(`
		var expText = "EXP_TEXT";

		// Check default behaviour
		var respDefault = http.get("HTTPBIN_URL/get-text").body;
		if (respDefault !== null) {
			throw new Error("default response body should be discarded and null but was " + respDefault);
		}

		// Check explicit text response
		var respTextExplicit = http.get("HTTPBIN_URL/get-text", { responseType: "text" }).body;
		if (respTextExplicit !== expText) {
			throw new Error("text response body should be '" + expText + "' but was '" + respTextExplicit + "'");
		}
		http.post("HTTPBIN_URL/compare-text", respTextExplicit);
	`))
	assert.NoError(t, err)
}

func checkErrorCode(t testing.TB, tags *metrics.SampleTags, code int, msg string) {
	errorMsg, ok := tags.Get("error")
	if msg == "" {
		assert.False(t, ok)
	} else {
		assert.Contains(t, errorMsg, msg)
	}
	errorCodeStr, ok := tags.Get("error_code")
	if code == 0 {
		assert.False(t, ok)
	} else {
		errorCode, err := strconv.Atoi(errorCodeStr)
		assert.NoError(t, err)
		assert.Equal(t, code, errorCode)
	}
}

func TestErrorCodes(t *testing.T) {
	t.Parallel()
	tb, state, samples, rt, _ := newRuntime(t)
	state.Options.Throw = null.BoolFrom(false)
	sr := tb.Replacer.Replace

	// Handple paths with custom logic
	tb.Mux.HandleFunc("/no-location-redirect", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(302)
	}))
	tb.Mux.HandleFunc("/bad-location-redirect", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "h\t:/") // \n is forbidden
		w.WriteHeader(302)
	}))

	testCases := []struct {
		name                string
		status              int
		moreSamples         int
		expectedErrorCode   int
		expectedErrorMsg    string
		expectedScriptError string
		script              string
	}{
		{
			name:              "Unroutable",
			expectedErrorCode: 1101,
			expectedErrorMsg:  "lookup: no such host",
			script:            `var res = http.request("GET", "http://sdafsgdhfjg.com/");`,
		},

		{
			name:              "404",
			status:            404,
			expectedErrorCode: 1404,
			script:            `var res = http.request("GET", "HTTPBIN_URL/status/404");`,
		},
		{
			name:              "Unroutable redirect",
			expectedErrorCode: 1101,
			expectedErrorMsg:  "lookup: no such host",
			moreSamples:       1,
			script:            `var res = http.request("GET", "HTTPBIN_URL/redirect-to?url=http://dafsgdhfjg.com/");`,
		},
		{
			name:              "Bad location redirect",
			expectedErrorCode: 1000,
			expectedErrorMsg:  "failed to parse Location header \"h\\t:/\": ",
			script:            `var res = http.request("GET", "HTTPBIN_URL/bad-location-redirect");`,
		},
		{
			name:              "Missing protocol",
			expectedErrorCode: 1000,
			expectedErrorMsg:  `unsupported protocol scheme ""`,
			script:            `var res = http.request("GET", "dafsgdhfjg.com/");`,
		},
		{
			name:        "Too many redirects",
			status:      302,
			moreSamples: 2,
			script: `
			var res = http.get("HTTPBIN_URL/relative-redirect/3", {redirects: 2});
			if (res.url != "HTTPBIN_URL/relative-redirect/1") { throw new Error("incorrect URL: " + res.url) }`,
		},
		{
			name:              "Connection refused redirect",
			status:            0,
			moreSamples:       1,
			expectedErrorMsg:  `dial: connection refused`,
			expectedErrorCode: 1212,
			script: `
			var res = http.get("HTTPBIN_URL/redirect-to?url=http%3A%2F%2F127.0.0.1%3A1%2Fpesho");
			if (res.url != "http://127.0.0.1:1/pesho") { throw new Error("incorrect URL: " + res.url) }`,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		// clear the Samples
		metrics.GetBufferedSamples(samples)
		t.Run(testCase.name, func(t *testing.T) {
			_, err := rt.RunString(sr(testCase.script + "\n" + fmt.Sprintf(`
			if (res.status != %d) { throw new Error("wrong status: "+ res.status);}
			if (res.error.indexOf(%q, 0) === -1) { throw new Error("wrong error: '" + res.error + "'");}
			if (res.error_code != %d) { throw new Error("wrong error_code: "+ res.error_code);}
			`, testCase.status, testCase.expectedErrorMsg, testCase.expectedErrorCode)))
			if testCase.expectedScriptError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Equal(t, err.Error(), testCase.expectedScriptError)
			}
			cs := metrics.GetBufferedSamples(samples)
			assert.Len(t, cs, 1+testCase.moreSamples)
			for _, c := range cs[len(cs)-1:] {
				assert.NotZero(t, len(c.GetSamples()))
				for _, sample := range c.GetSamples() {
					checkErrorCode(t, sample.GetTags(), testCase.expectedErrorCode, testCase.expectedErrorMsg)
				}
			}
		})
	}
}

func TestResponseWaitingAndReceivingTimings(t *testing.T) {
	t.Parallel()
	tb, state, _, rt, _ := newRuntime(t)

	// We don't expect any failed requests
	state.Options.Throw = null.BoolFrom(true)

	tb.Mux.HandleFunc("/slow-response", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		time.Sleep(1200 * time.Millisecond)
		n, err := w.Write([]byte("1st bytes!"))
		assert.NoError(t, err)
		assert.Equal(t, 10, n)

		flusher.Flush()
		time.Sleep(1200 * time.Millisecond)

		n, err = w.Write([]byte("2nd bytes!"))
		assert.NoError(t, err)
		assert.Equal(t, 10, n)
	}))

	_, err := rt.RunString(tb.Replacer.Replace(`
		var resp = http.get("HTTPBIN_URL/slow-response");

		if (resp.timings.waiting < 1000) {
			throw new Error("expected waiting time to be over 1000ms but was " + resp.timings.waiting);
		}

		if (resp.timings.receiving < 1000) {
			throw new Error("expected receiving time to be over 1000ms but was " + resp.timings.receiving);
		}

		if (resp.body !== "1st bytes!2nd bytes!") {
			throw new Error("wrong response body: " + resp.body);
		}
	`))
	assert.NoError(t, err)
}

func TestResponseTimingsWhenTimeout(t *testing.T) {
	t.Parallel()
	tb, state, _, rt, _ := newRuntime(t)

	// We expect a failed request
	state.Options.Throw = null.BoolFrom(false)

	_, err := rt.RunString(tb.Replacer.Replace(`
		var resp = http.get("HTTPBIN_URL/delay/10", { timeout: 2500 });

		if (resp.timings.waiting < 2000) {
			throw new Error("expected waiting time to be over 2000ms but was " + resp.timings.waiting);
		}

		if (resp.timings.duration < 2000) {
			throw new Error("expected duration time to be over 2000ms but was " + resp.timings.duration);
		}
	`))
	assert.NoError(t, err)
}

func TestNoResponseBodyMangling(t *testing.T) {
	t.Parallel()
	tb, state, _, rt, _ := newRuntime(t)

	// We don't expect any failed requests
	state.Options.Throw = null.BoolFrom(true)

	_, err := rt.RunString(tb.Replacer.Replace(`
	    var batchSize = 100;

		var requests = [];

		for (var i = 0; i < batchSize; i++) {
			requests.push(["GET", "HTTPBIN_URL/get?req=" + i, null, { responseType: (i % 2 ? "binary" : "text") }]);
		}

		var responses = http.batch(requests);

		for (var i = 0; i < batchSize; i++) {
			var reqNumber = parseInt(responses[i].json().args.req[0], 10);
			if (i !== reqNumber) {
				throw new Error("Response " + i + " has " + reqNumber + ", expected " + i)
			}
		}
	`))
	assert.NoError(t, err)
}

func TestRedirectMetricTags(t *testing.T) {
	tb, _, samples, rt, _ := newRuntime(t)

	tb.Mux.HandleFunc("/redirect/post", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/get", http.StatusMovedPermanently)
	}))

	sr := tb.Replacer.Replace
	script := sr(`
		http.post("HTTPBIN_URL/redirect/post", {data: "some data"});
	`)

	_, err := rt.RunString(script)
	require.NoError(t, err)

	require.Len(t, samples, 2)

	checkTags := func(sc metrics.SampleContainer, expTags map[string]string) {
		allSamples := sc.GetSamples()
		assert.Len(t, allSamples, 9)
		for _, s := range allSamples {
			assert.Equal(t, expTags, s.Tags.CloneTags())
		}
	}
	expPOSTtags := map[string]string{
		"group":             "",
		"method":            "POST",
		"url":               sr("HTTPBIN_URL/redirect/post"),
		"name":              sr("HTTPBIN_URL/redirect/post"),
		"status":            "301",
		"proto":             "HTTP/1.1",
		"expected_response": "true",
	}
	expGETtags := map[string]string{
		"group":             "",
		"method":            "GET",
		"url":               sr("HTTPBIN_URL/get"),
		"name":              sr("HTTPBIN_URL/get"),
		"status":            "200",
		"proto":             "HTTP/1.1",
		"expected_response": "true",
	}
	checkTags(<-samples, expPOSTtags)
	checkTags(<-samples, expGETtags)
}

func BenchmarkHandlingOfResponseBodies(b *testing.B) {
	tb, state, samples, rt, _ := newRuntime(b)

	state.BPool = bpool.NewBufferPool(100)

	go func() {
		ctxDone := tb.Context.Done()
		for {
			select {
			case <-samples:
			case <-ctxDone:
				return
			}
		}
	}()

	mbData := bytes.Repeat([]byte("0123456789"), 100000)
	tb.Mux.HandleFunc("/1mbdata", http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		_, err := resp.Write(mbData)
		if err != nil {
			b.Error(err)
		}
	}))

	testCodeTemplate := tb.Replacer.Replace(`
		http.get("HTTPBIN_URL/", { responseType: "TEST_RESPONSE_TYPE" });
		http.post("HTTPBIN_URL/post", { responseType: "TEST_RESPONSE_TYPE" });
		http.batch([
			["GET", "HTTPBIN_URL/gzip", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/gzip", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/deflate", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/deflate", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/redirect/5", null, { responseType: "TEST_RESPONSE_TYPE" }], // 6 requests
			["GET", "HTTPBIN_URL/get", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/html", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/bytes/100000", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/image/png", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/image/jpeg", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/image/jpeg", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/image/webp", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/image/svg", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/forms/post", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/bytes/100000", null, { responseType: "TEST_RESPONSE_TYPE" }],
			["GET", "HTTPBIN_URL/stream-bytes/100000", null, { responseType: "TEST_RESPONSE_TYPE" }],
		]);
		http.get("HTTPBIN_URL/get", { responseType: "TEST_RESPONSE_TYPE" });
		http.get("HTTPBIN_URL/get", { responseType: "TEST_RESPONSE_TYPE" });
		http.get("HTTPBIN_URL/1mbdata", { responseType: "TEST_RESPONSE_TYPE" });
	`)

	testResponseType := func(responseType string) func(b *testing.B) {
		testCode := strings.Replace(testCodeTemplate, "TEST_RESPONSE_TYPE", responseType, -1)
		return func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := rt.RunString(testCode)
				if err != nil {
					b.Error(err)
				}
			}
		}
	}

	b.ResetTimer()
	b.Run("text", testResponseType("text"))
	b.Run("binary", testResponseType("binary"))
	b.Run("none", testResponseType("none"))
}

func TestErrorsWithDecompression(t *testing.T) {
	t.Parallel()
	tb, state, _, rt, _ := newRuntime(t)

	state.Options.Throw = null.BoolFrom(false)

	tb.Mux.HandleFunc("/broken-archive", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := r.URL.Query()["encoding"][0]
		w.Header().Set("Content-Encoding", enc)
		_, _ = fmt.Fprintf(w, "Definitely not %s, but it's all cool...", enc)
	}))

	_, err := rt.RunString(tb.Replacer.Replace(`
		function handleResponseEncodingError (encoding) {
			var resp = http.get("HTTPBIN_URL/broken-archive?encoding=" + encoding);
			if (resp.error_code != 1701) {
				throw new Error("Expected error_code 1701 for '" + encoding +"', but got " + resp.error_code);
			}
		}

		["gzip", "deflate", "br", "zstd"].forEach(handleResponseEncodingError);
	`))
	assert.NoError(t, err)
}

func TestRequestAndBatchTLS(t *testing.T) {
	t.Parallel()

	t.Run("cert_expired", func(t *testing.T) {
		t.Parallel()
		_, state, _, rt, _ := newRuntime(t)
		cert, key := GenerateTLSCertificate(t, "expired.localhost", time.Now().Add(-time.Hour), 0)
		s, client := GetTestServerWithCertificate(t, cert, key)
		go func() {
			_ = s.Config.Serve(s.Listener)
		}()
		t.Cleanup(func() {
			require.NoError(t, s.Config.Close())
		})
		host, port, err := net.SplitHostPort(s.Listener.Addr().String())
		require.NoError(t, err)
		ip := net.ParseIP(host)
		mybadsslHostname, err := lib.NewHostAddress(ip, port)
		require.NoError(t, err)
		state.Transport = client.Transport
		state.TLSConfig = s.TLS
		state.Dialer = &netext.Dialer{Hosts: map[string]*lib.HostAddress{"expired.localhost": mybadsslHostname}}
		client.Transport.(*http.Transport).DialContext = state.Dialer.DialContext //nolint:forcetypeassert
		_, err = rt.RunString(`throw JSON.stringify(http.get("https://expired.localhost/"));`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "x509: certificate has expired or is not yet valid")
	})
	tlsVersionTests := []struct {
		Name, URL, Version string
	}{
		{Name: "tls10", URL: "tlsv10.localhost", Version: "http.TLS_1_0"},
		{Name: "tls11", URL: "tlsv11.localhost", Version: "http.TLS_1_1"},
		{Name: "tls12", URL: "tlsv12.localhost", Version: "http.TLS_1_2"},
	}
	for _, versionTest := range tlsVersionTests {
		versionTest := versionTest
		t.Run(versionTest.Name, func(t *testing.T) {
			t.Parallel()
			_, state, samples, rt, _ := newRuntime(t)
			cert, key := GenerateTLSCertificate(t, versionTest.URL, time.Now(), time.Hour)
			s, client := GetTestServerWithCertificate(t, cert, key)

			switch versionTest.Name {
			case "tls10":
				s.TLS.MaxVersion = tls.VersionTLS10
			case "tls11":
				s.TLS.MaxVersion = tls.VersionTLS11
			case "tls12":
				s.TLS.MaxVersion = tls.VersionTLS12
			default:
				panic(versionTest.Name + " unsupported")
			}
			go func() {
				_ = s.Config.Serve(s.Listener)
			}()
			t.Cleanup(func() {
				require.NoError(t, s.Config.Close())
			})
			host, port, err := net.SplitHostPort(s.Listener.Addr().String())
			require.NoError(t, err)
			ip := net.ParseIP(host)
			mybadsslHostname, err := lib.NewHostAddress(ip, port)
			require.NoError(t, err)
			state.Dialer = &netext.Dialer{Hosts: map[string]*lib.HostAddress{
				versionTest.URL: mybadsslHostname,
			}}
			state.Transport = client.Transport
			state.TLSConfig = s.TLS
			client.Transport.(*http.Transport).DialContext = state.Dialer.DialContext //nolint:forcetypeassert
			realURL := "https://" + versionTest.URL + "/"
			_, err = rt.RunString(fmt.Sprintf(`
            var res = http.get("%s");
					if (res.tls_version != %s) { throw new Error("wrong TLS version: " + res.tls_version); }
				`, realURL, versionTest.Version))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", realURL, "", 200, "")
		})
	}
	tlsCipherSuiteTests := []struct {
		Name, URL, CipherSuite string
		suite                  uint16
	}{
		{Name: "cipher_suite_cbc", URL: "cbc.localhost", CipherSuite: "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA", suite: tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA}, // TODO fix this to RSA instead of ECDSA
		{Name: "cipher_suite_ecc384", URL: "ecc384.localhost", CipherSuite: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", suite: tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256},
	}
	for _, cipherSuiteTest := range tlsCipherSuiteTests {
		cipherSuiteTest := cipherSuiteTest
		t.Run(cipherSuiteTest.Name, func(t *testing.T) {
			t.Parallel()
			_, state, samples, rt, _ := newRuntime(t)
			cert, key := GenerateTLSCertificate(t, cipherSuiteTest.URL, time.Now(), time.Hour)
			s, client := GetTestServerWithCertificate(t, cert, key, cipherSuiteTest.suite)
			go func() {
				_ = s.Config.Serve(s.Listener)
			}()
			t.Cleanup(func() {
				require.NoError(t, s.Config.Close())
			})
			host, port, err := net.SplitHostPort(s.Listener.Addr().String())
			require.NoError(t, err)
			ip := net.ParseIP(host)
			mybadsslHostname, err := lib.NewHostAddress(ip, port)
			require.NoError(t, err)
			state.Dialer = &netext.Dialer{Hosts: map[string]*lib.HostAddress{
				cipherSuiteTest.URL: mybadsslHostname,
			}}
			state.Transport = client.Transport
			state.TLSConfig = s.TLS
			client.Transport.(*http.Transport).DialContext = state.Dialer.DialContext //nolint:forcetypeassert
			realURL := "https://" + cipherSuiteTest.URL + "/"
			_, err = rt.RunString(fmt.Sprintf(`
					var res = http.get("%s");
					if (res.tls_cipher_suite != "%s") { throw new Error("wrong TLS cipher suite: " + res.tls_cipher_suite); }
				`, realURL, cipherSuiteTest.CipherSuite))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", realURL, "", 200, "")
		})
	}
	t.Run("ocsp_stapled_good", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("this doesn't work on windows for some reason")
		}
		website := "https://www.wikipedia.org/"
		tb, state, samples, rt, _ := newRuntime(t)
		state.Dialer = tb.Dialer
		_, err := rt.RunString(fmt.Sprintf(`
			var res = http.request("GET", "%s");
			if (res.ocsp.status != http.OCSP_STATUS_GOOD) { throw new Error("wrong ocsp stapled response status: " + res.ocsp.status); }
			`, website))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", website, "", 200, "")
	})
}

func TestDigestAuthWithBody(t *testing.T) {
	t.Parallel()
	tb, state, samples, rt, _ := newRuntime(t)

	state.Options.Throw = null.BoolFrom(true)
	state.Options.HTTPDebug = null.StringFrom("full")

	tb.Mux.HandleFunc("/digest-auth-with-post/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		body, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "super secret body", string(body))
		httpbin.New().DigestAuth(w, r) // this doesn't read the body
	}))

	urlWithCreds := tb.Replacer.Replace(
		"http://testuser:testpwd@HTTPBIN_IP:HTTPBIN_PORT/digest-auth-with-post/auth/testuser/testpwd",
	)

	_, err := rt.RunString(fmt.Sprintf(`
		var res = http.post(%q, "super secret body", { auth: "digest" });
		if (res.status !== 200) { throw new Error("wrong status: " + res.status); }
		if (res.error_code !== 0) { throw new Error("wrong error code: " + res.error_code); }
	`, urlWithCreds))
	require.NoError(t, err)

	urlRaw := tb.Replacer.Replace(
		"http://HTTPBIN_IP:HTTPBIN_PORT/digest-auth-with-post/auth/testuser/testpwd")

	sampleContainers := metrics.GetBufferedSamples(samples)
	assertRequestMetricsEmitted(t, sampleContainers[0:1], "POST", urlRaw, urlRaw, 401, "")
	assertRequestMetricsEmitted(t, sampleContainers[1:2], "POST", urlRaw, urlRaw, 200, "")
}

func TestBinaryResponseWithStatus0(t *testing.T) {
	t.Parallel()
	_, state, _, rt, _ := newRuntime(t) //nolint:dogsled
	state.Options.Throw = null.BoolFrom(false)
	_, err := rt.RunString(`
		var res = http.get("https://asdajkdahdqiuwhejkasdnakjdnadasdlkas.com", { responseType: "binary" });
		if (res.status !== 0) { throw new Error("wrong status: " + res.status); }
		if (res.body !== null) { throw new Error("wrong body: " + JSON.stringify(res.body)); }
	`)
	require.NoError(t, err)
}

func GenerateTLSCertificate(t *testing.T, host string, notBefore time.Time, validFor time.Duration) ([]byte, []byte) {
	priv, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	// priv, err := ecdsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// ECDSA, ED25519 and RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	keyUsage := x509.KeyUsageDigitalSignature
	// Only RSA subject keys should have the KeyEncipherment KeyUsage bits set. In
	// the context of TLS this KeyUsage is particular to RSA key exchange and
	// authentication.
	keyUsage |= x509.KeyUsageKeyEncipherment

	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	template.IsCA = true
	template.KeyUsage |= x509.KeyUsageCertSign

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	require.NoError(t, err)

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	keyPem := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	require.NoError(t, err)
	return certPem, keyPem
}

func GetTestServerWithCertificate(t *testing.T, certPem, key []byte, suitesIds ...uint16) (*httptest.Server, *http.Client) {
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		}),
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       time.Second,
	}
	s := &httptest.Server{}
	s.Config = server

	s.TLS = new(tls.Config)
	if s.TLS.NextProtos == nil {
		nextProtos := []string{"http/1.1"}
		if s.EnableHTTP2 {
			nextProtos = []string{"h2"}
		}
		s.TLS.NextProtos = nextProtos
	}
	cert, err := tls.X509KeyPair(certPem, key)
	require.NoError(t, err)
	s.TLS.Certificates = append(s.TLS.Certificates, cert)
	suites := tls.CipherSuites()
	if len(suitesIds) > 0 {
		newSuites := make([]*tls.CipherSuite, 0, len(suitesIds))
		for _, suite := range suites {
			for _, id := range suitesIds {
				if id == suite.ID {
					newSuites = append(newSuites, suite)
				}
			}
		}
		suites = newSuites
	}
	if len(suites) == 0 {
		panic("no suites enabled")
	}
	for _, suite := range suites {
		s.TLS.CipherSuites = append(s.TLS.CipherSuites, suite.ID)
	}
	certpool := x509.NewCertPool()
	certificate, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	certpool.AddCert(certificate)
	client := &http.Client{Transport: &http.Transport{}}
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{ //nolint:gosec
			RootCAs:      certpool,
			MinVersion:   tls.VersionTLS10,
			MaxVersion:   tls.VersionTLS12, // this so that the ciphersuite work
			CipherSuites: suitesIds,
		},
		ForceAttemptHTTP2:     s.EnableHTTP2,
		TLSHandshakeTimeout:   time.Second,
		ResponseHeaderTimeout: time.Second,
		IdleConnTimeout:       time.Second,
	}
	s.Listener, err = net.Listen("tcp", "")
	require.NoError(t, err)
	s.Listener = tls.NewListener(s.Listener, s.TLS)
	return s, client
}
