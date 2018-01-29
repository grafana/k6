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
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
	"github.com/oxtoacart/bpool"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	null "gopkg.in/guregu/null.v3"
)

func assertRequestMetricsEmitted(t *testing.T, samples []stats.Sample, method, url, name string, status int, group string) {
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
	for _, sample := range samples {
		if sample.Tags["url"] == url {
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

			assert.Equal(t, strconv.Itoa(status), sample.Tags["status"])
			assert.Equal(t, method, sample.Tags["method"])
			assert.Equal(t, group, sample.Tags["group"])
			assert.Equal(t, name, sample.Tags["name"])
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

func TestRequestAndBatch(t *testing.T) {
	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	logger := log.New()
	logger.Level = log.DebugLevel
	logger.Out = ioutil.Discard

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	state := &common.State{
		Options: lib.Options{
			MaxRedirects: null.IntFrom(10),
			UserAgent:    null.StringFrom("TestUserAgent"),
			Throw:        null.BoolFrom(true),
		},
		Logger: logger,
		Group:  root,
		HTTPTransport: &http.Transport{
			DialContext: (netext.NewDialer(net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			})).DialContext,
		},
		BPool: bpool.NewBufferPool(1),
	}

	ctx := new(context.Context)
	*ctx = context.Background()
	*ctx = common.WithState(*ctx, state)
	*ctx = common.WithRuntime(*ctx, rt)
	rt.Set("http", common.Bind(rt, New(), ctx))

	t.Run("Redirects", func(t *testing.T) {
		t.Run("10", func(t *testing.T) {
			_, err := common.RunString(rt, `http.get("https://httpbin.org/redirect/10")`)
			assert.NoError(t, err)
		})
		t.Run("11", func(t *testing.T) {
			_, err := common.RunString(rt, `
			let res = http.get("https://httpbin.org/redirect/11");
			if (res.status != 302) { throw new Error("wrong status: " + res.status) }
			if (res.url != "https://httpbin.org/relative-redirect/1") { throw new Error("incorrect URL: " + res.url) }
			if (res.headers["Location"] != "/get") { throw new Error("incorrect Location header: " + res.headers["Location"]) }
			`)
			assert.NoError(t, err)

			t.Run("Unset Max", func(t *testing.T) {
				hook := logtest.NewLocal(state.Logger)
				defer hook.Reset()

				oldOpts := state.Options
				defer func() { state.Options = oldOpts }()
				state.Options.MaxRedirects = null.NewInt(10, false)

				_, err := common.RunString(rt, `
				let res = http.get("https://httpbin.org/redirect/11");
				if (res.status != 302) { throw new Error("wrong status: " + res.status) }
				if (res.url != "https://httpbin.org/relative-redirect/1") { throw new Error("incorrect URL: " + res.url) }
				if (res.headers["Location"] != "/get") { throw new Error("incorrect Location header: " + res.headers["Location"]) }
				`)
				assert.NoError(t, err)

				logEntry := hook.LastEntry()
				if assert.NotNil(t, logEntry) {
					assert.Equal(t, log.WarnLevel, logEntry.Level)
					assert.Equal(t, "https://httpbin.org/redirect/11", logEntry.Data["url"])
					assert.Equal(t, "Stopped after 11 redirects and returned the redirection; pass { redirects: n } in request params or set global maxRedirects to silence this", logEntry.Message)
				}
			})
		})
		t.Run("requestScopeRedirects", func(t *testing.T) {
			_, err := common.RunString(rt, `
			let res = http.get("https://httpbin.org/redirect/1", {redirects: 3});
			if (res.status != 200) { throw new Error("wrong status: " + res.status) }
			if (res.url != "https://httpbin.org/get") { throw new Error("incorrect URL: " + res.url) }
			`)
			assert.NoError(t, err)
		})
		t.Run("requestScopeNoRedirects", func(t *testing.T) {
			_, err := common.RunString(rt, `
			let res = http.get("https://httpbin.org/redirect/1", {redirects: 0});
			if (res.status != 302) { throw new Error("wrong status: " + res.status) }
			if (res.url != "https://httpbin.org/redirect/1") { throw new Error("incorrect URL: " + res.url) }
			if (res.headers["Location"] != "/get") { throw new Error("incorrect Location header: " + res.headers["Location"]) }
			`)
			assert.NoError(t, err)
		})
	})
	t.Run("Timeout", func(t *testing.T) {
		t.Run("10s", func(t *testing.T) {
			_, err := common.RunString(rt, `
				http.get("https://httpbin.org/delay/1", {
					timeout: 5*1000,
				})
			`)
			assert.NoError(t, err)
		})
		t.Run("10s", func(t *testing.T) {
			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			startTime := time.Now()
			_, err := common.RunString(rt, `
				http.get("https://httpbin.org/delay/10", {
					timeout: 1*1000,
				})
			`)
			endTime := time.Now()
			assert.EqualError(t, err, "GoError: Get https://httpbin.org/delay/10: net/http: request canceled (Client.Timeout exceeded while awaiting headers)")
			assert.WithinDuration(t, startTime.Add(1*time.Second), endTime, 1*time.Second)

			logEntry := hook.LastEntry()
			if assert.NotNil(t, logEntry) {
				assert.Equal(t, log.WarnLevel, logEntry.Level)
				assert.EqualError(t, logEntry.Data["error"].(error), "Get https://httpbin.org/delay/10: net/http: request canceled (Client.Timeout exceeded while awaiting headers)")
				assert.Equal(t, "Request Failed", logEntry.Message)
			}
		})
	})
	t.Run("UserAgent", func(t *testing.T) {
		_, err := common.RunString(rt, `
			let res = http.get("http://httpbin.org/user-agent");
			if (res.json()['user-agent'] != "TestUserAgent") {
				throw new Error("incorrect user agent: " + res.json()['user-agent'])
			}
		`)
		assert.NoError(t, err)

		t.Run("Override", func(t *testing.T) {
			_, err := common.RunString(rt, `
				let res = http.get("http://httpbin.org/user-agent", {
					headers: { "User-Agent": "OtherUserAgent" },
				});
				if (res.json()['user-agent'] != "OtherUserAgent") {
					throw new Error("incorrect user agent: " + res.json()['user-agent'])
				}
			`)
			assert.NoError(t, err)
		})
	})
	t.Run("Compression", func(t *testing.T) {
		t.Run("gzip", func(t *testing.T) {
			_, err := common.RunString(rt, `
				let res = http.get("http://httpbin.org/gzip");
				if (res.json()['gzipped'] != true) {
					throw new Error("unexpected body data: " + res.json()['gzipped'])
				}
			`)
			assert.NoError(t, err)
		})
		t.Run("deflate", func(t *testing.T) {
			_, err := common.RunString(rt, `
				let res = http.get("http://httpbin.org/deflate");
				if (res.json()['deflated'] != true) {
					throw new Error("unexpected body data: " + res.json()['deflated'])
				}
			`)
			assert.NoError(t, err)
		})
	})
	t.Run("CompressionWithAcceptEncodingHeader", func(t *testing.T) {
		t.Run("gzip", func(t *testing.T) {
			_, err := common.RunString(rt, `
				let params = { headers: { "Accept-Encoding": "gzip" } };
				let res = http.get("http://httpbin.org/gzip", params);
				if (res.json()['gzipped'] != true) {
					throw new Error("unexpected body data: " + res.json()['gzipped'])
				}
			`)
			assert.NoError(t, err)
		})
		t.Run("deflate", func(t *testing.T) {
			_, err := common.RunString(rt, `
				let params = { headers: { "Accept-Encoding": "deflate" } };
				let res = http.get("http://httpbin.org/deflate", params);
				if (res.json()['deflated'] != true) {
					throw new Error("unexpected body data: " + res.json()['deflated'])
				}
			`)
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

		_, err := common.RunString(rt, `http.get("https://httpbin.org/get/");`)
		assert.Error(t, err)
		assert.Nil(t, hook.LastEntry())
	})
	t.Run("HTTP/2", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let res = http.request("GET", "https://http2.akamai.com/demo");
		if (res.status != 200) { throw new Error("wrong status: " + res.status) }
		if (res.proto != "HTTP/2.0") { throw new Error("wrong proto: " + res.proto) }
		`)
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, state.Samples, "GET", "https://http2.akamai.com/demo", "", 200, "")
		for _, sample := range state.Samples {
			assert.Equal(t, "HTTP/2.0", sample.Tags["proto"])
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
				assertRequestMetricsEmitted(t, state.Samples, "GET", versionTest.URL, "", 200, "")
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
				assertRequestMetricsEmitted(t, state.Samples, "GET", cipherSuiteTest.URL, "", 200, "")
			})
		}
		t.Run("ocsp_stapled_good", func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, `
			let res = http.request("GET", "https://stackoverflow.com/");
			if (res.ocsp.status != http.OCSP_STATUS_GOOD) { throw new Error("wrong ocsp stapled response status: " + res.ocsp.status); }
			`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "GET", "https://stackoverflow.com/", "", 200, "")
		})
	})
	t.Run("Invalid", func(t *testing.T) {
		hook := logtest.NewLocal(state.Logger)
		defer hook.Reset()

		_, err := common.RunString(rt, `http.request("", "");`)
		assert.EqualError(t, err, "GoError: Get : unsupported protocol scheme \"\"")

		logEntry := hook.LastEntry()
		if assert.NotNil(t, logEntry) {
			assert.Equal(t, log.WarnLevel, logEntry.Level)
			assert.EqualError(t, logEntry.Data["error"].(error), "Get : unsupported protocol scheme \"\"")
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
				assert.Equal(t, log.WarnLevel, logEntry.Level)
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
				state.Samples = nil
				_, err := common.RunString(rt, fmt.Sprintf(`
				let res = http.request("GET", "https://httpbin.org/headers", null, %s);
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`, literal))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/headers", "", 200, "")
			})
		}

		t.Run("cookies", func(t *testing.T) {
			t.Run("access", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				state.Samples = nil
				_, err = common.RunString(rt, `
				let res = http.request("GET", "https://httpbin.org/cookies/set?key=value", null, { redirects: 0 });
				if (res.cookies.key[0].value != "value") { throw new Error("wrong cookie value: " + res.cookies.key[0].value); }
				if (res.cookies.key[0].path != "/") { throw new Error("wrong cookie value: " + res.cookies.key[0].path); }
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/cookies/set?key=value", "", 302, "")
			})

			t.Run("vuJar", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				state.Samples = nil
				_, err = common.RunString(rt, `
				let jar = http.cookieJar();
				jar.set("https://httpbin.org/cookies", "key", "value");
				let res = http.request("GET", "https://httpbin.org/cookies", null, { cookies: { key2: "value2" } });
				if (res.json().cookies.key != "value") { throw new Error("wrong cookie value: " + res.json().cookies.key); }
				if (res.json().cookies.key2 != "value2") { throw new Error("wrong cookie value: " + res.json().cookies.key2); }
				let jarCookies = jar.cookiesForURL("https://httpbin.org/cookies");
				if (jarCookies.key[0] != "value") { throw new Error("wrong cookie value in jar"); }
				if (jarCookies.key2 != undefined) { throw new Error("unexpected cookie in jar"); }
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/cookies", "", 200, "")
			})

			t.Run("requestScope", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				state.Samples = nil
				_, err = common.RunString(rt, `
				let res = http.request("GET", "https://httpbin.org/cookies", null, { cookies: { key: "value" } });
				if (res.json().cookies.key != "value") { throw new Error("wrong cookie value: " + res.json().cookies.key); }
				let jar = http.cookieJar();
				let jarCookies = jar.cookiesForURL("https://httpbin.org/cookies");
				if (jarCookies.key != undefined) { throw new Error("unexpected cookie in jar"); }
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/cookies", "", 200, "")
			})

			t.Run("requestScopeReplace", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				state.Samples = nil
				_, err = common.RunString(rt, `
				let jar = http.cookieJar();
				jar.set("https://httpbin.org/cookies", "key", "value");
				let res = http.request("GET", "https://httpbin.org/cookies", null, { cookies: { key: { value: "replaced", replace: true } } });
				if (res.json().cookies.key != "replaced") { throw new Error("wrong cookie value: " + res.json().cookies.key); }
				let jarCookies = jar.cookiesForURL("https://httpbin.org/cookies");
				if (jarCookies.key[0] != "value") { throw new Error("wrong cookie value in jar"); }
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/cookies", "", 200, "")
			})

			t.Run("redirect", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				state.Samples = nil
				_, err = common.RunString(rt, `
				http.cookieJar().set("https://httpbin.org/cookies", "key", "value");
				let res = http.request("GET", "https://httpbin.org/cookies/set?key2=value2");
				if (res.json().cookies.key != "value") { throw new Error("wrong cookie value: " + res.body); }
				if (res.json().cookies.key2 != "value2") { throw new Error("wrong cookie value 2: " + res.body); }
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/cookies", "https://httpbin.org/cookies/set?key2=value2", 200, "")
			})

			t.Run("domain", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				state.Samples = nil
				_, err = common.RunString(rt, `
				let jar = http.cookieJar();
				jar.set("https://httpbin.org/cookies", "key", "value", { domain: "httpbin.org" });
				let res = http.request("GET", "https://httpbin.org/cookies");
				if (res.json().cookies.key != "value") {
					throw new Error("wrong cookie value: " + res.json().cookies.key);
				}
				jar.set("https://httpbin.org/cookies", "key2", "value2", { domain: "example.com" });
				res = http.request("GET", "http://httpbin.org/cookies");
				if (res.json().cookies.key != "value") {
					throw new Error("wrong cookie value: " + res.json().cookies.key);
				}
				if (res.json().cookies.key2 != undefined) {
					throw new Error("cookie 'key2' unexpectedly found");
				}
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/cookies", "", 200, "")
			})

			t.Run("path", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				state.Samples = nil
				_, err = common.RunString(rt, `
				let jar = http.cookieJar();
				jar.set("https://httpbin.org/cookies", "key", "value", { path: "/cookies" });
				let res = http.request("GET", "https://httpbin.org/cookies");
				if (res.json().cookies.key != "value") {
					throw new Error("wrong cookie value: " + res.json().cookies.key);
				}
				jar.set("https://httpbin.org/cookies", "key2", "value2", { path: "/some-other-path" });
				res = http.request("GET", "http://httpbin.org/cookies");
				if (res.json().cookies.key != "value") {
					throw new Error("wrong cookie value: " + res.json().cookies.key);
				}
				if (res.json().cookies.key2 != undefined) {
					throw new Error("cookie 'key2' unexpectedly found");
				}
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/cookies", "", 200, "")
			})

			t.Run("expires", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				state.Samples = nil
				_, err = common.RunString(rt, `
				let jar = http.cookieJar();
				jar.set("https://httpbin.org/cookies", "key", "value", { expires: "Sun, 24 Jul 1983 17:01:02 GMT" });
				let res = http.request("GET", "https://httpbin.org/cookies");
				if (res.json().cookies.key != undefined) {
					throw new Error("cookie 'key' unexpectedly found");
				}
				jar.set("https://httpbin.org/cookies", "key", "value", { expires: "Sat, 24 Jul 2083 17:01:02 GMT" });
				res = http.request("GET", "https://httpbin.org/cookies");
				if (res.json().cookies.key != "value") {
					throw new Error("cookie 'key' not found");
				}
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/cookies", "", 200, "")
			})

			t.Run("secure", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				state.Samples = nil
				_, err = common.RunString(rt, `
				let jar = http.cookieJar();
				jar.set("https://httpbin.org/cookies", "key", "value", { secure: true });
				let res = http.request("GET", "https://httpbin.org/cookies");
				if (res.json().cookies.key != "value") {
					throw new Error("wrong cookie value: " + res.json().cookies.key);
				}
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/cookies", "", 200, "")
			})

			t.Run("localJar", func(t *testing.T) {
				cookieJar, err := cookiejar.New(nil)
				assert.NoError(t, err)
				state.CookieJar = cookieJar
				state.Samples = nil
				_, err = common.RunString(rt, `
				let jar = new http.CookieJar();
				jar.set("https://httpbin.org/cookies", "key", "value");
				let res = http.request("GET", "https://httpbin.org/cookies", null, { cookies: { key2: "value2" }, jar: jar });
				if (res.json().cookies.key != "value") { throw new Error("wrong cookie value: " + res.json().cookies.key); }
				if (res.json().cookies.key2 != "value2") { throw new Error("wrong cookie value: " + res.json().cookies.key2); }
				let jarCookies = jar.cookiesForURL("https://httpbin.org/cookies");
				if (jarCookies.key[0] != "value") { throw new Error("wrong cookie value in jar: " + jarCookies.key[0]); }
				if (jarCookies.key2 != undefined) { throw new Error("unexpected cookie in jar"); }
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/cookies", "", 200, "")
			})
		})

		t.Run("headers", func(t *testing.T) {
			for _, literal := range []string{`null`, `undefined`} {
				state.Samples = nil
				t.Run(literal, func(t *testing.T) {
					_, err := common.RunString(rt, fmt.Sprintf(`
					let res = http.request("GET", "https://httpbin.org/headers", null, { headers: %s });
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`, literal))
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/headers", "", 200, "")
				})
			}

			t.Run("object", func(t *testing.T) {
				state.Samples = nil
				_, err := common.RunString(rt, `
				let res = http.request("GET", "https://httpbin.org/headers", null, {
					headers: { "X-My-Header": "value" },
				});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().headers["X-My-Header"] != "value") { throw new Error("wrong X-My-Header: " + res.json().headers["X-My-Header"]); }
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/headers", "", 200, "")
			})

			t.Run("Host", func(t *testing.T) {
				state.Samples = nil
				_, err := common.RunString(rt, `
				let res = http.request("GET", "http://httpbin.org/headers", null, {
					headers: { "Host": "www.httpbin.org" },
				});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().headers["Host"] != "www.httpbin.org") { throw new Error("wrong Host: " + res.json().headers["Host"]); }
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "http://httpbin.org/headers", "", 200, "")
			})
		})

		t.Run("tags", func(t *testing.T) {
			for _, literal := range []string{`null`, `undefined`} {
				t.Run(literal, func(t *testing.T) {
					state.Samples = nil
					_, err := common.RunString(rt, fmt.Sprintf(`
					let res = http.request("GET", "https://httpbin.org/headers", null, { tags: %s });
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					`, literal))
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/headers", "", 200, "")
				})
			}

			t.Run("object", func(t *testing.T) {
				state.Samples = nil
				_, err := common.RunString(rt, `
				let res = http.request("GET", "https://httpbin.org/headers", null, { tags: { tag: "value" } });
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/headers", "", 200, "")
				for _, sample := range state.Samples {
					assert.Equal(t, "value", sample.Tags["tag"])
				}
			})
		})
	})

	t.Run("GET", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let res = http.get("https://httpbin.org/get?a=1&b=2");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.json().args.a != "1") { throw new Error("wrong ?a: " + res.json().args.a); }
		if (res.json().args.b != "2") { throw new Error("wrong ?b: " + res.json().args.b); }
		`)
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/get?a=1&b=2", "", 200, "")

		t.Run("Tagged", func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, `
			let a = "1";
			let b = "2";
			let res = http.get(http.url`+"`"+`https://httpbin.org/get?a=${a}&b=${b}`+"`"+`);
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			if (res.json().args.a != a) { throw new Error("wrong ?a: " + res.json().args.a); }
			if (res.json().args.b != b) { throw new Error("wrong ?b: " + res.json().args.b); }
			`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/get?a=1&b=2", "https://httpbin.org/get?a=${}&b=${}", 200, "")
		})
	})
	t.Run("HEAD", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let res = http.head("https://httpbin.org/get?a=1&b=2");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.body.length != 0) { throw new Error("HEAD responses shouldn't have a body"); }
		`)
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, state.Samples, "HEAD", "https://httpbin.org/get?a=1&b=2", "", 200, "")
	})

	t.Run("OPTIONS", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let res = http.options("https://httpbin.org/get?a=1&b=2");
		if (res.body.length != 0) { throw new Error("OPTIONS responses shouldn't have a body " + res.body); }
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		`)
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, state.Samples, "OPTIONS", "https://httpbin.org/get?a=1&b=2", "", 200, "")
	})

	postMethods := map[string]string{
		"POST":   "post",
		"PUT":    "put",
		"PATCH":  "patch",
		"DELETE": "del",
	}
	for method, fn := range postMethods {
		t.Run(method, func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, fmt.Sprintf(`
			let res = http.%s("https://httpbin.org/%s", "data");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			if (res.json().data != "data") { throw new Error("wrong data: " + res.json().data); }
			if (res.json().headers["Content-Type"]) { throw new Error("content type set: " + res.json().headers["Content-Type"]); }
			`, fn, strings.ToLower(method)))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, method, "https://httpbin.org/"+strings.ToLower(method), "", 200, "")

			t.Run("object", func(t *testing.T) {
				state.Samples = nil
				_, err := common.RunString(rt, fmt.Sprintf(`
				let res = http.%s("https://httpbin.org/%s", {a: "a", b: 2});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.json().form.a != "a") { throw new Error("wrong a=: " + res.json().form.a); }
				if (res.json().form.b != "2") { throw new Error("wrong b=: " + res.json().form.b); }
				if (res.json().headers["Content-Type"] != "application/x-www-form-urlencoded") { throw new Error("wrong content type: " + res.json().headers["Content-Type"]); }
				`, fn, strings.ToLower(method)))
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, method, "https://httpbin.org/"+strings.ToLower(method), "", 200, "")

				t.Run("Content-Type", func(t *testing.T) {
					state.Samples = nil
					_, err := common.RunString(rt, fmt.Sprintf(`
					let res = http.%s("https://httpbin.org/%s", {a: "a", b: 2}, {headers: {"Content-Type": "application/x-www-form-urlencoded; charset=utf-8"}});
					if (res.status != 200) { throw new Error("wrong status: " + res.status); }
					if (res.json().form.a != "a") { throw new Error("wrong a=: " + res.json().form.a); }
					if (res.json().form.b != "2") { throw new Error("wrong b=: " + res.json().form.b); }
					if (res.json().headers["Content-Type"] != "application/x-www-form-urlencoded; charset=utf-8") { throw new Error("wrong content type: " + res.json().headers["Content-Type"]); }
					`, fn, strings.ToLower(method)))
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, state.Samples, method, "https://httpbin.org/"+strings.ToLower(method), "", 200, "")
				})
			})
		})
	}

	t.Run("Batch", func(t *testing.T) {
		t.Run("GET", func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, `
			let reqs = [
				["GET", "https://httpbin.org/"],
				["GET", "https://now.httpbin.org/"],
			];
			let res = http.batch(reqs);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + res[key].status); }
				if (res[key].url != reqs[key][1]) { throw new Error("wrong url: " + res[key].url); }
			}`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/", "", 200, "")
			assertRequestMetricsEmitted(t, state.Samples, "GET", "https://now.httpbin.org/", "", 200, "")

			t.Run("Tagged", func(t *testing.T) {
				state.Samples = nil
				_, err := common.RunString(rt, `
				let fragment = "get";
				let reqs = [
					["GET", http.url`+"`"+`https://httpbin.org/${fragment}`+"`"+`],
					["GET", http.url`+"`"+`https://now.httpbin.org/`+"`"+`],
				];
				let res = http.batch(reqs);
				for (var key in res) {
					if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
					if (res[key].url != reqs[key][1].url) { throw new Error("wrong url: " + key + ": " + res[key].url + " != " + reqs[key][1].url); }
				}`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/get", "https://httpbin.org/${}", 200, "")
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://now.httpbin.org/", "", 200, "")
			})

			t.Run("Shorthand", func(t *testing.T) {
				state.Samples = nil
				_, err := common.RunString(rt, `
				let reqs = [
					"https://httpbin.org/",
					"https://now.httpbin.org/",
				];
				let res = http.batch(reqs);
				for (var key in res) {
					if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
					if (res[key].url != reqs[key]) { throw new Error("wrong url: " + key + ": " + res[key].url); }
				}`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/", "", 200, "")
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://now.httpbin.org/", "", 200, "")

				t.Run("Tagged", func(t *testing.T) {
					state.Samples = nil
					_, err := common.RunString(rt, `
					let fragment = "get";
					let reqs = [
						http.url`+"`"+`https://httpbin.org/${fragment}`+"`"+`,
						http.url`+"`"+`https://now.httpbin.org/`+"`"+`,
					];
					let res = http.batch(reqs);
					for (var key in res) {
						if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
						if (res[key].url != reqs[key].url) { throw new Error("wrong url: " + key + ": " + res[key].url + " != " + reqs[key].url); }
					}`)
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/get", "https://httpbin.org/${}", 200, "")
					assertRequestMetricsEmitted(t, state.Samples, "GET", "https://now.httpbin.org/", "", 200, "")
				})
			})

			t.Run("ObjectForm", func(t *testing.T) {
				state.Samples = nil
				_, err := common.RunString(rt, `
				let reqs = [
					{ url: "https://httpbin.org/", method: "GET" },
					{ method: "GET", url: "https://now.httpbin.org/" },
				];
				let res = http.batch(reqs);
				for (var key in res) {
					if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
					if (res[key].url != reqs[key].url) { throw new Error("wrong url: " + key + ": " + res[key].url + " != " + reqs[key].url); }
				}`)
				assert.NoError(t, err)
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/", "", 200, "")
				assertRequestMetricsEmitted(t, state.Samples, "GET", "https://now.httpbin.org/", "", 200, "")
			})
		})
		t.Run("POST", func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, `
			let res = http.batch([ ["POST", "https://httpbin.org/post", { key: "value" }] ]);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
				if (res[key].json().form.key != "value") { throw new Error("wrong form: " + key + ": " + JSON.stringify(res[key].json().form)); }
			}`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "POST", "https://httpbin.org/post", "", 200, "")
		})
		t.Run("PUT", func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, `
			let res = http.batch([ ["PUT", "https://httpbin.org/put", { key: "value" }] ]);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + key + ": " + res[key].status); }
				if (res[key].json().form.key != "value") { throw new Error("wrong form: " + key + ": " + JSON.stringify(res[key].json().form)); }
			}`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "PUT", "https://httpbin.org/put", "", 200, "")
		})
	})

	t.Run("HTTPRequest", func(t *testing.T) {
		t.Run("EmptyBody", func(t *testing.T) {
			_, err := common.RunString(rt, `
				let reqUrl = "https://httpbin.org/cookies"
				let res = http.get(reqUrl);
				let jar = new http.CookieJar();

				jar.set("https://httpbin.org/cookies", "key", "value");
				res = http.request("GET", "https://httpbin.org/cookies", null, { cookies: { key2: "value2" }, jar: jar });

				if (res.json().cookies.key != "value") { throw new Error("wrong cookie value: " + res.json().cookies.key); }

				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.request["method"] !== "GET") { throw new Error("http request method was not \"GET\": " + JSON.stringify(res.request)) }
				if (res.request["body"].length != 0) { throw new Error("http request body was not null: " + JSON.stringify(res.request["body"])) }
				if (res.request["url"] != reqUrl) {
					throw new Error("wrong http request url: " + JSON.stringify(res.request))
				}
				if (res.request["cookies"]["key2"][0].name != "key2") { throw new Error("wrong http request cookies: " + JSON.stringify(JSON.stringify(res.request["cookies"]["key2"]))) }
				if (res.request["headers"]["User-Agent"][0] != "TestUserAgent") { throw new Error("wrong http request headers: " + JSON.stringify(res.request)) }
				`)
			assert.NoError(t, err)
		})
		t.Run("NonEmptyBody", func(t *testing.T) {
			_, err := common.RunString(rt, `
				let res = http.post("https://httpbin.org/post", {a: "a", b: 2}, {headers: {"Content-Type": "application/x-www-form-urlencoded; charset=utf-8"}});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.request["body"] != "a=a&b=2") { throw new Error("http request body was not set properly: " + JSON.stringify(res.request))}
				`)
			assert.NoError(t, err)
		})
	})
}
