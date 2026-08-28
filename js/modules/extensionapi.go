package modules

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/grafana/sobek"

	extensionapi "go.k6.io/k6-extension-api"

	"go.k6.io/k6/v2/lib/netext/httpext"
	"go.k6.io/k6/v2/metrics"
)

// extensionAPIModuleAdapter is the k6-owned translation layer between the
// standalone extension API and the legacy module resolver. No k6 type is
// exposed from the standalone API.
type extensionAPIModuleAdapter struct {
	module extensionapi.Module
}

func (a extensionAPIModuleAdapter) NewModuleInstance(vu VU) Instance {
	return extensionAPIInstanceAdapter{instance: a.module.NewModuleInstance(extensionAPIVU{vu: vu})}
}

type extensionAPIVU struct {
	vu VU
}

type extensionAPIFileSystem struct {
	open func(string) (fs.File, error)
}

func (f extensionAPIFileSystem) Open(name string) (fs.File, error) { return f.open(name) }

func (v extensionAPIVU) Do(
	ctx context.Context, request *http.Request, options extensionapi.HTTPOptions,
) (*extensionapi.HTTPResponse, error) {
	state := v.vu.State()
	initEnv := v.vu.InitEnv()
	if state == nil || state.Transport == nil || initEnv == nil || initEnv.Registry == nil {
		return nil, extensionapi.ErrHTTPUnavailable
	}
	tags := state.Tags.GetCurrentValues()
	if values, metadata := options.Tags.Values(), options.Tags.Metadata(); values != nil || metadata != nil {
		tags = metrics.TagsAndMeta{Tags: initEnv.Registry.RootTagSet().WithTagsFromMap(values), Metadata: metadata}
	}
	var jar http.CookieJar
	if state.CookieJar != nil {
		jar = state.CookieJar
	}
	if options.Jar != nil {
		jar = options.Jar
	}
	transport := state.Transport
	if options.ForceHTTP1 {
		baseTransport, ok := state.Transport.(*http.Transport)
		if !ok {
			return nil, fmt.Errorf("extension HTTP transport cannot disable HTTP/2")
		}
		forceHTTP1Transport := baseTransport.Clone()
		forceHTTP1Transport.ForceAttemptHTTP2 = false
		forceHTTP1Transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		if forceHTTP1Transport.TLSClientConfig != nil {
			forceHTTP1Transport.TLSClientConfig = forceHTTP1Transport.TLSClientConfig.Clone()
			forceHTTP1Transport.TLSClientConfig.NextProtos = []string{"http/1.1"}
		}
		transport = forceHTTP1Transport
	}
	response, err := httpext.MakeRequestWithLiveResponse(ctx, state, request, httpext.LiveRequestOptions{
		TagsAndMeta:      tags,
		Jar:              jar,
		ResponseCallback: options.ExpectedStatus,
		Transport:        transport,
	})
	if err != nil || response == nil || options.DeferMetrics {
		return &extensionapi.HTTPResponse{Response: response}, err
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	return &extensionapi.HTTPResponse{Response: response}, nil
}

func (v extensionAPIVU) ExecutionPhase() extensionapi.ExecutionPhase {
	if v.vu.State() == nil {
		return extensionapi.ExecutionPhaseInit
	}
	return extensionapi.ExecutionPhaseVU
}

func (v extensionAPIVU) FileSystem() (fs.FS, error) {
	if v.vu.State() != nil {
		return nil, extensionapi.ErrFileSystemUnavailable
	}
	initEnv := v.vu.InitEnv()
	if initEnv == nil {
		return nil, extensionapi.ErrFileSystemUnavailable
	}
	fileSystem, ok := initEnv.FileSystems["file"]
	if !ok {
		return nil, extensionapi.ErrFileSystemUnavailable
	}
	return extensionAPIFileSystem{open: func(name string) (fs.File, error) {
		return fileSystem.Open(initEnv.GetAbsFilePath(name))
	}}, nil
}

func (v extensionAPIVU) VUID() uint64 {
	if state := v.vu.State(); state != nil {
		return state.VUID
	}
	return 0
}

func (v extensionAPIVU) Context() context.Context {
	return v.vu.Context()
}

func (v extensionAPIVU) Runtime() *sobek.Runtime {
	return v.vu.Runtime()
}

func (v extensionAPIVU) LookupEnv(key string) (string, bool) {
	initEnv := v.vu.InitEnv()
	if initEnv == nil || initEnv.LookupEnv == nil {
		return "", false
	}

	return initEnv.LookupEnv(key)
}

func (v extensionAPIVU) Logger() *slog.Logger {
	if initEnv := v.vu.InitEnv(); initEnv != nil && initEnv.Logger != nil {
		return slog.New(newExtensionAPISlogHandler(initEnv.Logger))
	}
	if state := v.vu.State(); state != nil && state.Logger != nil {
		return slog.New(newExtensionAPISlogHandler(state.Logger))
	}
	return newExtensionAPIDiscardLogger()
}

func (v extensionAPIVU) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	state := v.vu.State()
	if state == nil || state.Dialer == nil {
		return nil, extensionapi.ErrNetworkUnavailable
	}

	return state.Dialer.DialContext(ctx, network, address)
}

