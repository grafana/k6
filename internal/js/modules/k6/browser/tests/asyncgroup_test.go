package tests

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/features"
	k6mod "go.k6.io/k6/v2/internal/js/modules/k6"
	k6exec "go.k6.io/k6/v2/internal/js/modules/k6/execution"
	k6common "go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/metrics"
)

func prepareAsyncGroupBrowser(t *testing.T, tb *testBrowser) {
	t.Helper()
	tb.vu.StartIteration(t)
	tb.vu.State().FeatureFlags = &features.Flags{AsyncMetricContext: true}
	tb.vu.Runtime().SetAsyncContextTracker(k6common.NewMetricContextTracker(tb.vu.State))

	k6Instance := k6mod.New().NewModuleInstance(tb.vu)
	tb.vu.SetVar(t, "k6", k6Instance.Exports().Named)
	execInstance := k6exec.New().NewModuleInstance(tb.vu)
	tb.vu.SetVar(t, "exec", execInstance.Exports().Default)
}

func collectBrowserSamples(tb *testBrowser) []metrics.Sample {
	var samples []metrics.Sample
	tb.vu.AssertSamples(func(sample metrics.Sample) {
		samples = append(samples, sample)
	})
	return samples
}

func assertBrowserCheckGroups(t *testing.T, samples []metrics.Sample, expected map[string]string) {
	t.Helper()

	seen := make(map[string]bool, len(expected))
	for _, sample := range samples {
		if sample.Metric.Name != metrics.ChecksName {
			continue
		}
		name, ok := sample.Tags.Get(metrics.TagCheck.String())
		require.True(t, ok)
		expectedGroup, ok := expected[name]
		require.Truef(t, ok, "unexpected check %q", name)
		assert.Equal(t, float64(1), sample.Value, "check %q failed", name)
		group, _ := sample.Tags.Get(metrics.TagGroup.String())
		assert.Equal(t, expectedGroup, group, "check %q ran under wrong group", name)
		seen[name] = true
	}
	for name := range expected {
		assert.Truef(t, seen[name], "missing check %q", name)
	}
}

func TestAsyncGroupBrowserEventListeners(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	prepareAsyncGroupBrowser(t, tb)

	serve := func(page, api, css string) {
		tb.withHandler(page, func(w http.ResponseWriter, _ *http.Request) {
			_, err := fmt.Fprintf(w, `<!DOCTYPE html><html><head>
<link rel="stylesheet" href="%s"></head><body>
<script>fetch('%s', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: '{}'})</script>
</body></html>`, css, api)
			require.NoError(t, err)
		})
		tb.withHandler(api, func(w http.ResponseWriter, _ *http.Request) {
			_, err := fmt.Fprint(w, `{"ok":true}`)
			require.NoError(t, err)
		})
		tb.withHandler(css, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/css")
			_, err := fmt.Fprint(w, `body{background:#fff}`)
			require.NoError(t, err)
		})
	}
	serve("/home1", "/api1", "/style1.css")
	serve("/home2", "/api2", "/style2.css")

	_, err := tb.vu.RunAsync(t, `
		async function loadPageInGroup(groupName, label, url, createFromContext) {
			await k6.group(groupName, async () => {
				let context;
				let page;
				if (createFromContext) {
					context = await browser.newContext();
					page = await context.newPage();
				} else {
					page = await browser.newPage();
					context = page.context();
				}
				exec.vu.metrics.tags.owner = label + ':registered';
				page.on('request', (request) => {
					k6.check(null, {[label]: true});
				});
				exec.vu.metrics.tags.owner = label + ':active';
				await page.goto(url, {waitUntil: 'networkidle'});
				await page.close();
				await context.close();
			});
		}

		await loadPageInGroup('page1', 'page1', '%s', false);
		await loadPageInGroup('page2', 'page2', '%s', true);
	`, tb.url("/home1"), tb.url("/home2"))
	require.NoError(t, err)

	samples := collectBrowserSamples(tb)
	assertBrowserCheckGroups(t, samples, map[string]string{
		"page1": "::page1",
		"page2": "::page2",
	})
	for _, sample := range samples {
		if sample.Metric.Name != metrics.ChecksName {
			continue
		}
		name, _ := sample.Tags.Get(metrics.TagCheck.String())
		owner, _ := sample.Tags.Get("owner")
		assert.Equal(t, name+":registered", owner, "check %q used the wrong metric context", name)
	}

	netGroups := map[string]int{}
	var netSamples int
	for _, s := range samples {
		switch s.Metric.Name {
		case "browser_data_sent", "browser_data_received":
			netSamples++
			g, _ := s.Tags.Get("group")
			netGroups[g]++
		}
	}
	require.Positive(t, netSamples, "expected browser network metric samples")
	assert.Zero(t, netGroups[""],
		"browser network metrics must not be attributed to the root group; got distribution %v", netGroups)
	assert.Positive(t, netGroups["::page1"], "expected browser network metrics attributed to ::page1")
	assert.Positive(t, netGroups["::page2"], "expected browser network metrics attributed to ::page2")
}

