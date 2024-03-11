package httprc

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

type fetchRequest struct {
	mu sync.RWMutex

	// client contains the HTTP Client that can be used to make a
	// request. By setting a custom *http.Client, you can for example
	// provide a custom http.Transport
	//
	// If not specified, http.DefaultClient will be used.
	client HTTPClient

	wl Whitelist

	// u contains the URL to be fetched
	url string

	// reply is a field that is only used by the internals of the fetcher
	// it is used to return the result of fetching
	reply chan *fetchResult
}

type fetchResult struct {
	mu  sync.RWMutex
	res *http.Response
	err error
}

func (fr *fetchResult) reply(ctx context.Context, reply chan *fetchResult) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case reply <- fr:
	}

	close(reply)
	return nil
}

type fetcher struct {
	requests chan *fetchRequest
}

type Fetcher interface {
	Fetch(context.Context, string, ...FetchOption) (*http.Response, error)
	fetch(context.Context, *fetchRequest) (*http.Response, error)
}

func NewFetcher(ctx context.Context, options ...FetcherOption) Fetcher {
	var nworkers int
	var wl Whitelist
	for _, option := range options {
		//nolint:forcetypeassert
		switch option.Ident() {
		case identFetcherWorkerCount{}:
			nworkers = option.Value().(int)
		case identWhitelist{}:
			wl = option.Value().(Whitelist)
		}
	}

	if nworkers < 1 {
		nworkers = 3
	}

	incoming := make(chan *fetchRequest)
	for i := 0; i < nworkers; i++ {
		go runFetchWorker(ctx, incoming, wl)
	}
	return &fetcher{
		requests: incoming,
	}
}

func (f *fetcher) Fetch(ctx context.Context, u string, options ...FetchOption) (*http.Response, error) {
	var client HTTPClient
	var wl Whitelist
	for _, option := range options {
		//nolint:forcetypeassert
		switch option.Ident() {
		case identHTTPClient{}:
			client = option.Value().(HTTPClient)
		case identWhitelist{}:
			wl = option.Value().(Whitelist)
		}
	}

	req := fetchRequest{
		client: client,
		url:    u,
		wl:     wl,
	}

	return f.fetch(ctx, &req)
}

// fetch (unexported) is the main fetching implemntation.
// it allows the caller to reuse the same *fetchRequest object
func (f *fetcher) fetch(ctx context.Context, req *fetchRequest) (*http.Response, error) {
	reply := make(chan *fetchResult, 1)
	req.mu.Lock()
	req.reply = reply
	req.mu.Unlock()

	// Send a request to the backend
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case f.requests <- req:
	}

	// wait until we get a reply
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case fr := <-reply:
		fr.mu.RLock()
		res := fr.res
		err := fr.err
		fr.mu.RUnlock()
		return res, err
	}
}

func runFetchWorker(ctx context.Context, incoming chan *fetchRequest, wl Whitelist) {
LOOP:
	for {
		select {
		case <-ctx.Done():
			break LOOP
		case req := <-incoming:
			req.mu.RLock()
			reply := req.reply
			client := req.client
			if client == nil {
				client = http.DefaultClient
			}
			url := req.url
			reqwl := req.wl
			req.mu.RUnlock()

			var wls []Whitelist
			for _, v := range []Whitelist{wl, reqwl} {
				if v != nil {
					wls = append(wls, v)
				}
			}

			if len(wls) > 0 {
				for _, wl := range wls {
					if !wl.IsAllowed(url) {
						r := &fetchResult{
							err: fmt.Errorf(`fetching url %q rejected by whitelist`, url),
						}
						if err := r.reply(ctx, reply); err != nil {
							break LOOP
						}
						continue LOOP
					}
				}
			}

			// The body is handled by the consumer of the fetcher
			//nolint:bodyclose
			res, err := client.Get(url)
			r := &fetchResult{
				res: res,
				err: err,
			}
			if err := r.reply(ctx, reply); err != nil {
				break LOOP
			}
		}
	}
}
