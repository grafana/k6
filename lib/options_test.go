/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package lib

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
)

func TestOptions(t *testing.T) {
	t.Run("Paused", func(t *testing.T) {
		opts := Options{}.Apply(Options{Paused: null.BoolFrom(true)})
		assert.True(t, opts.Paused.Valid)
		assert.True(t, opts.Paused.Bool)
	})
	t.Run("VUs", func(t *testing.T) {
		opts := Options{}.Apply(Options{VUs: null.IntFrom(12345)})
		assert.True(t, opts.VUs.Valid)
		assert.Equal(t, int64(12345), opts.VUs.Int64)
	})
	t.Run("Duration", func(t *testing.T) {
		opts := Options{}.Apply(Options{Duration: types.NullDurationFrom(2 * time.Minute)})
		assert.True(t, opts.Duration.Valid)
		assert.Equal(t, "2m0s", opts.Duration.String())
	})
	t.Run("Iterations", func(t *testing.T) {
		opts := Options{}.Apply(Options{Iterations: null.IntFrom(1234)})
		assert.True(t, opts.Iterations.Valid)
		assert.Equal(t, int64(1234), opts.Iterations.Int64)
	})
	t.Run("Stages", func(t *testing.T) {
		opts := Options{}.Apply(Options{Stages: []Stage{
			{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(10)},
			{Duration: types.NullDurationFrom(2 * time.Second), Target: null.IntFrom(20)},
		}})
		assert.NotNil(t, opts.Stages)
		assert.Len(t, opts.Stages, 2)
		assert.Equal(t, 1*time.Second, time.Duration(opts.Stages[0].Duration.Duration))
		assert.Equal(t, int64(10), opts.Stages[0].Target.Int64)
		assert.Equal(t, 2*time.Second, time.Duration(opts.Stages[1].Duration.Duration))
		assert.Equal(t, int64(20), opts.Stages[1].Target.Int64)

		emptyStages := []Stage{}
		assert.Equal(t, emptyStages, Options{}.Apply(Options{Stages: []Stage{{}}}).Stages)
		assert.Equal(t, emptyStages, Options{}.Apply(Options{Stages: []Stage{}}).Stages)
		assert.Equal(t, emptyStages, opts.Apply(Options{Stages: []Stage{}}).Stages)
		assert.Equal(t, emptyStages, opts.Apply(Options{Stages: []Stage{{}}}).Stages)

		assert.Equal(t, opts.Stages, opts.Apply(opts).Stages)

		oneStage := []Stage{{Duration: types.NullDurationFrom(5 * time.Second), Target: null.IntFrom(50)}}
		assert.Equal(t, oneStage, opts.Apply(Options{Stages: oneStage}).Stages)
		assert.Equal(t, oneStage, Options{}.Apply(opts).Apply(Options{Stages: oneStage}).Apply(Options{Stages: oneStage}).Stages)
	})
	// Execution overwriting is tested by the config consolidation test in cmd
	t.Run("RPS", func(t *testing.T) {
		opts := Options{}.Apply(Options{RPS: null.IntFrom(12345)})
		assert.True(t, opts.RPS.Valid)
		assert.Equal(t, int64(12345), opts.RPS.Int64)
	})
	t.Run("MaxRedirects", func(t *testing.T) {
		opts := Options{}.Apply(Options{MaxRedirects: null.IntFrom(12345)})
		assert.True(t, opts.MaxRedirects.Valid)
		assert.Equal(t, int64(12345), opts.MaxRedirects.Int64)
	})
	t.Run("UserAgent", func(t *testing.T) {
		opts := Options{}.Apply(Options{UserAgent: null.StringFrom("foo")})
		assert.True(t, opts.UserAgent.Valid)
		assert.Equal(t, "foo", opts.UserAgent.String)
	})
	t.Run("Batch", func(t *testing.T) {
		opts := Options{}.Apply(Options{Batch: null.IntFrom(12345)})
		assert.True(t, opts.Batch.Valid)
		assert.Equal(t, int64(12345), opts.Batch.Int64)
	})
	t.Run("BatchPerHost", func(t *testing.T) {
		opts := Options{}.Apply(Options{BatchPerHost: null.IntFrom(12345)})
		assert.True(t, opts.BatchPerHost.Valid)
		assert.Equal(t, int64(12345), opts.BatchPerHost.Int64)
	})
	t.Run("HTTPDebug", func(t *testing.T) {
		opts := Options{}.Apply(Options{HTTPDebug: null.StringFrom("foo")})
		assert.True(t, opts.HTTPDebug.Valid)
		assert.Equal(t, "foo", opts.HTTPDebug.String)
	})
	t.Run("InsecureSkipTLSVerify", func(t *testing.T) {
		opts := Options{}.Apply(Options{InsecureSkipTLSVerify: null.BoolFrom(true)})
		assert.True(t, opts.InsecureSkipTLSVerify.Valid)
		assert.True(t, opts.InsecureSkipTLSVerify.Bool)
	})
	t.Run("TLSCipherSuites", func(t *testing.T) {
		for suiteName, suiteID := range SupportedTLSCipherSuites {
			t.Run(suiteName, func(t *testing.T) {
				opts := Options{}.Apply(Options{TLSCipherSuites: &TLSCipherSuites{suiteID}})

				assert.NotNil(t, opts.TLSCipherSuites)
				assert.Len(t, *(opts.TLSCipherSuites), 1)
				assert.Equal(t, suiteID, (*opts.TLSCipherSuites)[0])
			})
		}

		t.Run("JSON", func(t *testing.T) {
			t.Run("String", func(t *testing.T) {
				var opts Options
				jsonStr := `{"tlsCipherSuites":["TLS_ECDHE_RSA_WITH_RC4_128_SHA"]}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, &TLSCipherSuites{tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA}, opts.TLSCipherSuites)

				t.Run("Roundtrip", func(t *testing.T) {
					data, err := json.Marshal(opts.TLSCipherSuites)
					assert.NoError(t, err)
					assert.Equal(t, `["TLS_ECDHE_RSA_WITH_RC4_128_SHA"]`, string(data))
					var vers2 TLSCipherSuites
					assert.NoError(t, json.Unmarshal(data, &vers2))
					assert.Equal(t, &vers2, opts.TLSCipherSuites)
				})
			})
			t.Run("Not a string", func(t *testing.T) {
				var opts Options
				jsonStr := `{"tlsCipherSuites":[1.2]}`
				assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
			})
			t.Run("Unknown cipher", func(t *testing.T) {
				var opts Options
				jsonStr := `{"tlsCipherSuites":["foo"]}`
				assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
			})
		})
	})
	t.Run("TLSVersion", func(t *testing.T) {
		versions := TLSVersions{Min: tls.VersionSSL30, Max: tls.VersionTLS12}
		opts := Options{}.Apply(Options{TLSVersion: &versions})

		assert.NotNil(t, opts.TLSVersion)
		assert.Equal(t, opts.TLSVersion.Min, TLSVersion(tls.VersionSSL30))
		assert.Equal(t, opts.TLSVersion.Max, TLSVersion(tls.VersionTLS12))

		t.Run("JSON", func(t *testing.T) {
			t.Run("Object", func(t *testing.T) {
				var opts Options
				jsonStr := `{"tlsVersion":{"min":"ssl3.0","max":"tls1.2"}}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, &TLSVersions{
					Min: TLSVersion(tls.VersionSSL30),
					Max: TLSVersion(tls.VersionTLS12),
				}, opts.TLSVersion)

				t.Run("Roundtrip", func(t *testing.T) {
					data, err := json.Marshal(opts.TLSVersion)
					assert.NoError(t, err)
					assert.Equal(t, `{"min":"ssl3.0","max":"tls1.2"}`, string(data))
					var vers2 TLSVersions
					assert.NoError(t, json.Unmarshal(data, &vers2))
					assert.Equal(t, &vers2, opts.TLSVersion)
				})
			})
			t.Run("String", func(t *testing.T) {
				var opts Options
				jsonStr := `{"tlsVersion":"tls1.2"}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, &TLSVersions{
					Min: TLSVersion(tls.VersionTLS12),
					Max: TLSVersion(tls.VersionTLS12),
				}, opts.TLSVersion)
			})
			t.Run("Blank", func(t *testing.T) {
				var opts Options
				jsonStr := `{"tlsVersion":""}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, &TLSVersions{}, opts.TLSVersion)
			})
			t.Run("Not a string", func(t *testing.T) {
				var opts Options
				jsonStr := `{"tlsVersion":1.2}`
				assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
			})
			t.Run("Unsupported version", func(t *testing.T) {
				var opts Options
				jsonStr := `{"tlsVersion":"-1"}`
				assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
			})
		})
	})
	t.Run("TLSAuth", func(t *testing.T) {
		tlsAuth := []*TLSAuth{
			{TLSAuthFields{
				Domains: []string{"example.com", "*.example.com"},
				Cert: "-----BEGIN CERTIFICATE-----\n" +
					"MIIBoTCCAUegAwIBAgIUQl0J1Gkd6U2NIMwMDnpfH8c1myEwCgYIKoZIzj0EAwIw\n" +
					"EDEOMAwGA1UEAxMFTXkgQ0EwHhcNMTcwODE1MTYxODAwWhcNMTgwODE1MTYxODAw\n" +
					"WjAQMQ4wDAYDVQQDEwV1c2VyMTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABLaf\n" +
					"xEOmBHkzbqd9/0VZX/39qO2yQq2Gz5faRdvy38kuLMCV+9HYrfMx6GYCZzTUIq6h\n" +
					"8QXOrlgYTixuUVfhJNWjfzB9MA4GA1UdDwEB/wQEAwIFoDAdBgNVHSUEFjAUBggr\n" +
					"BgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAdBgNVHQ4EFgQUxmQiq5K3\n" +
					"KUnVME945Byt3Ysvkh8wHwYDVR0jBBgwFoAU3qEhcpRgpsqo9V+LFns9a+oZIYww\n" +
					"CgYIKoZIzj0EAwIDSAAwRQIgSGxnJ+/cLUNTzt7fhr/mjJn7ShsTW33dAdfLM7H2\n" +
					"z/gCIQDyVf8DePtxlkMBScTxZmIlMQdNc6+6VGZQ4QscruVLmg==\n" +
					"-----END CERTIFICATE-----",
				Key: "-----BEGIN EC PRIVATE KEY-----\n" +
					"MHcCAQEEIAfJeoc+XgcqmYV0b4owmofx0LXwPRqOPXMO+PUKxZSgoAoGCCqGSM49\n" +
					"AwEHoUQDQgAEtp/EQ6YEeTNup33/RVlf/f2o7bJCrYbPl9pF2/LfyS4swJX70dit\n" +
					"8zHoZgJnNNQirqHxBc6uWBhOLG5RV+Ek1Q==\n" +
					"-----END EC PRIVATE KEY-----",
			}, nil},
			{TLSAuthFields{
				Domains: []string{"sub.example.com"},
				Cert: "-----BEGIN CERTIFICATE-----\n" +
					"MIIBojCCAUegAwIBAgIUWMpVQhmGoLUDd2x6XQYoOOV6C9AwCgYIKoZIzj0EAwIw\n" +
					"EDEOMAwGA1UEAxMFTXkgQ0EwHhcNMTcwODE1MTYxODAwWhcNMTgwODE1MTYxODAw\n" +
					"WjAQMQ4wDAYDVQQDEwV1c2VyMTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABBfF\n" +
					"85gu8fDbNGNlsrtnO+4HvuiP4IXA041jjGczD5kUQ8aihS7hg81tSrLNd1jgxkkv\n" +
					"Po+3TQjzniysiunG3iKjfzB9MA4GA1UdDwEB/wQEAwIFoDAdBgNVHSUEFjAUBggr\n" +
					"BgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAdBgNVHQ4EFgQUU0JfPCQb\n" +
					"2YpQZV4j1yiRXBa7J64wHwYDVR0jBBgwFoAU3qEhcpRgpsqo9V+LFns9a+oZIYww\n" +
					"CgYIKoZIzj0EAwIDSQAwRgIhANYDaM18sXAdkjybHccH8xTbBWUNpOYvoHhrGW32\n" +
					"Ov9JAiEA7QKGpm07tQl8p+t7UsOgZu132dHNZUtfgp1bjWfcapU=\n" +
					"-----END CERTIFICATE-----",
				Key: "-----BEGIN EC PRIVATE KEY-----\n" +
					"MHcCAQEEINVilD5qOBkSy+AYfd41X0QPB5N3Z6OzgoBj8FZmSJOFoAoGCCqGSM49\n" +
					"AwEHoUQDQgAEF8XzmC7x8Ns0Y2Wyu2c77ge+6I/ghcDTjWOMZzMPmRRDxqKFLuGD\n" +
					"zW1Kss13WODGSS8+j7dNCPOeLKyK6cbeIg==\n" +
					"-----END EC PRIVATE KEY-----",
			}, nil},
		}
		opts := Options{}.Apply(Options{TLSAuth: tlsAuth})
		assert.Equal(t, tlsAuth, opts.TLSAuth)

		t.Run("Roundtrip", func(t *testing.T) {
			optsData, err := json.Marshal(opts)
			assert.NoError(t, err)

			var opts2 Options
			assert.NoError(t, json.Unmarshal(optsData, &opts2))
			if assert.Len(t, opts2.TLSAuth, len(opts.TLSAuth)) {
				for i := 0; i < len(opts2.TLSAuth); i++ {
					assert.Equal(t, opts.TLSAuth[i].TLSAuthFields, opts2.TLSAuth[i].TLSAuthFields)
					cert, err := opts2.TLSAuth[i].Certificate()
					assert.NoError(t, err)
					assert.NotNil(t, cert)
				}
			}
		})

		t.Run("Invalid JSON", func(t *testing.T) {
			var opts Options
			jsonStr := `{"tlsAuth":["invalid"]}`
			assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
		})

		t.Run("Certificate error", func(t *testing.T) {
			var opts Options
			jsonStr := `{"tlsAuth":[{"Cert":""}]}`
			assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
		})
	})
	t.Run("NoConnectionReuse", func(t *testing.T) {
		opts := Options{}.Apply(Options{NoConnectionReuse: null.BoolFrom(true)})
		assert.True(t, opts.NoConnectionReuse.Valid)
		assert.True(t, opts.NoConnectionReuse.Bool)
	})
	t.Run("NoVUConnectionReuse", func(t *testing.T) {
		opts := Options{}.Apply(Options{NoVUConnectionReuse: null.BoolFrom(true)})
		assert.True(t, opts.NoVUConnectionReuse.Valid)
		assert.True(t, opts.NoVUConnectionReuse.Bool)
	})
	t.Run("NoCookiesReset", func(t *testing.T) {
		opts := Options{}.Apply(Options{NoCookiesReset: null.BoolFrom(true)})
		assert.True(t, opts.NoCookiesReset.Valid)
		assert.True(t, opts.NoCookiesReset.Bool)
	})
	t.Run("BlacklistIPs", func(t *testing.T) {
		opts := Options{}.Apply(Options{
			BlacklistIPs: []*IPNet{{
				IPNet: net.IPNet{
					IP:   net.IPv4zero,
					Mask: net.CIDRMask(1, 1),
				},
			}},
		})
		assert.NotNil(t, opts.BlacklistIPs)
		assert.NotEmpty(t, opts.BlacklistIPs)
		assert.Equal(t, net.IPv4zero, opts.BlacklistIPs[0].IP)
		assert.Equal(t, net.CIDRMask(1, 1), opts.BlacklistIPs[0].Mask)
	})
	t.Run("BlockedHostnames", func(t *testing.T) {
		blockedHostnames, err := types.NewNullHostnameTrie([]string{"test.k6.io", "*valid.pattern"})
		require.NoError(t, err)
		opts := Options{}.Apply(Options{BlockedHostnames: blockedHostnames})
		assert.NotNil(t, opts.BlockedHostnames)
		assert.Equal(t, blockedHostnames, opts.BlockedHostnames)
	})

	t.Run("Hosts", func(t *testing.T) {
		host, err := NewHostAddress(net.ParseIP("192.0.2.1"), "80")
		assert.NoError(t, err)

		opts := Options{}.Apply(Options{Hosts: map[string]*HostAddress{
			"test.loadimpact.com": host,
		}})
		assert.NotNil(t, opts.Hosts)
		assert.NotEmpty(t, opts.Hosts)
		assert.Equal(t, "192.0.2.1:80", opts.Hosts["test.loadimpact.com"].String())
	})

	t.Run("Throws", func(t *testing.T) {
		opts := Options{}.Apply(Options{Throw: null.BoolFrom(true)})
		assert.True(t, opts.Throw.Valid)
		assert.Equal(t, true, opts.Throw.Bool)
	})

	t.Run("Thresholds", func(t *testing.T) {
		opts := Options{}.Apply(Options{Thresholds: map[string]stats.Thresholds{
			"metric": {
				Thresholds: []*stats.Threshold{{}},
			},
		}})
		assert.NotNil(t, opts.Thresholds)
		assert.NotEmpty(t, opts.Thresholds)
	})
	t.Run("External", func(t *testing.T) {
		ext := map[string]json.RawMessage{"a": json.RawMessage("1")}
		opts := Options{}.Apply(Options{External: ext})
		assert.Equal(t, ext, opts.External)
	})

	t.Run("JSON", func(t *testing.T) {
		data, err := json.Marshal(Options{})
		assert.NoError(t, err)
		var opts Options
		assert.NoError(t, json.Unmarshal(data, &opts))
		assert.Equal(t, Options{}, opts)
	})
	t.Run("SystemTags", func(t *testing.T) {
		opts := Options{}.Apply(Options{SystemTags: stats.NewSystemTagSet(stats.TagProto)})
		assert.NotNil(t, opts.SystemTags)
		assert.NotEmpty(t, opts.SystemTags)
		assert.True(t, opts.SystemTags.Has(stats.TagProto))

		t.Run("JSON", func(t *testing.T) {
			t.Run("Array", func(t *testing.T) {
				var opts Options
				jsonStr := `{"systemTags":["url"]}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, *stats.NewSystemTagSet(stats.TagURL), *opts.SystemTags)

				t.Run("Roundtrip", func(t *testing.T) {
					data, err := json.Marshal(opts.SystemTags)
					assert.NoError(t, err)
					assert.Equal(t, `["url"]`, string(data))
					var vers2 stats.SystemTagSet
					assert.NoError(t, json.Unmarshal(data, &vers2))
					assert.Equal(t, vers2, *opts.SystemTags)
				})
			})
			t.Run("Blank", func(t *testing.T) {
				var opts Options
				jsonStr := `{"systemTags":[]}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, stats.SystemTagSet(0), *opts.SystemTags)
			})
		})
	})
	t.Run("SummaryTrendStats", func(t *testing.T) {
		stats := []string{"myStat1", "myStat2"}
		opts := Options{}.Apply(Options{SummaryTrendStats: stats})
		assert.Equal(t, stats, opts.SummaryTrendStats)
	})
	t.Run("RunTags", func(t *testing.T) {
		tags := stats.IntoSampleTags(&map[string]string{"myTag": "hello"})
		opts := Options{}.Apply(Options{RunTags: tags})
		assert.Equal(t, tags, opts.RunTags)
	})
	t.Run("DiscardResponseBodies", func(t *testing.T) {
		opts := Options{}.Apply(Options{DiscardResponseBodies: null.BoolFrom(true)})
		assert.True(t, opts.DiscardResponseBodies.Valid)
		assert.True(t, opts.DiscardResponseBodies.Bool)
	})
	t.Run("ClientIPRanges", func(t *testing.T) {
		clientIPRanges, err := types.NewIPPool("129.112.232.12,123.12.0.0/32")
		require.NoError(t, err)
		opts := Options{}.Apply(Options{LocalIPs: types.NullIPPool{Pool: clientIPRanges, Valid: true}})
		assert.NotNil(t, opts.LocalIPs)
	})
}

