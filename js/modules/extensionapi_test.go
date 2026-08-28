package modules

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/grafana/sobek"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	extensionapi "go.k6.io/k6-extension-api"

	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/fsext"
	"go.k6.io/k6/v2/metrics"
)

type extensionAPITestVU struct {
	state   *lib.State
	initEnv *common.InitEnvironment
}

func (vu extensionAPITestVU) Context() context.Context { return context.Background() }

func (extensionAPITestVU) Events() common.Events { return common.Events{} }

func (vu extensionAPITestVU) InitEnv() *common.InitEnvironment { return vu.initEnv }

func (vu extensionAPITestVU) State() *lib.State { return vu.state }

func (extensionAPITestVU) Runtime() *sobek.Runtime { return sobek.New() }

func (extensionAPITestVU) RegisterCallback() func(func() error) {
	return func(func() error) {}
}

type extensionAPITestDialer struct{}

func (extensionAPITestDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	connection, peer := net.Pipe()
	_ = peer.Close()
	return connection, nil
}

func (extensionAPITestDialer) ResolveAddr(string) (net.IP, int, error) {
	return net.ParseIP("192.0.2.1"), 0, nil
}

func (extensionAPITestDialer) CheckHost(string) error { return nil }

func TestExtensionAPIVUNetwork(t *testing.T) {
	t.Parallel()

	vu := extensionAPIVU{vu: extensionAPITestVU{state: &lib.State{Dialer: extensionAPITestDialer{}}}}
	network, ok := any(vu).(extensionapi.Network)
	require.True(t, ok)

	hosts, err := network.LookupHost(context.Background(), "example.test")
	require.NoError(t, err)
	require.Equal(t, []string{"192.0.2.1"}, hosts)

	connection, err := network.DialContext(context.Background(), "tcp", "example.test:443")
	require.NoError(t, err)
	require.NoError(t, connection.Close())
}

func TestExtensionAPIVUNetworkUnavailable(t *testing.T) {
	t.Parallel()

	vu := extensionAPIVU{vu: extensionAPITestVU{}}
	network := any(vu).(extensionapi.Network)

	_, err := network.LookupHost(context.Background(), "example.test")
	require.ErrorIs(t, err, extensionapi.ErrNetworkUnavailable)

	_, err = network.DialContext(context.Background(), "tcp", "example.test:443")
	require.ErrorIs(t, err, extensionapi.ErrNetworkUnavailable)
}

func TestExtensionAPIVUNetworkPolicy(t *testing.T) {
	t.Parallel()

	vu := extensionAPIVU{vu: extensionAPITestVU{state: &lib.State{Dialer: extensionAPITestDialer{}}}}
	policy, ok := any(vu).(extensionapi.NetworkPolicy)
	require.True(t, ok)
	require.NoError(t, policy.CheckHost(context.Background(), "example.test"))
}

func TestExtensionAPIVUNetworkPolicyUnavailable(t *testing.T) {
	t.Parallel()

	vu := extensionAPIVU{vu: extensionAPITestVU{}}
	policy := any(vu).(extensionapi.NetworkPolicy)
	require.ErrorIs(t, policy.CheckHost(context.Background(), "example.test"), extensionapi.ErrNetworkPolicyUnavailable)
}