func TestAsyncGroupBrowserTaskCallbacks(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	prepareAsyncGroupBrowser(t, tb)
	tb.withHandler("/callbacks", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, `<!doctype html><title>callbacks</title>`)
		require.NoError(t, err)
	})

	_, err := tb.vu.RunAsync(t, `
		await k6.group('callbacks', async () => {
			const context = await browser.newContext();
			const contextEvent = context.waitForEvent('page', {
				timeout: 5000,
				predicate: () => {
					k6.check(null, {'context.waitForEvent': true});
					return true;
				},
			});
			const page = await context.newPage();
			await contextEvent;

			await page.route('%s', route => {
				k6.check(null, {'page.route': true});
				route.continue();
			});
			const requestEvent = page.waitForEvent('request', {
				timeout: 5000,
				predicate: request => {
					k6.check(null, {'page.waitForEvent': true});
					return request.url() === '%s';
				},
			});
			await page.goto('%s');
			await requestEvent;
			await page.close();
			await context.close();
		});
		k6.check(null, {after: true});
	`, tb.url("/callbacks"), tb.url("/callbacks"), tb.url("/callbacks"))
	require.NoError(t, err)

	assertBrowserCheckGroups(t, collectBrowserSamples(tb), map[string]string{
		"context.waitForEvent": "::callbacks",
		"page.route":           "::callbacks",
		"page.waitForEvent":    "::callbacks",
		"after":                "",
	})
}

