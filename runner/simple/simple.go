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
			req, err := http.NewRequest(http.MethodGet, r.URL, nil)
			if err != nil {
				ch <- runner.Result{Error: err}
				continue
			}
			req.Close = true

			cancel := make(chan struct{})
			req.Cancel = cancel

			go func() {
				startTime := time.Now()
				res, err := r.Client.Do(req)
				duration := time.Since(startTime)
				if err != nil {
					ch <- runner.Result{Error: err}
				}
				res.Body.Close()
				ch <- runner.Result{Time: duration}
			}()

			_, keepGoing := <-ctx.Done()
			if !keepGoing {
				close(cancel)
				return
			}
		}
	}()

	return ch
}
