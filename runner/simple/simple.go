package simple

import (
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"time"
)

type SimpleRunner struct {
	Client *fasthttp.Client
}

func New() *SimpleRunner {
	return &SimpleRunner{
		Client: &fasthttp.Client{
			MaxIdleConnDuration: time.Duration(0),
		},
	}
}

func (r *SimpleRunner) Run(ctx context.Context, t loadtest.LoadTest, id int64) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		result := make(chan runner.Result, 1)
		for {
			go func() {
				req := fasthttp.AcquireRequest()
				defer fasthttp.ReleaseRequest(req)

				res := fasthttp.AcquireResponse()
				defer fasthttp.ReleaseResponse(res)

				req.SetRequestURI(t.URL)

				startTime := time.Now()
				err := r.Client.Do(req, res)
				duration := time.Since(startTime)

				result <- runner.Result{Error: err, Time: duration}
			}()

			select {
			case <-ctx.Done():
				return
			case res := <-result:
				ch <- res
			}
		}
	}()

	return ch
}
