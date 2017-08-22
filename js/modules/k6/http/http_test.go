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
	assert.True(t, seenSending, "url %s didn't emit Sending", url)
	assert.True(t, seenWaiting, "url %s didn't emit Waiting", url)
	assert.True(t, seenReceiving, "url %s didn't emit Receiving", url)
}

func TestRequest(t *testing.T) {
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
	rt.Set("http", common.Bind(rt, &HTTP{}, ctx))

	t.Run("Redirects", func(t *testing.T) {
		t.Run("9", func(t *testing.T) {
			_, err := common.RunString(rt, `http.get("https://httpbin.org/redirect/9")`)
			assert.NoError(t, err)
		})
		t.Run("10", func(t *testing.T) {
			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			_, err := common.RunString(rt, `http.get("https://httpbin.org/redirect/10")`)
			assert.EqualError(t, err, "GoError: Get /get: stopped after 10 redirects")

			logEntry := hook.LastEntry()
			if assert.NotNil(t, logEntry) {
				assert.Equal(t, log.WarnLevel, logEntry.Level)
				assert.EqualError(t, logEntry.Data["error"].(error), "Get /get: stopped after 10 redirects")
				assert.Equal(t, "Request Failed", logEntry.Message)
			}
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
	t.Run("BadSSL", func(t *testing.T) {
		_, err := common.RunString(rt, `http.get("https://expired.badssl.com/");`)
		assert.EqualError(t, err, "GoError: Get https://expired.badssl.com/: x509: certificate has expired or is not yet valid")
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

	t.Run("HTML", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let res = http.request("GET", "https://httpbin.org/html");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.body.indexOf("Herman Melville - Moby-Dick") == -1) { throw new Error("wrong body: " + res.body); }
		`)
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/html", "", 200, "")

		t.Run("html", func(t *testing.T) {
			_, err := common.RunString(rt, `
			if (res.html().find("h1").text() != "Herman Melville - Moby-Dick") { throw new Error("wrong title: " + res.body); }
			`)
			assert.NoError(t, err)

			t.Run("shorthand", func(t *testing.T) {
				_, err := common.RunString(rt, `
				if (res.html("h1").text() != "Herman Melville - Moby-Dick") { throw new Error("wrong title: " + res.body); }
				`)
				assert.NoError(t, err)
			})
		})

		t.Run("group", func(t *testing.T) {
			g, err := root.Group("my group")
			if assert.NoError(t, err) {
				old := state.Group
				state.Group = g
				defer func() { state.Group = old }()
			}

			state.Samples = nil
			_, err = common.RunString(rt, `
			let res = http.request("GET", "https://httpbin.org/html");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			if (res.body.indexOf("Herman Melville - Moby-Dick") == -1) { throw new Error("wrong body: " + res.body); }
			`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/html", "", 200, "::my group")
		})
	})
	t.Run("JSON", func(t *testing.T) {
		state.Samples = nil
		_, err := common.RunString(rt, `
		let res = http.request("GET", "https://httpbin.org/get?a=1&b=2");
		if (res.status != 200) { throw new Error("wrong status: " + res.status); }
		if (res.json().args.a != "1") { throw new Error("wrong ?a: " + res.json().args.a); }
		if (res.json().args.b != "2") { throw new Error("wrong ?b: " + res.json().args.b); }
		`)
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/get?a=1&b=2", "", 200, "")

		t.Run("Invalid", func(t *testing.T) {
			_, err := common.RunString(rt, `http.request("GET", "https://httpbin.org/html").json();`)
			assert.EqualError(t, err, "GoError: invalid character '<' looking for beginning of value")
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
					if (res[key].status != 200) { throw new Error("wrong status: " + res[key].status); }
					if (res[key].url != reqs[key][1].url) { throw new Error("wrong url: " + res[key].url); }
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
					if (res[key].status != 200) { throw new Error("wrong status: " + res[key].status); }
					if (res[key].url != reqs[key]) { throw new Error("wrong url: " + res[key].url); }
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
						if (res[key].status != 200) { throw new Error("wrong status: " + res[key].status); }
						if (res[key].url != reqs[key].url) { throw new Error("wrong url: " + res[key].url); }
					}`)
					assert.NoError(t, err)
					assertRequestMetricsEmitted(t, state.Samples, "GET", "https://httpbin.org/get", "https://httpbin.org/${}", 200, "")
					assertRequestMetricsEmitted(t, state.Samples, "GET", "https://now.httpbin.org/", "", 200, "")
				})
			})
		})
		t.Run("POST", func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, `
			let res = http.batch([ ["POST", "https://httpbin.org/post", { key: "value" }] ]);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + res[key].status); }
				if (res[key].json().form.key != "value") { throw new Error("wrong form: " + JSON.stringify(res[key].json().form)); }
			}`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "POST", "https://httpbin.org/post", "", 200, "")
		})
		t.Run("PUT", func(t *testing.T) {
			state.Samples = nil
			_, err := common.RunString(rt, `
			let res = http.batch([ ["PUT", "https://httpbin.org/put", { key: "value" }] ]);
			for (var key in res) {
				if (res[key].status != 200) { throw new Error("wrong status: " + res[key].status); }
				if (res[key].json().form.key != "value") { throw new Error("wrong form: " + JSON.stringify(res[key].json().form)); }
			}`)
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, state.Samples, "PUT", "https://httpbin.org/put", "", 200, "")
		})
	})
}

func TestTagURL(t *testing.T) {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	rt.Set("http", common.Bind(rt, &HTTP{}, nil))

	testdata := map[string]URLTag{
		`http://httpbin.org/anything/`:               {URL: "http://httpbin.org/anything/", Name: "http://httpbin.org/anything/"},
		`http://httpbin.org/anything/${1+1}`:         {URL: "http://httpbin.org/anything/2", Name: "http://httpbin.org/anything/${}"},
		`http://httpbin.org/anything/${1+1}/`:        {URL: "http://httpbin.org/anything/2/", Name: "http://httpbin.org/anything/${}/"},
		`http://httpbin.org/anything/${1+1}/${1+2}`:  {URL: "http://httpbin.org/anything/2/3", Name: "http://httpbin.org/anything/${}/${}"},
		`http://httpbin.org/anything/${1+1}/${1+2}/`: {URL: "http://httpbin.org/anything/2/3/", Name: "http://httpbin.org/anything/${}/${}/"},
	}
	for expr, tag := range testdata {
		t.Run("expr="+expr, func(t *testing.T) {
			v, err := common.RunString(rt, "http.url`"+expr+"`")
			if assert.NoError(t, err) {
				assert.Equal(t, tag, v.Export())
			}
		})
	}
}
