package influxdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

func TestParseURL(t *testing.T) {
	t.Parallel()
	testdata := map[string]Config{
		"":                             {},
		"dbname":                       {DB: null.StringFrom("dbname")},
		"/dbname":                      {DB: null.StringFrom("dbname")},
		"http://localhost:8086":        {Addr: null.StringFrom("http://localhost:8086")},
		"http://localhost:8086/dbname": {Addr: null.StringFrom("http://localhost:8086"), DB: null.StringFrom("dbname")},
	}
	queries := map[string]struct {
		Config Config
		Err    string
	}{
		"?":                {Config{}, ""},
		"?insecure=false":  {Config{Insecure: null.BoolFrom(false)}, ""},
		"?insecure=true":   {Config{Insecure: null.BoolFrom(true)}, ""},
		"?insecure=ture":   {Config{}, "insecure must be true or false, not ture"},
		"?payload_size=69": {Config{PayloadSize: null.IntFrom(69)}, ""},
		"?payload_size=a":  {Config{}, "strconv.Atoi: parsing \"a\": invalid syntax"},
	}
	for str, data := range testdata {
		str, data := str, data
		t.Run(str, func(t *testing.T) {
			t.Parallel()
			config, err := ParseURL(str)
			assert.NoError(t, err)
			assert.Equal(t, data, config)

			for q, qdata := range queries {
				t.Run(q, func(t *testing.T) {
					config, err := ParseURL(str + q)
					if qdata.Err != "" {
						assert.EqualError(t, err, qdata.Err)
					} else {
						expected2 := qdata.Config
						expected2.DB = data.DB
						expected2.Addr = data.Addr
						assert.Equal(t, expected2, config)
					}
				})
			}
		})
	}
}

func TestGetConsolidatedConfigHTTPProxy(t *testing.T) {
	t.Parallel()
	t.Run("Valid Proxy URL", func(t *testing.T) {
		t.Parallel()
		testdata := map[string]string{
			"K6_INFLUXDB_PROXY": "http://localhost:3128",
		}
		config, err := GetConsolidatedConfig(nil, testdata, "")
		assert.NoError(t, err)
		assert.Equal(t, "http://localhost:3128", config.Proxy.String)
	})
	t.Run("Invalid Proxy URL", func(t *testing.T) {
		t.Parallel()
		testdata := map[string]string{
			"K6_INFLUXDB_PROXY": "http://foo\x7f.com/",
		}
		config, err := GetConsolidatedConfig(nil, testdata, "")
		assert.NoError(t, err)
		_, err = MakeClient(config)
		assert.Error(t, err)
	})
}
