// Package extensionapitest provides dependency-free test hosts for extensions
// written against go.k6.io/k6-extension-api.
package extensionapitest

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"strings"
	"sync"
	"unicode"

	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
)

// VU is a configurable in-memory host for extension unit tests. It implements
// all current optional extension API capabilities without importing k6.
type VU struct {
	RuntimeValue *sobek.Runtime
	ContextValue context.Context

	LookupEnvFunc     func(string) (string, bool)
	LoggerValue       *slog.Logger
	DialContextFunc   func(context.Context, string, string) (net.Conn, error)
	LookupHostFunc    func(context.Context, string) ([]string, error)
	TLSClientFunc     func(context.Context, net.Conn, *tls.Config) (net.Conn, error)
	EnabledSystemTag  map[extensionapi.SystemTag]bool
	MetadataSystemTag map[extensionapi.SystemTag]bool

	mu       sync.Mutex
	metrics  map[string]extensionapi.MetricSpec
	builtins map[extensionapi.BuiltinMetric]extensionapi.Metric
	tags     extensionapi.Tags
	samples  []extensionapi.Sample

	callbackTasks  chan extensionapi.Task
	pendingTasks   int
	pendingRejects map[*sobek.Promise]struct{}
}

// Runtime groups a test VU with an event loop suitable for JavaScript module
// tests. It is intentionally smaller than k6's legacy modulestest.Runtime.
type Runtime struct {
	VU        *VU
	EventLoop *EventLoop
}

// EventLoop runs JavaScript and asynchronous extension callbacks on the VU's
// owning runtime goroutine.
type EventLoop struct{ vu *VU }

// NewRuntime returns a standalone runtime test host.
func NewRuntime() *Runtime {
	vu := NewVU()
	return &Runtime{VU: vu, EventLoop: &EventLoop{vu: vu}}
}

// Start runs first and waits for all callbacks reserved during the run.
func (e *EventLoop) Start(first func() error) error { return e.vu.Run(first) }

// NewVU returns a VU with a new Sobek runtime, a background context, and no
// host network or TLS access. Tests opt in to those capabilities with funcs.
func NewVU() *VU {
	runtime := sobek.New()
	runtime.SetFieldNameMapper(fieldNameMapper{})
	vu := &VU{
		RuntimeValue: runtime,
		ContextValue: context.Background(),
		LoggerValue:  slog.New(slog.NewTextHandler(discardWriter{}, nil)),
		metrics:      make(map[string]extensionapi.MetricSpec),
		builtins:     make(map[extensionapi.BuiltinMetric]extensionapi.Metric),
		// A callback may settle a promise synchronously while JavaScript is still
		// running. Keep a small queue so RegisterCallback never deadlocks that
		// call before Run starts draining scheduled work.
		callbackTasks:    make(chan extensionapi.Task, 64),
		EnabledSystemTag: make(map[extensionapi.SystemTag]bool),
		pendingRejects:   make(map[*sobek.Promise]struct{}),
	}
	runtime.SetPromiseRejectionTracker(vu.promiseRejectionTracker)
	return vu
}

// Context implements extensionapi.VU.
func (v *VU) Context() context.Context { return v.ContextValue }

// Runtime implements extensionapi.VU.
func (v *VU) Runtime() *sobek.Runtime { return v.RuntimeValue }

// LookupEnv implements extensionapi.Environment.
func (v *VU) LookupEnv(key string) (string, bool) {
	if v.LookupEnvFunc == nil {
		return "", false
	}
	return v.LookupEnvFunc(key)
}

// Logger implements extensionapi.Logger.
func (v *VU) Logger() *slog.Logger { return v.LoggerValue }

// DialContext implements extensionapi.Network.
func (v *VU) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if v.DialContextFunc == nil {
		return nil, extensionapi.ErrNetworkUnavailable
	}
	return v.DialContextFunc(ctx, network, address)
}

// LookupHost implements extensionapi.Network.
func (v *VU) LookupHost(ctx context.Context, host string) ([]string, error) {
	if v.LookupHostFunc == nil {
		return nil, extensionapi.ErrNetworkUnavailable
	}
	return v.LookupHostFunc(ctx, host)
}

// TLSClient implements extensionapi.TLS.
func (v *VU) TLSClient(ctx context.Context, conn net.Conn, config *tls.Config) (net.Conn, error) {
	if v.TLSClientFunc == nil {
		_ = conn.Close()
		return nil, extensionapi.ErrTLSUnavailable
	}
	return v.TLSClientFunc(ctx, conn, config)
}

// RegisterCallback implements extensionapi.Scheduler. Every returned callback
// reserves one Run iteration and must be called exactly once.
func (v *VU) RegisterCallback() func(extensionapi.Task) {
	v.mu.Lock()
	v.pendingTasks++
	v.mu.Unlock()

	var once sync.Once
	return func(task extensionapi.Task) {
		once.Do(func() { v.callbackTasks <- task })
	}
}

// NewPromise implements extensionapi.Promises.
func (v *VU) NewPromise() (*sobek.Promise, extensionapi.PromiseResolver) {
	promise, resolve, reject := v.RuntimeValue.NewPromise()
	enqueue := v.RegisterCallback()
	return promise, promiseResolver{enqueue: enqueue, resolve: resolve, reject: reject}
}

// Run executes fn and then every callback reserved by RegisterCallback. It
// must be called on the same goroutine that owns Runtime().
func (v *VU) Run(fn func() error) error {
	v.mu.Lock()
	v.pendingRejects = make(map[*sobek.Promise]struct{})
	v.mu.Unlock()

	if err := fn(); err != nil {
		return err
	}

	for {
		v.mu.Lock()
		pending := v.pendingTasks
		v.mu.Unlock()
		if pending == 0 {
			return v.unhandledRejection()
		}

		task := <-v.callbackTasks
		err := task()
		v.mu.Lock()
		v.pendingTasks--
		v.mu.Unlock()
		if err != nil {
			return err
		}
	}
}

