package execution

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"
)

//nolint:gochecknoglobals
var tagsAndMetricsPropertyNames = map[string]string{
	"tags":             "tag",
	"metrics.tags":     "tag",
	"metrics.metadata": "metadata",
}

func setupTagsExecEnv(t *testing.T) *modulestest.Runtime {
	testRuntime := modulestest.NewRuntime(t)
	m, ok := New().NewModuleInstance(testRuntime.VU).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, testRuntime.VU.Runtime().Set("exec", m.Exports().Default))

	return testRuntime
}

func TestVUTagsMetadatasGet(t *testing.T) {
	t.Parallel()

	for prop, propType := range tagsAndMetricsPropertyNames {
		prop, propType := prop, propType
		t.Run(prop, func(t *testing.T) {
			t.Parallel()
			tenv := setupTagsExecEnv(t)
			tenv.MoveToVUContext(&lib.State{
				Tags: lib.NewVUStateTags(metrics.NewRegistry().RootTagSet().With("tag-vu", "mytag")),
			})
			tenv.VU.StateField.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
				tagsAndMeta.SetMetadata("metadata-vu", "mymetadata")
			})
			tag, err := tenv.VU.Runtime().RunString(fmt.Sprintf(`exec.vu.%s["%s-vu"]`, prop, propType))
			require.NoError(t, err)
			assert.Equal(t, "my"+propType, tag.String())

			// not found
			tag, err = tenv.VU.Runtime().RunString(fmt.Sprintf(`exec.vu.%s["not-existing-%s"]`, prop, propType))
			require.NoError(t, err)
			assert.Equal(t, "undefined", tag.String())
		})
	}
}

func TestVUTagsMetadatasJSONEncoding(t *testing.T) {
	t.Parallel()

	for prop, propType := range tagsAndMetricsPropertyNames {
		prop, propType := prop, propType
		t.Run(prop, func(t *testing.T) {
			t.Parallel()
			tenv := setupTagsExecEnv(t)
			tenv.MoveToVUContext(&lib.State{
				Options: lib.Options{
					SystemTags: metrics.NewSystemTagSet(metrics.TagVU),
				},
				Tags: lib.NewVUStateTags(metrics.NewRegistry().RootTagSet()),
			})
			tenv.VU.State().Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
				tagsAndMeta.SetTag("tag-vu", "42")
				tagsAndMeta.SetMetadata("metadata-vu", "42")
				tagsAndMeta.SetTag("custom-tag", "mytag1")
				tagsAndMeta.SetMetadata("custom-metadata", "mymetadata1")
			})

			encoded, err := tenv.VU.Runtime().RunString(fmt.Sprintf(`JSON.stringify(exec.vu.%s)`, prop))
			require.NoError(t, err)
			assert.JSONEq(t, fmt.Sprintf(`{"%[1]s-vu":"42","custom-%[1]s":"my%[1]s1"}`, propType), encoded.String())
		})
	}
}

func TestVUTagMetadatasSetSuccessAcceptedTypes(t *testing.T) {
	t.Parallel()

	// bool and numbers are implicitly converted into string

	tests := map[string]struct {
		v   interface{}
		exp string
	}{
		"string": {v: `"tag1"`, exp: "tag1"},
		"bool":   {v: true, exp: "true"},
		"int":    {v: 101, exp: "101"},
		"float":  {v: 3.14, exp: "3.14"},
	}
	for prop := range tagsAndMetricsPropertyNames {
		prop := prop
		t.Run(prop, func(t *testing.T) {
			t.Parallel()
			tenv := setupTagsExecEnv(t)
			tenv.MoveToVUContext(&lib.State{
				Tags: lib.NewVUStateTags(metrics.NewRegistry().RootTagSet().With("vu", "42")),
			})

			for _, tc := range tests {
				_, err := tenv.VU.Runtime().RunString(fmt.Sprintf(`exec.vu.%s["mytag"] = %v`, prop, tc.v))
				require.NoError(t, err)

				val, err := tenv.VU.Runtime().RunString(fmt.Sprintf(`exec.vu.%s["mytag"]`, prop))
				require.NoError(t, err)

				assert.Equal(t, tc.exp, val.String())
			}
		})
	}
}

func TestVUTagsMetadatasSuccessOverwriteSystemTag(t *testing.T) {
	t.Parallel()

	tenv := setupTagsExecEnv(t)
	tenv.MoveToVUContext(&lib.State{
		Tags: lib.NewVUStateTags(metrics.NewRegistry().RootTagSet().With("vu", "42")),
	})

	_, err := tenv.VU.Runtime().RunString(`exec.vu.tags["vu"] = "vu101"`)
	require.NoError(t, err)
	val, err := tenv.VU.Runtime().RunString(`exec.vu.tags["vu"]`)
	require.NoError(t, err)
	assert.Equal(t, "vu101", val.String())
}

