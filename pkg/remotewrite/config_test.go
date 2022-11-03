package remotewrite

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/grafana/xk6-output-prometheus-remote/pkg/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

// TODO: create an issue?
// TODO: refactor, it should have the 3 way tested in general
// and each option should be tested in a specific table test
// for all the methods at the same time, it should prevent the possibility
// we forget to support or test an option in one of the available ways.
// It also should keep the single test shorter.
// Check the TestConfigTrendAsNativeHistogram as an example.

func TestApply(t *testing.T) {
	t.Parallel()

	fullConfig := Config{
		URL:                   null.StringFrom("some-url"),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		Username:              null.StringFrom("user"),
		Password:              null.StringFrom("pass"),
		PushInterval:          types.NullDurationFrom(10 * time.Second),
		Headers: map[string]string{
			"X-Header": "value",
		},
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

func TestConfigParseArg(t *testing.T) {
	t.Parallel()

	c, err := ParseArg("url=http://prometheus.remote:3412/write")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("http://prometheus.remote:3412/write"), c.URL)

	c, err = ParseArg("url=http://prometheus.remote:3412/write,insecureSkipTLSVerify=false")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("http://prometheus.remote:3412/write"), c.URL)
	assert.Equal(t, null.BoolFrom(false), c.InsecureSkipTLSVerify)

	c, err = ParseArg("url=https://prometheus.remote:3412/write")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("https://prometheus.remote:3412/write"), c.URL)

	c, err = ParseArg("url=https://prometheus.remote:3412/write,insecureSkipTLSVerify=false")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("https://prometheus.remote:3412/write"), c.URL)
	assert.Equal(t, null.BoolFrom(false), c.InsecureSkipTLSVerify)

	c, err = ParseArg("url=http://prometheus.remote:3412/write,pushInterval=2s")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("http://prometheus.remote:3412/write"), c.URL)
	assert.Equal(t, types.NullDurationFrom(time.Second*2), c.PushInterval)

	c, err = ParseArg("url=http://prometheus.remote:3412/write,headers.X-Header=value")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("http://prometheus.remote:3412/write"), c.URL)
	assert.Equal(t, map[string]string{"X-Header": "value"}, c.Headers)
}