func (v extensionAPIVU) LookupHost(ctx context.Context, host string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	state := v.vu.State()
	if state == nil {
		return nil, extensionapi.ErrNetworkUnavailable
	}

	resolver := state.GetAddrResolver()
	if resolver == nil {
		return nil, extensionapi.ErrNetworkUnavailable
	}

	ip, _, err := resolver.ResolveAddr(host)
	if err != nil {
		return nil, err
	}

	return []string{ip.String()}, nil
}

func (v extensionAPIVU) CheckHost(ctx context.Context, host string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	state := v.vu.State()
	if state == nil || state.Dialer == nil {
		return extensionapi.ErrNetworkPolicyUnavailable
	}

	policy, ok := state.Dialer.(interface{ CheckHost(string) error })
	if !ok {
		return extensionapi.ErrNetworkPolicyUnavailable
	}

	return policy.CheckHost(host)
}

func (v extensionAPIVU) TLSClient(ctx context.Context, conn net.Conn, config *tls.Config) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	state := v.vu.State()
	if state == nil {
		_ = conn.Close()
		return nil, extensionapi.ErrTLSUnavailable
	}

	tlsConfig := &tls.Config{}
	if state.TLSConfig != nil {
		tlsConfig = state.TLSConfig.Clone()
	}
	if config != nil {
		mergeExtensionAPITLSConfig(tlsConfig, config)
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return tlsConn, nil
}

func mergeExtensionAPITLSConfig(host, extension *tls.Config) {
	if extension.ServerName != "" {
		host.ServerName = extension.ServerName
	}
	if extension.NextProtos != nil {
		host.NextProtos = append([]string(nil), extension.NextProtos...)
	}
	if extension.RootCAs != nil {
		host.RootCAs = extension.RootCAs
	}
	if len(extension.Certificates) > 0 {
		host.Certificates = append(host.Certificates, extension.Certificates...)
	}
}

func (v extensionAPIVU) RegisterCallback() func(extensionapi.Task) {
	callback := v.vu.RegisterCallback()
	return func(task extensionapi.Task) {
		callback(func() error { return task() })
	}
}

func (v extensionAPIVU) NewPromise() (*sobek.Promise, extensionapi.PromiseResolver) {
	promise, resolve, reject := v.vu.Runtime().NewPromise()
	return promise, &extensionAPIPromiseResolver{
		enqueue: v.RegisterCallback(),
		resolve: resolve,
		reject:  reject,
	}
}

func (v extensionAPIVU) RegisterMetric(spec extensionapi.MetricSpec) (extensionapi.Metric, error) {
	initEnv := v.vu.InitEnv()
	if initEnv == nil || initEnv.Registry == nil {
		return extensionapi.Metric{}, extensionapi.ErrMetricsUnavailable
	}

	metricType, err := extensionAPIMetricType(spec.Kind)
	if err != nil {
		return extensionapi.Metric{}, err
	}
	valueType, err := extensionAPIMetricUnit(spec.Unit)
	if err != nil {
		return extensionapi.Metric{}, err
	}
	if _, err := initEnv.Registry.NewMetric(spec.Name, metricType, valueType); err != nil {
		return extensionapi.Metric{}, err
	}
	return extensionAPIMetric(spec.Name), nil
}

func (v extensionAPIVU) BuiltinMetric(builtin extensionapi.BuiltinMetric) (extensionapi.Metric, bool) {
	initEnv := v.vu.InitEnv()
	if initEnv == nil || initEnv.BuiltinMetrics == nil {
		return extensionapi.Metric{}, false
	}

	switch builtin {
	case extensionapi.BuiltinDataSent:
		return extensionAPIMetric(initEnv.BuiltinMetrics.DataSent.Name), true
	case extensionapi.BuiltinDataReceived:
		return extensionAPIMetric(initEnv.BuiltinMetrics.DataReceived.Name), true
	default:
		return extensionapi.Metric{}, false
	}
}

func (v extensionAPIVU) CurrentTags() extensionapi.Tags {
	state := v.vu.State()
	if state == nil || state.Tags == nil {
		return extensionapi.NewTags(nil, nil)
	}
	tagsAndMeta := state.Tags.GetCurrentValues()
	return extensionapi.NewTags(tagsAndMeta.Tags.Map(), tagsAndMeta.Metadata)
}

