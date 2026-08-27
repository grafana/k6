package sse

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6-extension-api"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

type testVU struct {
	*extensionapitest.VU
	client      *http.Client
	defaultJar  http.CookieJar
	completions atomic.Int32
}

func (v *testVU) Do(
	ctx context.Context, request *http.Request, options extensionapi.HTTPOptions,
) (*extensionapi.HTTPResponse, error) {
	request = request.Clone(ctx)
	client := *v.client
	if options.Jar != nil {
		client.Jar = options.Jar
	} else {
		client.Jar = v.defaultJar
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	return &extensionapi.HTTPResponse{
		Response: response,
		Complete: func() { v.completions.Add(1) },
	}, nil
}

func newTestModule(t *testing.T, client *http.Client) (*testVU, *sse) {
	t.Helper()
	vu := &testVU{VU: extensionapitest.NewVU(), client: client}
	vu.EnabledSystemTag[extensionapi.SystemTagURL] = true
	vu.EnabledSystemTag[extensionapi.SystemTagStatus] = true
	vu.EnabledSystemTag[extensionapi.SystemTagProto] = true
	vu.EnabledSystemTag[extensionapi.SystemTagSubproto] = true
	instance, ok := New().NewModuleInstance(vu).(*sse)
	require.True(t, ok)
	require.NoError(t, vu.Runtime().Set("sse", instance.Exports().Default))
	return vu, instance
}

func TestOpenEmitsEventsAndCompletesMetrics(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("id: one\ndata: first\n\n"))
		_, _ = w.Write([]byte("event: update\ndata: second\n\n"))
	}))
	defer server.Close()

	vu, _ := newTestModule(t, server.Client())
	_, err := vu.Runtime().RunString(`
		var received = [];
		var response = sse.open("` + server.URL + `", {tags: {source: "test"}}, function(client) {
			client.on("event", function(event) { received.push(event); });
		});
		globalThis.debug = JSON.stringify({status: response.status, received: received});
	`)
	require.NoError(t, err)
	require.Equal(t, `{"status":200,"received":[{"id":"one","comment":"","name":"","data":"first"},{"id":"","comment":"","name":"update","data":"second"}]}`, vu.Runtime().Get("debug").String())
	require.Equal(t, int32(1), vu.completions.Load())

	samples := vu.Samples()
	require.Len(t, samples, 2)
	for _, sample := range samples {
		require.Equal(t, MetricEventName, sample.Metric.Name())
		require.Equal(t, "test", sample.Tags.Values()["source"])
		require.Equal(t, server.URL, sample.Tags.Values()["url"])
		require.Equal(t, "200", sample.Tags.Values()["status"])
	}
}

func TestOpenUsesCustomCookieJar(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if cookie, err := request.Cookie("session"); err != nil || cookie.Value != "custom" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("data: ok\n\n"))
	}))
	defer server.Close()

	vu, _ := newTestModule(t, server.Client())
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	jar.SetCookies(serverURL, []*http.Cookie{{Name: "session", Value: "custom"}})
	require.NoError(t, vu.Runtime().Set("jar", jar))

	_, err = vu.Runtime().RunString(`
		var response = sse.open("` + server.URL + `", {jar: jar}, function() {});
		if (response.status !== 200) { throw new Error("cookie jar was not used"); }
	`)
	require.NoError(t, err)
}

func TestOpenRejectsInvalidArguments(t *testing.T) {
	t.Parallel()
	vu, _ := newTestModule(t, http.DefaultClient)
	_, err := vu.Runtime().RunString(`sse.open("http://[::1", function() {})`)
	require.Error(t, err)
	_, err = vu.Runtime().RunString(`sse.open("http://example.test", {timeout: "invalid"}, function() {})`)
	require.Error(t, err)
}

func TestExportedCookieJarSupportsOlderWrapper(t *testing.T) {
	t.Parallel()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	type legacyCookieJar struct{ Jar *cookiejar.Jar }
	actual, ok := exportedCookieJar(&legacyCookieJar{Jar: jar})
	require.True(t, ok)
	require.Same(t, jar, actual)
}
