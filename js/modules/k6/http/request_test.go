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
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/stats"
	"github.com/oxtoacart/bpool"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
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

func newRuntime(t *testing.T) (*testutils.HTTPMultiBin, *lib.State, chan stats.SampleContainer, *goja.Runtime, *context.Context) {
	tb := testutils.NewHTTPMultiBin(t)

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
		SystemTags:   lib.GetTagSet(lib.DefaultSystemTagList...),
		//HttpDebug:    null.StringFrom("full"),
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
	}

	ctx := new(context.Context)
	*ctx = context.Background()
	*ctx = lib.WithState(*ctx, state)
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

	// Handple paths with custom logic
	tb.Mux.HandleFunc("/digest-auth/failure", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	tb.Mux.HandleFunc("/set-cookie-before-redirect", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie := http.Cookie{
			Name:   "key-foo",
			Value:  "value-bar",
			Path:   "/",
			Domain: sr("HTTPBIN_DOMAIN"),
		}

		http.SetCookie(w, &cookie)

		http.Redirect(w, r, sr("HTTPBIN_URL/get"), http.StatusMovedPermanently)
	}))

	t.Run("Redirects", func(t *testing.T) {
		t.Run("tracing", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
			let res = http.get("HTTPBIN_URL/redirect/9");
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
			let res = http.get("HTTPBIN_URL/redirect/11");
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
				let res = http.get("HTTPBIN_URL/redirect/11");
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
			let res = http.get("HTTPBIN_URL/redirect/1", {redirects: 3});
			if (res.status != 200) { throw new Error("wrong status: " + res.status) }
			if (res.url != "HTTPBIN_URL/get") { throw new Error("incorrect URL: " + res.url) }
			`))
			assert.NoError(t, err)
		})
		t.Run("requestScopeNoRedirects", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
			let res = http.get("HTTPBIN_URL/redirect/1", {redirects: 0});
			if (res.status != 302) { throw new Error("wrong status: " + res.status) }
			if (res.url != "HTTPBIN_URL/redirect/1") { throw new Error("incorrect URL: " + res.url) }
			if (res.headers["Location"] != "/get") { throw new Error("incorrect Location header: " + res.headers["Location"]) }
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
			assert.EqualError(t, err, sr("GoError: Get HTTPBIN_URL/delay/10: net/http: request canceled (Client.Timeout exceeded while awaiting headers)"))
			assert.WithinDuration(t, startTime.Add(1*time.Second), endTime, 1*time.Second)

			logEntry := hook.LastEntry()
			if assert.NotNil(t, logEntry) {
				assert.Equal(t, logrus.WarnLevel, logEntry.Level)
				assert.EqualError(t, logEntry.Data["error"].(error), sr("Get HTTPBIN_URL/delay/10: net/http: request canceled (Client.Timeout exceeded while awaiting headers)"))
				assert.Equal(t, "Request Failed", logEntry.Message)
			}
		})
	})
	t.Run("UserAgent", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			let res = http.get("HTTPBIN_URL/user-agent");
			if (res.json()['user-agent'] != "TestUserAgent") {
				throw new Error("incorrect user agent: " + res.json()['user-agent'])
			}
		`))
		assert.NoError(t, err)

		t.Run("Override", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.get("HTTPBIN_URL/user-agent", {
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
				let res = http.get("HTTPSBIN_IP_URL/gzip");
				if (res.json()['gzipped'] != true) {
					throw new Error("unexpected body data: " + res.json()['gzipped'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("deflate", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.get("HTTPBIN_URL/deflate");
				if (res.json()['deflated'] != true) {
					throw new Error("unexpected body data: " + res.json()['deflated'])
				}
			`))
			assert.NoError(t, err)
		})
	})
	t.Run("CompressionWithAcceptEncodingHeader", func(t *testing.T) {
		t.Run("gzip", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let params = { headers: { "Accept-Encoding": "gzip" } };
				let res = http.get("HTTPBIN_URL/gzip", params);
				if (res.json()['gzipped'] != true) {
					throw new Error("unexpected body data: " + res.json()['gzipped'])
				}
			`))
			assert.NoError(t, err)
		})
		t.Run("deflate", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let params = { headers: { "Accept-Encoding": "deflate" } };
				let res = http.get("HTTPBIN_URL/deflate", params);
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
		_, err := common.RunString(rt, `
		let res = http.request("GET", "https://http2.akamai.com/demo");
		if (res.status != 200) { throw new Error("wrong status: " + res.status) }
		if (res.proto != "HTTP/2.0") { throw new Error("wrong proto: " + res.proto) }
		`)
		assert.NoError(t, err)

		bufSamples := stats.GetBufferedSamples(samples)
		assertRequestMetricsEmitted(t, bufSamples, "GET", "https://http2.akamai.com/demo", "", 200, "")
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
			assert.EqualError(t, err, "GoError: Get https://expired.badssl.com/: x509: certificate has expired or is not yet valid")
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
					let res = http.get("%s");
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
					let res = http.get("%s");
					if (res.tls_cipher_suite != "%s") { throw new Error("wrong TLS cipher suite: " + res.tls_cipher_suite); }
				`, cipherSuiteTest.URL, cipherSuiteTest.CipherSuite))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", cipherSuiteTest.URL, "", 200, "")
			})
		}
		t.Run("ocsp_stapled_good", func(t *testing.T) {
			_, err := common.RunString(rt, `
			let res = http.request("GET", "https://stackoverflow.com/");
			if (res.ocsp.status != http.OCSP_STATUS_GOOD) { throw new Error("wrong ocsp stapled response status: " + res.ocsp.status); }
			`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", "https://stackoverflow.com/", "", 200, "")
		})
	})
	t.Run("Invalid", func(t *testing.T) {
		hook := logtest.NewLocal(state.Logger)
		defer hook.Reset()

		_, err := common.RunString(rt, `http.request("", "");`)
		assert.EqualError(t, err, "GoError: Get : unsupported protocol scheme \"\"")

		logEntry := hook.LastEntry()
		if assert.NotNil(t, logEntry) {
			assert.Equal(t, logrus.WarnLevel, logEntry.Level)
			assert.Equal(t, "Get : unsupported protocol scheme \"\"", logEntry.Data["error"].(error).Error())
			assert.Equal(t, "Request Failed", logEntry.Message)
		}

		t.Run("throw=false", func(t *testing.T) {
			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			_, err := common.RunString(rt, `
				let res = http.request("", "", { throw: false });
				throw new Error(res.error);
			`)
			assert.EqualError(t, err, "GoError: Get : unsupported protocol scheme \"\"")

			logEntry := hook.LastEntry()
			if assert.NotNil(t, logEntry) {
				assert.Equal(t, logrus.WarnLevel, logEntry.Level)
				assert.EqualError(t, logEntry.Data["error"].(error), "Get : unsupported protocol scheme \"\"")
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
				let res = http.request("GET", "HTTPBIN_URL/headers", null, %s);
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
				let res = http.request("GET", "HTTPBIN_URL/cookies/set?key=value", null, { redirects: 0 });
				const props = ["name", "value", "domain", "path", "expires", "max_age", "secure", "http_only"];
				let cookie = res.cookies.key[0];
				for (let i = 0; i < props.length; i++) {
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
				let jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value");
				let res = http.request("GET", "HTTPBIN_URL/cookies", null, { cookies: { key2: "value2" } });
				if (res.json().key != "value") { throw new Error("wrong cookie value: " + res.json().key); }
				if (res.json().key2 != "value2") { throw new Error("wrong cookie value: " + res.json().key2); }
				let jarCookies = jar.cookiesForURL("HTTPBIN_URL/cookies");
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
				let res = http.request("GET", "HTTPBIN_URL/cookies", null, { cookies: { key: "value" } });
				if (res.json().key != "value") { throw new Error("wrong cookie value: " + res.json().key); }
				let jar = http.cookieJar();
				let jarCookies = jar.cookiesForURL("HTTPBIN_URL/cookies");
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
				let jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value");
				let res = http.request("GET", "HTTPBIN_URL/cookies", null, { cookies: { key: { value: "replaced", replace: true } } });
				if (res.json().key != "replaced") { throw new Error("wrong cookie value: " + res.json().key); }
				let jarCookies = jar.cookiesForURL("HTTPBIN_URL/cookies");
				if (jarCookies.key[0] != "value") { throw new Error("wrong cookie value in jar"); }
				`))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/cookies"), "", 200, "")
			})

			t.Run("redirect", func(t *testing.T) {
				t.Run("set cookie before redirect", func(t *testing.T) {
					cookieJar, err := cookiejar.New(nil)
					assert.NoError(t, err)
					state.CookieJar = cookieJar
					_, err = common.RunString(rt, sr(`
						let res = http.request("GET", "HTTPBIN_URL/set-cookie-before-redirect");
						if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`))
					assert.NoError(t, err)

					redirectURL, err := url.Parse(sr("HTTPBIN_URL"))
					assert.NoError(t, err)
					require.Len(t, cookieJar.Cookies(redirectURL), 1)
					assert.Equal(t, "key-foo", cookieJar.Cookies(redirectURL)[0].Name)
					assert.Equal(t, "value-bar", cookieJar.Cookies(redirectURL)[0].Value)

					assertRequestMetricsEmitted(
						t,
						stats.GetBufferedSamples(samples),
						"GET",
						sr("HTTPBIN_URL/get"),
						sr("HTTPBIN_URL/set-cookie-before-redirect"),
						200,
						"",
					)
				})
				t.Run("set cookie after redirect", func(t *testing.T) {
					cookieJar, err := cookiejar.New(nil)
					assert.NoError(t, err)
					state.CookieJar = cookieJar
					_, err = common.RunString(rt, sr(`
						let res = http.request("GET", "HTTPBIN_URL/redirect-to?url=HTTPSBIN_URL/cookies/set?key=value");
						if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`))
					assert.NoError(t, err)

					redirectURL, err := url.Parse(sr("HTTPSBIN_URL"))
					assert.NoError(t, err)

					require.Len(t, cookieJar.Cookies(redirectURL), 1)
					assert.Equal(t, "key", cookieJar.Cookies(redirectURL)[0].Name)
					assert.Equal(t, "value", cookieJar.Cookies(redirectURL)[0].Value)

					assertRequestMetricsEmitted(
						t,
						stats.GetBufferedSamples(samples),
						"GET",
						sr("HTTPSBIN_URL/cookies"),
						sr("HTTPBIN_URL/redirect-to?url=HTTPSBIN_URL/cookies/set?key=value"),
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
				let jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value", { domain: "HTTPBIN_DOMAIN" });
				let res = http.request("GET", "HTTPBIN_URL/cookies");
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
				let jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value", { path: "/cookies" });
				let res = http.request("GET", "HTTPBIN_URL/cookies");
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
				let jar = http.cookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value", { expires: "Sun, 24 Jul 1983 17:01:02 GMT" });
				let res = http.request("GET", "HTTPBIN_URL/cookies");
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
				let jar = http.cookieJar();
				jar.set("HTTPSBIN_IP_URL/cookies", "key", "value", { secure: true });
				let res = http.request("GET", "HTTPSBIN_IP_URL/cookies");
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
				let jar = new http.CookieJar();
				jar.set("HTTPBIN_URL/cookies", "key", "value");
				let res = http.request("GET", "HTTPBIN_URL/cookies", null, { cookies: { key2: "value2" }, jar: jar });
				if (res.json().key != "value") { throw new Error("wrong cookie value: " + res.json().key); }
				if (res.json().key2 != "value2") { throw new Error("wrong cookie value: " + res.json().key2); }
				let jarCookies = jar.cookiesForURL("HTTPBIN_URL/cookies");
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

				_, err := common.RunString(rt, fmt.Sprintf(`
				let res = http.request("GET", "%s", null, {});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`, url))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", url, "", 200, "")
			})
			t.Run("digest", func(t *testing.T) {
				t.Run("success", func(t *testing.T) {
					url := sr("http://bob:pass@HTTPBIN_IP:HTTPBIN_PORT/digest-auth/auth/bob/pass")

					_, err := common.RunString(rt, fmt.Sprintf(`
					let res = http.request("GET", "%s", null, { auth: "digest" });
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`, url))
					assert.NoError(t, err)

					sampleContainers := stats.GetBufferedSamples(samples)
					assertRequestMetricsEmitted(t, sampleContainers[0:1], "GET", sr("HTTPBIN_IP_URL/digest-auth/auth/bob/pass"), url, 401, "")
					assertRequestMetricsEmitted(t, sampleContainers[1:2], "GET", sr("HTTPBIN_IP_URL/digest-auth/auth/bob/pass"), url, 200, "")
				})
				t.Run("failure", func(t *testing.T) {
					url := sr("http://bob:pass@HTTPBIN_IP:HTTPBIN_PORT/digest-auth/failure")

					_, err := common.RunString(rt, fmt.Sprintf(`
					let res = http.request("GET", "%s", null, { auth: "digest", timeout: 1, throw: false });
					`, url))
					assert.NoError(t, err)
				})
			})
		})

		t.Run("headers", func(t *testing.T) {
			for _, literal := range []string{`null`, `undefined`} {
				t.Run(literal, func(t *testing.T) {
					_, err := common.RunString(rt, fmt.Sprintf(sr(`
					let res = http.request("GET", "HTTPBIN_URL/headers", null, { headers: %s });
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`), literal))
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
				})
			}

			t.Run("object", func(t *testing.T) {
				_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/headers", null, {
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
				let res = http.request("GET", "HTTPBIN_URL/headers", null, {
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
					let res = http.request("GET", "HTTPBIN_URL/headers", null, { tags: %s });
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`), literal))
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/headers"), "", 200, "")
				})
			}

			t.Run("object", func(t *testing.T) {
				_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/headers", null, { tags: { tag: "value" } });
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
				oldOpts := state.Options
				defer func() { state.Options = oldOpts }()
				state.Options.RunTags = stats.IntoSampleTags(&map[string]string{"runtag1": "val1", "runtag2": "val2"})

				_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/headers", null, { tags: { method: "test", name: "myName", runtag1: "fromreq" } });
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
		let res = http.get("HTTPBIN_URL/get?a=1&b=2");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.json().args.a != "1") { throw new Error("wrong ?a: " + res.json().args.a); }
		if (res.json().args.b != "2") { throw new Error("wrong ?b: " + res.json().args.b); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/get?a=1&b=2"), "", 200, "")

		t.Run("Tagged", func(t *testing.T) {
			_, err := common.RunString(rt, `
			let a = "1";
			let b = "2";
			let res = http.get(http.url`+"`"+sr(`HTTPBIN_URL/get?a=${a}&b=${b}`)+"`"+`);
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
		let res = http.head("HTTPBIN_URL/get?a=1&b=2");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.body.length != 0) { throw new Error("HEAD responses shouldn't have a body"); }
		if (!res.headers["Content-Length"]) { throw new Error("Missing or invalid Content-Length header!"); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "HEAD", sr("HTTPBIN_URL/get?a=1&b=2"), "", 200, "")
	})

	t.Run("OPTIONS", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
		let res = http.options("HTTPBIN_URL/?a=1&b=2");
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
		let res = http.del("HTTPBIN_URL/delete?test=mest");
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
				let res = http.%s("HTTPBIN_URL/%s", "data");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().data != "data") { throw new Error("wrong data: " + res.json().data); }
				if (res.json().headers["Content-Type"]) { throw new Error("content type set: " + res.json().headers["Content-Type"]); }
				`), fn, strings.ToLower(method)))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), method, sr("HTTPBIN_URL/")+strings.ToLower(method), "", 200, "")

			t.Run("object", func(t *testing.T) {
				_, err := common.RunString(rt, fmt.Sprintf(sr(`
				let res = http.%s("HTTPBIN_URL/%s", {a: "a", b: 2});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().form.a != "a") { throw new Error("wrong a=: " + res.json().form.a); }
				if (res.json().form.b != "2") { throw new Error("wrong b=: " + res.json().form.b); }
				if (res.json().headers["Content-Type"] != "application/x-www-form-urlencoded") { throw new Error("wrong content type: " + res.json().headers["Content-Type"]); }
				`), fn, strings.ToLower(method)))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), method, sr("HTTPBIN_URL/")+strings.ToLower(method), "", 200, "")
				t.Run("Content-Type", func(t *testing.T) {
					_, err := common.RunString(rt, fmt.Sprintf(sr(`
						let res = http.%s("HTTPBIN_URL/%s", {a: "a", b: 2}, {headers: {"Content-Type": "application/x-www-form-urlencoded; charset=utf-8"}});
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
		t.Run("GET", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
			let reqs = [
				["GET", "HTTPBIN_URL/"],
				["GET", "HTTPBIN_IP_URL/"],
			];
			let res = http.batch(reqs);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + res[key].status); }
				if (res[key].url != reqs[key][1]) { throw new Error("wrong url: " + res[key].url); }
			}`))
			assert.NoError(t, err)
			bufSamples := stats.GetBufferedSamples(samples)
			assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/"), "", 200, "")
			assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")

			t.Run("Tagged", func(t *testing.T) {
				_, err := common.RunString(rt, sr(`
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
				let reqs = [
					"HTTPBIN_URL/",
					"HTTPBIN_IP_URL/",
				];
				let res = http.batch(reqs);
				for (var key in res) {
					if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
					if (res[key].url != reqs[key]) { throw new Error("wrong url: " + key + ": " + res[key].url); }
				}`))
				assert.NoError(t, err)
				bufSamples := stats.GetBufferedSamples(samples)
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_URL/"), "", 200, "")
				assertRequestMetricsEmitted(t, bufSamples, "GET", sr("HTTPBIN_IP_URL/"), "", 200, "")

				t.Run("Tagged", func(t *testing.T) {
					_, err := common.RunString(rt, sr(`
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
				let reqs = [
					{ method: "GET", url: "HTTPBIN_URL/" },
					{ url: "HTTPBIN_IP_URL/", method: "GET"},
				];
				let res = http.batch(reqs);
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
				let reqs = {
					shorthand: "HTTPBIN_URL/get?r=shorthand",
					arr: ["GET", "HTTPBIN_URL/get?r=arr", null, {tags: {name: 'arr'}}],
					obj1: { method: "GET", url: "HTTPBIN_URL/get?r=obj1" },
					obj2: { url: "HTTPBIN_URL/get?r=obj2", params: {tags: {name: 'obj2'}}, method: "GET"},
				};
				let res = http.batch(reqs);
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
					let reqs = [
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
					let res = http.batch(reqs);
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
			let res = http.batch([ ["POST", "HTTPBIN_URL/post", { key: "value" }] ]);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
				if (res[key].json().form.key != "value") { throw new Error("wrong form: " + key + ": " + JSON.stringify(res[key].json().form)); }
			}`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "POST", sr("HTTPBIN_URL/post"), "", 200, "")
		})
		t.Run("PUT", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
			let res = http.batch([ ["PUT", "HTTPBIN_URL/put", { key: "value" }] ]);
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
				let reqUrl = "HTTPBIN_URL/cookies"
				let res = http.get(reqUrl);
				let jar = new http.CookieJar();

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
				let res = http.post("HTTPBIN_URL/post", {a: "a", b: 2}, {headers: {"Content-Type": "application/x-www-form-urlencoded; charset=utf-8"}});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.request["body"] != "a=a&b=2") { throw new Error("http request body was not set properly: " + JSON.stringify(res.request))}
				`))
			assert.NoError(t, err)
		})
	})
}
func TestSystemTags(t *testing.T) {
	t.Parallel()
	tb, state, samples, rt, _ := newRuntime(t)
	defer tb.Cleanup()

	// Handple paths with custom logic
	tb.Mux.HandleFunc("/wrong-redirect", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Location", "%")
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	httpGet := fmt.Sprintf(`http.get("%s");`, tb.ServerHTTP.URL)
	httpsGet := fmt.Sprintf(`http.get("%s");`, tb.ServerHTTPS.URL)

	httpURL, err := url.Parse(tb.ServerHTTP.URL)
	require.NoError(t, err)
	var connectionRefusedErrorText = "connect: connection refused"
	if runtime.GOOS == "windows" {
		connectionRefusedErrorText = "connectex: No connection could be made because the target machine actively refused it."
	}

	testedSystemTags := []struct{ tag, code, expVal string }{
		{"proto", httpGet, "HTTP/1.1"},
		{"status", httpGet, "200"},
		{"method", httpGet, "GET"},
		{"url", httpGet, tb.ServerHTTP.URL},
		{"url", httpsGet, tb.ServerHTTPS.URL},
		{"ip", httpGet, httpURL.Hostname()},
		{"name", httpGet, tb.ServerHTTP.URL},
		{"group", httpGet, ""},
		{"vu", httpGet, "0"},
		{"iter", httpGet, "0"},
		{"tls_version", httpsGet, "tls1.2"},
		{"ocsp_status", httpsGet, "unknown"},
		{
			"error",
			tb.Replacer.Replace(`http.get("http://127.0.0.1:56789");`),
			tb.Replacer.Replace(`dial tcp 127.0.0.1:56789: ` + connectionRefusedErrorText),
		},
	}

	state.Options.Throw = null.BoolFrom(false)

	for num, tc := range testedSystemTags {
		t.Run(fmt.Sprintf("TC %d with only %s", num, tc.tag), func(t *testing.T) {
			state.Options.SystemTags = lib.GetTagSet(tc.tag)

			_, err := common.RunString(rt, tc.code)
			assert.NoError(t, err)

			bufSamples := stats.GetBufferedSamples(samples)
			assert.NotEmpty(t, bufSamples)
			for _, sampleC := range bufSamples {

				for _, sample := range sampleC.GetSamples() {
					assert.NotEmpty(t, sample.Tags)
					for emittedTag, emittedVal := range sample.Tags.CloneTags() {
						assert.Equal(t, tc.tag, emittedTag)
						assert.Equal(t, tc.expVal, emittedVal)
					}
				}
			}
		})
	}
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
		let expText = "EXP_TEXT";
		let expBinLength = EXP_BIN_LEN;

		// Check default behaviour with a unicode text
		let respTextImplicit = http.get("HTTPBIN_URL/get-text").body;
		if (respTextImplicit !== expText) {
			throw new Error("default response body should be '" + expText + "' but was '" + respTextImplicit + "'");
		}
		http.post("HTTPBIN_URL/compare-text", respTextImplicit);

		// Check discarding of responses
		let respNone = http.get("HTTPBIN_URL/get-text", { responseType: "none" }).body;
		if (respNone != null) {
			throw new Error("none response body should be null but was " + respNone);
		}

		// Check binary transmission of the text response as well
		let respTextInBin = http.get("HTTPBIN_URL/get-text", { responseType: "binary" }).body;

		// Hack to convert a utf-8 array to a JS string
		let strConv = "";
		function pad(n) { return n.length < 2 ? "0" + n : n; }
		for( let i = 0; i < respTextInBin.length; i++ ) {
			strConv += ( "%" + pad(respTextInBin[i].toString(16)));
		}
		strConv = decodeURIComponent(strConv);
		if (strConv !== expText) {
			throw new Error("converted response body should be '" + expText + "' but was '" + strConv + "'");
		}
		http.post("HTTPBIN_URL/compare-text", respTextInBin);

		// Check binary response
		let respBin = http.get("HTTPBIN_URL/get-bin", { responseType: "binary" }).body;
		if (respBin.length !== expBinLength) {
			throw new Error("response body length should be '" + expBinLength + "' but was '" + respBin.length + "'");
		}
		for( let i = 0; i < respBin.length; i++ ) {
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
		let expText = "EXP_TEXT";

		// Check default behaviour
		let respDefault = http.get("HTTPBIN_URL/get-text").body;
		if (respDefault !== null) {
			throw new Error("default response body should be discarded and null but was " + respDefault);
		}

		// Check explicit text response
		let respTextExplicit = http.get("HTTPBIN_URL/get-text", { responseType: "text" }).body;
		if (respTextExplicit !== expText) {
			throw new Error("text response body should be '" + expText + "' but was '" + respTextExplicit + "'");
		}
		http.post("HTTPBIN_URL/compare-text", respTextExplicit);
	`))
	assert.NoError(t, err)
}
