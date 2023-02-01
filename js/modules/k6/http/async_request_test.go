package http

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
)

func wrapInAsyncLambda(input string) string {
	// This makes it possible to use `await` freely on the "top" level
	return "(async () => {\n " + input + "\n })()"
}

func runOnEventLoop(runtime *modulestest.Runtime, code string) error {
	// TODO move this in modulestest.Runtime and extend it
	err := runtime.EventLoop.Start(func() error {
		_, err := runtime.VU.Runtime().RunString(wrapInAsyncLambda(code))
		return err
	})
	runtime.EventLoop.WaitOnRegistered()
	return err
}

func TestAsyncRequest(t *testing.T) {
	t.Parallel()
	t.Run("EmptyBody", func(t *testing.T) {
		t.Parallel()
		ts := newTestCase(t)

		sr := ts.tb.Replacer.Replace
		err := runOnEventLoop(ts.runtime, sr(`
				var reqUrl = "HTTPBIN_URL/cookies"
				var res = await http.asyncRequest("GET", reqUrl);
				var jar = new http.CookieJar();

				jar.set("HTTPBIN_URL/cookies", "key", "value");
				res = await http.asyncRequest("GET", reqUrl, null, { cookies: { key2: "value2" }, jar: jar });

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
		t.Parallel()
		ts := newTestCase(t)

		sr := ts.tb.Replacer.Replace
		err := runOnEventLoop(ts.runtime, sr(`
				var res = await http.asyncRequest("POST", "HTTPBIN_URL/post", {a: "a", b: 2}, {headers: {"Content-Type": "application/x-www-form-urlencoded; charset=utf-8"}});
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.request["body"] != "a=a&b=2") { throw new Error("http request body was not set properly: " + JSON.stringify(res.request))}
				`))
		assert.NoError(t, err)
	})
	t.Run("Concurrent", func(t *testing.T) {
		t.Parallel()
		ts := newTestCase(t)
		sr := ts.tb.Replacer.Replace
		err := runOnEventLoop(ts.runtime, sr(`
            let start = Date.now()
            let p1 = http.asyncRequest("GET", "HTTPBIN_URL/delay/200ms").then(() => { return Date.now() - start})
            let p2 = http.asyncRequest("GET", "HTTPBIN_URL/delay/100ms").then(() =>  { return Date.now() - start})
            let time1 = await p1;
            let time2 = await p2;
            if (time1 < time2) {
                throw("request that should've taken 200ms took less time then one that should take 100ms " + time1 +">" + time2 )
            }

				`))
		assert.NoError(t, err)
	})
}

func TestAsyncRequestResponseCallbackRace(t *testing.T) {
	// This test is here only to tease out race conditions
	t.Parallel()
	ts := newTestCase(t)
	err := ts.runtime.VU.Runtime().Set("q", func(f func()) {
		rg := ts.runtime.EventLoop.RegisterCallback()
		time.AfterFunc(time.Millisecond*5, func() {
			rg(func() error {
				f()
				return nil
			})
		})
	})
	require.NoError(t, err)
	err = ts.runtime.VU.Runtime().Set("log", func(s string) {
		// t.Log(s) // uncomment for debugging
	})
	require.NoError(t, err)
	err = runOnEventLoop(ts.runtime, ts.tb.Replacer.Replace(`
        let call = (i) => {
            log("s"+i)
            if (i > 200) { return null; }
            http.setResponseCallback(http.expectedStatuses(i))
            q(() => call(i+1)) // don't use promises as they resolve before eventloop callbacks such as the one from asyncRequest
        }
        for (let j = 0; j< 50; j++) {
            call(0)
            await http.asyncRequest("GET", "HTTPBIN_URL/redirect/20").then(() => log("!!!!!!!!!!!!!!!"+j))
        }
    `))
	require.NoError(t, err)
}

func TestAsyncRequestErrors(t *testing.T) {
	// This likely should have a way to do the same for http.request and http.asyncRequest with the same tests
	t.Parallel()
	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		t.Run("unsupported protocol", func(t *testing.T) {
			t.Parallel()
			ts := newTestCase(t)
			state := ts.runtime.VU.State()

			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			err := runOnEventLoop(ts.runtime, `
            try {
                http.asyncRequest("", "").catch((e) => globalThis.promiseRejected = e )
            } catch (e) {
                globalThis.exceptionThrown = e
            }
            `)
			require.NoError(t, err)
			promiseRejected := ts.runtime.VU.Runtime().Get("promiseRejected")
			exceptionThrown := ts.runtime.VU.Runtime().Get("exceptionThrown")
			require.NotNil(t, promiseRejected)
			require.True(t, promiseRejected.ToBoolean())
			require.Nil(t, exceptionThrown)
			assert.Contains(t, promiseRejected.ToString(), "unsupported protocol scheme")

			logEntry := hook.LastEntry()
			assert.Nil(t, logEntry)
		})

		t.Run("throw=false", func(t *testing.T) {
			t.Parallel()
			ts := newTestCase(t)
			state := ts.runtime.VU.State()
			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			err := runOnEventLoop(ts.runtime, `
				var res = await http.asyncRequest("GET", "some://example.com", null, { throw: false });
				if (res.error.search('unsupported protocol scheme "some"')  == -1) {
					throw new Error("wrong error:" + res.error);
				}
				throw new Error("another error");
			`)
			require.ErrorContains(t, err, "another error")

			logEntry := hook.LastEntry()
			if assert.NotNil(t, logEntry) {
				assert.Equal(t, logrus.WarnLevel, logEntry.Level)
				err, ok := logEntry.Data["error"].(error)
				require.True(t, ok)
				assert.ErrorContains(t, err, "unsupported protocol scheme")
				assert.Equal(t, "Request Failed", logEntry.Message)
			}
		})
	})
	t.Run("InvalidURL", func(t *testing.T) {
		t.Parallel()

		expErr := `invalid URL: parse "https:// test.k6.io": invalid character " " in host name`
		t.Run("throw=true", func(t *testing.T) {
			t.Parallel()
			ts := newTestCase(t)

			js := `
                try {
				    http.asyncRequest("GET", "https:// test.k6.io").catch((e) => globalThis.promiseRejected = e )
                } catch (e) {
                    globalThis.exceptionThrown = e
                }
			`
			err := runOnEventLoop(ts.runtime, js)
			require.NoError(t, err)
			promiseRejected := ts.runtime.VU.Runtime().Get("promiseRejected")
			exceptionThrown := ts.runtime.VU.Runtime().Get("exceptionThrown")
			require.NotNil(t, promiseRejected)
			require.True(t, promiseRejected.ToBoolean())
			require.Nil(t, exceptionThrown)
			assert.Contains(t, promiseRejected.ToString(), expErr)
		})

		t.Run("throw=false", func(t *testing.T) {
			t.Parallel()
			ts := newTestCase(t)
			rt := ts.runtime.VU.Runtime()
			state := ts.runtime.VU.State()
			state.Options.Throw.Bool = false
			defer func() { state.Options.Throw.Bool = true }()

			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			js := `
                var r = await http.asyncRequest("GET", "https:// test.k6.io");
                globalThis.ret = {error: r.error, error_code: r.error_code};
			`
			err := runOnEventLoop(ts.runtime, js)
			require.NoError(t, err)
			ret := rt.GlobalObject().Get("ret")
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
			err, ok = logEntry.Data["error"].(error)
			require.True(t, ok)
			assert.ErrorContains(t, err, expErr)
			assert.Equal(t, "Request Failed", logEntry.Message)
		})

		t.Run("throw=false,nopanic", func(t *testing.T) {
			t.Parallel()
			ts := newTestCase(t)
			rt := ts.runtime.VU.Runtime()
			state := ts.runtime.VU.State()
			state.Options.Throw.Bool = false
			defer func() { state.Options.Throw.Bool = true }()

			hook := logtest.NewLocal(state.Logger)
			defer hook.Reset()

			js := `
                var r = await http.asyncRequest("GET", "https:// test.k6.io");
                r.html();
                r.json();
                globalThis.ret = r.error_code; // not reached because of json()
			`
			err := runOnEventLoop(ts.runtime, js)
			ret := rt.GlobalObject().Get("ret")
			require.Error(t, err)
			assert.Nil(t, ret)
			assert.Contains(t, err.Error(), "unexpected end of JSON input")

			logEntry := hook.LastEntry()
			require.NotNil(t, logEntry)
			assert.Equal(t, logrus.WarnLevel, logEntry.Level)
			err, ok := logEntry.Data["error"].(error)
			require.True(t, ok)
			assert.ErrorContains(t, err, expErr)
			assert.Equal(t, "Request Failed", logEntry.Message)
		})
	})

	t.Run("Unroutable", func(t *testing.T) {
		t.Parallel()
		ts := newTestCase(t)
		err := runOnEventLoop(ts.runtime, `
            try {
                http.asyncRequest("GET", "http://sdafsgdhfjg/").catch((e) => globalThis.promiseRejected = e )
            } catch (e) {
                globalThis.exceptionThrown = e
            }`)
		expErr := "lookup sdafsgdhfjg"
		require.NoError(t, err)
		promiseRejected := ts.runtime.VU.Runtime().Get("promiseRejected")
		exceptionThrown := ts.runtime.VU.Runtime().Get("exceptionThrown")
		require.NotNil(t, promiseRejected)
		require.True(t, promiseRejected.ToBoolean())
		require.Nil(t, exceptionThrown)
		assert.Contains(t, promiseRejected.ToString(), expErr)
	})
}
