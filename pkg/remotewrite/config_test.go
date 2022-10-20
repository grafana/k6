package remotewrite

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

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

	c, err = ParseArg("url=https://prometheus.remote:3412/write,insecureSkipTLSVerify=false,user=user,password=pass")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("https://prometheus.remote:3412/write"), c.URL)
	assert.Equal(t, null.BoolFrom(false), c.InsecureSkipTLSVerify)
	assert.Equal(t, null.StringFrom("user"), c.Username)
	assert.Equal(t, null.StringFrom("pass"), c.Password)

	c, err = ParseArg("url=http://prometheus.remote:3412/write,pushInterval=2s")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("http://prometheus.remote:3412/write"), c.URL)
	assert.Equal(t, types.NullDurationFrom(time.Second*2), c.PushInterval)

	c, err = ParseArg("url=http://prometheus.remote:3412/write,headers.X-Header=value")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("http://prometheus.remote:3412/write"), c.URL)
	assert.Equal(t, map[string]string{"X-Header": "value"}, c.Headers)
}

func TestConstructRemoteConfig(t *testing.T) {
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

	exprcc := &remote.ClientConfig{
		URL:     &promConfig.URL{URL: u},
		Timeout: model.Duration(time.Minute),
		HTTPClientConfig: promConfig.HTTPClientConfig{
			FollowRedirects: true,
			TLSConfig: promConfig.TLSConfig{
				InsecureSkipVerify: true,
			},
			BasicAuth: &promConfig.BasicAuth{
				Username: "myuser",
				Password: "mypass",
			},
		},
		RetryOnRateLimit: true,
		Headers: map[string]string{
			"X-MYCUSTOM-HEADER": "val1",
		},
	}
	rcc, err := config.ConstructRemoteConfig()
	require.NoError(t, err)
	assert.Equal(t, exprcc, rcc)
}

func TestConifgConsolidation(t *testing.T) {
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
			jsonRaw: json.RawMessage(fmt.Sprintf(`{"url":"%s","mapping":"raw"}`, u.String())),
			env:     map[string]string{"K6_PROMETHEUS_INSECURE_SKIP_TLS_VERIFY": "false", "K6_PROMETHEUS_USER": "u"},
			arg:     "user=user",
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