func TestConfigRemoteConfig(t *testing.T) {
	u, err := url.Parse("https://prometheus.ie/remote")
	require.NoError(t, err)

	config := Config{
		URL:                   null.StringFrom(u.String()),
		InsecureSkipTLSVerify: null.BoolFrom(true),
		Username:              null.StringFrom("myuser"),
		Password:              null.StringFrom("mypass"),
		Headers: map[string]string{
			"X-MYCUSTOM-HEADER": "val1",
		},
	}

	headers := http.Header{}
	headers.Set("X-MYCUSTOM-HEADER", "val1")
	exprcc := &remote.HTTPConfig{
		Timeout: 5 * time.Second,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
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

func TestConfigConsolidation(t *testing.T) {
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
		"default": {
			jsonRaw: nil,
			env:     nil,
			arg:     "",
			config: Config{
				URL:                   null.StringFrom("http://localhost:9090/api/v1/write"),
				InsecureSkipTLSVerify: null.BoolFrom(true),
				Username:              null.NewString("", false),
				Password:              null.NewString("", false),
				PushInterval:          types.NullDurationFrom(5 * time.Second),
				Headers:               make(map[string]string),
			},
		},
		"json_success": {
			jsonRaw: json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, u.String())),
			env:     nil,
			arg:     "",
			config: Config{
				URL:                   null.StringFrom(u.String()),
				InsecureSkipTLSVerify: null.BoolFrom(true),
				Username:              null.NewString("", false),
				Password:              null.NewString("", false),
				PushInterval:          types.NullDurationFrom(defaultPushInterval),
				Headers:               make(map[string]string),
			},
		},
		"mixed_success": {
			jsonRaw: json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, u.String())),
			env:     map[string]string{"K6_PROMETHEUS_INSECURE_SKIP_TLS_VERIFY": "false", "K6_PROMETHEUS_USER": "u"},
			arg:     "username=user",
			config: Config{
				URL:                   null.StringFrom(u.String()),
				InsecureSkipTLSVerify: null.BoolFrom(false),
				Username:              null.NewString("user", true),
				Password:              null.NewString("", false),
				PushInterval:          types.NullDurationFrom(defaultPushInterval),
				Headers:               make(map[string]string),
			},
			errString: "",
		},
		"invalid_duration": {
			jsonRaw:   json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, u.String())),
			env:       map[string]string{"K6_PROMETHEUS_PUSH_INTERVAL": "d"},
			arg:       "",
			config:    Config{},
			errString: "strconv.ParseInt",
		},
		"invalid_insecureSkipTLSVerify": {
			jsonRaw:   json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, u.String())),
			env:       map[string]string{"K6_PROMETHEUS_INSECURE_SKIP_TLS_VERIFY": "d"},
			arg:       "",
			config:    Config{},
			errString: "strconv.ParseBool",
		},
		"remote_write_with_headers_json": {
			jsonRaw: json.RawMessage(fmt.Sprintf(`{"url":"%s","mapping":"mapping", "headers":{"X-Header":"value"}}`, u.String())),
			env:     nil,
			arg:     "",
			config: Config{
				URL:                   null.StringFrom(u.String()),
				InsecureSkipTLSVerify: null.BoolFrom(true),
				Username:              null.NewString("", false),
				Password:              null.NewString("", false),
				PushInterval:          types.NullDurationFrom(defaultPushInterval),
				Headers: map[string]string{
					"X-Header": "value",
				},
			},
			errString: "",
		},
		"remote_write_with_headers_env": {
			jsonRaw: json.RawMessage(fmt.Sprintf(`{"url":"%s","mapping":"mapping", "headers":{"X-Header":"value"}}`, u.String())),
			env: map[string]string{
				"K6_PROMETHEUS_HEADERS_X-Header": "value_from_env",
			},
			arg: "",
			config: Config{
				URL:                   null.StringFrom(u.String()),
				InsecureSkipTLSVerify: null.BoolFrom(true),
				Username:              null.NewString("", false),
				Password:              null.NewString("", false),
				PushInterval:          types.NullDurationFrom(defaultPushInterval),
				Headers: map[string]string{
					"X-Header": "value_from_env",
				},
			},
			errString: "",
		},
		"remote_write_with_headers_arg": {
			jsonRaw: json.RawMessage(fmt.Sprintf(`{"url":"%s","mapping":"mapping", "headers":{"X-Header":"value"}}`, u.String())),
			env: map[string]string{
				"K6_PROMETHEUS_HEADERS_X-Header": "value_from_env",
			},
			arg: "headers.X-Header=value_from_arg",
			config: Config{
				URL:                   null.StringFrom(u.String()),
				InsecureSkipTLSVerify: null.BoolFrom(true),
				Username:              null.NewString("", false),
				Password:              null.NewString("", false),
				PushInterval:          types.NullDurationFrom(defaultPushInterval),
				Headers: map[string]string{
					"X-Header": "value_from_arg",
				},
			},
			errString: "",
		},
	}

	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			c, err := GetConsolidatedConfig(testCase.jsonRaw, testCase.env, testCase.arg)
			if len(testCase.errString) > 0 {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), testCase.errString)
				return
			}
			assert.Equal(t, c, testCase.config)
		})
	}
}

func TestConfigBasicAuth(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(`{"username":"user1","password":"pass1"}`)},
		"Env":  {env: map[string]string{"K6_PROMETHEUS_USERNAME": "user1", "K6_PROMETHEUS_PASSWORD": "pass1"}},
		"Arg":  {arg: "username=user1,password=pass1"},
	}

	expconfig := Config{
		URL:                   null.StringFrom("http://localhost:9090/api/v1/write"),
		InsecureSkipTLSVerify: null.BoolFrom(true),
		Username:              null.StringFrom("user1"),
		Password:              null.StringFrom("pass1"),
		PushInterval:          types.NullDurationFrom(5 * time.Second),
		Headers:               make(map[string]string),
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}

func TestConfigTrendAsNativeHistogram(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arg     string
		env     map[string]string
		jsonRaw json.RawMessage
	}{
		"JSON": {jsonRaw: json.RawMessage(`{"trendAsNativeHistogram":true}`)},
		"Env":  {env: map[string]string{"K6_PROMETHEUS_TREND_AS_NATIVE_HISTOGRAM": "true"}},
		"Arg":  {arg: "trendAsNativeHistogram=true"},
	}

	expconfig := Config{
		URL:                    null.StringFrom("http://localhost:9090/api/v1/write"),
		InsecureSkipTLSVerify:  null.BoolFrom(true),
		Username:               null.NewString("", false),
		Password:               null.NewString("", false),
		PushInterval:           types.NullDurationFrom(5 * time.Second),
		Headers:                make(map[string]string),
		TrendAsNativeHistogram: null.BoolFrom(true),
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			c, err := GetConsolidatedConfig(
				tc.jsonRaw, tc.env, tc.arg)
			require.NoError(t, err)
			assert.Equal(t, expconfig, c)
		})
	}
}
