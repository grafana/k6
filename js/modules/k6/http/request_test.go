/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package http

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/dop251/goja"
	"github.com/klauspost/compress/zstd"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/stats"
	"github.com/mccutchen/go-httpbin/httpbin"
	"github.com/oxtoacart/bpool"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v4"
)

func assertRequestMetricsEmitted(t *testing.T, sampleContainers []stats.SampleContainer, method, url, name string, status int, group string) {
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
				switch sample.Metric {
				case metrics.HTTPReqDuration:
					seenDuration = true
				case metrics.HTTPReqBlocked:
					seenBlocked = true
				case metrics.HTTPReqConnecting:
					seenConnecting = true
				case metrics.HTTPReqTLSHandshaking:
					seenTLSHandshaking = true
				case metrics.HTTPReqSending:
					seenSending = true
				case metrics.HTTPReqWaiting:
					seenWaiting = true
				case metrics.HTTPReqReceiving:
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

func newRuntime(
	t testing.TB,
) (*httpmultibin.HTTPMultiBin, *lib.State, chan stats.SampleContainer, *goja.Runtime, *context.Context) {
	tb := httpmultibin.NewHTTPMultiBin(t)

	root, err := lib.NewGroup("", nil)
	require.NoError(t, err)

	logger := logrus.New()
	logger.Level = logrus.DebugLevel

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	options := lib.Options{
		MaxRedirects: null.IntFrom(10),
		UserAgent:    null.StringFrom("TestUserAgent"),
		Throw:        null.BoolFrom(true),
		SystemTags:   &stats.DefaultSystemTagSet,
		Batch:        null.IntFrom(20),
		BatchPerHost: null.IntFrom(20),
		//HTTPDebug:    null.StringFrom("full"),
	}
	samples := make(chan stats.SampleContainer, 1000)

	state := &lib.State{
		Options:   options,
		Logger:    logger,
		Group:     root,
		TLSConfig: tb.TLSClientConfig,
		Transport: tb.HTTPTransport,
		BPool:     bpool.NewBufferPool(1),
		Samples:   samples,
		Tags:      map[string]string{"group": root.Path},
	}

	ctx := new(context.Context)
	*ctx = lib.WithState(tb.Context, state)
	*ctx = common.WithRuntime(*ctx, rt)
	rt.Set("http", common.Bind(rt, New(), ctx))

	return tb, state, samples, rt, ctx
}

func TestRequestAndBatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	t.Parallel()
	tb, state, samples, rt, ctx := newRuntime(t)
	defer tb.Cleanup()
	sr := tb.Replacer.Replace

	// Handle paths with custom logic
	tb.Mux.HandleFunc("/digest-auth/failure", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))

	t.Run("Redirects", func(t *testing.T) {
		t.Run("tracing", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
			var res = http.get("HTTPBIN_URL/redirect/9");
			`))
			assert.NoError(t, err)
			bufSamples := stats.GetBufferedSamples(samples)

			reqsCount := 0
			for _, container := range bufSamples {
				for _, sample := range container.GetSamples() {
					if sample.Metric.Name == "http_reqs" {
						reqsCount++
					}
				}
			}

			assert.Equal(t, 10, reqsCount)
			assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get"), sr("HTTPBIN_URL/redirect/9"), 200, "")
		})

		t.Run("10", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`http.get("HTTPBIN_URL/redirect/10")`))
			assert.NoError(t, err)
		})
		t.Run("11", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
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

				_, err := common.RunString(rt, sr(`
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
			_, err := common.RunString(rt, sr(`
			var res = http.get("HTTPBIN_URL/redirect/1", {redirects: 3});
			if (res.status != 200) { throw new Error("wrong status: " + res.status) }
			if (res.url != "HTTPBIN_URL/get") { throw new Error("incorrect URL: " + res.url) }
			`))
			assert.NoError(t, err)
		})
		t.Run("requestScopeNoRedirects", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
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
			_, err := common.RunString(rt, sr(`
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
			_, err := common.RunString(rt, sr(`
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
			_, err := common.RunString(rt, sr(`
				http.get("HTTPBIN_URL/delay/10", {
					timeout: 1*1000,
				})
			`))
			endTime := time.Now()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "context deadline exceeded")
			assert.WithinDuration(t, startTime.Add(1*time.Second), endTime, 2*time.Second)

			logEntry := hook.LastEntry()
			if assert.NotNil(t, logEntry) {
				assert.Equal(t, logrus.WarnLevel, logEntry.Level)
				assert.Contains(t, logEntry.Data["error"].(error).Error(), "context deadline exceeded")
				assert.Equal(t, "Request Failed", logEntry.Message)
			}
		})
	})
	t.Run("UserAgent", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			var res = http.get("HTTPBIN_URL/user-agent");
			if (res.json()['user-agent'] != "TestUserAgent") {
				throw new Error("incorrect user agent: " + res.json()['user-agent'])
			}
		`))
		assert.NoError(t, err)

		t.Run("Override", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				var res = http.get("HTTPBIN_URL/user-agent", {
					headers: { "User-Agent": "OtherUserAgent" },
				});
				if (res.json()['user-agent'] != "OtherUserAgent") {
					throw new Error("incorrect user agent: " + res.json()['user-agent'])
				}
			`))
			assert.NoError(t, err)
		})
	})
	t.Run("Compression", func(t *testing.T) {
		t.Run("gzip", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				var res = http.get("HTTPSBIN_IP_URL/gzip");
				if (res.json()['gzipped'] != true) {
					throw new Error("unexpected body data: " + res.json()['gzipped'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("deflate", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				var res = http.get("HTTPBIN_URL/deflate");
				if (res.json()['deflated'] != true) {
					throw new Error("unexpected body data: " + res.json()['deflated'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("zstd", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				var res = http.get("HTTPSBIN_IP_URL/zstd");
				if (res.json()['compression'] != 'zstd') {
					throw new Error("unexpected body data: " + res.json()['compression'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("brotli", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				var res = http.get("HTTPSBIN_IP_URL/brotli");
				if (res.json()['compression'] != 'br') {
					throw new Error("unexpected body data: " + res.json()['compression'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("zstd-br", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
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

			_, err := common.RunString(rt, sr(`
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
			_, err := common.RunString(rt, sr(`
				var params = { headers: { "Accept-Encoding": "gzip" } };
				var res = http.get("HTTPBIN_URL/gzip", params);
				if (res.json()['gzipped'] != true) {
					throw new Error("unexpected body data: " + res.json()['gzipped'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("deflate", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				var params = { headers: { "Accept-Encoding": "deflate" } };
				var res = http.get("HTTPBIN_URL/deflate", params);
				if (res.json()['deflated'] != true) {
					throw new Error("unexpected body data: " + res.json()['deflated'])
				}
			`))
			assert.NoError(t, err)
		})
	})
	t.Run("Cancelled", func(t *testing.T) {
		hook := logtest.NewLocal(state.Logger)
		defer hook.Reset()

		oldctx := *ctx
		newctx, cancel := context.WithCancel(oldctx)
		cancel()
		*ctx = newctx
		defer func() { *ctx = oldctx }()

		_, err := common.RunString(rt, sr(`http.get("HTTPBIN_URL/get/");`))
		assert.Error(t, err)
		assert.Nil(t, hook.LastEntry())
	})
	t.Run("HTTP/2", func(t *testing.T) {
		stats.GetBufferedSamples(samples) // Clean up buffered samples from previous tests
		_, err := common.RunString(rt, sr(`
		var res = http.request("GET", "HTTP2BIN_URL/get");
		if (res.status != 200) { throw new Error("wrong status: " + res.status) }
		if (res.proto != "HTTP/2.0") { throw new Error("wrong proto: " + res.proto) }
		`))
		assert.NoError(t, err)

		bufSamples := stats.GetBufferedSamples(samples)
		assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTP2BIN_URL/get"), "", 200, "")
		for _, sampleC := range bufSamples {
			for _, sample := range sampleC.GetSamples() {
				proto, ok := sample.Tags.Get("proto")
				assert.True(t, ok)
				assert.Equal(t, "HTTP/2.0", proto)
			}
		}
	})
	t.Run("TLS", func(t *testing.T) {
		t.Run("cert_expired", func(t *testing.T) {
			_, err := common.RunString(rt, `http.get("https://expired.badssl.com/");`)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "x509: certificate has expired or is not yet valid")
		})
		tlsVersionTests := []struct {
			Name, URL, Version string
		}{
			{Name: "tls10", URL: "https://tls-v1-0.badssl.com:1010/", Version: "http.TLS_1_0"},
			{Name: "tls11", URL: "https://tls-v1-1.badssl.com:1011/", Version: "http.TLS_1_1"},
			{Name: "tls12", URL: "https://badssl.com/", Version: "http.TLS_1_2"},
		}
		for _, versionTest := range tlsVersionTests {
			t.Run(versionTest.Name, func(t *testing.T) {
				_, err := common.RunString(rt, fmt.Sprintf(`
					var res = http.get("%s");
					if (res.tls_version != %s) { throw new Error("wrong TLS version: " + res.tls_version); }
				`, versionTest.URL, versionTest.Version))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", versionTest.URL, "", 200, "")
			})
		}
		tlsCipherSuiteTests := []struct {
			Name, URL, CipherSuite string
		}{
			{Name: "cipher_suite_cbc", URL: "https://cbc.badssl.com/", CipherSuite: "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA"},
			{Name: "cipher_suite_ecc384", URL: "https://ecc384.badssl.com/", CipherSuite: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"},
		}
		for _, cipherSuiteTest := range tlsCipherSuiteTests {
			t.Run(cipherSuiteTest.Name, func(t *testing.T) {
				_, err := common.RunString(rt, fmt.Sprintf(`
					var res = http.get("%s");
					if (res.tls_cipher_suite != "%s") { throw new Error("wrong TLS cipher suite: " + res.tls_cipher_suite); }
				`, cipherSuiteTest.URL, cipherSuiteTest.CipherSuite))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", cipherSuiteTest.URL, "", 200, "")
			})
		}
		t.Run("ocsp_stapled_good", func(t *testing.T) {
			_, err := common.RunString(rt, `
			var res = http.request("GET", "https://www.microsoft.com/");
			if (res.ocsp.status != http.OCSP_STATUS_GOOD) { throw new Error("wrong ocsp stapled response status: " + res.ocsp.status); }
			`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", "https://www.microsoft.com/", "", 200, "")
		})
	})
	t.Run("Invalid", func(t *testing.T) {
		hook := logtest.NewLocal(state.Logger)
		defer hook.Reset()

		_, err := common.RunString(rt, `http.request("", "");`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported protocol scheme")

		logEntry := hook.LastEntry()
		if assert.NotNil(t, logEntry) {
			assert.Equal(t, logrus.WarnLevel, logEntry.Level)
			assert.Contains(t, logEntry.Data["error"].(error).Error(), "unsupported protocol scheme")
			assert.Equal(t, "Request Failed", logEntry.Message)
		}

		t.Run("throw=false", func(t *testing.T) {
			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			_, err := common.RunString(rt, `
				var res = http.request("", "", { throw: false });
				throw new Error(res.error);
			`)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported protocol scheme")

			logEntry := hook.LastEntry()
			if assert.NotNil(t, logEntry) {
				assert.Equal(t, logrus.WarnLevel, logEntry.Level)
				assert.Contains(t, logEntry.Data["error"].(error).Error(), "unsupported protocol scheme")
				assert.Equal(t, "Request Failed", logEntry.Message)
			}
		})
	})
	t.Run("Unroutable", func(t *testing.T) {
		_, err := common.RunString(rt, `http.request("GET", "http://sdafsgdhfjg/");`)
		assert.Error(t, err)
	})

	t.Run("Params", func(t *testing.T) {
		for _, literal := range []string{`undefined`, `null`} {
			t.Run(literal, func(t *testing.T) {
				_, err := common.RunString(rt, fmt.Sprintf(sr(`
				var res = http.request("GET", "HTTPBIN_URL/headers", null, %s);
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`), literal))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
			})
		}

		t.Run("cookies", func(t *testing.T) {
			t.Run("access", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = common.RunString(rt, sr(`
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
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies/set?key=value"), "", 302, "")
			})

			t.Run("vuJar", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = common.RunString(rt, sr(`
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
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("requestScope", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = common.RunString(rt, sr(`
				var res = http.request("GET", "HTTPBIN_URL/cookies", null, { cookies: { key: "value" } });
				if (res.json().key != "value") { throw new Error("wrong cookie value: " + res.json().key); }
				var jar = http.cookieJar();
				var jarCookies = jar.cookiesForURL("HTTPBIN_URL/cookies");
				if (jarCookies.key != undefined) { throw new Error("unexpected cookie in jar"); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("requestScopeReplace", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = common.RunString(rt, sr(`
				var jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value");
				var res = http.request("GET", "HTTPBIN_URL/cookies", null, { cookies: { key: { value: "replaced", replace: true } } });
				if (res.json().key != "replaced") { throw new Error("wrong cookie value: " + res.json().key); }
				var jarCookies = jar.cookiesForURL("HTTPBIN_URL/cookies");
				if (jarCookies.key[0] != "value") { throw new Error("wrong cookie value in jar"); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
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
					_, err = common.RunString(rt, sr(`
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
						stats.GetBufferedSamples(samples),
						"GET",
						sr("HTTPSBIN_URL/set-cookie-without-redirect"),
						sr("HTTPBIN_URL/redirect-to?url=HTTPSBIN_URL/set-cookie-without-redirect"),
						200,
						"",
					)
				})
				t.Run("set cookie before redirect", func(t *testing.T) {
					cookieJar, err := cookiejar.New(nil)
					require.NoError(t, err)
					state.CookieJar = cookieJar
					_, err = common.RunString(rt, sr(`
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
						stats.GetBufferedSamples(samples),
						"GET",
						sr("HTTPSBIN_URL/cookies"),
						sr("HTTPSBIN_URL/cookies/set?key=value"),
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

					_, err = common.RunString(rt, sr(`
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
						stats.GetBufferedSamples(samples),
						"GET",
						sr("HTTPBIN_IP_URL/get"),
						sr("HTTPBIN_IP_URL/redirect-to?url=HTTPSBIN_URL/set-cookie-and-redirect"),
						200,
						"",
					)
				})
			})

			t.Run("domain", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = common.RunString(rt, sr(`
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
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("path", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = common.RunString(rt, sr(`
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
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("expires", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = common.RunString(rt, sr(`
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
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("secure", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = common.RunString(rt, sr(`
				var jar = http.cookieJar();
				jar.set("HTTPSBIN_IP_URL/cookies", "key", "value", { secure: true });
				var res = http.request("GET", "HTTPSBIN_IP_URL/cookies");
				if (res.json().key != "value") {
					throw new Error("wrong cookie value: " + res.json().key);
				}
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPSBIN_IP_URL/cookies"), "", 200, "")
			})

			t.Run("localJar", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				_, err = common.RunString(rt, sr(`
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
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})
		})

		t.Run("auth", func(t *testing.T) {
			t.Run("basic", func(t *testing.T) {
				url := sr("http://bob:pass@HTTPBIN_IP:HTTPBIN_PORT/basic-auth/bob/pass")
				urlExpected := sr("http://****:****@HTTPBIN_IP:HTTPBIN_PORT/basic-auth/bob/pass")

				_, err := common.RunString(rt, fmt.Sprintf(`
				var res = http.request("GET", "%s", null, {});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`, url))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", urlExpected, "", 200, "")
			})
			t.Run("digest", func(t *testing.T) {
				t.Run("success", func(t *testing.T) {
					url := sr("http://bob:pass@HTTPBIN_IP:HTTPBIN_PORT/digest-auth/auth/bob/pass")
					urlExpected := sr("http://****:****@HTTPBIN_IP:HTTPBIN_PORT/digest-auth/auth/bob/pass")

					_, err := common.RunString(rt, fmt.Sprintf(`
					var res = http.request("GET", "%s", null, { auth: "digest" });
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					if (res.error_code != 0) { throw new Error("wrong error code: " + res.error_code); }
					`, url))
					assert.NoError(t, err)

					sampleContainers := stats.GetBufferedSamples(samples)
					assertRequestMetricsEmitted(t, sampleContainers[0:1], "GET",
						sr("HTTPBIN_IP_URL/digest-auth/auth/bob/pass"), urlExpected, 401, "")
					assertRequestMetricsEmitted(t, sampleContainers[1:2], "GET",
						sr("HTTPBIN_IP_URL/digest-auth/auth/bob/pass"), urlExpected, 200, "")
				})
				t.Run("failure", func(t *testing.T) {
					url := sr("http://bob:pass@HTTPBIN_IP:HTTPBIN_PORT/digest-auth/failure")

					_, err := common.RunString(rt, fmt.Sprintf(`
					var res = http.request("GET", "%s", null, { auth: "digest", timeout: 1, throw: false });
					`, url))
					assert.NoError(t, err)
				})
			})
		})

		t.Run("headers", func(t *testing.T) {
			for _, literal := range []string{`null`, `undefined`} {
				t.Run(literal, func(t *testing.T) {
					_, err := common.RunString(rt, fmt.Sprintf(sr(`
					var res = http.request("GET", "HTTPBIN_URL/headers", null, { headers: %s });
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`), literal))
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
				})
			}

			t.Run("object", func(t *testing.T) {
				_, err := common.RunString(rt, sr(`
				var res = http.request("GET", "HTTPBIN_URL/headers", null, {
					headers: { "X-My-Header": "value" },
				});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().headers["X-My-Header"] != "value") { throw new Error("wrong X-My-Header: " + res.json().headers["X-My-Header"]); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
			})

			t.Run("Host", func(t *testing.T) {
				_, err := common.RunString(rt, sr(`
				var res = http.request("GET", "HTTPBIN_URL/headers", null, {
					headers: { "Host": "HTTPBIN_DOMAIN" },
				});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().headers["Host"] != "HTTPBIN_DOMAIN") { throw new Error("wrong Host: " + res.json().headers["Host"]); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
			})
		})

		t.Run("tags", func(t *testing.T) {
			for _, literal := range []string{`null`, `undefined`} {
				t.Run(literal, func(t *testing.T) {
					_, err := common.RunString(rt, fmt.Sprintf(sr(`
					var res = http.request("GET", "HTTPBIN_URL/headers", null, { tags: %s });
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`), literal))
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
				})
			}

			t.Run("object", func(t *testing.T) {
				_, err := common.RunString(rt, sr(`
				var res = http.request("GET", "HTTPBIN_URL/headers", null, { tags: { tag: "value" } });
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`))
				assert.NoError(t, err)
				bufSamples := stats.GetBufferedSamples(samples)
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
				state.Tags = map[string]string{"runtag1": "val1", "runtag2": "val2"}

				_, err := common.RunString(rt, sr(`
				var res = http.request("GET", "HTTPBIN_URL/headers", null, { tags: { method: "test", name: "myName", runtag1: "fromreq" } });
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`))
				assert.NoError(t, err)

				bufSamples := stats.GetBufferedSamples(samples)
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
		_, err := common.RunString(rt, sr(`
		var res = http.get("HTTPBIN_URL/get?a=1&b=2");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.json().args.a != "1") { throw new Error("wrong ?a: " + res.json().args.a); }
		if (res.json().args.b != "2") { throw new Error("wrong ?b: " + res.json().args.b); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/get?a=1&b=2"), "", 200, "")

		t.Run("Tagged", func(t *testing.T) {
			_, err := common.RunES6String(rt, `
			var a = "1";
			var b = "2";
			var res = http.get(http.url`+"`"+sr(`HTTPBIN_URL/get?a=${a}&b=${b}`)+"`"+`);
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			if (res.json().args.a != a) { throw new Error("wrong ?a: " + res.json().args.a); }
			if (res.json().args.b != b) { throw new Error("wrong ?b: " + res.json().args.b); }
			`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/get?a=1&b=2"), sr("HTTPBIN_URL/get?a=${}&b=${}"), 200, "")
		})
	})
	t.Run("HEAD", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		var res = http.head("HTTPBIN_URL/get?a=1&b=2");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.body.length != 0) { throw new Error("HEAD responses shouldn't have a body"); }
		if (!res.headers["Content-Length"]) { throw new Error("Missing or invalid Content-Length header!"); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "HEAD", sr("HTTPBIN_URL/get?a=1&b=2"), "", 200, "")
	})

	t.Run("OPTIONS", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		var res = http.options("HTTPBIN_URL/?a=1&b=2");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (!res.headers["Access-Control-Allow-Methods"]) { throw new Error("Missing Access-Control-Allow-Methods header!"); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "OPTIONS", sr("HTTPBIN_URL/?a=1&b=2"), "", 200, "")
	})

	// DELETE HTTP requests shouldn't usually send a request body, they should use url parameters instead; references:
	// https://golang.org/pkg/net/http/#Request.ParseForm
	// https://stackoverflow.com/questions/299628/is-an-entity-body-allowed-for-an-http-delete-request
	// https://tools.ietf.org/html/rfc7231#section-4.3.5
	t.Run("DELETE", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		var res = http.del("HTTPBIN_URL/delete?test=mest");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.json().args.test != "mest") { throw new Error("wrong args: " + JSON.stringify(res.json().args)); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "DELETE", sr("HTTPBIN_URL/delete?test=mest"), "", 200, "")
	})

	postMethods := map[string]string{
		"POST":  "post",
		"PUT":   "put",
		"PATCH": "patch",
	}
	for method, fn := range postMethods {
		t.Run(method, func(t *testing.T) {
			_, err := common.RunString(rt, fmt.Sprintf(sr(`
				var res = http.%s("HTTPBIN_URL/%s", "data");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().data != "data") { throw new Error("wrong data: " + res.json().data); }
				if (res.json().headers["Content-Type"]) { throw new Error("content type set: " + res.json().headers["Content-Type"]); }
				`), fn, strings.ToLower(method)))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), method, sr("HTTPBIN_URL/")+strings.ToLower(method), "", 200, "")

			t.Run("object", func(t *testing.T) {
				_, err := common.RunString(rt, fmt.Sprintf(sr(`
				var res = http.%s("HTTPBIN_URL/%s", {a: "a", b: 2});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().form.a != "a") { throw new Error("wrong a=: " + res.json().form.a); }
				if (res.json().form.b != "2") { throw new Error("wrong b=: " + res.json().form.b); }
				if (res.json().headers["Content-Type"] != "application/x-www-form-urlencoded") { throw new Error("wrong content type: " + res.json().headers["Content-Type"]); }
				`), fn, strings.ToLower(method)))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), method, sr("HTTPBIN_URL/")+strings.ToLower(method), "", 200, "")
				t.Run("Content-Type", func(t *testing.T) {
					_, err := common.RunString(rt, fmt.Sprintf(sr(`
						var res = http.%s("HTTPBIN_URL/%s", {a: "a", b: 2}, {headers: {"Content-Type": "application/x-www-form-urlencoded; charset=utf-8"}});
						if (res.status != 200) { throw new Error("wrong status: " + res.status); }
						if (res.json().form.a != "a") { throw new Error("wrong a=: " + res.json().form.a); }
						if (res.json().form.b != "2") { throw new Error("wrong b=: " + res.json().form.b); }
						if (res.json().headers["Content-Type"] != "application/x-www-form-urlencoded; charset=utf-8") { throw new Error("wrong content type: " + res.json().headers["Content-Type"]); }
						`), fn, strings.ToLower(method)))
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), method, sr("HTTPBIN_URL/")+strings.ToLower(method), "", 200, "")
				})
			})
		})
	}

	t.Run("Batch", func(t *testing.T) {
		t.Run("error", func(t *testing.T) {
			_, err := common.RunString(rt, `var res = http.batch("https://somevalidurl.com");`)
			require.Error(t, err)
		})
		t.Run("GET", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
			var reqs = [
				["GET", "HTTPBIN_URL/"],
				["GET", "HTTPBIN_IP_URL/"],
			];
			var res = http.batch(reqs);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + res[key].status); }
				if (res[key].url != reqs[key][1]) { throw new Error("wrong url: " + res[key].url); }
			}`))
			require.NoError(t, err)
			bufSamples := stats.GetBufferedSamples(samples)
			assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/"), "", 200, "")
			assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")

			t.Run("Tagged", func(t *testing.T) {
				_, err := common.RunES6String(rt, sr(`
				let fragment = "get";
				let reqs = [
					["GET", http.url`+"`"+`HTTPBIN_URL/${fragment}`+"`"+`],
					["GET", http.url`+"`"+`HTTPBIN_IP_URL/`+"`"+`],
				];
				let res = http.batch(reqs);
				for (var key in res) {
					if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
					if (res[key].url != reqs[key][1].url) { throw new Error("wrong url: " + key + ": " + res[key].url + " != " + reqs[key][1].url); }
				}`))
				assert.NoError(t, err)
				bufSamples := stats.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get"), sr("HTTPBIN_URL/${}"), 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")
			})

			t.Run("Shorthand", func(t *testing.T) {
				_, err := common.RunString(rt, sr(`
				var reqs = [
					"HTTPBIN_URL/",
					"HTTPBIN_IP_URL/",
				];
				var res = http.batch(reqs);
				for (var key in res) {
					if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
					if (res[key].url != reqs[key]) { throw new Error("wrong url: " + key + ": " + res[key].url); }
				}`))
				assert.NoError(t, err)
				bufSamples := stats.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/"), "", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")

				t.Run("Tagged", func(t *testing.T) {
					_, err := common.RunES6String(rt, sr(`
					let fragment = "get";
					let reqs = [
						http.url`+"`"+`HTTPBIN_URL/${fragment}`+"`"+`,
						http.url`+"`"+`HTTPBIN_IP_URL/`+"`"+`,
					];
					let res = http.batch(reqs);
					for (var key in res) {
						if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
						if (res[key].url != reqs[key].url) { throw new Error("wrong url: " + key + ": " + res[key].url + " != " + reqs[key].url); }
					}`))
					assert.NoError(t, err)
					bufSamples := stats.GetBufferedSamples(samples)
					assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get"), sr("HTTPBIN_URL/${}"), 200, "")
					assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")
				})
			})

			t.Run("ObjectForm", func(t *testing.T) {
				_, err := common.RunString(rt, sr(`
				var reqs = [
					{ method: "GET", url: "HTTPBIN_URL/" },
					{ url: "HTTPBIN_IP_URL/", method: "GET"},
				];
				var res = http.batch(reqs);
				for (var key in res) {
					if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
					if (res[key].url != reqs[key].url) { throw new Error("wrong url: " + key + ": " + res[key].url + " != " + reqs[key].url); }
				}`))
				assert.NoError(t, err)
				bufSamples := stats.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/"), "", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")
			})

			t.Run("ObjectKeys", func(t *testing.T) {
				_, err := common.RunString(rt, sr(`
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
				bufSamples := stats.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get?r=shorthand"), "", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get?r=arr"), "arr", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get?r=obj1"), "", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/get?r=obj2"), "obj2", 200, "")
			})

			t.Run("BodyAndParams", func(t *testing.T) {
				testStr := "testbody"
				rt.Set("someStrFile", testStr)
				rt.Set("someBinFile", []byte(testStr))

				_, err := common.RunString(rt, sr(`
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
				bufSamples := stats.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "POST", sr("HTTPBIN_URL/post"), "", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "POST", sr("HTTPBIN_IP_URL/post"), "myname", 200, "")
			})
		})
		t.Run("POST", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
			var res = http.batch([ ["POST", "HTTPBIN_URL/post", { key: "value" }] ]);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
				if (res[key].json().form.key != "value") { throw new Error("wrong form: " + key + ": " + JSON.stringify(res[key].json().form)); }
			}`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "POST", sr("HTTPBIN_URL/post"), "", 200, "")
		})
		t.Run("PUT", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
			var res = http.batch([ ["PUT", "HTTPBIN_URL/put", { key: "value" }] ]);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
				if (res[key].json().form.key != "value") { throw new Error("wrong form: " + key + ": " + JSON.stringify(res[key].json().form)); }
			}`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "PUT", sr("HTTPBIN_URL/put"), "", 200, "")
		})
	})

	t.Run("HTTPRequest", func(t *testing.T) {
		t.Run("EmptyBody", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
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
			_, err := common.RunString(rt, sr(`
				var res = http.post("HTTPBIN_URL/post", {a: "a", b: 2}, {headers: {"Content-Type": "application/x-www-form-urlencoded; charset=utf-8"}});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.request["body"] != "a=a&b=2") { throw new Error("http request body was not set properly: " + JSON.stringify(res.request))}
				`))
			assert.NoError(t, err)
		})
	})
}

func TestRequestCompression(t *testing.T) {
	t.Parallel()
	tb, state, _, rt, _ := newRuntime(t)
	defer tb.Cleanup()

	logHook := testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.WarnLevel}}
	state.Logger.AddHook(&logHook)

	// We don't expect any failed requests
	state.Options.Throw = null.BoolFrom(true)

	var text = `
	Lorem ipsum dolor sit amet, consectetur adipiscing elit.
	Maecenas sed pharetra sapien. Nunc laoreet molestie ante ac gravida.
	Etiam interdum dui viverra posuere egestas. Pellentesque at dolor tristique,
	mattis turpis eget, commodo purus. Nunc orci aliquam.`

	var decompress = func(algo string, input io.Reader) io.Reader {
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
		var algos = strings.Split(actualEncoding, ", ")
		var compressedBuf = new(bytes.Buffer)
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

	var testCases = []struct {
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
			var algos = strings.Split(testCase.compression, ",")
			for i, algo := range algos {
				algos[i] = strings.TrimSpace(algo)
			}
			expectedEncoding = strings.Join(algos, ", ")
			actualEncoding = expectedEncoding
			_, err := common.RunES6String(rt, tb.Replacer.Replace(`
		http.post("HTTPBIN_URL/compressed-text", `+"`"+text+"`"+`,  {"compression": "`+testCase.compression+`"});
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
			_, err := common.RunES6String(rt, tb.Replacer.Replace(`
				http.post("HTTPBIN_URL/compressed-text", `+"`"+text+"`"+`,
					{"compression": "`+actualEncoding+`",
					 "headers": {"Content-Encoding": "`+expectedEncoding+`"}
					}
				);
			`))
			require.NoError(t, err)
			require.NotEmpty(t, logHook.Drain())
		})

		t.Run("encoding and length", func(t *testing.T) {
			_, err := common.RunES6String(rt, tb.Replacer.Replace(`
				http.post("HTTPBIN_URL/compressed-text", `+"`"+text+"`"+`,
					{"compression": "`+actualEncoding+`",
					 "headers": {"Content-Encoding": "`+expectedEncoding+`",
								 "Content-Length": "12"}
					}
				);
			`))
			require.NoError(t, err)
			require.NotEmpty(t, logHook.Drain())
		})

		expectedEncoding = actualEncoding
		t.Run("correct encoding", func(t *testing.T) {
			_, err := common.RunES6String(rt, tb.Replacer.Replace(`
				http.post("HTTPBIN_URL/compressed-text", `+"`"+text+"`"+`,
					{"compression": "`+actualEncoding+`",
					 "headers": {"Content-Encoding": "`+actualEncoding+`"}
					}
				);
			`))
			require.NoError(t, err)
			require.Empty(t, logHook.Drain())
		})

		//TODO: move to some other test?
		t.Run("correct length", func(t *testing.T) {
			_, err := common.RunString(rt, tb.Replacer.Replace(
				`http.post("HTTPBIN_URL/post", "0123456789", { "headers": {"Content-Length": "10"}});`,
			))
			require.NoError(t, err)
			require.Empty(t, logHook.Drain())
		})

		t.Run("content-length is set", func(t *testing.T) {
			_, err := common.RunString(rt, tb.Replacer.Replace(`
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
	defer tb.Cleanup()

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

	_, err := common.RunString(rt, replace(`
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
		var respTextInBin = http.get("HTTPBIN_URL/get-text", { responseType: "binary" }).body;

		// Hack to convert a utf-8 array to a JS string
		var strConv = "";
		function pad(n) { return n.length < 2 ? "0" + n : n; }
		for( var i = 0; i < respTextInBin.length; i++ ) {
			strConv += ( "%" + pad(respTextInBin[i].toString(16)));
		}
		strConv = decodeURIComponent(strConv);
		if (strConv !== expText) {
			throw new Error("converted response body should be '" + expText + "' but was '" + strConv + "'");
		}
		http.post("HTTPBIN_URL/compare-text", respTextInBin);

		// Check binary response
		var respBin = http.get("HTTPBIN_URL/get-bin", { responseType: "binary" }).body;
		if (respBin.length !== expBinLength) {
			throw new Error("response body length should be '" + expBinLength + "' but was '" + respBin.length + "'");
		}
		for( var i = 0; i < respBin.length; i++ ) {
			if ( respBin[i] !== i%256 ) {
				throw new Error("expected value " + (i%256) + " to be at position " + i + " but it was " + respBin[i]);
			}
		}
		http.post("HTTPBIN_URL/compare-bin", respBin);
	`))
	assert.NoError(t, err)

	// Verify that if we enable discardResponseBodies globally, the default value is none
	state.Options.DiscardResponseBodies = null.BoolFrom(true)

	_, err = common.RunString(rt, replace(`
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

func checkErrorCode(t testing.TB, tags *stats.SampleTags, code int, msg string) {
	var errorMsg, ok = tags.Get("error")
	if msg == "" {
		assert.False(t, ok)
	} else {
		assert.Contains(t, errorMsg, msg)
	}
	errorCodeStr, ok := tags.Get("error_code")
	if code == 0 {
		assert.False(t, ok)
	} else {
		var errorCode, err = strconv.Atoi(errorCodeStr)
		assert.NoError(t, err)
		assert.Equal(t, code, errorCode)
	}
}

func TestErrorCodes(t *testing.T) {
	t.Parallel()
	tb, state, samples, rt, _ := newRuntime(t)
	state.Options.Throw = null.BoolFrom(false)
	defer tb.Cleanup()
	sr := tb.Replacer.Replace

	// Handple paths with custom logic
	tb.Mux.HandleFunc("/no-location-redirect", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(302)
	}))
	tb.Mux.HandleFunc("/bad-location-redirect", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "h\t:/") // \n is forbidden
		w.WriteHeader(302)
	}))

	var testCases = []struct {
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
			script:            `var res = http.request("GET", "http://sdafsgdhfjg/");`,
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
			script:            `var res = http.request("GET", "HTTPBIN_URL/redirect-to?url=http://dafsgdhfjg/");`,
		},
		{
			name:              "Non location redirect",
			expectedErrorCode: 1000,
			expectedErrorMsg:  "302 response missing Location header",
			script:            `var res = http.request("GET", "HTTPBIN_URL/no-location-redirect");`,
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
			script:            `var res = http.request("GET", "dafsgdhfjg/");`,
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
		stats.GetBufferedSamples(samples)
		t.Run(testCase.name, func(t *testing.T) {
			_, err := common.RunString(rt,
				sr(testCase.script+"\n"+fmt.Sprintf(`
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
			var cs = stats.GetBufferedSamples(samples)
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
	defer tb.Cleanup()

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

	_, err := common.RunString(rt, tb.Replacer.Replace(`
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
	defer tb.Cleanup()

	// We expect a failed request
	state.Options.Throw = null.BoolFrom(false)

	_, err := common.RunString(rt, tb.Replacer.Replace(`
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
	defer tb.Cleanup()

	// We don't expect any failed requests
	state.Options.Throw = null.BoolFrom(true)

	_, err := common.RunString(rt, tb.Replacer.Replace(`
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

func BenchmarkHandlingOfResponseBodies(b *testing.B) {
	tb, state, samples, rt, _ := newRuntime(b)
	defer tb.Cleanup()

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
				_, err := common.RunString(rt, testCode)
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
	defer tb.Cleanup()

	state.Options.Throw = null.BoolFrom(false)

	tb.Mux.HandleFunc("/broken-archive", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := r.URL.Query()["encoding"][0]
		w.Header().Set("Content-Encoding", enc)
		_, _ = fmt.Fprintf(w, "Definitely not %s, but it's all cool...", enc)
	}))

	_, err := common.RunString(rt, tb.Replacer.Replace(`
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

func TestDigestAuthWithBody(t *testing.T) {
	t.Parallel()
	tb, state, samples, rt, _ := newRuntime(t)
	defer tb.Cleanup()

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

	_, err := common.RunString(rt, fmt.Sprintf(`
		var res = http.post(%q, "super secret body", { auth: "digest" });
		if (res.status !== 200) { throw new Error("wrong status: " + res.status); }
		if (res.error_code !== 0) { throw new Error("wrong error code: " + res.error_code); }
	`, urlWithCreds))
	require.NoError(t, err)

	expectedURL := tb.Replacer.Replace(
		"http://HTTPBIN_IP:HTTPBIN_PORT/digest-auth-with-post/auth/testuser/testpwd")
	expectedName := tb.Replacer.Replace(
		"http://****:****@HTTPBIN_IP:HTTPBIN_PORT/digest-auth-with-post/auth/testuser/testpwd")

	sampleContainers := stats.GetBufferedSamples(samples)
	assertRequestMetricsEmitted(t, sampleContainers[0:1], "POST", expectedURL, expectedName, 401, "")
	assertRequestMetricsEmitted(t, sampleContainers[1:2], "POST", expectedURL, expectedName, 200, "")
}
