package lib

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/cookiejar"
	"sync"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"

	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/metrics"
)

// DialContexter is an interface that can dial with a context
type DialContexter interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// TracerProvider provides methods for OTEL tracers initialization.
type TracerProvider interface {
	Tracer(name string, options ...trace.TracerOption) trace.Tracer
}

// State provides the volatile state for a VU.
//
// TODO: rename to VUState or, better yet, move to some other Go package outside
// of lib/, where it's more obvious that this is the VU state.
type State struct {
	// Global options and built-in metrics.
	//
	// TODO: remove them from here, the built-in metrics and the script options
	// are not part of a VU's unique "state", they are global and the same for
	// all VUs. Figure out how to thread them some other way, e.g. through the
	// TestPreInitState. The Samples channel might also benefit from that...
	Options        Options
	BuiltinMetrics *metrics.BuiltinMetrics

	// Logger instance for every VU.
	Logger logrus.FieldLogger

	// Networking equipment.
	Dialer DialContexter

	// TODO: move a lot of the things below to the k6/http ModuleInstance, see
	// https://github.com/grafana/k6/issues/2293.
	Transport http.RoundTripper
	CookieJar *cookiejar.Jar
	TLSConfig *tls.Config

	// Rate limits.
	RPSLimit *rate.Limiter

	// Sample channel, possibly buffered
	Samples chan<- metrics.SampleContainer

	// Buffer pool; use instead of allocating fresh buffers when possible.
	BufferPool *BufferPool

	VUID, VUIDGlobal uint64
	Iteration        int64

	// TODO: rename this field with one more representative
	// because it includes now also the metadata.
	Tags *VUStateTags

	// These will be assigned on VU activation.
	// Returns the iteration number of this VU in the current scenario.
	GetScenarioVUIter func() uint64
	// Returns the iteration number across all VUs in the current scenario
	// unique to this single k6 instance.
	// TODO: Maybe this doesn't belong here but in ScenarioState?
	GetScenarioLocalVUIter func() uint64
	// Returns the iteration number across all VUs in the current scenario
	// unique globally across k6 instances (taking into account execution
	// segments).
	GetScenarioGlobalVUIter func() uint64

	// Tracing instrumentation.
	TracerProvider TracerProvider

	// Usage is a way to report usage statistics
	Usage *usage.Usage
}

// VUStateTags wraps the current VU's tags and ensures a thread-safe way to
// access and modify them exists. This is necessary because the VU tags and
// metadata can be modified from the JS scripts via the `vu.tags` API in the
// `k6/execution` built-in module.
type VUStateTags struct {
	mutex       sync.RWMutex
	tagsAndMeta *metrics.TagsAndMeta
}

// NewVUStateTags initializes a new VUStateTags and returns it. It's important
// that tags is not nil and initialized via metrics.Registry.RootTagSet().
func NewVUStateTags(tags *metrics.TagSet) *VUStateTags {
	if tags == nil {
		panic("the metrics.TagSet must be initialized for creating a new lib.VUStateTags")
	}
	return &VUStateTags{
		mutex: sync.RWMutex{},
		tagsAndMeta: &metrics.TagsAndMeta{
			Tags: tags,
			// metadata is intentionally nil by default
		},
	}
}

// GetCurrentValues returns the current value of the VU tags and a copy of the
// metadata (if any) in a thread-safe way.
func (tg *VUStateTags) GetCurrentValues() metrics.TagsAndMeta {
	tg.mutex.RLock()
	defer tg.mutex.RUnlock()
	return tg.tagsAndMeta.Clone()
}

// Modify allows the thread-safe modification of the current VU tags and metadata.
func (tg *VUStateTags) Modify(callback func(tagsAndMeta *metrics.TagsAndMeta)) {
	tg.mutex.Lock()
	defer tg.mutex.Unlock()
	callback(tg.tagsAndMeta)
}
