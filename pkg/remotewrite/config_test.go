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
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

func TestApply(t *testing.T) {
	t.Parallel()

	fullConfig := Config{
		Url:                   null.StringFrom("some-url"),
		InsecureSkipTLSVerify: null.BoolFrom(false),
		CACert:                null.StringFrom("some-file"),
		User:                  null.StringFrom("user"),
		Password:              null.StringFrom("pass"),
		FlushPeriod:           types.NullDurationFrom(10 * time.Second),
	}

	// Defaults should be overwritten by valid values
	c := NewConfig()
	c = c.Apply(fullConfig)
	assert.Equal(t, fullConfig.Url, c.Url)
	assert.Equal(t, fullConfig.InsecureSkipTLSVerify, c.InsecureSkipTLSVerify)
	assert.Equal(t, fullConfig.CACert, c.CACert)
	assert.Equal(t, fullConfig.User, c.User)
	assert.Equal(t, fullConfig.Password, c.Password)
	assert.Equal(t, fullConfig.FlushPeriod, c.FlushPeriod)

	// Defaults shouldn't be impacted by invalid values
	c = NewConfig()
	c = c.Apply(Config{
		User:                  null.NewString("user", false),
		Password:              null.NewString("pass", false),
		InsecureSkipTLSVerify: null.NewBool(false, false),
	})
	assert.Equal(t, false, c.User.Valid)
	assert.Equal(t, false, c.Password.Valid)
	assert.Equal(t, true, c.InsecureSkipTLSVerify.Valid)
}

func TestConfigParseArg(t *testing.T) {
	t.Parallel()

	c, err := ParseArg("url=http://prometheus.remote:3412/write")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("http://prometheus.remote:3412/write"), c.Url)

	c, err = ParseArg("url=http://prometheus.remote:3412/write,insecureSkipTLSVerify=false")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("http://prometheus.remote:3412/write"), c.Url)
	assert.Equal(t, null.BoolFrom(false), c.InsecureSkipTLSVerify)

	c, err = ParseArg("url=https://prometheus.remote:3412/write,caCertFile=f.crt")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("https://prometheus.remote:3412/write"), c.Url)
	assert.Equal(t, null.StringFrom("f.crt"), c.CACert)

	c, err = ParseArg("url=https://prometheus.remote:3412/write,insecureSkipTLSVerify=false,caCertFile=f.crt,user=user,password=pass")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("https://prometheus.remote:3412/write"), c.Url)
	assert.Equal(t, null.BoolFrom(false), c.InsecureSkipTLSVerify)
	assert.Equal(t, null.StringFrom("f.crt"), c.CACert)
	assert.Equal(t, null.StringFrom("user"), c.User)
	assert.Equal(t, null.StringFrom("pass"), c.Password)

	c, err = ParseArg("url=http://prometheus.remote:3412/write,flushPeriod=2s")
	assert.Nil(t, err)
	assert.Equal(t, null.StringFrom("http://prometheus.remote:3412/write"), c.Url)
	assert.Equal(t, types.NullDurationFrom(time.Second*2), c.FlushPeriod)
}

