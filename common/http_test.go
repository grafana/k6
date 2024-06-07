package common

import (
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/k6ext/k6test"
)

func TestRequest(t *testing.T) {
	t.Parallel()

	ts := cdp.MonotonicTime(time.Now())
	wt := cdp.TimeSinceEpoch(time.Now())
	headers := map[string]any{"key": "value"}
	evt := &network.EventRequestWillBeSent{
		RequestID: network.RequestID("1234"),
		Request: &network.Request{
			URL:      "https://test/post",
			Method:   "POST",
			Headers:  network.Headers(headers),
			PostData: "hello",
		},
		Timestamp: &ts,
		WallTime:  &wt,
	}
	vu := k6test.NewVU(t)
	req, err := NewRequest(vu.Context(), NewRequestParams{
		event:          evt,
		interceptionID: "intercept",
	})
	require.NoError(t, err)

	t.Run("error_parse_url", func(t *testing.T) {
		t.Parallel()

		evt := &network.EventRequestWillBeSent{
			RequestID: network.RequestID("1234"),
			Request: &network.Request{
				URL:      ":",
				Method:   "POST",
				Headers:  network.Headers(headers),
				PostData: "hello",
			},
			Timestamp: &ts,
			WallTime:  &wt,
		}
		vu := k6test.NewVU(t)
		req, err := NewRequest(vu.Context(), NewRequestParams{
			event:          evt,
			interceptionID: "intercept",
		})
		require.EqualError(t, err, `parsing URL ":": missing protocol scheme`)
		require.Nil(t, req)
	})

	t.Run("Headers()", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, map[string]string{"key": "value"}, req.Headers())
	})

	t.Run("HeadersArray()", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, []HTTPHeader{
			{Name: "key", Value: "value"},
		}, req.HeadersArray())
	})

	t.Run("HeaderValue()_key", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "value", req.HeaderValue("key").Export())
	})

	t.Run("HeaderValue()_KEY", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "value", req.HeaderValue("KEY").Export())
	})

	t.Run("Size()", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t,
			HTTPMessageSize{Headers: int64(33), Body: int64(5)},
			req.Size())
	})
}

func TestResponse(t *testing.T) {
	t.Parallel()

	ts := cdp.MonotonicTime(time.Now())
	headers := map[string]any{"key": "value"}
	vu := k6test.NewVU(t)
	vu.ActivateVU()
	req := &Request{
		offset: 0,
	}
	res := NewHTTPResponse(vu.Context(), req, &network.Response{
		URL:     "https://test/post",
		Headers: network.Headers(headers),
	}, &ts)

	t.Run("HeaderValue()_key", func(t *testing.T) {
		t.Parallel()

		got, ok := res.HeaderValue("key")
		assert.True(t, ok)
		assert.Equal(t, "value", got)
	})

	t.Run("HeaderValue()_KEY", func(t *testing.T) {
		t.Parallel()

		got, ok := res.HeaderValue("KEY")
		assert.True(t, ok)
		assert.Equal(t, "value", got)
	})
}
