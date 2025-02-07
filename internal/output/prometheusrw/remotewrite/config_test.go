package remotewrite

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/output/prometheusrw/remote"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

func TestConfigApply(t *testing.T) {
	t.Parallel()

	fullConfig := Config{
		ServerURL:             null.StringFrom("some-url"),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		Username:              null.StringFrom("user"),
		Password:              null.StringFrom("pass"),
		PushInterval:          types.NullDurationFrom(10 * time.Second),
		Headers: map[string]string{
			"X-Header": "value",
		},
		TrendStats:   []string{"p(99)"},
		StaleMarkers: null.BoolFrom(true),
	}

	// Defaults should be overwritten by valid values
	c := NewConfig()
	c = c.Apply(fullConfig)
	assert.Equal(t, fullConfig, c)

	// Defaults shouldn't be impacted by invalid values
	c = NewConfig()
	c = c.Apply(Config{
		Username:              null.NewString("user", false),
		Password:              null.NewString("pass", false),
		InsecureSkipTLSVerify: null.NewBool(false, false),
	})
	assert.Equal(t, false, c.Username.Valid)
	assert.Equal(t, false, c.Password.Valid)
	assert.Equal(t, true, c.InsecureSkipTLSVerify.Valid)
}

func TestConfigRemoteConfig(t *testing.T) {
	t.Parallel()
	u, err := url.Parse("https://prometheus.ie/remote")
	require.NoError(t, err)

	config := Config{
		ServerURL:             null.StringFrom(u.String()),
		InsecureSkipTLSVerify: null.BoolFrom(true),
		Username:              null.StringFrom("myuser"),
		Password:              null.StringFrom("mypass"),
		Headers: map[string]string{
			"X-MYCUSTOM-HEADER": "val1",
			// it asserts that Authz header is overwritten if the token is set
			"Authorization": "pre-set-token",
		},
		BearerToken: null.StringFrom("my-fake-token"),
	}

	headers := http.Header{}
	headers.Set("X-MYCUSTOM-HEADER", "val1")
	headers.Set("Authorization", "Bearer my-fake-token")
	exprcc := &remote.HTTPConfig{
		Timeout: 5 * time.Second,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		},
		BasicAuth: &remote.BasicAuth{
			Username: "myuser",
			Password: "mypass",
		},
		Headers: headers,
	}
	rcc, err := config.RemoteConfig()
	require.NoError(t, err)
	assert.Equal(t, exprcc, rcc)
}

func TestConfigRemoteConfigClientCertificateError(t *testing.T) {
	t.Parallel()

	config := Config{
		ClientCertificate:    null.StringFrom("bad-cert-value"),
		ClientCertificateKey: null.StringFrom("bad-cert-key"),
	}

	rcc, err := config.RemoteConfig()
	assert.ErrorContains(t, err, "TLS certificate")
	assert.Nil(t, rcc)
}

func TestGetConsolidatedConfig(t *testing.T) {
	t.Parallel()

	u, err := url.Parse("https://prometheus.ie/remote")
	require.NoError(t, err)

	testCases := map[string]struct {
		jsonRaw   json.RawMessage
		env       map[string]string
		arg       string
		config    Config
		errString string
	}{
		"Defaults": {
			jsonRaw: nil,
			env:     nil,
			arg:     "",
			config: Config{
				ServerURL:             null.StringFrom("http://localhost:9090/api/v1/write"),
				InsecureSkipTLSVerify: null.BoolFrom(false),
				Username:              null.NewString("", false),
				Password:              null.NewString("", false),
				PushInterval:          types.NullDurationFrom(5 * time.Second),
				Headers:               make(map[string]string),
				TrendStats:            []string{"p(99)"},
				StaleMarkers:          null.BoolFrom(false),
			},
		},
		"JSONSuccess": {
			jsonRaw: json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, u.String())),
			config: Config{
				ServerURL:             null.StringFrom(u.String()),
				InsecureSkipTLSVerify: null.BoolFrom(false),
				Username:              null.NewString("", false),
				Password:              null.NewString("", false),
				PushInterval:          types.NullDurationFrom(defaultPushInterval),
				Headers:               make(map[string]string),
				TrendStats:            []string{"p(99)"},
				StaleMarkers:          null.BoolFrom(false),
			},
		},
		"MixedSuccess": {
			jsonRaw: json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, u.String())),
			env: map[string]string{
				"K6_PROMETHEUS_RW_INSECURE_SKIP_TLS_VERIFY": "false",
				"K6_PROMETHEUS_RW_USERNAME":                 "u",
			},
			// arg: "username=user",
			config: Config{
				ServerURL:             null.StringFrom(u.String()),
				InsecureSkipTLSVerify: null.BoolFrom(false),
				Username:              null.NewString("u", true),
				Password:              null.NewString("", false),
				PushInterval:          types.NullDurationFrom(defaultPushInterval),
				Headers:               make(map[string]string),
				TrendStats:            []string{"p(99)"},
				StaleMarkers:          null.BoolFrom(false),
			},
		},
		"OrderOfPrecedence": {
			jsonRaw: json.RawMessage(`{"url":"http://json:9090","username":"json","password":"json"}`),
			env: map[string]string{
				"K6_PROMETHEUS_RW_USERNAME": "env",
				"K6_PROMETHEUS_RW_PASSWORD": "env",
			},
			// arg: "password=arg",
			config: Config{
				ServerURL:             null.StringFrom("http://json:9090"),
				InsecureSkipTLSVerify: null.BoolFrom(false),
				Username:              null.StringFrom("env"),
				Password:              null.StringFrom("env"),
				PushInterval:          types.NullDurationFrom(defaultPushInterval),
				Headers:               make(map[string]string),
				TrendStats:            []string{"p(99)"},
				StaleMarkers:          null.BoolFrom(false),
			},
		},
		"InvalidJSON": {
			jsonRaw:   json.RawMessage(`{"invalid-json "astring"}`),
			errString: "parse JSON options failed",
		},
		"InvalidEnv": {
			env:       map[string]string{"K6_PROMETHEUS_RW_INSECURE_SKIP_TLS_VERIFY": "d"},
			errString: "parse environment variables options failed",
		},
		//nolint:gocritic
		//"InvalidArg": {
		//arg:       "insecureSkipTLSVerify=wrongtime",
		//errString: "parse argument string options failed",
		//},
	}

	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := GetConsolidatedConfig(testCase.jsonRaw, testCase.env, testCase.arg)
			if len(testCase.errString) > 0 {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), testCase.errString)
				return
			}
			assert.Equal(t, testCase.config, c)
		})
	}
}

