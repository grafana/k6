package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

// TestPageRouteContinueOverridesRequest runs the script from
// https://github.com/grafana/k6/issues/5012 against a local server: the
// request seen through response.request() must reflect the headers, method
// and post data overridden by route.continue, which is what was sent.
func TestPageRouteContinueOverridesRequest(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	tb.withHandler("/home", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, `<!DOCTYPE html><html><body><script>fetch('/api/pizza')</script></body></html>`)
		require.NoError(t, err)
	})

	var (
		mu       sync.Mutex
		received struct{ method, body, foo string }
	)
	tb.withHandler("/api/pizza", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		mu.Lock()
		received.method, received.body, received.foo = r.Method, string(body), r.Header.Get("foo")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, err = fmt.Fprint(w, `{"name": "Classic Pizza"}`)
		require.NoError(t, err)
	})

	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)
	defer tb.vu.EndIteration(t)

	gv, err := tb.vu.RunAsync(t, `
		const page = await browser.newPage();

		await page.route(/.*\/api\/pizza$/, async function (route) {
			await route.continue({
				headers: {
					...route.request().headers(),
					'foo': 'bar'
				},
				method: 'POST',
				postData: JSON.stringify({customName: 'Classic Pizza'}),
			});
		});

		let resolveRequest;
		const requestSeen = new Promise((resolve) => { resolveRequest = resolve; });
		page.on('response', async (response) => {
			if (!response.url().endsWith('/api/pizza')) {
				return;
			}
			resolveRequest({
				url: response.request().url(),
				method: response.request().method(),
				headers: response.request().headers(),
				allHeaders: await response.request().allHeaders(),
				postData: response.request().postData(),
			});
		});

		await page.goto('%s', {waitUntil: 'networkidle'});
		const request = await requestSeen;
		await page.close();

		return JSON.stringify(request);
	`, tb.url("/home"))
	require.NoError(t, err)

	var got struct {
		URL        string            `json:"url"`
		Method     string            `json:"method"`
		Headers    map[string]string `json:"headers"`
		AllHeaders map[string]string `json:"allHeaders"`
		PostData   string            `json:"postData"`
	}
	require.NoError(t, json.Unmarshal([]byte(k6test.ToPromise(t, gv).Result().String()), &got))

	mu.Lock()
	defer mu.Unlock()
	// The server received the overridden request...
	assert.Equal(t, "POST", received.method)
	assert.Equal(t, `{"customName":"Classic Pizza"}`, received.body)
	assert.Equal(t, "bar", received.foo)
	// ...and the request seen from the response handler reflects it.
	assert.Equal(t, tb.url("/api/pizza"), got.URL)
	assert.Equal(t, "POST", got.Method)
	assert.Equal(t, `{"customName":"Classic Pizza"}`, got.PostData)
	assert.Equal(t, "bar", got.Headers["foo"])
	assert.Equal(t, "bar", got.AllHeaders["foo"])
}

// TestPageRouteFulfillResponse checks the other half of
// https://github.com/grafana/k6/issues/5012: a request fulfilled by
// route.fulfill is never sent, and its response reports the fulfilled
// status, headers and body, while response.request() keeps the original
// request.
func TestPageRouteFulfillResponse(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	tb.withHandler("/home", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, `<!DOCTYPE html><html><body><script>fetch('/api/pizza', {method: 'POST', body: 'original'})</script></body></html>`)
		require.NoError(t, err)
	})
	tb.withHandler("/api/pizza", func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("a fulfilled request must not reach the server")
	})

	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)
	defer tb.vu.EndIteration(t)

	gv, err := tb.vu.RunAsync(t, `
		const page = await browser.newPage();

		await page.route(/.*\/api\/pizza$/, async function (route) {
			await route.fulfill({
				status: 201,
				headers: {'x-fulfilled': 'yes'},
				contentType: 'application/json',
				body: JSON.stringify({fulfilled: true}),
			});
		});

		let resolveResponse;
		const responseSeen = new Promise((resolve) => { resolveResponse = resolve; });
		page.on('response', async (res) => {
			if (!res.url().endsWith('/api/pizza')) {
				return;
			}
			resolveResponse({
				status: res.status(),
				headers: await res.allHeaders(),
				body: await res.text(),
				requestMethod: res.request().method(),
				requestPostData: res.request().postData(),
			});
		});

		await page.goto('%s', {waitUntil: 'networkidle'});
		const response = await responseSeen;
		await page.close();

		return JSON.stringify(response);
	`, tb.url("/home"))
	require.NoError(t, err)

	var got struct {
		Status          int               `json:"status"`
		Headers         map[string]string `json:"headers"`
		Body            string            `json:"body"`
		RequestMethod   string            `json:"requestMethod"`
		RequestPostData string            `json:"requestPostData"`
	}
	require.NoError(t, json.Unmarshal([]byte(k6test.ToPromise(t, gv).Result().String()), &got))

	assert.Equal(t, 201, got.Status)
	assert.Equal(t, "yes", got.Headers["x-fulfilled"])
	assert.Equal(t, "application/json", got.Headers["content-type"])
	assert.Equal(t, `{"fulfilled":true}`, got.Body)
	assert.Equal(t, "POST", got.RequestMethod)
	assert.Equal(t, "original", got.RequestPostData)
}
