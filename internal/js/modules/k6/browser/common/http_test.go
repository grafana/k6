package common

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

func TestRequest(t *testing.T) {
	t.Parallel()

	ts := cdp.MonotonicTime(time.Now())
	wt := cdp.TimeSinceEpoch(time.Now())
	headers := map[string]any{"key": "value"}
	evt := &network.EventRequestWillBeSent{
		RequestID: network.RequestID("1234"),
		Request: &network.Request{
			URL:             "https://test/post",
			Method:          "POST",
			Headers:         network.Headers(headers),
			PostDataEntries: []*network.PostDataEntry{{Bytes: "aGVsbG8="}}, // base64 encoded "hello"
		},
		Timestamp: &ts,
		WallTime:  &wt,
	}
	vu := k6test.NewVU(t)
	req, err := NewRequest(vu.Context(), log.NewNullLogger(), NewRequestParams{
		event:          evt,
		interceptionID: "intercept",
	})
	require.NoError(t, err)

	t.Run("error_parse_url", func(t *testing.T) {
		t.Parallel()

		evt := &network.EventRequestWillBeSent{
			RequestID: network.RequestID("1234"),
			Request: &network.Request{
				URL:             ":",
				Method:          "POST",
				Headers:         network.Headers(headers),
				PostDataEntries: []*network.PostDataEntry{{Bytes: "aGVsbG8="}}, // base64 encoded "hello"
			},
			Timestamp: &ts,
			WallTime:  &wt,
		}
		vu := k6test.NewVU(t)
		req, err := NewRequest(vu.Context(), log.NewNullLogger(), NewRequestParams{
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
		got, ok := req.HeaderValue("key")
		assert.True(t, ok)
		assert.Equal(t, "value", got)
	})

	t.Run("HeaderValue()_KEY", func(t *testing.T) {
		t.Parallel()
		got, ok := req.HeaderValue("KEY")
		assert.True(t, ok)
		assert.Equal(t, "value", got)
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

func TestValidateResourceType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: ResourceTypeDocument, input: network.ResourceTypeDocument.String(), want: ResourceTypeDocument},
		{name: ResourceTypeStylesheet, input: network.ResourceTypeStylesheet.String(), want: ResourceTypeStylesheet},
		{name: ResourceTypeImage, input: network.ResourceTypeImage.String(), want: ResourceTypeImage},
		{name: ResourceTypeMedia, input: network.ResourceTypeMedia.String(), want: ResourceTypeMedia},
		{name: ResourceTypeFont, input: network.ResourceTypeFont.String(), want: ResourceTypeFont},
		{name: ResourceTypeScript, input: network.ResourceTypeScript.String(), want: ResourceTypeScript},
		{name: ResourceTypeTextTrack, input: network.ResourceTypeTextTrack.String(), want: ResourceTypeTextTrack},
		{name: ResourceTypeXHR, input: network.ResourceTypeXHR.String(), want: ResourceTypeXHR},
		{name: ResourceTypeFetch, input: network.ResourceTypeFetch.String(), want: ResourceTypeFetch},
		{name: ResourceTypePrefetch, input: network.ResourceTypePrefetch.String(), want: ResourceTypePrefetch},
		{name: ResourceTypeEventSource, input: network.ResourceTypeEventSource.String(), want: ResourceTypeEventSource},
		{name: ResourceTypeWebSocket, input: network.ResourceTypeWebSocket.String(), want: ResourceTypeWebSocket},
		{name: ResourceTypeManifest, input: network.ResourceTypeManifest.String(), want: ResourceTypeManifest},
		{name: ResourceTypeSignedExchange, input: network.ResourceTypeSignedExchange.String(), want: ResourceTypeSignedExchange},
		{name: ResourceTypePing, input: network.ResourceTypePing.String(), want: ResourceTypePing},
		{name: ResourceTypeCSPViolationReport, input: network.ResourceTypeCSPViolationReport.String(), want: ResourceTypeCSPViolationReport},
		{name: ResourceTypePreflight, input: network.ResourceTypePreflight.String(), want: ResourceTypePreflight},
		{name: ResourceTypeOther, input: network.ResourceTypeOther.String(), want: ResourceTypeOther},
		{name: "fake", input: "fake", want: ResourceTypeUnknown},
		{name: "amended_existing", input: strings.ToLower(network.ResourceTypeOther.String()), want: ResourceTypeUnknown},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := validateResourceType(log.NewNullLogger(), tt.input)
			assert.Equal(t, got, tt.want)
		})
	}
}
