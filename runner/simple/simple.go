package simple

import (
	"github.com/loadimpact/speedboat/runner"
	"golang.org/x/net/context"
	"net/http"
	"time"
)

type SimpleRunner struct {
	URL    string
	Client *http.Client
}

func New() *SimpleRunner {
	return &SimpleRunner{
		Client: &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}
}

func (r *SimpleRunner) Run(ctx context.Context) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		// We can reuse the same request across multiple iterations; if something goes awry here,
		// we abort the test, since it normally means a user failure (like a malformed URL).
		req, err := http.NewRequest(http.MethodGet, r.URL, nil)
		if err != nil {
			ch <- runner.Result{Error: err}
			return
		}
		req.Close = true

		// Close this channel to abort the request on the spot. The old, transport-based way of
		// doing this is deprecated, as it doesn't play nice with HTTP/2 requests.
		cancelRequest := make(chan struct{})
		req.Cancel = cancelRequest

		results := make(chan runner.Result, 1)
		for {
			go func() {
				startTime := time.Now()
				res, err := r.Client.Do(req)
				duration := time.Since(startTime)

				if err != nil {
					results <- runner.Result{Error: err, Time: duration}
					return
				}
				res.Body.Close()

				results <- runner.Result{Time: duration}
			}()

			select {
			case res := <-results:
				ch <- res
			case <-ctx.Done():
				close(cancelRequest)
				return
			}
		}
	}()

	return ch
}
