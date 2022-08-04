package lib

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/mstoykov/envconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

func TestOptions(t *testing.T) {
	t.Parallel()
	t.Run("Paused", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{Paused: null.BoolFrom(true)})
		assert.True(t, opts.Paused.Valid)
		assert.True(t, opts.Paused.Bool)
	})
	t.Run("VUs", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{VUs: null.IntFrom(12345)})
		assert.True(t, opts.VUs.Valid)
		assert.Equal(t, int64(12345), opts.VUs.Int64)
	})
	t.Run("Duration", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{Duration: types.NullDurationFrom(2 * time.Minute)})
		assert.True(t, opts.Duration.Valid)
		assert.Equal(t, "2m0s", opts.Duration.String())
	})
	t.Run("Iterations", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{Iterations: null.IntFrom(1234)})
		assert.True(t, opts.Iterations.Valid)
		assert.Equal(t, int64(1234), opts.Iterations.Int64)
	})
	t.Run("Stages", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{Stages: []Stage{
			{Duration: types.NullDurationFrom(1 * time.Second), Target: null.IntFrom(10)},
			{Duration: types.NullDurationFrom(2 * time.Second), Target: null.IntFrom(20)},
		}})
		assert.NotNil(t, opts.Stages)
		assert.Len(t, opts.Stages, 2)
		assert.Equal(t, 1*time.Second, opts.Stages[0].Duration.TimeDuration())
		assert.Equal(t, int64(10), opts.Stages[0].Target.Int64)
		assert.Equal(t, 2*time.Second, opts.Stages[1].Duration.TimeDuration())
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
		t.Parallel()
		opts := Options{}.Apply(Options{RPS: null.IntFrom(12345)})
		assert.True(t, opts.RPS.Valid)
		assert.Equal(t, int64(12345), opts.RPS.Int64)
	})
	t.Run("MaxRedirects", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{MaxRedirects: null.IntFrom(12345)})
		assert.True(t, opts.MaxRedirects.Valid)
		assert.Equal(t, int64(12345), opts.MaxRedirects.Int64)
	})
	t.Run("UserAgent", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{UserAgent: null.StringFrom("foo")})
		assert.True(t, opts.UserAgent.Valid)
		assert.Equal(t, "foo", opts.UserAgent.String)
	})
	t.Run("Batch", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{Batch: null.IntFrom(12345)})
		assert.True(t, opts.Batch.Valid)
		assert.Equal(t, int64(12345), opts.Batch.Int64)
	})
	t.Run("BatchPerHost", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{BatchPerHost: null.IntFrom(12345)})
		assert.True(t, opts.BatchPerHost.Valid)
		assert.Equal(t, int64(12345), opts.BatchPerHost.Int64)
	})
	t.Run("HTTPDebug", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{HTTPDebug: null.StringFrom("foo")})
		assert.True(t, opts.HTTPDebug.Valid)
		assert.Equal(t, "foo", opts.HTTPDebug.String)
	})
	t.Run("InsecureSkipTLSVerify", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{InsecureSkipTLSVerify: null.BoolFrom(true)})
		assert.True(t, opts.InsecureSkipTLSVerify.Valid)
		assert.True(t, opts.InsecureSkipTLSVerify.Bool)
	})
	t.Run("TLSCipherSuites", func(t *testing.T) {
		t.Parallel()
		for suiteName, suiteID := range SupportedTLSCipherSuites {
			t.Run(suiteName, func(t *testing.T) {
				t.Parallel()
				opts := Options{}.Apply(Options{TLSCipherSuites: &TLSCipherSuites{suiteID}})

				assert.NotNil(t, opts.TLSCipherSuites)
				assert.Len(t, *(opts.TLSCipherSuites), 1)
				assert.Equal(t, suiteID, (*opts.TLSCipherSuites)[0])
			})
		}

		t.Run("JSON", func(t *testing.T) {
			t.Parallel()
			t.Run("String", func(t *testing.T) {
				t.Parallel()
				var opts Options
				jsonStr := `{"tlsCipherSuites":["TLS_ECDHE_RSA_WITH_RC4_128_SHA"]}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, &TLSCipherSuites{tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA}, opts.TLSCipherSuites)

				t.Run("Roundtrip", func(t *testing.T) {
					t.Parallel()
					data, err := json.Marshal(opts.TLSCipherSuites)
					assert.NoError(t, err)
					assert.Equal(t, `["TLS_ECDHE_RSA_WITH_RC4_128_SHA"]`, string(data))
					var vers2 TLSCipherSuites
					assert.NoError(t, json.Unmarshal(data, &vers2))
					assert.Equal(t, &vers2, opts.TLSCipherSuites)
				})
			})
			t.Run("Not a string", func(t *testing.T) {
				t.Parallel()
				var opts Options
				jsonStr := `{"tlsCipherSuites":[1.2]}`
				assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
			})
			t.Run("Unknown cipher", func(t *testing.T) {
				t.Parallel()
				var opts Options
				jsonStr := `{"tlsCipherSuites":["foo"]}`
				assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
			})
		})
	})
	t.Run("TLSVersion", func(t *testing.T) {
		t.Parallel()
		versions := TLSVersions{Min: tls.VersionSSL30, Max: tls.VersionTLS12}
		opts := Options{}.Apply(Options{TLSVersion: &versions})

		assert.NotNil(t, opts.TLSVersion)
		assert.Equal(t, opts.TLSVersion.Min, TLSVersion(tls.VersionSSL30))
		assert.Equal(t, opts.TLSVersion.Max, TLSVersion(tls.VersionTLS12))

		t.Run("JSON", func(t *testing.T) {
			t.Parallel()
			t.Run("Object", func(t *testing.T) {
				t.Parallel()
				var opts Options
				jsonStr := `{"tlsVersion":{"min":"tls1.0","max":"tls1.2"}}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, &TLSVersions{
					Min: TLSVersion(tls.VersionTLS10),
					Max: TLSVersion(tls.VersionTLS12),
				}, opts.TLSVersion)

				t.Run("Roundtrip", func(t *testing.T) {
					t.Parallel()
					data, err := json.Marshal(opts.TLSVersion)
					assert.NoError(t, err)
					assert.Equal(t, `{"min":"tls1.0","max":"tls1.2"}`, string(data))
					var vers2 TLSVersions
					assert.NoError(t, json.Unmarshal(data, &vers2))
					assert.Equal(t, &vers2, opts.TLSVersion)
				})
			})
			t.Run("String", func(t *testing.T) {
				t.Parallel()
				var opts Options
				jsonStr := `{"tlsVersion":"tls1.2"}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, &TLSVersions{
					Min: TLSVersion(tls.VersionTLS12),
					Max: TLSVersion(tls.VersionTLS12),
				}, opts.TLSVersion)
			})
			t.Run("Blank", func(t *testing.T) {
				t.Parallel()
				var opts Options
				jsonStr := `{"tlsVersion":""}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, &TLSVersions{}, opts.TLSVersion)
			})
			t.Run("Not a string", func(t *testing.T) {
				t.Parallel()
				var opts Options
				jsonStr := `{"tlsVersion":1.2}`
				assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
			})
			t.Run("Unsupported version", func(t *testing.T) {
				t.Parallel()
				var opts Options
				jsonStr := `{"tlsVersion":"-1"}`
				assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))

				jsonStr = `{"tlsVersion":"ssl3.0"}`
				assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
			})
		})
	})
	t.Run("TLSAuth", func(t *testing.T) {
		t.Parallel()
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
			t.Parallel()
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
			t.Parallel()
			var opts Options
			jsonStr := `{"tlsAuth":["invalid"]}`
			assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
		})

		t.Run("Certificate error", func(t *testing.T) {
			t.Parallel()
			var opts Options
			jsonStr := `{"tlsAuth":[{"Cert":""}]}`
			assert.Error(t, json.Unmarshal([]byte(jsonStr), &opts))
		})
	})
	t.Run("TLSAuth with", func(t *testing.T) {
		t.Parallel()
		domains := []string{"example.com", "*.example.com"}
		cert := "-----BEGIN CERTIFICATE-----\n" +
			"MIIBoTCCAUegAwIBAgIUQl0J1Gkd6U2NIMwMDnpfH8c1myEwCgYIKoZIzj0EAwIw\n" +
			"EDEOMAwGA1UEAxMFTXkgQ0EwHhcNMTcwODE1MTYxODAwWhcNMTgwODE1MTYxODAw\n" +
			"WjAQMQ4wDAYDVQQDEwV1c2VyMTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABLaf\n" +
			"xEOmBHkzbqd9/0VZX/39qO2yQq2Gz5faRdvy38kuLMCV+9HYrfMx6GYCZzTUIq6h\n" +
			"8QXOrlgYTixuUVfhJNWjfzB9MA4GA1UdDwEB/wQEAwIFoDAdBgNVHSUEFjAUBggr\n" +
			"BgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAdBgNVHQ4EFgQUxmQiq5K3\n" +
			"KUnVME945Byt3Ysvkh8wHwYDVR0jBBgwFoAU3qEhcpRgpsqo9V+LFns9a+oZIYww\n" +
			"CgYIKoZIzj0EAwIDSAAwRQIgSGxnJ+/cLUNTzt7fhr/mjJn7ShsTW33dAdfLM7H2\n" +
			"z/gCIQDyVf8DePtxlkMBScTxZmIlMQdNc6+6VGZQ4QscruVLmg==\n" +
			"-----END CERTIFICATE-----"
		tests := []struct {
			name         string
			privateKey   string
			password     string
			hasError     bool
			errorMessage string
		}{
			{
				name: "encrypted key and invalid password",
				privateKey: "-----BEGIN EC PRIVATE KEY-----\n" +
					"Proc-Type: 4,ENCRYPTED\n" +
					"DEK-Info: AES-256-CBC,DF2445CBFE2E5B112FB2B721063757E5\n" +
					"o/VKNZjQcRM2hatqUkQ0dTolL7i2i5hJX9XYsl+TMsq8ZkC83uY/JdR986QS+W2c\n" +
					"EoQGtVGVeL0KGvGpzjTX3YAKXM7Lg5btAeS8GvJ9S7YFd8s0q1pqDdffl2RyjJav\n" +
					"t1jx6XvLu2nBrOUARvHqjkkJQCTdRf2a34GJdbZqE+4=\n" +
					"-----END EC PRIVATE KEY-----",
				password:     "iZfYGcrgFHOg4nweEo7ufT",
				hasError:     true,
				errorMessage: "x509: decryption password incorrect",
			},
			{
				name: "encrypted key and valid password",
				privateKey: "-----BEGIN EC PRIVATE KEY-----\n" +
					"Proc-Type: 4,ENCRYPTED\n" +
					"DEK-Info: AES-256-CBC,DF2445CBFE2E5B112FB2B721063757E5\n" +
					"o/VKNZjQcRM2hatqUkQ0dTolL7i2i5hJX9XYsl+TMsq8ZkC83uY/JdR986QS+W2c\n" +
					"EoQGtVGVeL0KGvGpzjTX3YAKXM7Lg5btAeS8GvJ9S7YFd8s0q1pqDdffl2RyjJav\n" +
					"t1jx6XvLu2nBrOUARvHqjkkJQCTdRf2a34GJdbZqE+4=\n" +
					"-----END EC PRIVATE KEY-----",
				password:     "12345",
				hasError:     false,
				errorMessage: "",
			},
			{
				name: "encrypted pks8 format key and valid password",
				privateKey: "-----BEGIN ENCRYPTED PRIVATE KEY-----\n" +
					"MIHsMFcGCSqGSIb3DQEFDTBKMCkGCSqGSIb3DQEFDDAcBAjcfarGfrRgUgICCAAw\n" +
					"DAYIKoZIhvcNAgkFADAdBglghkgBZQMEASoEEFmtmKEFmThbkbpxmC6iBvoEgZCE\n" +
					"pDCpH/yCLmSpjdi/PC74I794nzHyCWf/oS0JhM0Q7J+abZP+p5pnreKft1f15Dbw\n" +
					"QG9alfoM6EffJcVo3gf1tgQrpGGFMwczc4VhQgSGDy0XjZSbd2K0QCFGSmD2ZIR1\n" +
					"qPG3WepWjKmIsYffGeKZx+FjXHSFeGk7RnssNAyKcPruDQIdWWyXxX1+ugBKuBw=\n" +
					"-----END ENCRYPTED PRIVATE KEY-----\n",
				password:     "12345",
				hasError:     true,
				errorMessage: "encrypted pkcs8 formatted key is not supported",
			},
			{
				name: "non encrypted key and password",
				privateKey: "-----BEGIN EC PRIVATE KEY-----\n" +
					"MHcCAQEEINVilD5qOBkSy+AYfd41X0QPB5N3Z6OzgoBj8FZmSJOFoAoGCCqGSM49\n" +
					"AwEHoUQDQgAEF8XzmC7x8Ns0Y2Wyu2c77ge+6I/ghcDTjWOMZzMPmRRDxqKFLuGD\n" +
					"zW1Kss13WODGSS8+j7dNCPOeLKyK6cbeIg==\n" +
					"-----END EC PRIVATE KEY-----",
				password:     "12345",
				hasError:     true,
				errorMessage: "x509: no DEK-Info header in block",
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				tlsAuth := []*TLSAuth{
					{TLSAuthFields{
						Domains:  domains,
						Cert:     cert,
						Key:      tc.privateKey,
						Password: null.StringFrom(tc.password),
					}, nil},
				}
				opts := Options{}.Apply(Options{TLSAuth: tlsAuth})
				assert.Equal(t, tlsAuth, opts.TLSAuth)

				t.Run("Roundtrip", func(t *testing.T) {
					optsData, err := json.Marshal(opts)
					assert.NoError(t, err)

					var opts2 Options
					err = json.Unmarshal(optsData, &opts2)
					if tc.hasError {
						assert.Error(t, err)
						assert.Contains(t, err.Error(), tc.errorMessage)
					} else {
						assert.NoError(t, err)
					}
				})
			})
		}
	})
	t.Run("NoConnectionReuse", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{NoConnectionReuse: null.BoolFrom(true)})
		assert.True(t, opts.NoConnectionReuse.Valid)
		assert.True(t, opts.NoConnectionReuse.Bool)
	})
	t.Run("NoVUConnectionReuse", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{NoVUConnectionReuse: null.BoolFrom(true)})
		assert.True(t, opts.NoVUConnectionReuse.Valid)
		assert.True(t, opts.NoVUConnectionReuse.Bool)
	})
	t.Run("NoCookiesReset", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{NoCookiesReset: null.BoolFrom(true)})
		assert.True(t, opts.NoCookiesReset.Valid)
		assert.True(t, opts.NoCookiesReset.Bool)
	})
	t.Run("BlacklistIPs", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{
			BlacklistIPs: []*IPNet{{
				IPNet: net.IPNet{
					IP:   net.IPv4bcast,
					Mask: net.CIDRMask(31, 32),
				},
			}},
		})
		assert.NotNil(t, opts.BlacklistIPs)
		assert.NotEmpty(t, opts.BlacklistIPs)
		assert.Equal(t, net.IPv4bcast, opts.BlacklistIPs[0].IP)
		assert.Equal(t, net.CIDRMask(31, 32), opts.BlacklistIPs[0].Mask)

		t.Run("JSON", func(t *testing.T) {
			t.Parallel()

			b, err := json.Marshal(opts)
			require.NoError(t, err)

			var uopts Options
			err = json.Unmarshal(b, &uopts)
			require.NoError(t, err)
			require.Len(t, uopts.BlacklistIPs, 1)
			require.Equal(t, "255.255.255.254/31", uopts.BlacklistIPs[0].String())
		})
	})
	t.Run("BlockedHostnames", func(t *testing.T) {
		t.Parallel()
		blockedHostnames, err := types.NewNullHostnameTrie([]string{"test.k6.io", "*valid.pattern"})
		require.NoError(t, err)
		opts := Options{}.Apply(Options{BlockedHostnames: blockedHostnames})
		assert.NotNil(t, opts.BlockedHostnames)
		assert.Equal(t, blockedHostnames, opts.BlockedHostnames)
	})

	t.Run("Hosts", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
		opts := Options{}.Apply(Options{Throw: null.BoolFrom(true)})
		assert.True(t, opts.Throw.Valid)
		assert.Equal(t, true, opts.Throw.Bool)
	})

	t.Run("Thresholds", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{Thresholds: map[string]metrics.Thresholds{
			"metric": {
				Thresholds: []*metrics.Threshold{{}},
			},
		}})
		assert.NotNil(t, opts.Thresholds)
		assert.NotEmpty(t, opts.Thresholds)
	})
	t.Run("External", func(t *testing.T) {
		t.Parallel()
		ext := map[string]json.RawMessage{"a": json.RawMessage("1")}
		opts := Options{}.Apply(Options{External: ext})
		assert.Equal(t, ext, opts.External)
	})

	t.Run("JSON", func(t *testing.T) {
		t.Parallel()
		data, err := json.Marshal(Options{})
		assert.NoError(t, err)
		var opts Options
		assert.NoError(t, json.Unmarshal(data, &opts))
		assert.Equal(t, Options{}, opts)
	})
	t.Run("SystemTags", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{SystemTags: metrics.NewSystemTagSet(metrics.TagProto)})
		assert.NotNil(t, opts.SystemTags)
		assert.NotEmpty(t, opts.SystemTags)
		assert.True(t, opts.SystemTags.Has(metrics.TagProto))

		t.Run("JSON", func(t *testing.T) {
			t.Parallel()
			t.Run("Array", func(t *testing.T) {
				t.Parallel()
				var opts Options
				jsonStr := `{"systemTags":["url"]}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, *metrics.NewSystemTagSet(metrics.TagURL), *opts.SystemTags)

				t.Run("Roundtrip", func(t *testing.T) {
					t.Parallel()
					data, err := json.Marshal(opts.SystemTags)
					assert.NoError(t, err)
					assert.Equal(t, `["url"]`, string(data))
					var vers2 metrics.SystemTagSet
					assert.NoError(t, json.Unmarshal(data, &vers2))
					assert.Equal(t, vers2, *opts.SystemTags)
				})
			})
			t.Run("Blank", func(t *testing.T) {
				t.Parallel()
				var opts Options
				jsonStr := `{"systemTags":[]}`
				assert.NoError(t, json.Unmarshal([]byte(jsonStr), &opts))
				assert.Equal(t, metrics.SystemTagSet(0), *opts.SystemTags)
			})
		})
	})
	t.Run("SummaryTrendStats", func(t *testing.T) {
		t.Parallel()
		stats := []string{"myStat1", "myStat2"}
		opts := Options{}.Apply(Options{SummaryTrendStats: stats})
		assert.Equal(t, stats, opts.SummaryTrendStats)
	})
	t.Run("RunTags", func(t *testing.T) {
		t.Parallel()
		tags := map[string]string{"myTag": "hello"}
		opts := Options{}.Apply(Options{RunTags: tags})
		assert.Equal(t, tags, opts.RunTags)
	})
	t.Run("DiscardResponseBodies", func(t *testing.T) {
		t.Parallel()
		opts := Options{}.Apply(Options{DiscardResponseBodies: null.BoolFrom(true)})
		assert.True(t, opts.DiscardResponseBodies.Valid)
		assert.True(t, opts.DiscardResponseBodies.Bool)
	})
	t.Run("ClientIPRanges", func(t *testing.T) {
		t.Parallel()
		clientIPRanges := types.NullIPPool{}
		err := clientIPRanges.UnmarshalText([]byte("129.112.232.12,123.12.0.0/32"))
		require.NoError(t, err)
		opts := Options{}.Apply(Options{LocalIPs: clientIPRanges})
		assert.NotNil(t, opts.LocalIPs)
	})
}

func TestOptionsEnv(t *testing.T) {
	t.Parallel()
	mustNullIPPool := func(s string) types.NullIPPool {
		p := types.NullIPPool{}
		err := p.UnmarshalText([]byte(s))
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
		{"NoSetup", "K6_NO_SETUP"}: {
			"":      null.Bool{},
			"true":  null.BoolFrom(true),
			"false": null.BoolFrom(false),
		},
		{"NoTeardown", "K6_NO_TEARDOWN"}: {
			"":      null.Bool{},
			"true":  null.BoolFrom(true),
			"false": null.BoolFrom(false),
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
			"192.168.220.2":    mustNullIPPool("192.168.220.2"),
			"192.168.220.0/24": mustNullIPPool("192.168.220.0/24"),
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
			t.Parallel()
			for str, val := range data {
				str, val := str, val
				t.Run(`"`+str+`"`, func(t *testing.T) {
					t.Parallel()
					var opts Options
					assert.NoError(t, envconfig.Process("k6", &opts, func(k string) (string, bool) {
						if k == field.Key {
							return str, true
						}
						return "", false
					}))
					assert.Equal(t, val, reflect.ValueOf(opts).FieldByName(field.Name).Interface())
				})
			}
		})
	}
}

func TestCIDRUnmarshal(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			actualIPNet := &IPNet{}
			err := actualIPNet.UnmarshalText([]byte(data.input))

			if data.expectFailure {
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid CIDR address: "+data.input)
			} else {
				require.NoError(t, err)
				assert.Equal(t, data.expectedOutput, actualIPNet)
			}
		})
	}
}

func TestHostAddressUnmarshal(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