func TestAsyncGroupBrowserAsyncCallbacksKeepContextMutations(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	prepareAsyncGroupBrowser(t, tb)
	tb.withHandler("/async-callbacks", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, `<!doctype html><title>async callbacks</title>`)
		require.NoError(t, err)
	})

	_, err := tb.vu.RunAsync(t, `
		exec.vu.metrics.tags.owner = 'root';
		exec.vu.metrics.tags.phase = 'root';
		exec.vu.metrics.metadata.trace = 'root';
		exec.vu.metrics.metadata.step = 'root';

		const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
		const deferred = () => {
			let resolve;
			let reject;
			const promise = new Promise((res, rej) => {
				resolve = res;
				reject = rej;
			});
			return {promise, resolve, reject};
		};
		const phases = ['first', 'second', 'third'];
		const changeContext = (name, phase) => {
			const value = name + ':' + phase;
			exec.vu.metrics.tags.phase = value;
			exec.vu.metrics.metadata.step = value;
		};
		const crossBoundary = phase => phase === 'second' ? delay(0) : Promise.resolve();
		const recordContext = (name, phase) => k6.check(null, {[name + ':' + phase]: true});

		await k6.group('browser-callbacks', async () => {
			exec.vu.metrics.tags.owner = 'browser';
			exec.vu.metrics.metadata.trace = 'browser';
			const page = await browser.newPage();
			const eventFinished = deferred();
			const routeFinished = deferred();

			page.on('request', async request => {
				if (request.url() !== '%s') return;
				try {
					for (const phase of phases) {
						changeContext('event', phase);
						await crossBoundary(phase);
						recordContext('event', phase);
					}
					eventFinished.resolve();
				} catch (error) {
					eventFinished.reject(error);
				}
			});
			await page.route('%s', async route => {
				try {
					for (const phase of phases) {
						changeContext('route', phase);
						await crossBoundary(phase);
						recordContext('route', phase);
					}
					await route.continue();
					routeFinished.resolve();
				} catch (error) {
					routeFinished.reject(error);
				}
			});

			await page.goto('%s');
			await Promise.all([eventFinished.promise, routeFinished.promise]);
			await page.close();
		});
		k6.check(null, {after: true});
	`, tb.url("/async-callbacks"), tb.url("/async-callbacks"), tb.url("/async-callbacks"))
	require.NoError(t, err)

	expected := map[string]string{"after": "root"}
	for _, callback := range []string{"event", "route"} {
		for _, phase := range []string{"first", "second", "third"} {
			value := callback + ":" + phase
			expected[value] = value
		}
	}
	for _, sample := range collectBrowserSamples(tb) {
		if sample.Metric.Name != metrics.ChecksName {
			continue
		}
		name, _ := sample.Tags.Get(metrics.TagCheck.String())
		phase, ok := expected[name]
		require.Truef(t, ok, "unexpected check %q", name)
		assert.Equal(t, float64(1), sample.Value, "check %q failed", name)
		assert.Equal(t, phase, sample.Metadata["step"], "check %q used the wrong metadata", name)
		actualPhase, _ := sample.Tags.Get("phase")
		assert.Equal(t, phase, actualPhase, "check %q used the wrong tag", name)
		if name == "after" {
			group, _ := sample.Tags.Get(metrics.TagGroup.String())
			assert.Empty(t, group)
			owner, _ := sample.Tags.Get("owner")
			assert.Equal(t, "root", owner)
			assert.Equal(t, "root", sample.Metadata["trace"])
		} else {
			group, _ := sample.Tags.Get(metrics.TagGroup.String())
			assert.Equal(t, "::browser-callbacks", group)
			owner, _ := sample.Tags.Get("owner")
			assert.Equal(t, "browser", owner)
			assert.Equal(t, "browser", sample.Metadata["trace"])
		}
		delete(expected, name)
	}
	require.Empty(t, expected)
}

func TestAsyncGroupBrowserInputActions(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	prepareAsyncGroupBrowser(t, tb)
	tb.withHandler("/inputs", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, `<!doctype html>
<input id="keyboard">
<button id="mouse">mouse</button>
<button id="touch">touch</button>
<script>
document.querySelector('#keyboard').addEventListener('keydown', event => {
    if (event.key === 'Enter') fetch('/keyboard');
});
document.querySelector('#mouse').addEventListener('click', () => fetch('/mouse'));
document.querySelector('#touch').addEventListener('click', () => fetch('/touch'));
</script>`)
		require.NoError(t, err)
	})
	for _, path := range []string{"/keyboard", "/mouse", "/touch"} {
		tb.withHandler(path, func(w http.ResponseWriter, _ *http.Request) {
			_, err := fmt.Fprint(w, `{"ok":true}`)
			require.NoError(t, err)
		})
	}

	_, err := tb.vu.RunAsync(t, `
		const context = await browser.newContext({hasTouch: true});
		const page = await context.newPage();
		await page.goto('%s');

		const waitForRequest = path => page.waitForEvent('request', {
			timeout: 5000,
			predicate: request => request.url().endsWith(path),
		});

		await page.locator('#keyboard').focus();
		await k6.group('keyboard', async () => {
			const request = waitForRequest('/keyboard');
			await page.keyboard.press('Enter');
			await request;
		});

		const mouseBox = await page.locator('#mouse').boundingBox();
		await k6.group('mouse', async () => {
			const request = waitForRequest('/mouse');
			await page.mouse.click(mouseBox.x + mouseBox.width / 2, mouseBox.y + mouseBox.height / 2);
			await request;
		});

		const touchBox = await page.locator('#touch').boundingBox();
		await k6.group('touch', async () => {
			const request = waitForRequest('/touch');
			await page.touchscreen.tap(touchBox.x + touchBox.width / 2, touchBox.y + touchBox.height / 2);
			await request;
		});
		await page.close();
		await context.close();
	`, tb.url("/inputs"))
	require.NoError(t, err)

	expected := map[string]string{
		tb.url("/keyboard"): "::keyboard",
		tb.url("/mouse"):    "::mouse",
		tb.url("/touch"):    "::touch",
	}
	seen := make(map[string]bool, len(expected))
	for _, sample := range collectBrowserSamples(tb) {
		if sample.Metric.Name != "browser_data_sent" && sample.Metric.Name != "browser_data_received" {
			continue
		}
		url, _ := sample.Tags.Get(metrics.TagURL.String())
		expectedGroup, ok := expected[url]
		if !ok {
			continue
		}
		group, _ := sample.Tags.Get(metrics.TagGroup.String())
		assert.Equal(t, expectedGroup, group, "%s had the wrong group", url)
		seen[url] = true
	}
	for url := range expected {
		assert.Truef(t, seen[url], "missing browser network samples for %s", url)
	}
}

