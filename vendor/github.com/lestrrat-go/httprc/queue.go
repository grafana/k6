package httprc

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/lestrrat-go/httpcc"
)

// ErrSink is an abstraction that allows users to consume errors
// produced while the cache queue is running.
type ErrSink interface {
	// Error accepts errors produced during the cache queue's execution.
	// The method should never block, otherwise the fetch loop may be
	// paused for a prolonged amount of time.
	Error(error)
}

type ErrSinkFunc func(err error)

func (f ErrSinkFunc) Error(err error) {
	f(err)
}

// Transformer is responsible for converting an HTTP response
// into an appropriate form of your choosing.
type Transformer interface {
	// Transform receives an HTTP response object, and should
	// return an appropriate object that suits your needs.
	//
	// If you happen to use the response body, you are responsible
	// for closing the body
	Transform(string, *http.Response) (interface{}, error)
}

type TransformFunc func(string, *http.Response) (interface{}, error)

func (f TransformFunc) Transform(u string, res *http.Response) (interface{}, error) {
	return f(u, res)
}

// BodyBytes is the default Transformer applied to all resources.
// It takes an *http.Response object and extracts the body
// of the response as `[]byte`
type BodyBytes struct{}

func (BodyBytes) Transform(_ string, res *http.Response) (interface{}, error) {
	buf, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf(`failed to read response body: %w`, err)
	}

	return buf, nil
}

type rqentry struct {
	fireAt time.Time
	url    string
}

// entry represents a resource to be fetched over HTTP,
// long with optional specifications such as the *http.Client
// object to use.
type entry struct {
	mu  sync.RWMutex
	sem chan struct{}

	lastFetch time.Time

	// Interval between refreshes are calculated two ways.
	// 1) You can set an explicit refresh interval by using WithRefreshInterval().
	//    In this mode, it doesn't matter what the HTTP response says in its
	//    Cache-Control or Expires headers
	// 2) You can let us calculate the time-to-refresh based on the key's
	//    Cache-Control or Expires headers.
	//    First, the user provides us the absolute minimum interval before
	//    refreshes. We will never check for refreshes before this specified
	//    amount of time.
	//
	//    Next, max-age directive in the Cache-Control header is consulted.
	//    If `max-age` is not present, we skip the following section, and
	//    proceed to the next option.
	//    If `max-age > user-supplied minimum interval`, then we use the max-age,
	//    otherwise the user-supplied minimum interval is used.
	//
	//    Next, the value specified in Expires header is consulted.
	//    If the header is not present, we skip the following seciont and
	//    proceed to the next option.
	//    We take the time until expiration `expires - time.Now()`, and
	//    if `time-until-expiration > user-supplied minimum interval`, then
	//    we use the expires value, otherwise the user-supplied minimum interval is used.
	//
	//    If all of the above fails, we used the user-supplied minimum interval
	refreshInterval    time.Duration
	minRefreshInterval time.Duration

	request *fetchRequest

	transform Transformer
	data      interface{}
}

func (e *entry) acquireSem() {
	e.sem <- struct{}{}
}

func (e *entry) releaseSem() {
	<-e.sem
}

func (e *entry) hasBeenFetched() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return !e.lastFetch.IsZero()
}

// queue is responsible for updating the contents of the storage
type queue struct {
	mu         sync.RWMutex
	registry   map[string]*entry
	windowSize time.Duration
	fetch      Fetcher
	fetchCond  *sync.Cond
	fetchQueue []*rqentry

	// list is a sorted list of urls to their expected fire time
	// when we get a new tick in the RQ loop, we process everything
	// that can be fired up to the point the tick was called
	list []*rqentry

	// clock is really only used by testing
	clock interface {
		Now() time.Time
	}
}

type clockFunc func() time.Time

func (cf clockFunc) Now() time.Time {
	return cf()
}