func TestVUTagsMetadataErrorOutOnInvalidValues(t *testing.T) {
	t.Parallel()

	cases := []string{
		"null",
		"undefined",
		"[]",
		"{}",
		`[1, 3, 5]`,
		`{f1: "value1", f2: 4}`,
		`{"foo": "bar"}`,
	}
	for prop, propType := range tagsAndMetricsPropertyNames {
		prop, propType := prop, propType
		t.Run(prop, func(t *testing.T) {
			t.Parallel()
			for _, val := range cases {
				logHook := testutils.NewLogHook(logrus.WarnLevel)
				testLog := logrus.New()
				testLog.AddHook(logHook)
				testLog.SetOutput(io.Discard)
				tenv := setupTagsExecEnv(t)
				tenv.MoveToVUContext(&lib.State{
					Options: lib.Options{
						SystemTags: metrics.NewSystemTagSet(metrics.TagVU),
					},
					Tags:   lib.NewVUStateTags(metrics.NewRegistry().RootTagSet().With("vu", "42")),
					Logger: testLog,
				})
				_, err := tenv.VU.Runtime().RunString(fmt.Sprintf(`exec.vu.%s["custom"] = %s`, prop, val))
				assert.ErrorContains(t, err, "TypeError: invalid value for metric "+propType+" 'custom'")
			}
		})
	}
}

func TestAbortTest(t *testing.T) { //nolint:tparallel
	t.Parallel()

	var (
		rt    = sobek.New()
		state = &lib.State{}
		ctx   = context.Background()
	)

	m, ok := New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			InitEnvField: &common.InitEnvironment{},
			CtxField:     ctx,
			StateField:   state,
		},
	).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("exec", m.Exports().Default))

	prove := func(t *testing.T, script, reason string) {
		_, err := rt.RunString(script)
		require.NotNil(t, err)
		var x *sobek.InterruptedError
		assert.ErrorAs(t, err, &x)
		v, ok := x.Value().(*errext.InterruptError)
		require.True(t, ok)
		require.Equal(t, v.Reason, reason)
	}

	t.Run("default reason", func(t *testing.T) { //nolint:paralleltest
		prove(t, "exec.test.abort()", errext.AbortTest)
	})
	t.Run("custom reason", func(t *testing.T) { //nolint:paralleltest
		prove(t, `exec.test.abort("mayday")`, fmt.Sprintf("%s: mayday", errext.AbortTest))
	})
}

