package jwk

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lestrrat-go/httprc"
	"github.com/lestrrat-go/iter/arrayiter"
	"github.com/lestrrat-go/iter/mapiter"
)

type Transformer = httprc.Transformer
type HTTPClient = httprc.HTTPClient
type ErrSink = httprc.ErrSink

// Whitelist describes a set of rules that allows users to access
// a particular URL. By default all URLs are blocked for security
// reasons. You will HAVE to provide some sort of whitelist. See
// the documentation for github.com/lestrrat-go/httprc for more details.
type Whitelist = httprc.Whitelist

// Cache is a container that keeps track of Set object by their source URLs.
// The Set objects are stored in memory, and are refreshed automatically
// behind the scenes.
//
// Before retrieving the Set objects, the user must pre-register the
// URLs they intend to use by calling `Register()`
//
//	c := jwk.NewCache(ctx)
//	c.Register(url, options...)
//
// Once registered, you can call `Get()` to retrieve the Set object.
//
// All JWKS objects that are retrieved via this mechanism should be
// treated read-only, as they are shared among all consumers, as well
// as the `jwk.Cache` object.
//
// There are cases where `jwk.Cache` and `jwk.CachedSet` should and
// should not be used.
//
// First and foremost, do NOT use a cache for those JWKS objects that
// need constant checking. For example, unreliable or user-provided JWKS (i.e. those
// JWKS that are not from a well-known provider) should not be fetched
// through a `jwk.Cache` or `jwk.CachedSet`.
//
// For example, if you have a flaky JWKS server for development
// that can go down often, you should consider alternatives such as
// providing `http.Client` with a caching `http.RoundTripper` configured
// (see `jwk.WithHTTPClient`), setting up a reverse proxy, etc.
// These techniques allow you to setup a more robust way to both cache
// and report precise causes of the problems than using `jwk.Cache` or
// `jwk.CachedSet`. If you handle the caching at the HTTP level like this,
// you will be able to use a simple `jwk.Fetch` call and not worry about the cache.
//
// User-provided JWKS objects may also be problematic, as it may go down
// unexpectedly (and frequently!), and it will be hard to detect when
// the URLs or its contents are swapped.
//
// A good use-case for `jwk.Cache` and `jwk.CachedSet` are for "stable"
// JWKS objects.
//
// When we say "stable", we are thinking of JWKS that should mostly be
// ALWAYS available. A good example are those JWKS objects provided by
// major cloud providers such as Google Cloud, AWS, or Azure.
// Stable JWKS may still experience intermittent network connectivity problems,
// but you can expect that they will eventually recover in relatively
// short period of time. They rarely change URLs, and the contents are
// expected to be valid or otherwise it would cause havoc to those providers
//
// We also know that these stable JWKS objects are rotated periodically,
// which is a perfect use for `jwk.Cache` and `jwk.CachedSet`. The caches
// can be configured to perodically refresh the JWKS thereby keeping them
// fresh without extra intervention from the developer.
//
// Notice that for these recommended use-cases the requirement to check
// the validity or the availability of the JWKS objects are non-existent,
// as it is expected that they will be available and will be valid. The
// caching mechanism can hide intermittent connectivity problems as well
// as keep the objects mostly fresh.
type Cache struct {
	cache *httprc.Cache
}

// PostFetcher is an interface for objects that want to perform
// operations on the `Set` that was fetched.
type PostFetcher interface {
	// PostFetch revceives the URL and the JWKS, after a successful
	// fetch and parse.
	//
	// It should return a `Set`, optionally modified, to be stored
	// in the cache for subsequent use
	PostFetch(string, Set) (Set, error)
}

// PostFetchFunc is a PostFetcher based on a functon.
type PostFetchFunc func(string, Set) (Set, error)

func (f PostFetchFunc) PostFetch(u string, set Set) (Set, error) {
	return f(u, set)
}

// httprc.Transofmer that transforms the response into a JWKS
type jwksTransform struct {
	postFetch    PostFetcher
	parseOptions []ParseOption
}

// Default transform has no postFetch. This can be shared
// by multiple fetchers
var defaultTransform = &jwksTransform{}

func (t *jwksTransform) Transform(u string, res *http.Response) (interface{}, error) {
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(`failed to process response: non-200 response code %q`, res.Status)
	}
	buf, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf(`failed to read response body status: %w`, err)
	}

	set, err := Parse(buf, t.parseOptions...)
	if err != nil {
		return nil, fmt.Errorf(`failed to parse JWK set at %q: %w`, u, err)
	}

	if pf := t.postFetch; pf != nil {
		v, err := pf.PostFetch(u, set)
		if err != nil {
			return nil, fmt.Errorf(`failed to execute PostFetch: %w`, err)
		}
		set = v
	}

	return set, nil
}