// testing both GetConsolidatedConfig and ConstructRemoteConfig here until it's future config refactor takes shape (k6 #883)
func TestConstructRemoteConfig(t *testing.T) {
	u, _ := url.Parse("https://prometheus.ie/remote")

	t.Parallel()

	testCases := map[string]struct {
		jsonRaw      json.RawMessage
		env          map[string]string
		arg          string
		config       Config
		errString    string
		remoteConfig *remote.ClientConfig
	}{
		"json_success": {
			jsonRaw: json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, u.String())),
			env:     nil,
			arg:     "",
			config: Config{
				Mapping:               null.StringFrom("prometheus"),
				Url:                   null.StringFrom(u.String()),
				InsecureSkipTLSVerify: null.BoolFrom(true),
				CACert:                null.NewString("", false),
				User:                  null.NewString("", false),
				Password:              null.NewString("", false),
				FlushPeriod:           types.NullDurationFrom(defaultFlushPeriod),
				KeepTags:              null.BoolFrom(true),
				KeepNameTag:           null.BoolFrom(false),
			},
			errString: "",
			remoteConfig: &remote.ClientConfig{
				URL:     &promConfig.URL{URL: u},
				Timeout: model.Duration(defaultPrometheusTimeout),
				HTTPClientConfig: promConfig.HTTPClientConfig{
					FollowRedirects: true,
					TLSConfig: promConfig.TLSConfig{
						InsecureSkipVerify: true,
					},
				},
				RetryOnRateLimit: false,
			},
		},
		"mixed_success": {
			jsonRaw: json.RawMessage(fmt.Sprintf(`{"url":"%s","mapping":"raw"}`, u.String())),
			env:     map[string]string{"K6_PROMETHEUS_INSECURE_SKIP_TLS_VERIFY": "false", "K6_PROMETHEUS_USER": "u"},
			arg:     "user=user",
			config: Config{
				Mapping:               null.StringFrom("raw"),
				Url:                   null.StringFrom(u.String()),
				InsecureSkipTLSVerify: null.BoolFrom(false),
				CACert:                null.NewString("", false),
				User:                  null.NewString("user", true),
				Password:              null.NewString("", false),
				FlushPeriod:           types.NullDurationFrom(defaultFlushPeriod),
				KeepTags:              null.BoolFrom(true),
				KeepNameTag:           null.BoolFrom(false),
			},
			errString: "",
			remoteConfig: &remote.ClientConfig{
				URL:     &promConfig.URL{URL: u},
				Timeout: model.Duration(defaultPrometheusTimeout),
				HTTPClientConfig: promConfig.HTTPClientConfig{
					FollowRedirects: true,
					TLSConfig: promConfig.TLSConfig{
						InsecureSkipVerify: false,
					},
					BasicAuth: &promConfig.BasicAuth{
						Username: "user",
					},
				},
				RetryOnRateLimit: false,
			},
		},
		"invalid_duration": {
			jsonRaw:      json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, u.String())),
			env:          map[string]string{"K6_PROMETHEUS_FLUSH_PERIOD": "d"},
			arg:          "",
			config:       Config{},
			errString:    "strconv.ParseInt",
			remoteConfig: nil,
		},
		"invalid_insecureSkipTLSVerify": {
			jsonRaw:      json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, u.String())),
			env:          map[string]string{"K6_PROMETHEUS_INSECURE_SKIP_TLS_VERIFY": "d"},
			arg:          "",
			config:       Config{},
			errString:    "strconv.ParseBool",
			remoteConfig: nil,
		},
	}

	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			c, err := GetConsolidatedConfig(testCase.jsonRaw, testCase.env, testCase.arg)
			if len(testCase.errString) > 0 {
				assert.Contains(t, err.Error(), testCase.errString)
				return
			}
			assertConfig(t, c, testCase.config)

			// there can be error only on url.Parse at the moment so skipping that
			remoteConfig, _ := c.ConstructRemoteConfig()
			assertRemoteConfig(t, remoteConfig, testCase.remoteConfig)
		})
	}
}

func assertConfig(t *testing.T, actual, expected Config) {
	assert.Equal(t, expected.Mapping, actual.Mapping)
	assert.Equal(t, expected.Url, actual.Url)
	assert.Equal(t, expected.InsecureSkipTLSVerify, actual.InsecureSkipTLSVerify)
	assert.Equal(t, expected.CACert, actual.CACert)
	assert.Equal(t, expected.User, actual.User)
	assert.Equal(t, expected.Password, actual.Password)
	assert.Equal(t, expected.FlushPeriod, actual.FlushPeriod)
	assert.Equal(t, expected.KeepTags, actual.KeepTags)
	assert.Equal(t, expected.KeepNameTag, expected.KeepNameTag)
}

func assertRemoteConfig(t *testing.T, actual, expected *remote.ClientConfig) {
	assert.Equal(t, expected.URL, actual.URL)
	assert.Equal(t, expected.Timeout, actual.Timeout)
	assert.Equal(t, expected.HTTPClientConfig.TLSConfig.CAFile, actual.HTTPClientConfig.TLSConfig.CAFile)
	if expected.HTTPClientConfig.BasicAuth == nil {
		assert.Nil(t, actual.HTTPClientConfig.BasicAuth)
	} else {
		assert.Equal(t, expected.HTTPClientConfig.BasicAuth.Username, actual.HTTPClientConfig.BasicAuth.Username)
		assert.Equal(t, expected.HTTPClientConfig.BasicAuth.Password, actual.HTTPClientConfig.BasicAuth.Password)
	}
}