func TestOptionsTestFull(t *testing.T) {
	t.Parallel()

	expected := `{"paused":true,"scenarios":{"const-vus":{"executor":"constant-vus","options":{"browser":{"someOption":true}},"startTime":"10s","gracefulStop":"30s","env":{"FOO":"bar"},"exec":"default","tags":{"tagkey":"tagvalue"},"vus":50,"duration":"10m0s"}},"executionSegment":"0:1/4","executionSegmentSequence":"0,1/4,1/2,1","noSetup":true,"setupTimeout":"1m0s","noTeardown":true,"teardownTimeout":"5m0s","rps":100,"dns":{"ttl":"1m","select":"roundRobin","policy":"any"},"maxRedirects":3,"userAgent":"k6-user-agent","batch":15,"batchPerHost":5,"httpDebug":"full","insecureSkipTLSVerify":true,"tlsCipherSuites":["TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"],"tlsVersion":{"min":"tls1.2","max":"tls1.3"},"tlsAuth":[{"domains":["example.com"],"cert":"mycert.pem","key":"mycert-key.pem","password":"mypwd"}],"throw":true,"thresholds":{"http_req_duration":[{"threshold":"rate>0.01","abortOnFail":true,"delayAbortEval":"10s"}]},"blacklistIPs":["192.0.2.0/24"],"blockHostnames":["test.k6.io","*.example.com"],"hosts":{"test.k6.io":"1.2.3.4:8443"},"noConnectionReuse":true,"noVUConnectionReuse":true,"minIterationDuration":"10s","ext":{"ext-one":{"rawkey":"rawvalue"}},"summaryTrendStats":["avg","min","max"],"summaryTimeUnit":"ms","systemTags":["iter","vu"],"tags":null,"metricSamplesBufferSize":8,"noCookiesReset":true,"discardResponseBodies":true,"consoleOutput":"loadtest.log","tags":{"runtag-key":"runtag-value"},"localIPs":"192.168.20.12-192.168.20.15,192.168.10.0/27"}`

	var (
		rt    = sobek.New()
		state = &lib.State{
			Options: lib.Options{
				Paused: null.BoolFrom(true),
				Scenarios: map[string]lib.ExecutorConfig{
					"const-vus": executor.ConstantVUsConfig{
						BaseConfig: executor.BaseConfig{
							Name:         "const-vus",
							Type:         "constant-vus",
							StartTime:    types.NullDurationFrom(10 * time.Second),
							GracefulStop: types.NullDurationFrom(30 * time.Second),
							Env: map[string]string{
								"FOO": "bar",
							},
							Exec: null.StringFrom("default"),
							Tags: map[string]string{
								"tagkey": "tagvalue",
							},
							Options: &lib.ScenarioOptions{
								Browser: map[string]any{
									"someOption": true,
								},
							},
						},
						VUs:      null.IntFrom(50),
						Duration: types.NullDurationFrom(10 * time.Minute),
					},
				},
				ExecutionSegment: func() *lib.ExecutionSegment {
					seg, err := lib.NewExecutionSegmentFromString("0:1/4")
					require.NoError(t, err)
					return seg
				}(),
				ExecutionSegmentSequence: func() *lib.ExecutionSegmentSequence {
					seq, err := lib.NewExecutionSegmentSequenceFromString("0,1/4,1/2,1")
					require.NoError(t, err)
					return &seq
				}(),
				NoSetup:               null.BoolFrom(true),
				NoTeardown:            null.BoolFrom(true),
				NoConnectionReuse:     null.BoolFrom(true),
				NoVUConnectionReuse:   null.BoolFrom(true),
				InsecureSkipTLSVerify: null.BoolFrom(true),
				Throw:                 null.BoolFrom(true),
				NoCookiesReset:        null.BoolFrom(true),
				DiscardResponseBodies: null.BoolFrom(true),
				RPS:                   null.IntFrom(100),
				MaxRedirects:          null.IntFrom(3),
				UserAgent:             null.StringFrom("k6-user-agent"),
				Batch:                 null.IntFrom(15),
				BatchPerHost:          null.IntFrom(5),
				SetupTimeout:          types.NullDurationFrom(1 * time.Minute),
				TeardownTimeout:       types.NullDurationFrom(5 * time.Minute),
				MinIterationDuration:  types.NullDurationFrom(10 * time.Second),
				HTTPDebug:             null.StringFrom("full"),
				DNS: types.DNSConfig{
					TTL:    null.StringFrom("1m"),
					Select: types.NullDNSSelect{DNSSelect: types.DNSroundRobin, Valid: true},
					Policy: types.NullDNSPolicy{DNSPolicy: types.DNSany, Valid: true},
					Valid:  true,
				},
				TLSVersion: &lib.TLSVersions{
					Min: tls.VersionTLS12,
					Max: tls.VersionTLS13,
				},
				TLSAuth: []*lib.TLSAuth{
					{
						TLSAuthFields: lib.TLSAuthFields{
							Cert:     "mycert.pem",
							Key:      "mycert-key.pem",
							Password: null.StringFrom("mypwd"),
							Domains:  []string{"example.com"},
						},
					},
				},
				TLSCipherSuites: &lib.TLSCipherSuites{
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				},
				BlacklistIPs: []*lib.IPNet{
					{
						IPNet: func() net.IPNet {
							_, ipv4net, err := net.ParseCIDR("192.0.2.1/24")
							require.NoError(t, err)
							return *ipv4net
						}(),
					},
				},
				Thresholds: map[string]metrics.Thresholds{
					"http_req_duration": {
						Thresholds: []*metrics.Threshold{
							{
								Source:           "rate>0.01",
								LastFailed:       true,
								AbortOnFail:      true,
								AbortGracePeriod: types.NullDurationFrom(10 * time.Second),
							},
						},
					},
				},
				BlockedHostnames: func() types.NullHostnameTrie {
					bh, err := types.NewNullHostnameTrie([]string{"test.k6.io", "*.example.com"})
					require.NoError(t, err)
					return bh
				}(),
				Hosts: func() types.NullHosts {
					hs, err := types.NewNullHosts(map[string]types.Host{
						"test.k6.io": {
							IP:   []byte{0x01, 0x02, 0x03, 0x04},
							Port: 8443,
						},
					})
					require.NoError(t, err)
					return hs
				}(),
				External: map[string]json.RawMessage{
					"ext-one": json.RawMessage(`{"rawkey":"rawvalue"}`),
				},
				SummaryTrendStats: []string{"avg", "min", "max"},
				SummaryTimeUnit:   null.StringFrom("ms"),
				SystemTags: func() *metrics.SystemTagSet {
					sysm := metrics.SystemTagSet(metrics.TagIter | metrics.TagVU)
					return &sysm
				}(),
				RunTags:                 map[string]string{"runtag-key": "runtag-value"},
				MetricSamplesBufferSize: null.IntFrom(8),
				ConsoleOutput:           null.StringFrom("loadtest.log"),
				LocalIPs: func() types.NullIPPool {
					npool := types.NullIPPool{}
					err := npool.UnmarshalText([]byte("192.168.20.12-192.168.20.15,192.168.10.0/27"))
					require.NoError(t, err)
					return npool
				}(),

				// The following fields are not expected to be
				// in the final test.options object
				VUs:        null.IntFrom(50),
				Iterations: null.IntFrom(100),
				Duration:   types.NullDurationFrom(10 * time.Second),
				Stages: []lib.Stage{
					{
						Duration: types.NullDurationFrom(2 * time.Second),
						Target:   null.IntFrom(2),
					},
				},
			},
		}
		ctx = context.Background()
	)

	m, ok := New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			CtxField:     ctx,
			StateField:   state,
		},
	).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("exec", m.Exports().Default))

	opts, err := rt.RunString(`JSON.stringify(exec.test.options)`)
	require.NoError(t, err)
	require.NotNil(t, opts)
	assert.JSONEq(t, expected, opts.String())
}