// NewCache creates a new `jwk.Cache` object.
//
// Please refer to the documentation for `httprc.New` for more
// details.
func NewCache(ctx context.Context, options ...CacheOption) *Cache {
	var hrcopts []httprc.CacheOption
	for _, option := range options {
		//nolint:forcetypeassert
		switch option.Ident() {
		case identRefreshWindow{}:
			hrcopts = append(hrcopts, httprc.WithRefreshWindow(option.Value().(time.Duration)))
		case identErrSink{}:
			hrcopts = append(hrcopts, httprc.WithErrSink(option.Value().(ErrSink)))
		}
	}

	return &Cache{
		cache: httprc.NewCache(ctx, hrcopts...),
	}
}

// Register registers a URL to be managed by the cache. URLs must
// be registered before issuing `Get`
//
// This method is almost identical to `(httprc.Cache).Register`, except
// it accepts some extra options.
//
// Use `jwk.WithParser` to configure how the JWKS should be parsed,
// such as passing it extra options.
//
// Please refer to the documentation for `(httprc.Cache).Register` for more
// details.
//
// Register does not check for the validity of the url being registered.
// If you need to make sure that a url is valid before entering your main
// loop, call `Refresh` once to make sure the JWKS is available.
//
//	_ = cache.Register(url)
//	if _, err := cache.Refresh(ctx, url); err != nil {
//	  // url is not a valid JWKS
//	  panic(err)
//	}
func (c *Cache) Register(u string, options ...RegisterOption) error {
	var hrropts []httprc.RegisterOption
	var pf PostFetcher
	var parseOptions []ParseOption

	// Note: we do NOT accept Transform option
	for _, option := range options {
		if parseOpt, ok := option.(ParseOption); ok {
			parseOptions = append(parseOptions, parseOpt)
			continue
		}

		//nolint:forcetypeassert
		switch option.Ident() {
		case identHTTPClient{}:
			hrropts = append(hrropts, httprc.WithHTTPClient(option.Value().(HTTPClient)))
		case identRefreshInterval{}:
			hrropts = append(hrropts, httprc.WithRefreshInterval(option.Value().(time.Duration)))
		case identMinRefreshInterval{}:
			hrropts = append(hrropts, httprc.WithMinRefreshInterval(option.Value().(time.Duration)))
		case identFetchWhitelist{}:
			hrropts = append(hrropts, httprc.WithWhitelist(option.Value().(httprc.Whitelist)))
		case identPostFetcher{}:
			pf = option.Value().(PostFetcher)
		}
	}

	var t *jwksTransform
	if pf == nil && len(parseOptions) == 0 {
		t = defaultTransform
	} else {
		// User-supplied PostFetcher is attached to the transformer
		t = &jwksTransform{
			postFetch:    pf,
			parseOptions: parseOptions,
		}
	}

	// Set the transfomer at the end so that nobody can override it
	hrropts = append(hrropts, httprc.WithTransformer(t))
	return c.cache.Register(u, hrropts...)
}

// Get returns the stored JWK set (`Set`) from the cache.
//
// Please refer to the documentation for `(httprc.Cache).Get` for more
// details.
func (c *Cache) Get(ctx context.Context, u string) (Set, error) {
	v, err := c.cache.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	set, ok := v.(Set)
	if !ok {
		return nil, fmt.Errorf(`cached object is not a Set (was %T)`, v)
	}
	return set, nil
}

// Refresh is identical to Get(), except it always fetches the
// specified resource anew, and updates the cached content
//
// Please refer to the documentation for `(httprc.Cache).Refresh` for
// more details
func (c *Cache) Refresh(ctx context.Context, u string) (Set, error) {
	v, err := c.cache.Refresh(ctx, u)
	if err != nil {
		return nil, err
	}

	set, ok := v.(Set)
	if !ok {
		return nil, fmt.Errorf(`cached object is not a Set (was %T)`, v)
	}
	return set, nil
}

// IsRegistered returns true if the given URL `u` has already been registered
// in the cache.
//
// Please refer to the documentation for `(httprc.Cache).IsRegistered` for more
// details.
func (c *Cache) IsRegistered(u string) bool {
	return c.cache.IsRegistered(u)
}

