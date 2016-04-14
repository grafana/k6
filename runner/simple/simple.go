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

		for {
			startTime := time.Now()
			res, err := r.Client.Get(r.URL)
			duration := time.Since(startTime)

			select {
			case <-ctx.Done():
				return
			default:
				if err != nil {
					ch <- runner.Result{Error: err, Time: duration}
				} else {
					res.Body.Close()
					ch <- runner.Result{Time: duration}
				}
			}
		}
	}()

	return ch
}