func TestExtensionAPIVUTLSClient(t *testing.T) {
	t.Parallel()

	serverNames := make(chan string, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serverNames <- request.TLS.ServerName
		writer.WriteHeader(http.StatusNoContent)
	}))
	server.StartTLS()
	defer server.Close()

	hostTLSConfig := server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	hostTLSConfig.NextProtos = nil
	vu := extensionAPIVU{vu: extensionAPITestVU{state: &lib.State{TLSConfig: hostTLSConfig}}}
	capability, ok := any(vu).(extensionapi.TLS)
	require.True(t, ok)

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	dialer := &net.Dialer{}
	rawConnection, err := dialer.DialContext(context.Background(), "tcp", serverURL.Host)
	require.NoError(t, err)
	extensionTLSConfig := &tls.Config{
		ServerName:         "example.com",
		NextProtos:         []string{"http/1.1"},
		InsecureSkipVerify: true, // The host policy must not inherit this.
	}
	connection, err := capability.TLSClient(context.Background(), rawConnection, extensionTLSConfig)
	require.NoError(t, err)
	defer func() { require.NoError(t, connection.Close()) }()

	_, err = connection.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"))
	require.NoError(t, err)
	response := make([]byte, 1024)
	_, err = connection.Read(response)
	require.NoError(t, err)
	require.Equal(t, "example.com", <-serverNames)
	require.Nil(t, hostTLSConfig.NextProtos)
	require.False(t, hostTLSConfig.InsecureSkipVerify)

	rawConnection, err = dialer.DialContext(context.Background(), "tcp", serverURL.Host)
	require.NoError(t, err)
	_, err = capability.TLSClient(context.Background(), rawConnection, &tls.Config{
		ServerName: "example.com",
		RootCAs:    x509.NewCertPool(),
	})
	require.Error(t, err, "extension roots replace the host roots")
}

func TestExtensionAPIVUTLSClientUnavailable(t *testing.T) {
	t.Parallel()

	connection, peer := net.Pipe()
	defer func() { require.NoError(t, peer.Close()) }()
	vu := extensionAPIVU{vu: extensionAPITestVU{}}
	capability := any(vu).(extensionapi.TLS)
	_, err := capability.TLSClient(context.Background(), connection, nil)
	require.ErrorIs(t, err, extensionapi.ErrTLSUnavailable)
}

func TestExtensionAPIInitFileSystem(t *testing.T) {
	t.Parallel()
	memoryFS := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(memoryFS, "/keystore.jks", []byte("key"), 0o600))
	vu := extensionAPIVU{vu: extensionAPITestVU{initEnv: &common.InitEnvironment{
		FileSystems: map[string]fsext.Fs{"file": memoryFS},
		CWD:         &url.URL{Path: "/"},
	}}}
	fileSystem, ok := any(vu).(extensionapi.InitFileSystem)
	require.True(t, ok)
	provided, err := fileSystem.FileSystem()
	require.NoError(t, err)
	data, err := fs.ReadFile(provided, "keystore.jks")
	require.NoError(t, err)
	require.Equal(t, []byte("key"), data)
}

func TestExtensionAPIHTTPForceHTTP1(t *testing.T) {
	t.Parallel()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	baseTransport := server.Client().Transport.(*http.Transport).Clone()
	baseTransport.ForceAttemptHTTP2 = true
	preflight := &http.Client{Transport: baseTransport.Clone()}
	preflightRequest, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	preflightResponse, err := preflight.Do(preflightRequest)
	require.NoError(t, err)
	require.Equal(t, "HTTP/2.0", preflightResponse.Proto)
	require.NoError(t, preflightResponse.Body.Close())

	registry := metrics.NewRegistry()
	builtins := metrics.RegisterBuiltinMetrics(registry)
	systemTags := metrics.NewSystemTagSet()
	vu := extensionAPIVU{vu: extensionAPITestVU{
		state: &lib.State{
			BuiltinMetrics: builtins,
			Options:        lib.Options{SystemTags: systemTags},
			Samples:        make(chan metrics.SampleContainer, 1),
			Tags:           lib.NewVUStateTags(registry.RootTagSet()),
			Transport:      baseTransport,
		},
		initEnv: &common.InitEnvironment{TestPreInitState: &lib.TestPreInitState{Registry: registry}},
	}}
	httpAPI := any(vu).(extensionapi.HTTP)
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	response, err := httpAPI.Do(context.Background(), request, extensionapi.HTTPOptions{
		ForceHTTP1:   true,
		DeferMetrics: true,
	})
	require.NoError(t, err)
	require.Equal(t, "HTTP/1.1", response.Proto)
	require.NoError(t, response.Body.Close())
}

