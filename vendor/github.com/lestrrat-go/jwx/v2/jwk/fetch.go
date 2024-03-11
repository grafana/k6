package jwk

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/lestrrat-go/httprc"
)

type Fetcher interface {
	Fetch(context.Context, string, ...FetchOption) (Set, error)
}

type FetchFunc func(context.Context, string, ...FetchOption) (Set, error)

func (f FetchFunc) Fetch(ctx context.Context, u string, options ...FetchOption) (Set, error) {
	return f(ctx, u, options...)
}

var globalFetcher httprc.Fetcher
var muGlobalFetcher sync.Mutex
var fetcherChanged uint32

func init() {
	atomic.StoreUint32(&fetcherChanged, 1)
}

func getGlobalFetcher() httprc.Fetcher {
	if v := atomic.LoadUint32(&fetcherChanged); v == 0 {
		return globalFetcher
	}

	muGlobalFetcher.Lock()
	defer muGlobalFetcher.Unlock()
	if globalFetcher == nil {
		var nworkers int
		v := os.Getenv(`JWK_FETCHER_WORKER_COUNT`)
		if c, err := strconv.ParseInt(v, 10, 64); err == nil {
			if c > math.MaxInt {
				nworkers = math.MaxInt
			} else {
				nworkers = int(c)
			}
		}
		if nworkers < 1 {
			nworkers = 3
		}

		globalFetcher = httprc.NewFetcher(context.Background(), httprc.WithFetcherWorkerCount(nworkers))
	}

	atomic.StoreUint32(&fetcherChanged, 0)
	return globalFetcher
}

// SetGlobalFetcher allows users to specify a custom global fetcher,
// which is used by the `Fetch` function. Assigning `nil` forces
// the default fetcher to be (re)created when the next call to
// `jwk.Fetch` occurs
//
// You only need to call this function when you want to
// either change the fetching behavior (for example, you want to change
// how the default whitelist is handled), or when you want to control
// the lifetime of the global fetcher, for example for tests
// that require a clean shutdown.
//
// If you do use this function to set a custom fetcher and you
// control its termination, make sure that you call `jwk.SetGlobalFetcher()`
// one more time (possibly with `nil`) to assign a valid fetcher.
// Otherwise, once the fetcher is invalidated, subsequent calls to `jwk.Fetch`
// may hang, causing very hard to debug problems.
//
// If you are sure you no longer need `jwk.Fetch` after terminating the
// fetcher, then you the above caution is not necessary.
func SetGlobalFetcher(f httprc.Fetcher) {
	muGlobalFetcher.Lock()
	globalFetcher = f
	muGlobalFetcher.Unlock()
	atomic.StoreUint32(&fetcherChanged, 1)
}

// Fetch fetches a JWK resource specified by a URL. The url must be
// pointing to a resource that is supported by `net/http`.
//
// If you are using the same `jwk.Set` for long periods of time during
// the lifecycle of your program, and would like to periodically refresh the
// contents of the object with the data at the remote resource,
// consider using `jwk.Cache`, which automatically refreshes
// jwk.Set objects asynchronously.
//
// Please note that underneath the `jwk.Fetch` function, it uses a global
// object that spawns goroutines that are present until the go runtime
// exits. Initially this global variable is uninitialized, but upon
// calling `jwk.Fetch` once, it is initialized and goroutines are spawned.
// If you want to control the lifetime of these goroutines, you can
// call `jwk.SetGlobalFetcher` with a custom fetcher which is tied to
// a `context.Context` object that you can control.
func Fetch(ctx context.Context, u string, options ...FetchOption) (Set, error) {
	var hrfopts []httprc.FetchOption
	var parseOptions []ParseOption
	for _, option := range options {
		if parseOpt, ok := option.(ParseOption); ok {
			parseOptions = append(parseOptions, parseOpt)
			continue
		}

		//nolint:forcetypeassert
		switch option.Ident() {
		case identHTTPClient{}:
			hrfopts = append(hrfopts, httprc.WithHTTPClient(option.Value().(HTTPClient)))
		case identFetchWhitelist{}:
			hrfopts = append(hrfopts, httprc.WithWhitelist(option.Value().(httprc.Whitelist)))
		}
	}

	res, err := getGlobalFetcher().Fetch(ctx, u, hrfopts...)
	if err != nil {
		return nil, fmt.Errorf(`failed to fetch %q: %w`, u, err)
	}

	buf, err := io.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf(`failed to read response body for %q: %w`, u, err)
	}

	return Parse(buf, parseOptions...)
}
