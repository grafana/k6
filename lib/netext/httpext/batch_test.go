package httpext

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/lib"
)

func TestMakeBatchRequestsRejectsNonPositiveConcurrency(t *testing.T) {
	t.Parallel()

	u, err := NewURL("https://example.com/", "https://example.com/")
	require.NoError(t, err)

	requests := []BatchParsedHTTPRequest{
		{
			ParsedHTTPRequest: &ParsedHTTPRequest{
				URL: &u,
				Req: &http.Request{Method: http.MethodGet, URL: u.GetURL()},
			},
			Response: new(Response),
		},
		{
			ParsedHTTPRequest: &ParsedHTTPRequest{
				URL: &u,
				Req: &http.Request{Method: http.MethodGet, URL: u.GetURL()},
			},
			Response: new(Response),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for _, limit := range []int{0, -1} {
		t.Run(string(rune('a'+limit+1)), func(t *testing.T) {
			t.Parallel()
			errs := MakeBatchRequests(ctx, &lib.State{}, requests, len(requests), limit, 1)
			for i := 0; i < len(requests); i++ {
				select {
				case err := <-errs:
					require.Error(t, err)
					assert.Contains(t, err.Error(), "batch concurrency must be a positive number")
				case <-ctx.Done():
					t.Fatal("MakeBatchRequests hung with non-positive globalLimit")
				}
			}
		})
	}
}