func TestOptionsEnv(t *testing.T) {
	mustIPPool := func(s string) *types.IPPool {
		p, err := types.NewIPPool(s)
		require.NoError(t, err)
		return p
	}

	testdata := map[struct{ Name, Key string }]map[string]interface{}{
		{"Paused", "K6_PAUSED"}: {
			"":      null.Bool{},
			"true":  null.BoolFrom(true),
			"false": null.BoolFrom(false),
		},
		{"VUs", "K6_VUS"}: {
			"":    null.Int{},
			"123": null.IntFrom(123),
		},
		{"Duration", "K6_DURATION"}: {
			"":    types.NullDuration{},
			"10s": types.NullDurationFrom(10 * time.Second),
		},
		{"Iterations", "K6_ITERATIONS"}: {
			"":    null.Int{},
			"123": null.IntFrom(123),
		},
		{"Stages", "K6_STAGES"}: {
			// "": []Stage{},
			"1s": []Stage{
				{
					Duration: types.NullDurationFrom(1 * time.Second),
				},
			},
			"1s:100": []Stage{
				{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(100)},
			},
			"1s,2s:100": []Stage{
				{Duration: types.NullDurationFrom(1 * time.Second)},
				{Duration: types.NullDurationFrom(2 * time.Second), Target: null.IntFrom(100)},
			},
		},
		{"MaxRedirects", "K6_MAX_REDIRECTS"}: {
			"":    null.Int{},
			"123": null.IntFrom(123),
		},
		{"InsecureSkipTLSVerify", "K6_INSECURE_SKIP_TLS_VERIFY"}: {
			"":      null.Bool{},
			"true":  null.BoolFrom(true),
			"false": null.BoolFrom(false),
		},
		// TLSCipherSuites
		// TLSVersion
		// TLSAuth
		{"NoConnectionReuse", "K6_NO_CONNECTION_REUSE"}: {
			"":      null.Bool{},
			"true":  null.BoolFrom(true),
			"false": null.BoolFrom(false),
		},
		{"NoVUConnectionReuse", "K6_NO_VU_CONNECTION_REUSE"}: {
			"":      null.Bool{},
			"true":  null.BoolFrom(true),
			"false": null.BoolFrom(false),
		},
		{"UserAgent", "K6_USER_AGENT"}: {
			"":    null.String{},
			"Hi!": null.StringFrom("Hi!"),
		},
		{"LocalIPs", "K6_LOCAL_IPS"}: {
			"":                 types.NullIPPool{},
			"192.168.220.2":    types.NullIPPool{Pool: mustIPPool("192.168.220.2"), Valid: true},
			"192.168.220.2/24": types.NullIPPool{Pool: mustIPPool("192.168.220.0/24"), Valid: true},
		},
		{"Throw", "K6_THROW"}: {
			"":      null.Bool{},
			"true":  null.BoolFrom(true),
			"false": null.BoolFrom(false),
		},
		{"NoCookiesReset", "K6_NO_COOKIES_RESET"}: {
			"":      null.Bool{},
			"true":  null.BoolFrom(true),
			"false": null.BoolFrom(false),
		},
		// Thresholds
		// External
	}
	for field, data := range testdata {
		field, data := field, data
		t.Run(field.Name, func(t *testing.T) {
			for str, val := range data {
				str, val := str, val
				t.Run(`"`+str+`"`, func(t *testing.T) {
					restore := testutils.SetEnv(t, []string{fmt.Sprintf("%s=%s", field.Key, str)})
					defer restore()
					var opts Options
					assert.NoError(t, envconfig.Process("k6", &opts))
					assert.Equal(t, val, reflect.ValueOf(opts).FieldByName(field.Name).Interface())
				})
			}
		})
	}
}