func TestParseServerURL(t *testing.T) {
	t.Parallel()

	c, err := parseArg("url=http://prometheus.remote:3412/write")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("http://prometheus.remote:3412/write"), c.ServerURL)

	c, err = parseArg("url=http://prometheus.remote:3412/write,insecureSkipTLSVerify=false,pushInterval=2s")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("http://prometheus.remote:3412/write"), c.ServerURL)
	assert.Equal(t, null.BoolFrom(false), c.InsecureSkipTLSVerify)
	assert.Equal(t, types.NullDurationFrom(time.Second*2), c.PushInterval)

	c, err = parseArg("headers.X-Header=value")
	assert.Nil(t, err)
	assert.Equal(t, map[string]string{"X-Header": "value"}, c.Headers)
}

// TODO: replace all the expconfigs below
// with a function that returns the expected default values,
// then override only the values to expect differently.

func TestOptionServerURL(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(`{"url":"http://prometheus:9090/api/v1/write"}`)},
		"Env":  {env: map[string]string{"K6_PROMETHEUS_RW_SERVER_URL": "http://prometheus:9090/api/v1/write"}},
		//nolint:gocritic
		//"Arg":  {arg: "url=http://prometheus:9090/api/v1/write"},
	}

	expconfig := Config{
		ServerURL:             null.StringFrom("http://prometheus:9090/api/v1/write"),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		Username:              null.NewString("", false),
		Password:              null.NewString("", false),
		PushInterval:          types.NullDurationFrom(5 * time.Second),
		Headers:               make(map[string]string),
		TrendStats:            []string{"p(99)"},
		StaleMarkers:          null.BoolFrom(false),
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}

func TestOptionHeaders(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(
			`{"headers":{"X-MY-HEADER1":"hval1","X-MY-HEADER2":"hval2","X-Scope-OrgID":"my-org-id","another-header":"true","empty":""}}`)},
		"Env": {env: map[string]string{
			"K6_PROMETHEUS_RW_HEADERS_X-MY-HEADER1": "hval1",
			"K6_PROMETHEUS_RW_HEADERS_X-MY-HEADER2": "hval2",
			// it assert that the new method using HTTP_HEADERS overwrites it
			"K6_PROMETHEUS_RW_HEADERS_X-Scope-OrgID": "my-org-id-old-method",
			"K6_PROMETHEUS_RW_HTTP_HEADERS":          "X-Scope-OrgID:my-org-id,another-header:true,empty:",
		}},
		//nolint:gocritic
		//"Arg":  {arg: "headers.X-MY-HEADER1=hval1,headers.X-MY-HEADER2=hval2"},
	}

	expconfig := Config{
		ServerURL:             null.StringFrom("http://localhost:9090/api/v1/write"),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		PushInterval:          types.NullDurationFrom(5 * time.Second),
		Headers: map[string]string{
			"X-MY-HEADER1":   "hval1",
			"X-MY-HEADER2":   "hval2",
			"X-Scope-OrgID":  "my-org-id",
			"another-header": "true",
			"empty":          "",
		},
		TrendStats:   []string{"p(99)"},
		StaleMarkers: null.BoolFrom(false),
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}

func TestOptionInsecureSkipTLSVerify(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(`{"insecureSkipTLSVerify":false}`)},
		"Env":  {env: map[string]string{"K6_PROMETHEUS_RW_INSECURE_SKIP_TLS_VERIFY": "false"}},
		//nolint:gocritic
		//"Arg":  {arg: "insecureSkipTLSVerify=false"},
	}

	expconfig := Config{
		ServerURL:             null.StringFrom(defaultServerURL),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		PushInterval:          types.NullDurationFrom(defaultPushInterval),
		Headers:               make(map[string]string),
		TrendStats:            []string{"p(99)"},
		StaleMarkers:          null.BoolFrom(false),
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}