func (v *VU) promiseRejectionTracker(promise *sobek.Promise, operation sobek.PromiseRejectionOperation) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if operation == sobek.PromiseRejectionReject {
		v.pendingRejects[promise] = struct{}{}
		return
	}
	delete(v.pendingRejects, promise)
}

func (v *VU) unhandledRejection() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	for promise := range v.pendingRejects {
		return fmt.Errorf("uncaught promise rejection: %s", promise.Result())
	}
	return nil
}

// RegisterBuiltinMetric makes metric available through BuiltinMetric.
func (v *VU) RegisterBuiltinMetric(kind extensionapi.BuiltinMetric, name string) extensionapi.Metric {
	v.mu.Lock()
	defer v.mu.Unlock()
	metric := extensionapi.MetricFromName(name)
	v.builtins[kind] = metric
	return metric
}

// SetCurrentTags changes the tags returned by CurrentTags.
func (v *VU) SetCurrentTags(tags extensionapi.Tags) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.tags = extensionapi.NewTags(tags.Values(), tags.Metadata())
}

// Samples returns a snapshot of emitted samples.
func (v *VU) Samples() []extensionapi.Sample {
	v.mu.Lock()
	defer v.mu.Unlock()
	samples := make([]extensionapi.Sample, len(v.samples))
	copy(samples, v.samples)
	return samples
}

// RegisterMetric implements extensionapi.Metrics.
func (v *VU) RegisterMetric(spec extensionapi.MetricSpec) (extensionapi.Metric, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if previous, exists := v.metrics[spec.Name]; exists && previous != spec {
		return extensionapi.Metric{}, fmt.Errorf("metric %q has an incompatible declaration", spec.Name)
	}
	v.metrics[spec.Name] = spec
	return extensionapi.MetricFromName(spec.Name), nil
}

// BuiltinMetric implements extensionapi.Metrics.
func (v *VU) BuiltinMetric(kind extensionapi.BuiltinMetric) (extensionapi.Metric, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	metric, ok := v.builtins[kind]
	return metric, ok
}

// CurrentTags implements extensionapi.Metrics.
func (v *VU) CurrentTags() extensionapi.Tags {
	v.mu.Lock()
	defer v.mu.Unlock()
	return extensionapi.NewTags(v.tags.Values(), v.tags.Metadata())
}

// WithSystemTags implements extensionapi.Metrics.
func (v *VU) WithSystemTags(tags extensionapi.Tags, systemTags map[extensionapi.SystemTag]string) extensionapi.Tags {
	values := tags.Values()
	metadata := tags.Metadata()
	for kind, value := range systemTags {
		if !v.EnabledSystemTag[kind] {
			continue
		}
		if v.MetadataSystemTag[kind] {
			metadata[string(kind)] = value
		} else {
			values[string(kind)] = value
		}
	}
	return extensionapi.NewTags(values, metadata)
}

// Emit implements extensionapi.Metrics.
func (v *VU) Emit(ctx context.Context, samples []extensionapi.Sample) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, sample := range samples {
		if _, registered := v.metrics[sample.Metric.Name()]; !registered {
			builtin := false
			for _, metric := range v.builtins {
				if metric.Name() == sample.Metric.Name() {
					builtin = true
					break
				}
			}
			if !builtin {
				return errors.New("extension API metric is not registered")
			}
		}
		v.samples = append(v.samples, extensionapi.Sample{
			Metric: sample.Metric,
			Value:  sample.Value,
			Time:   sample.Time,
			Tags:   extensionapi.NewTags(sample.Tags.Values(), sample.Tags.Metadata()),
		})
	}
	return nil
}

type promiseResolver struct {
	enqueue func(extensionapi.Task)
	resolve func(any) error
	reject  func(any) error
}

func (r promiseResolver) Resolve(value any) {
	r.enqueue(func() error { return r.resolve(value) })
}

func (r promiseResolver) Reject(reason any) {
	r.enqueue(func() error { return r.reject(reason) })
}

type discardWriter struct{}

func (discardWriter) Write(data []byte) (int, error) { return len(data), nil }

type fieldNameMapper struct{}

func (fieldNameMapper) FieldName(_ reflect.Type, field reflect.StructField) string {
	if field.PkgPath != "" {
		return ""
	}
	if tag := field.Tag.Get("js"); tag != "" {
		if tag == "-" {
			return ""
		}
		return tag
	}
	return snakeCase(field.Name)
}

func (fieldNameMapper) MethodName(_ reflect.Type, method reflect.Method) string {
	return lowerFirst(method.Name)
}

func lowerFirst(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToLower(name[:1]) + name[1:]
}

func snakeCase(name string) string {
	var result strings.Builder
	runes := []rune(name)
	for index, character := range runes {
		if index > 0 && unicode.IsUpper(character) {
			previous := runes[index-1]
			nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			if unicode.IsLower(previous) || nextIsLower {
				result.WriteByte('_')
			}
		}
		result.WriteRune(unicode.ToLower(character))
	}
	return result.String()
}

var (
	_ extensionapi.VU          = (*VU)(nil)
	_ extensionapi.Environment = (*VU)(nil)
	_ extensionapi.Logger      = (*VU)(nil)
	_ extensionapi.Network     = (*VU)(nil)
	_ extensionapi.TLS         = (*VU)(nil)
	_ extensionapi.Scheduler   = (*VU)(nil)
	_ extensionapi.Promises    = (*VU)(nil)
	_ extensionapi.Metrics     = (*VU)(nil)
)
