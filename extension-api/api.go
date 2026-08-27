// Package extensionapi defines the stable API for k6 JavaScript extensions.
//
// It intentionally depends only on the Go standard library and Sobek. Host
// integrations provide the implementation of VU; extensions must not depend
// on k6 packages to use this API.
package extensionapi

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/grafana/sobek"
)

const importPrefix = "k6/x/"

// Module creates one module Instance for each JavaScript runtime that imports
// it.
type Module interface {
	NewModuleInstance(VU) Instance
}

// Instance provides the JavaScript exports for one Module in one JavaScript
// runtime.
type Instance interface {
	Exports() Exports
}

// VU exposes the capabilities available to every v1 extension module
// instance. Further host capabilities will be introduced as separate,
// optional interfaces instead of expanding this base interface.
type VU interface {
	Context() context.Context
	Runtime() *sobek.Runtime
}

// Environment is an optional VU capability for looking up environment values
// supplied by the host. Extensions obtain it with a type assertion from VU so
// hosts can provide the base API without exposing an environment.
type Environment interface {
	LookupEnv(key string) (value string, ok bool)
}

// ErrHTTPUnavailable is returned when the host cannot execute an HTTP request
// in the current context.
var ErrHTTPUnavailable = errors.New("extension API HTTP capability is unavailable")

// HTTPOptions selects host-owned HTTP behavior for an extension request.
type HTTPOptions struct {
	Jar            http.CookieJar
	Tags           Tags
	ForceHTTP1     bool
	DeferMetrics   bool
	ExpectedStatus func(int) bool
}

// HTTPResponse wraps the standard-library response returned by the host.
type HTTPResponse struct{ *http.Response }

// HTTP is an optional VU capability for executing requests through the host's
// HTTP stack, including its cookies, transport policies, and metrics.
type HTTP interface {
	Do(context.Context, *http.Request, HTTPOptions) (*HTTPResponse, error)
}

// ExecutionPhase identifies whether a module instance is running in init or
// VU execution context.
type ExecutionPhase uint8

const (
	ExecutionPhaseInit ExecutionPhase = iota
	ExecutionPhaseVU
)

// Execution is an optional VU capability exposing the current execution phase.
type Execution interface{ ExecutionPhase() ExecutionPhase }

// VUIdentity is an optional VU capability exposing a stable numeric VU ID.
type VUIdentity interface{ VUID() uint64 }

// Logger is an optional VU capability for structured extension logging. The
// logger uses the Go standard library's log/slog API; extensions must not
// depend on a host logging implementation.
type Logger interface {
	Logger() *slog.Logger
}

// ErrMetricsUnavailable is returned when a host cannot register or emit
// metrics in the current context.
var ErrMetricsUnavailable = errors.New("extension API metrics capability is unavailable")

// MetricKind determines how a host aggregates metric samples.
type MetricKind uint8

const (
	MetricCounter MetricKind = iota
	MetricGauge
	MetricTrend
	MetricRate
)

// MetricUnit describes the values emitted for a metric.
type MetricUnit uint8

const (
	MetricUnitDefault MetricUnit = iota
	MetricUnitTime
	MetricUnitData
)

// MetricSpec declares a custom metric. Registration is idempotent for an
// identical specification and returns an error for incompatible redeclarations.
type MetricSpec struct {
	Name string
	Kind MetricKind
	Unit MetricUnit
}

// Metric is a name-based handle returned by Metrics. Hosts verify that its
// name is registered before accepting an emitted Sample.
type Metric struct{ name string }

// Name returns the metric's stable host-visible name.
func (m Metric) Name() string { return m.name }

// MetricFromName creates a metric handle for host adapters and test doubles.
// Extensions should obtain handles from Metrics.RegisterMetric or BuiltinMetric.
func MetricFromName(name string) Metric { return Metric{name: name} }

// BuiltinMetric identifies a host-provided metric that an extension may emit.
type BuiltinMetric string

const (
	BuiltinDataSent     BuiltinMetric = "data_sent"
	BuiltinDataReceived BuiltinMetric = "data_received"
)

// SystemTag identifies a host-controlled system tag. Hosts decide whether a
// requested system tag is enabled and whether it is indexed or metadata.
type SystemTag string

const (
	SystemTagIP       SystemTag = "ip"
	SystemTagMethod   SystemTag = "method"
	SystemTagProto    SystemTag = "proto"
	SystemTagStatus   SystemTag = "status"
	SystemTagSubproto SystemTag = "subproto"
	SystemTagURL      SystemTag = "url"
)

// Tags is an immutable snapshot of indexed tags and unindexed metadata.
// Its methods always copy supplied and returned maps.
type Tags struct {
	values   map[string]string
	metadata map[string]string
}

// NewTags creates an immutable tag snapshot from values and metadata.
func NewTags(values, metadata map[string]string) Tags {
	return Tags{values: maps.Clone(values), metadata: maps.Clone(metadata)}
}

// Values returns a copy of the snapshot's indexed tags.
func (t Tags) Values() map[string]string { return maps.Clone(t.values) }

// Metadata returns a copy of the snapshot's unindexed metadata.
func (t Tags) Metadata() map[string]string { return maps.Clone(t.metadata) }