func TestOptionBasicAuth(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(`{"username":"user1","password":"pass1"}`)},
		"Env":  {env: map[string]string{"K6_PROMETHEUS_RW_USERNAME": "user1", "K6_PROMETHEUS_RW_PASSWORD": "pass1"}},
		//nolint:gocritic
		//"Arg":  {arg: "username=user1,password=pass1"},
	}

	expconfig := Config{
		ServerURL:             null.StringFrom("http://localhost:9090/api/v1/write"),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		Username:              null.StringFrom("user1"),
		Password:              null.StringFrom("pass1"),
		PushInterval:          types.NullDurationFrom(5 * time.Second),
		Headers:               make(map[string]string),
		TrendStats:            []string{"p(99)"},
		StaleMarkers:          null.BoolFrom(false),
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}

func TestOptionBearerToken(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(`{"bearerToken":"my-bearer-token"}`)},
		"Env":  {env: map[string]string{"K6_PROMETHEUS_RW_BEARER_TOKEN": "my-bearer-token"}},
	}

	expconfig := Config{
		ServerURL:             null.StringFrom("http://localhost:9090/api/v1/write"),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		BearerToken:           null.StringFrom("my-bearer-token"),
		PushInterval:          types.NullDurationFrom(5 * time.Second),
		Headers:               make(map[string]string),
		TrendStats:            []string{"p(99)"},
		StaleMarkers:          null.BoolFrom(false),
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}

func TestOptionClientCertificate(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(`{"clientCertificate":"client.crt","clientCertificateKey":"client.key"}`)},
		"Env":  {env: map[string]string{"K6_PROMETHEUS_RW_CLIENT_CERTIFICATE": "client.crt", "K6_PROMETHEUS_RW_CLIENT_CERTIFICATE_KEY": "client.key"}},
	}

	expconfig := Config{
		ServerURL:             null.StringFrom("http://localhost:9090/api/v1/write"),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		PushInterval:          types.NullDurationFrom(5 * time.Second),
		Headers:               make(map[string]string),
		TrendStats:            []string{"p(99)"},
		ClientCertificate:     null.StringFrom("client.crt"),
		ClientCertificateKey:  null.StringFrom("client.key"),
		StaleMarkers:          null.BoolFrom(false),
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}

func TestOptionTrendAsNativeHistogram(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(`{"trendAsNativeHistogram":true}`)},
		"Env":  {env: map[string]string{"K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM": "true"}},
		//nolint:gocritic
		//"Arg":  {arg: "trendAsNativeHistogram=true"},
	}

	expconfig := Config{
		ServerURL:              null.StringFrom("http://localhost:9090/api/v1/write"),
		InsecureSkipTLSVerify:  null.BoolFrom(false),
		Username:               null.NewString("", false),
		Password:               null.NewString("", false),
		PushInterval:           types.NullDurationFrom(5 * time.Second),
		Headers:                make(map[string]string),
		TrendAsNativeHistogram: null.BoolFrom(true),
		TrendStats:             []string{"p(99)"},
		StaleMarkers:           null.BoolFrom(false),
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}

func TestOptionPushInterval(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(`{"pushInterval":"1m2s"}`)},
		"Env":  {env: map[string]string{"K6_PROMETHEUS_RW_PUSH_INTERVAL": "1m2s"}},
	}

	expconfig := Config{
		ServerURL:             null.StringFrom("http://localhost:9090/api/v1/write"),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		Username:              null.NewString("", false),
		Password:              null.NewString("", false),
		PushInterval:          types.NullDurationFrom((1 * time.Minute) + (2 * time.Second)),
		Headers:               make(map[string]string),
		TrendStats:            []string{"p(99)"},
		StaleMarkers:          null.BoolFrom(false),
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}

func TestConfigTrendStats(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(`{"trendStats":["max","p(95)"]}`)},
		"Env":  {env: map[string]string{"K6_PROMETHEUS_RW_TREND_STATS": "max,p(95)"}},
		// TODO: support arg, check the comment in the code
		//nolint:gocritic
		//"Arg":  {arg: "trendStats=max,p(95)"},
	}

	expconfig := Config{
		ServerURL:             null.StringFrom("http://localhost:9090/api/v1/write"),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		PushInterval:          types.NullDurationFrom(5 * time.Second),
		Headers:               make(map[string]string),
		TrendStats:            []string{"max", "p(95)"},
		StaleMarkers:          null.BoolFrom(false),
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}

func TestOptionStaleMarker(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(`{"staleMarkers":true}`)},
		"Env":  {env: map[string]string{"K6_PROMETHEUS_RW_STALE_MARKERS": "true"}},
	}

	expconfig := Config{
		ServerURL:             null.StringFrom("http://localhost:9090/api/v1/write"),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		PushInterval:          types.NullDurationFrom(5 * time.Second),
		Headers:               make(map[string]string),
		TrendStats:            []string{"p(99)"},
		StaleMarkers:          null.BoolFrom(true),
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}