func TestAsyncGroupPopupInheritsNetworkGroup(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	prepareAsyncGroupBrowser(t, tb)
	tb.withHandler("/opener", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, `<!doctype html><script>window.open('/popup')</script>`)
		require.NoError(t, err)
	})
	tb.withHandler("/popup", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, `<!doctype html><script>setTimeout(() => fetch('/popup-api'), 50)</script>`)
		require.NoError(t, err)
	})
	tb.withHandler("/popup-api", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, `{"ok":true}`)
		require.NoError(t, err)
	})

	_, err := tb.vu.RunAsync(t, `
		await k6.group('popup', async () => {
			const context = await browser.newContext();
			const opener = await context.newPage();
			const popupEvent = context.waitForEvent('page');
			await opener.goto('%s');
			await popupEvent;
			await new Promise(resolve => setTimeout(resolve, 250));
			await context.close();
		});
	`, tb.url("/opener"))
	require.NoError(t, err)

	var popupSamples int
	for _, sample := range collectBrowserSamples(tb) {
		if sample.Metric.Name != "browser_data_sent" && sample.Metric.Name != "browser_data_received" {
			continue
		}
		url, _ := sample.Tags.Get(metrics.TagURL.String())
		if !strings.Contains(url, "/popup") {
			continue
		}
		popupSamples++
		group, _ := sample.Tags.Get(metrics.TagGroup.String())
		assert.Equal(t, "::popup", group, "popup network sample had the wrong group")
	}
	require.Positive(t, popupSamples, "expected popup network metric samples")
}

func TestAsyncGroupBrowserRequestRetainsTagsAndMetadata(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	prepareAsyncGroupBrowser(t, tb)

	releaseSlow := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseSlow) }) })
	tb.vu.SetVar(t, "releaseSlow", func() { releaseOnce.Do(func() { close(releaseSlow) }) })
	tb.withHandler("/empty", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, `<!doctype html><title>snapshot</title>`)
		require.NoError(t, err)
	})
	tb.withHandler("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-releaseSlow:
		case <-r.Context().Done():
		}
		_, err := fmt.Fprint(w, `{"ok":true}`)
		require.NoError(t, err)
	})

	_, err := tb.vu.RunAsync(t, `
		const page = await browser.newPage();
		await page.goto('%s');
		const started = page.waitForEvent('request', {
			timeout: 5000,
			predicate: request => request.url() === '%s',
		});

		exec.vu.metrics.tags.phase = 'request';
		exec.vu.metrics.metadata.trace = 'request';
		const slow = k6.group('request', () => page.evaluate(() => fetch('%s')));
		// Wait until NetworkManager has created the Request and copied the page snapshot.
		await started;

		exec.vu.metrics.tags.phase = 'after';
		exec.vu.metrics.metadata.trace = 'after';
		k6.group('after', () => page.url());

		releaseSlow();
		await slow;
		await page.close();
	`, tb.url("/empty"), tb.url("/slow"), tb.url("/slow"))
	require.NoError(t, err)

	var samples int
	for _, sample := range collectBrowserSamples(tb) {
		url, _ := sample.Tags.Get(metrics.TagURL.String())
		if url != tb.url("/slow") {
			continue
		}
		samples++
		group, _ := sample.Tags.Get(metrics.TagGroup.String())
		assert.Equal(t, "::request", group)
		phase, _ := sample.Tags.Get("phase")
		assert.Equal(t, "request", phase)
		assert.Equal(t, "request", sample.Metadata["trace"])
	}
	require.Positive(t, samples, "expected browser network metric samples for the delayed request")
}