func (v extensionAPIVU) WithSystemTags(
	tags extensionapi.Tags, systemTags map[extensionapi.SystemTag]string,
) extensionapi.Tags {
	state := v.vu.State()
	if state == nil || state.Options.SystemTags == nil || len(systemTags) == 0 {
		return tags
	}

	initEnv := v.vu.InitEnv()
	if initEnv == nil || initEnv.Registry == nil {
		return tags
	}
	tagsAndMeta := metrics.TagsAndMeta{
		Tags:     initEnv.Registry.RootTagSet().WithTagsFromMap(tags.Values()),
		Metadata: tags.Metadata(),
	}
	for systemTag, value := range systemTags {
		if k6SystemTag, ok := extensionAPIToK6SystemTag(systemTag); ok {
			tagsAndMeta.SetSystemTagOrMetaIfEnabled(state.Options.SystemTags, k6SystemTag, value)
		}
	}
	return extensionapi.NewTags(tagsAndMeta.Tags.Map(), tagsAndMeta.Metadata)
}

func (v extensionAPIVU) Emit(ctx context.Context, samples []extensionapi.Sample) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(samples) == 0 {
		return nil
	}
	state := v.vu.State()
	initEnv := v.vu.InitEnv()
	if state == nil || state.Samples == nil || initEnv == nil || initEnv.Registry == nil {
		return extensionapi.ErrMetricsUnavailable
	}

	k6Samples := make(metrics.Samples, 0, len(samples))
	for _, sample := range samples {
		metric := initEnv.Registry.Get(sample.Metric.Name())
		if metric == nil {
			return fmt.Errorf("extension API metric %q is not registered", sample.Metric.Name())
		}
		timestamp := sample.Time
		if timestamp.IsZero() {
			timestamp = time.Now()
		}
		k6Samples = append(k6Samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: metric,
				Tags:   initEnv.Registry.RootTagSet().WithTagsFromMap(sample.Tags.Values()),
			},
			Time:     timestamp,
			Value:    sample.Value,
			Metadata: sample.Tags.Metadata(),
		})
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case state.Samples <- k6Samples:
		return nil
	}
}

func extensionAPIMetric(name string) extensionapi.Metric {
	return extensionapi.MetricFromName(name)
}

func extensionAPIMetricType(kind extensionapi.MetricKind) (metrics.MetricType, error) {
	switch kind {
	case extensionapi.MetricCounter:
		return metrics.Counter, nil
	case extensionapi.MetricGauge:
		return metrics.Gauge, nil
	case extensionapi.MetricTrend:
		return metrics.Trend, nil
	case extensionapi.MetricRate:
		return metrics.Rate, nil
	default:
		return 0, fmt.Errorf("unsupported extension API metric kind %d", kind)
	}
}

func extensionAPIMetricUnit(unit extensionapi.MetricUnit) (metrics.ValueType, error) {
	switch unit {
	case extensionapi.MetricUnitDefault:
		return metrics.Default, nil
	case extensionapi.MetricUnitTime:
		return metrics.Time, nil
	case extensionapi.MetricUnitData:
		return metrics.Data, nil
	default:
		return 0, fmt.Errorf("unsupported extension API metric unit %d", unit)
	}
}

func extensionAPIToK6SystemTag(tag extensionapi.SystemTag) (metrics.SystemTag, bool) {
	switch tag {
	case extensionapi.SystemTagIP:
		return metrics.TagIP, true
	case extensionapi.SystemTagMethod:
		return metrics.TagMethod, true
	case extensionapi.SystemTagProto:
		return metrics.TagProto, true
	case extensionapi.SystemTagStatus:
		return metrics.TagStatus, true
	case extensionapi.SystemTagSubproto:
		return metrics.TagSubproto, true
	case extensionapi.SystemTagURL:
		return metrics.TagURL, true
	default:
		return 0, false
	}
}

type extensionAPIPromiseResolver struct {
	once    sync.Once
	enqueue func(extensionapi.Task)
	resolve func(any) error
	reject  func(any) error
}

func (r *extensionAPIPromiseResolver) Resolve(value any) {
	r.once.Do(func() {
		r.enqueue(func() error { return r.resolve(value) })
	})
}

func (r *extensionAPIPromiseResolver) Reject(reason any) {
	r.once.Do(func() {
		r.enqueue(func() error { return r.reject(reason) })
	})
}

type extensionAPIInstanceAdapter struct {
	instance extensionapi.Instance
}

func (a extensionAPIInstanceAdapter) Exports() Exports {
	exports := a.instance.Exports()
	return Exports{Default: exports.Default, Named: exports.Named}
}