func TestCIDRUnmarshal(t *testing.T) {
	testData := []struct {
		input          string
		expectedOutput *IPNet
		expectFailure  bool
	}{
		{
			"10.0.0.0/8",
			&IPNet{IPNet: net.IPNet{
				IP:   net.IP{10, 0, 0, 0},
				Mask: net.IPv4Mask(255, 0, 0, 0),
			}},
			false,
		},
		{
			"fc00:1234:5678::/48",
			&IPNet{IPNet: net.IPNet{
				IP:   net.ParseIP("fc00:1234:5678::"),
				Mask: net.CIDRMask(48, 128),
			}},
			false,
		},
		{"10.0.0.0", nil, true},
		{"fc00:1234:5678::", nil, true},
		{"fc00::1234::/48", nil, true},
	}

	for _, data := range testData {
		data := data
		t.Run(data.input, func(t *testing.T) {
			actualIPNet := &IPNet{}
			err := actualIPNet.UnmarshalText([]byte(data.input))

			if data.expectFailure {
				require.EqualError(t, err, "Failed to parse CIDR: invalid CIDR address: "+data.input)
			} else {
				require.NoError(t, err)
				assert.Equal(t, data.expectedOutput, actualIPNet)
			}
		})
	}
}

func TestHostAddressUnmarshal(t *testing.T) {
	testData := []struct {
		input          string
		expectedOutput *HostAddress
		expectFailure  string
	}{
		{
			"1.2.3.4",
			&HostAddress{IP: net.ParseIP("1.2.3.4")},
			"",
		},
		{
			"1.2.3.4:80",
			&HostAddress{IP: net.ParseIP("1.2.3.4"), Port: 80},
			"",
		},
		{
			"1.2.3.4:asdf",
			nil,
			"strconv.Atoi: parsing \"asdf\": invalid syntax",
		},
		{
			"2001:0db8:0000:0000:0000:ff00:0042:8329",
			&HostAddress{IP: net.ParseIP("2001:0db8:0000:0000:0000:ff00:0042:8329")},
			"",
		},
		{
			"2001:db8::68",
			&HostAddress{IP: net.ParseIP("2001:db8::68")},
			"",
		},
		{
			"[2001:db8::68]:80",
			&HostAddress{IP: net.ParseIP("2001:db8::68"), Port: 80},
			"",
		},
		{
			"[2001:db8::68]:asdf",
			nil,
			"strconv.Atoi: parsing \"asdf\": invalid syntax",
		},
	}

	for _, data := range testData {
		data := data
		t.Run(data.input, func(t *testing.T) {
			actualHost := &HostAddress{}
			err := actualHost.UnmarshalText([]byte(data.input))

			if data.expectFailure != "" {
				require.EqualError(t, err, data.expectFailure)
			} else {
				require.NoError(t, err)
				assert.Equal(t, data.expectedOutput, actualHost)
			}
		})
	}
}
