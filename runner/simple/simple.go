package simple

import (
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

func (r *SimpleRunner) Run(ctx context.Context) <-chan time.Duration {
	ch := make(chan time.Duration)

	go func() {
		defer close(ch)
		for {
			startTime := time.Now()
			res, err := r.Client.Get(r.URL)
			duration := time.Since(startTime)
			if err != nil {
				panic(err)
			}
			res.Body.Close()

			select {
			case <-ctx.Done():
				return
			default:
				ch <- duration
			}
		}
	}()

	return ch
}