func newQueue(ctx context.Context, window time.Duration, fetch Fetcher, errSink ErrSink) *queue {
	fetchLocker := &sync.Mutex{}
	rq := &queue{
		windowSize: window,
		fetch:      fetch,
		fetchCond:  sync.NewCond(fetchLocker),
		registry:   make(map[string]*entry),
		clock:      clockFunc(time.Now),
	}

	go rq.refreshLoop(ctx, errSink)

	return rq
}

func (q *queue) Register(u string, options ...RegisterOption) error {
	var refreshInterval time.Duration
	var client HTTPClient
	var wl Whitelist
	var transform Transformer = BodyBytes{}

	minRefreshInterval := 15 * time.Minute
	for _, option := range options {
		//nolint:forcetypeassert
		switch option.Ident() {
		case identHTTPClient{}:
			client = option.Value().(HTTPClient)
		case identRefreshInterval{}:
			refreshInterval = option.Value().(time.Duration)
		case identMinRefreshInterval{}:
			minRefreshInterval = option.Value().(time.Duration)
		case identTransformer{}:
			transform = option.Value().(Transformer)
		case identWhitelist{}:
			wl = option.Value().(Whitelist)
		}
	}

	q.mu.RLock()
	rWindow := q.windowSize
	q.mu.RUnlock()

	if refreshInterval > 0 && refreshInterval < rWindow {
		return fmt.Errorf(`refresh interval (%s) is smaller than refresh window (%s): this will not as expected`, refreshInterval, rWindow)
	}

	e := entry{
		sem:                make(chan struct{}, 1),
		minRefreshInterval: minRefreshInterval,
		transform:          transform,
		refreshInterval:    refreshInterval,
		request: &fetchRequest{
			client: client,
			url:    u,
			wl:     wl,
		},
	}
	q.mu.Lock()
	q.registry[u] = &e
	q.mu.Unlock()
	return nil
}

func (q *queue) Unregister(u string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	_, ok := q.registry[u]
	if !ok {
		return fmt.Errorf(`url %q has not been registered`, u)
	}
	delete(q.registry, u)
	return nil
}

func (q *queue) getRegistered(u string) (*entry, bool) {
	q.mu.RLock()
	e, ok := q.registry[u]
	q.mu.RUnlock()

	return e, ok
}

func (q *queue) IsRegistered(u string) bool {
	_, ok := q.getRegistered(u)
	return ok
}

func (q *queue) fetchLoop(ctx context.Context, errSink ErrSink) {
	for {
		q.fetchCond.L.Lock()
		for len(q.fetchQueue) <= 0 {
			select {
			case <-ctx.Done():
				return
			default:
				q.fetchCond.Wait()
			}
		}
		list := make([]*rqentry, len(q.fetchQueue))
		copy(list, q.fetchQueue)
		q.fetchQueue = q.fetchQueue[:0]
		q.fetchCond.L.Unlock()

		for _, rq := range list {
			select {
			case <-ctx.Done():
				return
			default:
			}

			e, ok := q.getRegistered(rq.url)
			if !ok {
				continue
			}
			if err := q.fetchAndStore(ctx, e); err != nil {
				if errSink != nil {
					errSink.Error(&RefreshError{
						URL: rq.url,
						Err: err,
					})
				}
			}
		}
	}
}

// This loop is responsible for periodically updating the cached content
func (q *queue) refreshLoop(ctx context.Context, errSink ErrSink) {
	// Tick every q.windowSize duration.
	ticker := time.NewTicker(q.windowSize)

	go q.fetchLoop(ctx, errSink)
	defer q.fetchCond.Signal()

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			t = t.Round(time.Second)
			// To avoid getting stuck here, we just copy the relevant
			// items, and release the lock within this critical section
			var list []*rqentry
			q.mu.Lock()
			var max int
			for i, r := range q.list {
				if r.fireAt.Before(t) || r.fireAt.Equal(t) {
					max = i
					list = append(list, r)
					continue
				}
				break
			}

			if len(list) > 0 {
				q.list = q.list[max+1:]
			}
			q.mu.Unlock() // release lock

			if len(list) > 0 {
				// Now we need to fetch these, but do this elsewhere so
				// that we don't block this main loop
				q.fetchCond.L.Lock()
				q.fetchQueue = append(q.fetchQueue, list...)
				q.fetchCond.L.Unlock()
				q.fetchCond.Signal()
			}
		}
	}
}