func TestExtensionAPISlogHandler(t *testing.T) {
	t.Parallel()

	logger, hook := logtest.NewNullLogger()
	slog.New(newExtensionAPISlogHandler(logger)).With("extension", "test").WithGroup("request").Warn(
		"request failed", "status", 503, slog.Group("response", "retryable", true),
	)

	require.Len(t, hook.Entries, 1)
	entry := hook.LastEntry()
	require.Equal(t, "request failed", entry.Message)
	require.Equal(t, "warning", entry.Level.String())
	require.Equal(t, "test", entry.Data["extension"])
	require.EqualValues(t, 503, entry.Data["request.status"])
	require.Equal(t, true, entry.Data["request.response.retryable"])
}

func TestExtensionAPIMetrics(t *testing.T) {
	t.Parallel()
	registry := metrics.NewRegistry()
	builtins := metrics.RegisterBuiltinMetrics(registry)
	systemTags := metrics.SystemTagSet(metrics.TagURL)
	samples := make(chan metrics.SampleContainer, 1)
	state := &lib.State{
		BuiltinMetrics: builtins,
		Options:        lib.Options{SystemTags: &systemTags},
		Samples:        samples,
		Tags:           lib.NewVUStateTags(registry.RootTagSet().With("scenario", "test")),
	}
	vu := extensionAPIVU{vu: extensionAPITestVU{
		state: state,
		initEnv: &common.InitEnvironment{TestPreInitState: &lib.TestPreInitState{
			Registry:       registry,
			BuiltinMetrics: builtins,
		}},
	}}
	api, ok := any(vu).(extensionapi.Metrics)
	require.True(t, ok)

	metric, err := api.RegisterMetric(extensionapi.MetricSpec{
		Name: "extension_api_metric", Kind: extensionapi.MetricCounter, Unit: extensionapi.MetricUnitData,
	})
	require.NoError(t, err)
	duplicate, err := api.RegisterMetric(extensionapi.MetricSpec{
		Name: "extension_api_metric", Kind: extensionapi.MetricCounter, Unit: extensionapi.MetricUnitData,
	})
	require.NoError(t, err)
	require.Equal(t, metric.Name(), duplicate.Name())
	_, err = api.RegisterMetric(extensionapi.MetricSpec{
		Name: "extension_api_metric", Kind: extensionapi.MetricGauge, Unit: extensionapi.MetricUnitData,
	})
	require.Error(t, err)

	tags := api.CurrentTags()
	state.Tags.Modify(func(current *metrics.TagsAndMeta) { current.SetTag("scenario", "changed") })
	require.Equal(t, "test", tags.Values()["scenario"])
	tags = api.WithSystemTags(tags.With(map[string]string{"operation": "write"}), map[extensionapi.SystemTag]string{
		extensionapi.SystemTagURL: "https://example.test",
		extensionapi.SystemTagIP:  "192.0.2.1",
	})
	require.Equal(t, "https://example.test", tags.Values()["url"])
	require.NotContains(t, tags.Values(), "ip")

	require.NoError(t, api.Emit(context.Background(), []extensionapi.Sample{{
		Metric: metric,
		Value:  42,
		Tags:   tags.WithMetadata(map[string]string{"request_id": "abc"}),
	}}))
	emitted := (<-samples).GetSamples()
	require.Len(t, emitted, 1)
	require.Equal(t, "extension_api_metric", emitted[0].Metric.Name)
	require.Equal(t, float64(42), emitted[0].Value)
	require.Equal(t, "https://example.test", emitted[0].Tags.Map()["url"])
	require.Equal(t, "abc", emitted[0].Metadata["request_id"])

	dataSent, ok := api.BuiltinMetric(extensionapi.BuiltinDataSent)
	require.True(t, ok)
	require.Equal(t, builtins.DataSent.Name, dataSent.Name())

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, api.Emit(canceled, []extensionapi.Sample{{Metric: metric, Value: 1}}), context.Canceled)
	require.Empty(t, metrics.GetBufferedSamples(samples))

	blockedSamples := make(chan metrics.SampleContainer)
	state.Samples = blockedSamples
	blockingContext, cancelBlocking := context.WithCancel(context.Background())
	blockedResult := make(chan error, 1)
	go func() {
		blockedResult <- api.Emit(blockingContext, []extensionapi.Sample{{Metric: metric, Value: 1}})
	}()
	cancelBlocking()
	require.ErrorIs(t, <-blockedResult, context.Canceled)
}