// Unregister removes the given URL `u` from the cache.
//
// Please refer to the documentation for `(httprc.Cache).Unregister` for more
// details.
func (c *Cache) Unregister(u string) error {
	return c.cache.Unregister(u)
}

func (c *Cache) Snapshot() *httprc.Snapshot {
	return c.cache.Snapshot()
}

// CachedSet is a thin shim over jwk.Cache that allows the user to cloack
// jwk.Cache as if it's a `jwk.Set`. Behind the scenes, the `jwk.Set` is
// retrieved from the `jwk.Cache` for every operation.
//
// Since `jwk.CachedSet` always deals with a cached version of the `jwk.Set`,
// all operations that mutate the object (such as AddKey(), RemoveKey(), et. al)
// are no-ops and return an error.
//
// Note that since this is a utility shim over `jwk.Cache`, you _will_ lose
// the ability to control the finer details (such as controlling how long to
// wait for in case of a fetch failure using `context.Context`)
//
// Make sure that you read the documentation for `jwk.Cache` as well.
type CachedSet struct {
	cache *Cache
	url   string
}

var _ Set = &CachedSet{}

func NewCachedSet(cache *Cache, url string) Set {
	return &CachedSet{
		cache: cache,
		url:   url,
	}
}

func (cs *CachedSet) cached() (Set, error) {
	return cs.cache.Get(context.Background(), cs.url)
}

// Add is a no-op for `jwk.CachedSet`, as the `jwk.Set` should be treated read-only
func (*CachedSet) AddKey(_ Key) error {
	return fmt.Errorf(`(jwk.Cachedset).AddKey: jwk.CachedSet is immutable`)
}

// Clear is a no-op for `jwk.CachedSet`, as the `jwk.Set` should be treated read-only
func (*CachedSet) Clear() error {
	return fmt.Errorf(`(jwk.CachedSet).Clear: jwk.CachedSet is immutable`)
}

// Set is a no-op for `jwk.CachedSet`, as the `jwk.Set` should be treated read-only
func (*CachedSet) Set(_ string, _ interface{}) error {
	return fmt.Errorf(`(jwk.CachedSet).Set: jwk.CachedSet is immutable`)
}

// Remove is a no-op for `jwk.CachedSet`, as the `jwk.Set` should be treated read-only
func (*CachedSet) Remove(_ string) error {
	// TODO: Remove() should be renamed to Remove(string) error
	return fmt.Errorf(`(jwk.CachedSet).Remove: jwk.CachedSet is immutable`)
}

// RemoveKey is a no-op for `jwk.CachedSet`, as the `jwk.Set` should be treated read-only
func (*CachedSet) RemoveKey(_ Key) error {
	return fmt.Errorf(`(jwk.CachedSet).RemoveKey: jwk.CachedSet is immutable`)
}

func (cs *CachedSet) Clone() (Set, error) {
	set, err := cs.cached()
	if err != nil {
		return nil, fmt.Errorf(`failed to get cached jwk.Set: %w`, err)
	}

	return set.Clone()
}

// Get returns the value of non-Key field stored in the jwk.Set
func (cs *CachedSet) Get(name string) (interface{}, bool) {
	set, err := cs.cached()
	if err != nil {
		return nil, false
	}

	return set.Get(name)
}

// Key returns the Key at the specified index
func (cs *CachedSet) Key(idx int) (Key, bool) {
	set, err := cs.cached()
	if err != nil {
		return nil, false
	}

	return set.Key(idx)
}

func (cs *CachedSet) Index(key Key) int {
	set, err := cs.cached()
	if err != nil {
		return -1
	}

	return set.Index(key)
}

func (cs *CachedSet) Keys(ctx context.Context) KeyIterator {
	set, err := cs.cached()
	if err != nil {
		return arrayiter.New(nil)
	}

	return set.Keys(ctx)
}

func (cs *CachedSet) Iterate(ctx context.Context) HeaderIterator {
	set, err := cs.cached()
	if err != nil {
		return mapiter.New(nil)
	}

	return set.Iterate(ctx)
}

func (cs *CachedSet) Len() int {
	set, err := cs.cached()
	if err != nil {
		return -1
	}

	return set.Len()
}

func (cs *CachedSet) LookupKeyID(kid string) (Key, bool) {
	set, err := cs.cached()
	if err != nil {
		return nil, false
	}

	return set.LookupKeyID(kid)
}