func (q *queue) fetchAndStore(ctx context.Context, e *entry) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// synchronously go fetch
	e.lastFetch = time.Now()
	res, err := q.fetch.fetch(ctx, e.request)
	if err != nil {
		// Even if the request failed, we need to queue the next fetch
		q.enqueueNextFetch(nil, e)
		return fmt.Errorf(`failed to fetch %q: %w`, e.request.url, err)
	}

	q.enqueueNextFetch(res, e)

	data, err := e.transform.Transform(e.request.url, res)
	if err != nil {
		return fmt.Errorf(`failed to transform HTTP response for %q: %w`, e.request.url, err)
	}
	e.data = data

	return nil
}

func (q *queue) Enqueue(u string, interval time.Duration) error {
	fireAt := q.clock.Now().Add(interval).Round(time.Second)

	q.mu.Lock()
	defer q.mu.Unlock()

	list := q.list

	ll := len(list)
	if ll == 0 || list[ll-1].fireAt.Before(fireAt) {
		list = append(list, &rqentry{
			fireAt: fireAt,
			url:    u,
		})
	} else {
		for i := 0; i < ll; i++ {
			if i == ll-1 || list[i].fireAt.After(fireAt) {
				// insert here
				list = append(list[:i+1], list[i:]...)
				list[i] = &rqentry{fireAt: fireAt, url: u}
				break
			}
		}
	}

	q.list = list
	return nil
}

func (q *queue) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(`{"list":[`)
	q.mu.RLock()
	for i, e := range q.list {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, `{"fire_at":%q,"url":%q}`, e.fireAt.Format(time.RFC3339), e.url)
	}
	q.mu.RUnlock()
	buf.WriteString(`]}`)
	return buf.Bytes(), nil
}

func (q *queue) enqueueNextFetch(res *http.Response, e *entry) {
	dur := calculateRefreshDuration(res, e)
	// TODO send to error sink
	_ = q.Enqueue(e.request.url, dur)
}

func calculateRefreshDuration(res *http.Response, e *entry) time.Duration {
	if e.refreshInterval > 0 {
		return e.refreshInterval
	}

	if res != nil {
		if v := res.Header.Get(`Cache-Control`); v != "" {
			dir, err := httpcc.ParseResponse(v)
			if err == nil {
				maxAge, ok := dir.MaxAge()
				if ok {
					resDuration := time.Duration(maxAge) * time.Second
					if resDuration > e.minRefreshInterval {
						return resDuration
					}
					return e.minRefreshInterval
				}
				// fallthrough
			}
			// fallthrough
		}

		if v := res.Header.Get(`Expires`); v != "" {
			expires, err := http.ParseTime(v)
			if err == nil {
				resDuration := time.Until(expires)
				if resDuration > e.minRefreshInterval {
					return resDuration
				}
				return e.minRefreshInterval
			}
			// fallthrough
		}
	}

	// Previous fallthroughs are a little redandunt, but hey, it's all good.
	return e.minRefreshInterval
}

type SnapshotEntry struct {
	URL         string      `json:"url"`
	Data        interface{} `json:"data"`
	LastFetched time.Time   `json:"last_fetched"`
}
type Snapshot struct {
	Entries []SnapshotEntry `json:"entries"`
}

// Snapshot returns the contents of the cache at the given moment.
func (q *queue) snapshot() *Snapshot {
	q.mu.RLock()
	list := make([]SnapshotEntry, 0, len(q.registry))

	for url, e := range q.registry {
		list = append(list, SnapshotEntry{
			URL:         url,
			LastFetched: e.lastFetch,
			Data:        e.data,
		})
	}
	q.mu.RUnlock()

	return &Snapshot{
		Entries: list,
	}
}