// With returns a snapshot with the supplied indexed tags merged into it.
func (t Tags) With(values map[string]string) Tags {
	merged := t.Values()
	if merged == nil {
		merged = make(map[string]string, len(values))
	}
	for key, value := range values {
		merged[key] = value
	}
	return NewTags(merged, t.metadata)
}

// WithMetadata returns a snapshot with the supplied metadata merged into it.
func (t Tags) WithMetadata(metadata map[string]string) Tags {
	merged := t.Metadata()
	if merged == nil {
		merged = make(map[string]string, len(metadata))
	}
	for key, value := range metadata {
		merged[key] = value
	}
	return NewTags(t.values, merged)
}

// Sample is one metric measurement. A zero Time asks the host to use its
// current time. Tags should normally come from Metrics.CurrentTags().
type Sample struct {
	Metric Metric
	Value  float64
	Time   time.Time
	Tags   Tags
}

// Metrics is an optional VU capability for custom metrics, built-in byte
// metrics, immutable tag snapshots, and cancellation-aware sample emission.
// Emit returns ctx.Err() if the context is cancelled before the host accepts
// the samples.
type Metrics interface {
	RegisterMetric(MetricSpec) (Metric, error)
	BuiltinMetric(BuiltinMetric) (Metric, bool)
	CurrentTags() Tags
	WithSystemTags(Tags, map[SystemTag]string) Tags
	Emit(context.Context, []Sample) error
}

// ErrNetworkUnavailable is returned when a host does not provide network
// access in the current VU context, such as k6's init context.
var ErrNetworkUnavailable = errors.New("extension API network capability is unavailable")

// Network is an optional VU capability for host-policy-aware network access.
// Hosts must apply their DNS, hostname, and address policies to both methods.
// Extensions obtain it with a type assertion from VU.
type Network interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// ErrNetworkPolicyUnavailable is returned when a host cannot check a logical
// hostname against its network policy in the current VU context.
var ErrNetworkPolicyUnavailable = errors.New("extension API network policy capability is unavailable")

// NetworkPolicy is an optional VU capability for checking a logical hostname
// against the host's network policy without opening a connection or resolving
// the name. Extensions retain ownership of protocol-specific operations, such
// as DNS queries or packet sockets, and may call CheckHost before those
// operations. Extensions obtain it with a type assertion from VU.
type NetworkPolicy interface {
	CheckHost(ctx context.Context, host string) error
}

// ErrTLSUnavailable is returned when a host cannot apply its TLS policy in
// the current context, such as k6's init context.
var ErrTLSUnavailable = errors.New("extension API TLS capability is unavailable")

// TLS is an optional VU capability for client TLS handshakes. The host owns
// and applies its TLS policy. Extensions may supply a configuration with
// connection-specific ServerName or NextProtos values and extension-specific
// roots or client certificates; hosts must not expose their owned TLS config.
//
// TLSClient takes ownership of conn. It closes conn when the handshake fails.
// Extensions obtain it with a type assertion from VU.
type TLS interface {
	TLSClient(ctx context.Context, conn net.Conn, config *tls.Config) (net.Conn, error)
}

// Task is JavaScript-runtime work that a Scheduler executes on the owning
// runtime goroutine. A Task must not be run directly by an extension.
type Task func() error

// Scheduler is an optional VU capability for safely resuming JavaScript work
// after asynchronous Go work completes.
//
// RegisterCallback must be called on the JavaScript runtime goroutine. Its
// returned callback is safe to call from another goroutine, but must be called
// exactly once. This reservation keeps the host event loop alive until the
// supplied Task runs.
type Scheduler interface {
	RegisterCallback() func(Task)
}

// PromiseResolver settles a JavaScript Promise. Resolve and Reject are safe to
// call from another goroutine; the host schedules the settlement on the owning
// JavaScript runtime. Only the first call has an effect.
type PromiseResolver interface {
	Resolve(value any)
	Reject(reason any)
}

// Promises is an optional VU capability for creating Promises that are safe to
// settle from asynchronous Go code. NewPromise must be called on the owning
// JavaScript runtime goroutine.
type Promises interface {
	NewPromise() (*sobek.Promise, PromiseResolver)
}

// Exports represents the ESM exports of an Instance.
type Exports struct {
	Default any
	Named   map[string]any
}

var (
	registryMu sync.RWMutex
	registry   = make(map[string]any)
)

// Register makes mod available for import under name. Names must begin with
// "k6/x/" and may be registered only once. mod may be a Module or a raw Go
// value; a host chooses how to expose raw values to JavaScript.
func Register(name string, mod any) {
	if !strings.HasPrefix(name, importPrefix) {
		panic(fmt.Errorf("extension module names must be prefixed with %q, got %q", importPrefix, name))
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := registry[name]; exists {
		panic(fmt.Errorf("extension module already registered: %s", name))
	}
	registry[name] = mod
}

// Registered returns a snapshot of registered extension modules, keyed by
// their JavaScript import specifier.
func Registered() map[string]any {
	registryMu.RLock()
	defer registryMu.RUnlock()

	return maps.Clone(registry)
}
