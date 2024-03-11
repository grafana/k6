package httprc

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ErrSink is an abstraction that allows users to consume errors
// produced while the cache queue is running.
type HTTPClient interface {
	Get(string) (*http.Response, error)
}

// Cache represents a cache that stores resources locally, while
// periodically refreshing the contents based on HTTP header values
// and/or user-supplied hints.
//
// Refresh is performed _periodically_, and therefore the contents
// are not kept up-to-date in real time. The interval between checks
// for refreshes is called the refresh window.
//
// The default refresh window is 15 minutes. This means that if a
// resource is fetched is at time T, and it is supposed to be
// refreshed in 20 minutes, the next refresh for this resource will
// happen at T+30 minutes (15+15 minutes).
type Cache struct {
	mu    sync.RWMutex
	queue *queue
	wl    Whitelist
}

const defaultRefreshWindow = 15 * time.Minute

// New creates a new Cache object.
//
// The context object in the argument controls the life-cycle of the
// auto-refresh worker. If you cancel the `ctx`, then the automatic
// refresh will stop working.
//
// Refresh will only be performed periodically where the interval between
// refreshes are controlled by the `refresh window` variable. For example,
// if the refresh window is every 5 minutes and the resource was queued
// to be refreshed at 7 minutes, the resource will be refreshed after 10
// minutes (in 2 refresh window time).
//
// The refresh window can be configured by using `httprc.WithRefreshWindow`
// option. If you want refreshes to be performed more often, provide a smaller
// refresh window. If you specify a refresh window that is smaller than 1
// second, it will automatically be set to the default value, which is 15
// minutes.
//
// Internally the HTTP fetching is done using a pool of HTTP fetch
// workers. The default number of workers is 3. You may change this
// number by specifying the `httprc.WithFetcherWorkerCount`
func NewCache(ctx context.Context, options ...CacheOption) *Cache {
	var refreshWindow time.Duration
	var errSink ErrSink
	var wl Whitelist
	var fetcherOptions []FetcherOption
	for _, option := range options {
		//nolint:forcetypeassert
		switch option.Ident() {
		case identRefreshWindow{}:
			refreshWindow = option.Value().(time.Duration)
		case identFetcherWorkerCount{}, identWhitelist{}:
			fetcherOptions = append(fetcherOptions, option)
		case identErrSink{}:
			errSink = option.Value().(ErrSink)
		}
	}

	if refreshWindow < time.Second {
		refreshWindow = defaultRefreshWindow
	}

	fetch := NewFetcher(ctx, fetcherOptions...)
	queue := newQueue(ctx, refreshWindow, fetch, errSink)

	return &Cache{
		queue: queue,
		wl:    wl,
	}
}

// Register configures a URL to be stored in the cache.
//
// For any given URL, the URL must be registered _BEFORE_ it is
// accessed using `Get()` method.
func (c *Cache) Register(u string, options ...RegisterOption) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if wl := c.wl; wl != nil {
		if !wl.IsAllowed(u) {
			return fmt.Errorf(`httprc.Cache: url %q has been rejected by whitelist`, u)
		}
	}

	return c.queue.Register(u, options...)
}

// Unregister removes the given URL `u` from the cache.
//
// Subsequent calls to `Get()` will fail until `u` is registered again.
func (c *Cache) Unregister(u string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.queue.Unregister(u)
}

// IsRegistered returns true if the given URL `u` has already been
// registered in the cache.
func (c *Cache) IsRegistered(u string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.queue.IsRegistered(u)
}

// Refresh is identical to Get(), except it always fetches the
// specified resource anew, and updates the cached content
func (c *Cache) Refresh(ctx context.Context, u string) (interface{}, error) {
	return c.getOrFetch(ctx, u, true)
}

// Get returns the cached object.
//
// The context.Context argument is used to control the timeout for
// synchronous fetches, when they need to happen. Synchronous fetches
// will be performed when the cache does not contain the specified
// resource.
func (c *Cache) Get(ctx context.Context, u string) (interface{}, error) {
	return c.getOrFetch(ctx, u, false)
}

func (c *Cache) getOrFetch(ctx context.Context, u string, forceRefresh bool) (interface{}, error) {
	c.mu.RLock()
	e, ok := c.queue.getRegistered(u)
	if !ok {
		c.mu.RUnlock()
		return nil, fmt.Errorf(`url %q is not registered (did you make sure to call Register() first?)`, u)
	}
	c.mu.RUnlock()

	// Only one goroutine may enter this section.
	e.acquireSem()

	// has this entry been fetched? (but ignore and do a fetch
	// if forceRefresh is true)
	if forceRefresh || !e.hasBeenFetched() {
		if err := c.queue.fetchAndStore(ctx, e); err != nil {
			e.releaseSem()
			return nil, fmt.Errorf(`failed to fetch %q: %w`, u, err)
		}
	}

	e.releaseSem()

	e.mu.RLock()
	data := e.data
	e.mu.RUnlock()

	return data, nil
}

func (c *Cache) Snapshot() *Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.queue.snapshot()
}
