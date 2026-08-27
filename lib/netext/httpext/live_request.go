package httpext

import (
	"context"
	"io"
	"net/http"
	"sync"

	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

// LiveRequestOptions configures a request that retains its response body.
type LiveRequestOptions struct {
	TagsAndMeta      metrics.TagsAndMeta
	Jar              http.CookieJar
	ResponseCallback func(int) bool
	Transport        http.RoundTripper
}

// MakeRequestWithLiveResponse executes req with k6's measured transport. The
// returned body emits the final request samples when it reaches EOF or closes.
func MakeRequestWithLiveResponse(
	ctx context.Context, state *lib.State, req *http.Request, options LiveRequestOptions,
) (*http.Response, error) {
	traced := newTransport(ctx, state, &options.TagsAndMeta, options.ResponseCallback, options.Transport)
	client := &http.Client{Transport: traced, Jar: options.Jar}
	response, err := client.Do(req.WithContext(ctx)) //nolint:gosec
	if err != nil {
		traced.processLastSavedRequest(err)
		return response, err
	}
	response.Body = &measuredBody{ReadCloser: response.Body, finish: traced.processLastSavedRequest}
	return response, nil
}

type measuredBody struct {
	io.ReadCloser
	once   sync.Once
	finish func(error) *finishedRequest
	err    error
}

func (b *measuredBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		if err != io.EOF {
			b.err = err
		}
		b.complete(b.err)
	}
	return n, err
}

func (b *measuredBody) Close() error {
	err := b.ReadCloser.Close()
	if err != nil {
		b.err = err
	}
	b.complete(b.err)
	return err
}

func (b *measuredBody) complete(err error) { b.once.Do(func() { b.finish(err) }) }