func TestOptionsTestSetPropertyDenied(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	m, ok := New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			CtxField:     context.Background(),
			StateField: &lib.State{
				Options: lib.Options{
					Paused: null.BoolFrom(true),
				},
			},
		},
	).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("exec", m.Exports().Default))

	_, err := rt.RunString(`exec.test.options.paused = false`)
	require.NoError(t, err)
	paused, err := rt.RunString(`exec.test.options.paused`)
	require.NoError(t, err)
	assert.Equal(t, true, rt.ToValue(paused).ToBoolean())
}

func TestScenarioNoAvailableInInitContext(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	m, ok := New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			CtxField:     context.Background(),
			StateField: &lib.State{
				Options: lib.Options{
					Paused: null.BoolFrom(true),
				},
			},
		},
	).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("exec", m.Exports().Default))

	scenarioExportedProps := []string{"name", "executor", "startTime", "progress", "iterationInInstance", "iterationInTest"}

	for _, code := range scenarioExportedProps {
		prop := fmt.Sprintf("exec.scenario.%s", code)
		_, err := rt.RunString(prop)
		require.Error(t, err)
		require.ErrorContains(t, err, "getting scenario information outside of the VU context is not supported")
	}
}

func TestOptionsNoAvailableInInitContext(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	m, ok := New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			CtxField:     context.Background(),
		},
	).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("exec", m.Exports().Default))

	_, err := rt.RunString("exec.test.options")
	require.ErrorContains(t, err, "getting test options in the init context is not supported")
}

func TestVUDefaultDetails(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	m, ok := New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			CtxField:     context.Background(),
			StateField: &lib.State{
				Options: lib.Options{
					Paused: null.BoolFrom(true),
				},
			},
		},
	).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("exec", m.Exports().Default))

	props := []string{"idInInstance", "idInTest", "iterationInInstance", "iterationInScenario"}

	for _, code := range props {
		prop := fmt.Sprintf("exec.vu.%s", code)
		res, err := rt.RunString(prop)
		require.NoError(t, err)
		require.Equal(t, "0", res.String())
	}
}

func TestTagsDynamicObjectGet(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	tdo := tagsDynamicObject{
		runtime: rt,
		state: &lib.State{
			Tags: lib.NewVUStateTags(metrics.NewRegistry().RootTagSet().With("vu", "42")),
		},
	}
	val := tdo.Get("vu")
	require.NotNil(t, val)
	assert.Equal(t, val.ToInteger(), int64(42))
}

func TestTagsDynamicObjectSet(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	tdo := tagsDynamicObject{
		runtime: rt,
		state: &lib.State{
			Tags: lib.NewVUStateTags(metrics.NewRegistry().RootTagSet().With("vu", "42")),
		},
	}
	require.True(t, tdo.Set("k1", rt.ToValue("v1")))

	val := tdo.Get("k1")
	require.NotNil(t, val)
	assert.Equal(t, val.String(), "v1")
}