func TestAsyncGroupBrowserReadOnlyCallDoesNotRetagActiveOperation(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())
	prepareAsyncGroupBrowser(t, tb)
	tb.withHandler("/empty", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, `<!doctype html><title>operation context</title>
<button id="request">request</button>
<script>document.querySelector('#request').addEventListener('click', () => fetch('/locator'))</script>`)
		require.NoError(t, err)
	})
	for _, path := range []string{"/operation", "/locator", "/frame", "/element", "/element-handle"} {
		tb.withHandler(path, func(w http.ResponseWriter, _ *http.Request) {
			_, err := fmt.Fprint(w, `{"ok":true}`)
			require.NoError(t, err)
		})
	}

	_, err := tb.vu.RunAsync(t, `
		const page = await browser.newPage();
		await page.goto('%s');

		const operation = k6.group('operation', () => {
			exec.vu.metrics.tags.phase = 'operation';
			exec.vu.metrics.metadata.trace = 'operation';
			return page.evaluate(() => new Promise((resolve, reject) => {
				setTimeout(() => fetch('%s').then(() => resolve(), reject), 100);
			}));
		});

		await new Promise(resolve => setTimeout(resolve, 25));
		k6.group('inspection', () => {
			exec.vu.metrics.tags.phase = 'inspection';
			exec.vu.metrics.metadata.trace = 'inspection';
			page.url();
		});

		await operation;

		const locator = page.locator('#request');
		const locatorResponse = page.waitForResponse('%s');
		await k6.group('locator-operation', () => {
			exec.vu.metrics.tags.phase = 'locator';
			exec.vu.metrics.metadata.trace = 'locator';
			return locator.click();
		});
		await locatorResponse;

		const frame = page.mainFrame();
		await k6.group('frame-operation', () => {
			exec.vu.metrics.tags.phase = 'frame';
			exec.vu.metrics.metadata.trace = 'frame';
			return frame.evaluate(() => fetch('%s').then(() => null));
		});

		const element = await page.$('#request');
		await k6.group('element-operation', () => {
			exec.vu.metrics.tags.phase = 'element';
			exec.vu.metrics.metadata.trace = 'element';
			return element.evaluate((_, url) => fetch(url).then(() => null), '%s');
		});

		const elementHandleResponse = page.waitForResponse('%s');
		const evaluatedHandle = await k6.group('element-handle-operation', () => {
			exec.vu.metrics.tags.phase = 'element-handle';
			exec.vu.metrics.metadata.trace = 'element-handle';
			return element.evaluateHandle((node, url) => {
				fetch(url);
				return node;
			}, '%s');
		});
		await elementHandleResponse;
		await evaluatedHandle.dispose();

		await page.close();
	`, tb.url("/empty"), tb.url("/operation"), tb.url("/locator"), tb.url("/frame"),
		tb.url("/element"), tb.url("/element-handle"), tb.url("/element-handle"))
	require.NoError(t, err)

	expected := map[string]struct {
		group string
		phase string
	}{
		tb.url("/operation"): {group: "::operation", phase: "operation"},
		tb.url("/locator"):   {group: "::locator-operation", phase: "locator"},
		tb.url("/frame"):     {group: "::frame-operation", phase: "frame"},
		tb.url("/element"):   {group: "::element-operation", phase: "element"},
		tb.url("/element-handle"): {
			group: "::element-handle-operation", phase: "element-handle",
		},
	}
	seen := make(map[string]bool, len(expected))
	for _, sample := range collectBrowserSamples(tb) {
		url, _ := sample.Tags.Get(metrics.TagURL.String())
		want, ok := expected[url]
		if !ok {
			continue
		}
		seen[url] = true
		group, _ := sample.Tags.Get(metrics.TagGroup.String())
		assert.Equal(t, want.group, group)
		phase, _ := sample.Tags.Get("phase")
		assert.Equal(t, want.phase, phase)
		assert.Equal(t, want.phase, sample.Metadata["trace"])
	}
	for url := range expected {
		assert.Truef(t, seen[url], "expected browser network metric samples for %s", url)
	}
}
