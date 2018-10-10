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

package js

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/loadimpact/k6/core"
	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/dummy"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
)

func TestRunnerNew(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		r, err := New(&lib.SourceData{
			Filename: "/script.js",
			Data: []byte(`
			let counter = 0;
			export default function() { counter++; }
		`),
		}, afero.NewMemMapFs(), lib.RuntimeOptions{})
		assert.NoError(t, err)

		t.Run("NewVU", func(t *testing.T) {
			vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
			assert.NoError(t, err)
			vuc, ok := vu.(*VU)
			assert.True(t, ok)
			assert.Equal(t, int64(0), vuc.Runtime.Get("counter").Export())

			t.Run("RunOnce", func(t *testing.T) {
				err = vu.RunOnce(context.Background())
				assert.NoError(t, err)
				assert.Equal(t, int64(1), vuc.Runtime.Get("counter").Export())
			})
		})
	})

	t.Run("Invalid", func(t *testing.T) {
		_, err := New(&lib.SourceData{
			Filename: "/script.js",
			Data:     []byte(`blarg`),
		}, afero.NewMemMapFs(), lib.RuntimeOptions{})
		assert.EqualError(t, err, "ReferenceError: blarg is not defined at /script.js:1:1(0)")
	})
}

func TestRunnerGetDefaultGroup(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data:     []byte(`export default function() {};`),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if assert.NoError(t, err) {
		assert.NotNil(t, r1.GetDefaultGroup())
	}

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if assert.NoError(t, err) {
		assert.NotNil(t, r2.GetDefaultGroup())
	}
}

func TestRunnerOptions(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data:     []byte(`export default function() {};`),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, r.Bundle.Options, r.GetOptions())
			assert.Equal(t, null.NewBool(false, false), r.Bundle.Options.Paused)
			r.SetOptions(lib.Options{Paused: null.BoolFrom(true)})
			assert.Equal(t, r.Bundle.Options, r.GetOptions())
			assert.Equal(t, null.NewBool(true, true), r.Bundle.Options.Paused)
			r.SetOptions(lib.Options{Paused: null.BoolFrom(false)})
			assert.Equal(t, r.Bundle.Options, r.GetOptions())
			assert.Equal(t, null.NewBool(false, true), r.Bundle.Options.Paused)
		})
	}
}

func TestOptionsSettingToScript(t *testing.T) {
	t.Parallel()

	optionVariants := []string{
		"",
		"let options = null;",
		"let options = undefined;",
		"let options = {};",
		"let options = {teardownTimeout: '1s'};",
	}

	for i, variant := range optionVariants {
		variant := variant
		t.Run(fmt.Sprintf("Variant#%d", i), func(t *testing.T) {
			t.Parallel()
			src := &lib.SourceData{
				Filename: "/script.js",
				Data: []byte(variant + `
					export default function() {
						if (!options) {
							throw new Error("Expected options to be defined!");
						}
						if (options.teardownTimeout != __ENV.expectedTeardownTimeout) {
							throw new Error("expected teardownTimeout to be " + __ENV.expectedTeardownTimeout + " but it was " + options.teardownTimeout);
						}
					};
				`),
			}
			r, err := New(src, afero.NewMemMapFs(), lib.RuntimeOptions{Env: map[string]string{"expectedTeardownTimeout": "4s"}})
			require.NoError(t, err)

			newOptions := lib.Options{TeardownTimeout: types.NullDurationFrom(4 * time.Second)}
			r.SetOptions(newOptions)
			require.Equal(t, newOptions, r.GetOptions())

			samples := make(chan stats.SampleContainer, 100)
			vu, err := r.NewVU(samples)
			if assert.NoError(t, err) {
				err := vu.RunOnce(context.Background())
				assert.NoError(t, err)
			}
		})
	}
}

func TestOptionsPropagationToScript(t *testing.T) {
	t.Parallel()
	src := &lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
			export let options = { setupTimeout: "1s", myOption: "test" };
			export default function() {
				if (options.external) {
					throw new Error("Unexpected property external!");
				}
				if (options.myOption != "test") {
					throw new Error("expected myOption to remain unchanged but it was '" + options.myOption + "'");
				}
				if (options.setupTimeout != __ENV.expectedSetupTimeout) {
					throw new Error("expected setupTimeout to be " + __ENV.expectedSetupTimeout + " but it was " + options.setupTimeout);
				}
			};
		`),
	}

	expScriptOptions := lib.Options{SetupTimeout: types.NullDurationFrom(1 * time.Second)}
	r1, err := New(src, afero.NewMemMapFs(), lib.RuntimeOptions{Env: map[string]string{"expectedSetupTimeout": "1s"}})
	require.NoError(t, err)
	require.Equal(t, expScriptOptions, r1.GetOptions())

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{Env: map[string]string{"expectedSetupTimeout": "3s"}})
	require.NoError(t, err)
	require.Equal(t, expScriptOptions, r2.GetOptions())

	newOptions := lib.Options{SetupTimeout: types.NullDurationFrom(3 * time.Second)}
	r2.SetOptions(newOptions)
	require.Equal(t, newOptions, r2.GetOptions())

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		t.Run(name, func(t *testing.T) {
			samples := make(chan stats.SampleContainer, 100)

			vu, err := r.NewVU(samples)
			if assert.NoError(t, err) {
				err := vu.RunOnce(context.Background())
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetupDataIsolation(t *testing.T) {
	tb := testutils.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	script := []byte(tb.Replacer.Replace(`
		import { Counter } from "k6/metrics";

		export let options = {
			vus: 2,
			vusMax: 10,
			iterations: 500,
			teardownTimeout: "1s",
			setupTimeout: "1s",
		};
		let myCounter = new Counter("mycounter");

		export function setup() {
			return { v: 0 };
		}

		export default function(data) {
			if (data.v !== __ITER) {
				throw new Error("default: wrong data for iter " + __ITER + ": " + JSON.stringify(data));
			}
			data.v += 1;
			myCounter.add(1);
		}

		export function teardown(data) {
			if (data.v !== 0) {
				throw new Error("teardown: wrong data: " + data.v);
			}
			myCounter.add(1);
		}
	`))

	runner, err := New(
		&lib.SourceData{Filename: "/script.js", Data: script},
		afero.NewMemMapFs(),
		lib.RuntimeOptions{},
	)
	require.NoError(t, err)

	engine, err := core.NewEngine(local.New(runner), runner.GetOptions())
	require.NoError(t, err)

	collector := &dummy.Collector{}
	engine.Collectors = []lib.Collector{collector}

	ctx, cancel := context.WithCancel(context.Background())
	errC := make(chan error)
	go func() { errC <- engine.Run(ctx) }()

	select {
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("Test timed out")
	case err := <-errC:
		cancel()
		require.NoError(t, err)
		require.False(t, engine.IsTainted())
	}
	var count int
	for _, s := range collector.Samples {
		if s.Metric.Name == "mycounter" {
			count += int(s.Value)
		}
	}
	require.Equal(t, 501, count, "mycounter should be the number of iterations + 1 for the teardown")
}

func testSetupDataHelper(t *testing.T, src *lib.SourceData) {
	t.Helper()
	expScriptOptions := lib.Options{
		SetupTimeout:    types.NullDurationFrom(1 * time.Second),
		TeardownTimeout: types.NullDurationFrom(1 * time.Second),
	}
	r1, err := New(src, afero.NewMemMapFs(), lib.RuntimeOptions{})
	require.NoError(t, err)
	require.Equal(t, expScriptOptions, r1.GetOptions())

	testdata := map[string]*Runner{"Source": r1}
	for name, r := range testdata {
		t.Run(name, func(t *testing.T) {
			samples := make(chan stats.SampleContainer, 100)

			if !assert.NoError(t, r.Setup(context.Background(), samples)) {
				return
			}
			vu, err := r.NewVU(samples)
			if assert.NoError(t, err) {
				err := vu.RunOnce(context.Background())
				assert.NoError(t, err)
			}
		})
	}
}
func TestSetupDataReturnValue(t *testing.T) {
	src := &lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
			export let options = { setupTimeout: "1s", teardownTimeout: "1s" };
			export function setup() {
				return 42;
			}
			export default function(data) {
				if (data != 42) {
					throw new Error("default: wrong data: " + JSON.stringify(data))
				}
			};

			export function teardown(data) {
				if (data != 42) {
					throw new Error("teardown: wrong data: " + JSON.stringify(data))
				}
			};
		`),
	}
	testSetupDataHelper(t, src)
}

func TestSetupDataNoSetup(t *testing.T) {
	src := &lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
			export let options = { setupTimeout: "1s", teardownTimeout: "1s" };
			export default function(data) {
				if (data !== undefined) {
					throw new Error("default: wrong data: " + JSON.stringify(data))
				}
			};

			export function teardown(data) {
				if (data !== undefined) {
					console.log(data);
					throw new Error("teardown: wrong data: " + JSON.stringify(data))
				}
			};
		`),
	}
	testSetupDataHelper(t, src)
}

func TestSetupDataNoReturn(t *testing.T) {
	src := &lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
			export let options = { setupTimeout: "1s", teardownTimeout: "1s" };
			export function setup() { }
			export default function(data) {
				if (data !== undefined) {
					throw new Error("default: wrong data: " + JSON.stringify(data))
				}
			};

			export function teardown(data) {
				if (data !== undefined) {
					throw new Error("teardown: wrong data: " + JSON.stringify(data))
				}
			};
		`),
	}
	testSetupDataHelper(t, src)
}
func TestRunnerIntegrationImports(t *testing.T) {
	t.Run("Modules", func(t *testing.T) {
		modules := []string{
			"k6",
			"k6/http",
			"k6/metrics",
			"k6/html",
		}
		for _, mod := range modules {
			t.Run(mod, func(t *testing.T) {
				t.Run("Source", func(t *testing.T) {
					_, err := New(&lib.SourceData{
						Filename: "/script.js",
						Data:     []byte(fmt.Sprintf(`import "%s"; export default function() {}`, mod)),
					}, afero.NewMemMapFs(), lib.RuntimeOptions{})
					assert.NoError(t, err)
				})
			})
		}
	})

	t.Run("Files", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		assert.NoError(t, fs.MkdirAll("/path/to", 0755))
		assert.NoError(t, afero.WriteFile(fs, "/path/to/lib.js", []byte(`export default "hi!";`), 0644))

		testdata := map[string]struct{ filename, path string }{
			"Absolute":       {"/path/script.js", "/path/to/lib.js"},
			"Relative":       {"/path/script.js", "./to/lib.js"},
			"Adjacent":       {"/path/to/script.js", "./lib.js"},
			"STDIN-Absolute": {"-", "/path/to/lib.js"},
			"STDIN-Relative": {"-", "./path/to/lib.js"},
		}
		for name, data := range testdata {
			t.Run(name, func(t *testing.T) {
				r1, err := New(&lib.SourceData{
					Filename: data.filename,
					Data: []byte(fmt.Sprintf(`
					import hi from "%s";
					export default function() {
						if (hi != "hi!") { throw new Error("incorrect value"); }
					}`, data.path)),
				}, fs, lib.RuntimeOptions{})
				if !assert.NoError(t, err) {
					return
				}

				r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
				if !assert.NoError(t, err) {
					return
				}

				testdata := map[string]*Runner{"Source": r1, "Archive": r2}
				for name, r := range testdata {
					t.Run(name, func(t *testing.T) {
						vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
						if !assert.NoError(t, err) {
							return
						}
						err = vu.RunOnce(context.Background())
						assert.NoError(t, err)
					})
				}
			})
		}
	})
}

func TestVURunContext(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
		export let options = { vus: 10 };
		export default function() { fn(); }
		`),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(r1.GetOptions().Apply(lib.Options{Throw: null.BoolFrom(true)}))

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		t.Run(name, func(t *testing.T) {
			vu, err := r.newVU(make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}

			fnCalled := false
			vu.Runtime.Set("fn", func() {
				fnCalled = true

				assert.Equal(t, vu.Runtime, common.GetRuntime(*vu.Context), "incorrect runtime in context")

				state := common.GetState(*vu.Context)
				if assert.NotNil(t, state) {
					assert.Equal(t, null.IntFrom(10), state.Options.VUs)
					assert.Equal(t, null.BoolFrom(true), state.Options.Throw)
					assert.NotNil(t, state.Logger)
					assert.Equal(t, r.GetDefaultGroup(), state.Group)
					assert.Equal(t, vu.Transport, state.Transport)
				}
			})
			err = vu.RunOnce(context.Background())
			assert.NoError(t, err)
			assert.True(t, fnCalled, "fn() not called")
		})
	}
}

func TestVURunInterrupt(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
		export default function() { while(true) {} }
		`),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)})

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		t.Run(name, func(t *testing.T) {
			vu, err := r.newVU(make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			err = vu.RunOnce(ctx)
			assert.EqualError(t, err, "context cancelled at /script.js:1:1(1)")
		})
	}
}

func TestVUIntegrationGroups(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
		import { group } from "k6";
		export default function() {
			fnOuter();
			group("my group", function() {
				fnInner();
				group("nested group", function() {
					fnNested();
				})
			});
		}
		`),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		t.Run(name, func(t *testing.T) {
			vu, err := r.newVU(make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}

			fnOuterCalled := false
			fnInnerCalled := false
			fnNestedCalled := false
			vu.Runtime.Set("fnOuter", func() {
				fnOuterCalled = true
				assert.Equal(t, r.GetDefaultGroup(), common.GetState(*vu.Context).Group)
			})
			vu.Runtime.Set("fnInner", func() {
				fnInnerCalled = true
				g := common.GetState(*vu.Context).Group
				assert.Equal(t, "my group", g.Name)
				assert.Equal(t, r.GetDefaultGroup(), g.Parent)
			})
			vu.Runtime.Set("fnNested", func() {
				fnNestedCalled = true
				g := common.GetState(*vu.Context).Group
				assert.Equal(t, "nested group", g.Name)
				assert.Equal(t, "my group", g.Parent.Name)
				assert.Equal(t, r.GetDefaultGroup(), g.Parent.Parent)
			})
			err = vu.RunOnce(context.Background())
			assert.NoError(t, err)
			assert.True(t, fnOuterCalled, "fnOuter() not called")
			assert.True(t, fnInnerCalled, "fnInner() not called")
			assert.True(t, fnNestedCalled, "fnNested() not called")
		})
	}
}

func TestVUIntegrationMetrics(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
		import { group } from "k6";
		import { Trend } from "k6/metrics";
		let myMetric = new Trend("my_metric");
		export default function() { myMetric.add(5); }
		`),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		t.Run(name, func(t *testing.T) {
			samples := make(chan stats.SampleContainer, 100)
			vu, err := r.newVU(samples)
			if !assert.NoError(t, err) {
				return
			}

			err = vu.RunOnce(context.Background())
			assert.NoError(t, err)
			sampleCount := 0
			for i, sampleC := range stats.GetBufferedSamples(samples) {
				for j, s := range sampleC.GetSamples() {
					sampleCount++
					switch i + j {
					case 0:
						assert.Equal(t, 5.0, s.Value)
						assert.Equal(t, "my_metric", s.Metric.Name)
						assert.Equal(t, stats.Trend, s.Metric.Type)
					case 1:
						assert.Equal(t, 0.0, s.Value)
						assert.Equal(t, metrics.DataSent, s.Metric, "`data_sent` sample is before `data_received` and `iteration_duration`")
					case 2:
						assert.Equal(t, 0.0, s.Value)
						assert.Equal(t, metrics.DataReceived, s.Metric, "`data_received` sample is after `data_received`")
					case 3:
						assert.Equal(t, metrics.IterationDuration, s.Metric, "`iteration-duration` sample is after `data_received`")
					}
				}
			}
			assert.Equal(t, sampleCount, 4)
		})
	}
}

func TestVUIntegrationInsecureRequests(t *testing.T) {
	testdata := map[string]struct {
		opts   lib.Options
		errMsg string
	}{
		"Null": {
			lib.Options{},
			"GoError: Get https://expired.badssl.com/: x509: certificate has expired or is not yet valid",
		},
		"False": {
			lib.Options{InsecureSkipTLSVerify: null.BoolFrom(false)},
			"GoError: Get https://expired.badssl.com/: x509: certificate has expired or is not yet valid",
		},
		"True": {
			lib.Options{InsecureSkipTLSVerify: null.BoolFrom(true)},
			"",
		},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			r1, err := New(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					import http from "k6/http";
					export default function() { http.get("https://expired.badssl.com/"); }
				`),
			}, afero.NewMemMapFs(), lib.RuntimeOptions{})
			if !assert.NoError(t, err) {
				return
			}
			r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)}.Apply(data.opts))

			r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
			if !assert.NoError(t, err) {
				return
			}

			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for name, r := range runners {
				t.Run(name, func(t *testing.T) {
					r.Logger, _ = logtest.NewNullLogger()

					vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
					if !assert.NoError(t, err) {
						return
					}
					err = vu.RunOnce(context.Background())
					if data.errMsg != "" {
						assert.EqualError(t, err, data.errMsg)
					} else {
						assert.NoError(t, err)
					}
				})
			}
		})
	}
}

func TestVUIntegrationBlacklist(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
					import http from "k6/http";
					export default function() { http.get("http://10.1.2.3/"); }
				`),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	_, cidr, err := net.ParseCIDR("10.0.0.0/8")
	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(lib.Options{
		Throw:        null.BoolFrom(true),
		BlacklistIPs: []*net.IPNet{cidr},
	})

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		t.Run(name, func(t *testing.T) {
			vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}
			err = vu.RunOnce(context.Background())
			assert.EqualError(t, err, "GoError: Get http://10.1.2.3/: IP (10.1.2.3) is in a blacklisted range (10.0.0.0/8)")
		})
	}
}

func TestVUIntegrationHosts(t *testing.T) {
	tb := testutils.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(tb.Replacer.Replace(`
					import { check, fail } from "k6";
					import http from "k6/http";
					export default function() {
						let res = http.get("http://test.loadimpact.com:HTTPBIN_PORT/");
						check(res, {
							"is correct IP": (r) => r.remote_ip === "127.0.0.1"
						}) || fail("failed to override dns");
					}
				`)),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	r1.SetOptions(lib.Options{
		Throw: null.BoolFrom(true),
		Hosts: map[string]net.IP{
			"test.loadimpact.com": net.ParseIP("127.0.0.1"),
		},
	})

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		t.Run(name, func(t *testing.T) {
			vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}

			err = vu.RunOnce(context.Background())
			if !assert.NoError(t, err) {
				return
			}
		})
	}
}

func TestVUIntegrationTLSConfig(t *testing.T) {
	testdata := map[string]struct {
		opts   lib.Options
		errMsg string
	}{
		"NullCipherSuites": {
			lib.Options{},
			"",
		},
		"SupportedCipherSuite": {
			lib.Options{TLSCipherSuites: &lib.TLSCipherSuites{tls.TLS_RSA_WITH_AES_128_GCM_SHA256}},
			"",
		},
		"UnsupportedCipherSuite": {
			lib.Options{TLSCipherSuites: &lib.TLSCipherSuites{tls.TLS_RSA_WITH_RC4_128_SHA}},
			"GoError: Get https://sha256.badssl.com/: remote error: tls: handshake failure",
		},
		"NullVersion": {
			lib.Options{},
			"",
		},
		"SupportedVersion": {
			lib.Options{TLSVersion: &lib.TLSVersions{Min: tls.VersionTLS12, Max: tls.VersionTLS12}},
			"",
		},
		"UnsupportedVersion": {
			lib.Options{TLSVersion: &lib.TLSVersions{Min: tls.VersionSSL30, Max: tls.VersionSSL30}},
			"GoError: Get https://sha256.badssl.com/: remote error: tls: handshake failure",
		},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			r1, err := New(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					import http from "k6/http";
					export default function() { http.get("https://sha256.badssl.com/"); }
				`),
			}, afero.NewMemMapFs(), lib.RuntimeOptions{})
			if !assert.NoError(t, err) {
				return
			}
			r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)}.Apply(data.opts))

			r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
			if !assert.NoError(t, err) {
				return
			}

			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for name, r := range runners {
				t.Run(name, func(t *testing.T) {
					r.Logger, _ = logtest.NewNullLogger()

					vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
					if !assert.NoError(t, err) {
						return
					}
					err = vu.RunOnce(context.Background())
					if data.errMsg != "" {
						assert.EqualError(t, err, data.errMsg)
					} else {
						assert.NoError(t, err)
					}
				})
			}
		})
	}
}

func TestVUIntegrationHTTP2(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
			import http from "k6/http";
			export default function() {
				let res = http.request("GET", "https://http2.akamai.com/demo");
				if (res.status != 200) { throw new Error("wrong status: " + res.status) }
				if (res.proto != "HTTP/2.0") { throw new Error("wrong proto: " + res.proto) }
			}
		`),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(lib.Options{
		Throw:      null.BoolFrom(true),
		SystemTags: lib.GetTagSet("proto"),
	})

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		t.Run(name, func(t *testing.T) {
			samples := make(chan stats.SampleContainer, 100)
			vu, err := r.NewVU(samples)
			if !assert.NoError(t, err) {
				return
			}
			err = vu.RunOnce(context.Background())
			assert.NoError(t, err)

			protoFound := false
			for _, sampleC := range stats.GetBufferedSamples(samples) {
				for _, sample := range sampleC.GetSamples() {
					if proto, ok := sample.Tags.Get("proto"); ok {
						protoFound = true
						assert.Equal(t, "HTTP/2.0", proto)
					}
				}
			}
			assert.True(t, protoFound)
		})
	}
}

func TestVUIntegrationOpenFunctionError(t *testing.T) {
	r, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
			export default function() { open("/tmp/foo") }
		`),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	assert.NoError(t, err)

	vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
	assert.NoError(t, err)
	err = vu.RunOnce(context.Background())
	assert.EqualError(t, err, "GoError: \"open\" function is only available to the init code (aka global scope), see https://docs.k6.io/docs/test-life-cycle for more information")
}

func TestVUIntegrationCookiesReset(t *testing.T) {

	tb := testutils.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(tb.Replacer.Replace(`
			import http from "k6/http";
			export default function() {
				let url = "HTTPBIN_URL";
				let preRes = http.get(url + "/cookies");
				if (preRes.status != 200) { throw new Error("wrong status (pre): " + preRes.status); }
				if (preRes.json().k1 || preRes.json().k2) {
					throw new Error("cookies persisted: " + preRes.body);
				}

				let res = http.get(url + "/cookies/set?k2=v2&k1=v1");
				if (res.status != 200) { throw new Error("wrong status: " + res.status) }
				if (res.json().k1 != "v1" || res.json().k2 != "v2") {
					throw new Error("wrong cookies: " + res.body);
				}
			}
		`)),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(lib.Options{
		Throw:        null.BoolFrom(true),
		MaxRedirects: null.IntFrom(10),
		Hosts:        tb.Dialer.Hosts,
	})

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		t.Run(name, func(t *testing.T) {
			vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}
			for i := 0; i < 2; i++ {
				err = vu.RunOnce(context.Background())
				assert.NoError(t, err)
			}
		})
	}
}

func TestVUIntegrationCookiesNoReset(t *testing.T) {
	tb := testutils.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(tb.Replacer.Replace(`
			import http from "k6/http";
			export default function() {
				let url = "HTTPBIN_URL";
				if (__ITER == 0) {
					let res = http.get(url + "/cookies/set?k2=v2&k1=v1");
					if (res.status != 200) { throw new Error("wrong status: " + res.status) }
					if (res.json().k1 != "v1" || res.json().k2 != "v2") {
						throw new Error("wrong cookies: " + res.body);
					}
				}

				if (__ITER == 1) {
					let res = http.get(url + "/cookies");
					if (res.status != 200) { throw new Error("wrong status (pre): " + res.status); }
					if (res.json().k1 != "v1" || res.json().k2 != "v2") {
						throw new Error("wrong cookies: " + res.body);
					}
				}
			}
		`)),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(lib.Options{
		Throw:          null.BoolFrom(true),
		MaxRedirects:   null.IntFrom(10),
		Hosts:          tb.Dialer.Hosts,
		NoCookiesReset: null.BoolFrom(true),
	})

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		t.Run(name, func(t *testing.T) {
			vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}

			err = vu.RunOnce(context.Background())
			assert.NoError(t, err)

			err = vu.RunOnce(context.Background())
			assert.NoError(t, err)
		})
	}
}

func TestVUIntegrationVUID(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
			export default function() {
				if (__VU != 1234) { throw new Error("wrong __VU: " + __VU); }
			}`,
		),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)})

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		t.Run(name, func(t *testing.T) {
			vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}
			assert.NoError(t, vu.Reconfigure(1234))
			err = vu.RunOnce(context.Background())
			assert.NoError(t, err)
		})
	}
}

func TestVUIntegrationClientCerts(t *testing.T) {
	clientCAPool := x509.NewCertPool()
	assert.True(t, clientCAPool.AppendCertsFromPEM(
		[]byte("-----BEGIN CERTIFICATE-----\n"+
			"MIIBYzCCAQqgAwIBAgIUMYw1pqZ1XhXdFG0S2ITXhfHBsWgwCgYIKoZIzj0EAwIw\n"+
			"EDEOMAwGA1UEAxMFTXkgQ0EwHhcNMTcwODE1MTYxODAwWhcNMjIwODE0MTYxODAw\n"+
			"WjAQMQ4wDAYDVQQDEwVNeSBDQTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABFWO\n"+
			"fg4dgL8cdvjoSWDQFLBJxlbQFlZfOSyUR277a4g91BD07KWX+9ny+Q8WuUODog06\n"+
			"xH1g8fc6zuaejllfzM6jQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTAD\n"+
			"AQH/MB0GA1UdDgQWBBTeoSFylGCmyqj1X4sWez1r6hkhjDAKBggqhkjOPQQDAgNH\n"+
			"ADBEAiAfuKi6u/BVXenCkgnU2sfXsYjel6rACuXEcx01yaaWuQIgXAtjrDisdlf4\n"+
			"0ZdoIoYjNhDAXUtnyRBt+V6+rIklv/8=\n"+
			"-----END CERTIFICATE-----"),
	))
	serverCert, err := tls.X509KeyPair(
		[]byte("-----BEGIN CERTIFICATE-----\n"+
			"MIIBxjCCAW2gAwIBAgIUICcYHG1bI28NZm676wHlMPxL+CEwCgYIKoZIzj0EAwIw\n"+
			"EDEOMAwGA1UEAxMFTXkgQ0EwHhcNMTcwODE3MTQwNjAwWhcNMTgwODE3MTQwNjAw\n"+
			"WjAZMRcwFQYDVQQDEw4xMjcuMC4wLjE6Njk2OTBZMBMGByqGSM49AgEGCCqGSM49\n"+
			"AwEHA0IABCdD1IqowucJ5oUjGYCZZnXvgi7EMD4jD1osbOkzOFFnHSLRvdm6fcJu\n"+
			"vPUcl4g8zUs466sC0AVUNpk21XbA/QajgZswgZgwDgYDVR0PAQH/BAQDAgWgMB0G\n"+
			"A1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMB0GA1Ud\n"+
			"DgQWBBTeAc8HY3sgGIV+fu/lY0OKr2Ho0jAfBgNVHSMEGDAWgBTeoSFylGCmyqj1\n"+
			"X4sWez1r6hkhjDAZBgNVHREEEjAQgg4xMjcuMC4wLjE6Njk2OTAKBggqhkjOPQQD\n"+
			"AgNHADBEAiAt3gC5FGQfSJXQ5DloXAOeJDFnKIL7d6xhftgPS5O08QIgRuAyysB8\n"+
			"5JXHvvze5DMN/clHYptos9idVFc+weUZAUQ=\n"+
			"-----END CERTIFICATE-----\n"+
			"-----BEGIN CERTIFICATE-----\n"+
			"MIIBYzCCAQqgAwIBAgIUMYw1pqZ1XhXdFG0S2ITXhfHBsWgwCgYIKoZIzj0EAwIw\n"+
			"EDEOMAwGA1UEAxMFTXkgQ0EwHhcNMTcwODE1MTYxODAwWhcNMjIwODE0MTYxODAw\n"+
			"WjAQMQ4wDAYDVQQDEwVNeSBDQTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABFWO\n"+
			"fg4dgL8cdvjoSWDQFLBJxlbQFlZfOSyUR277a4g91BD07KWX+9ny+Q8WuUODog06\n"+
			"xH1g8fc6zuaejllfzM6jQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTAD\n"+
			"AQH/MB0GA1UdDgQWBBTeoSFylGCmyqj1X4sWez1r6hkhjDAKBggqhkjOPQQDAgNH\n"+
			"ADBEAiAfuKi6u/BVXenCkgnU2sfXsYjel6rACuXEcx01yaaWuQIgXAtjrDisdlf4\n"+
			"0ZdoIoYjNhDAXUtnyRBt+V6+rIklv/8=\n"+
			"-----END CERTIFICATE-----"),
		[]byte("-----BEGIN EC PRIVATE KEY-----\n"+
			"MHcCAQEEIKYptA4VtQ8UOKL+d1wkhl+51aPpvO+ppY62nLF9Z1w5oAoGCCqGSM49\n"+
			"AwEHoUQDQgAEJ0PUiqjC5wnmhSMZgJlmde+CLsQwPiMPWixs6TM4UWcdItG92bp9\n"+
			"wm689RyXiDzNSzjrqwLQBVQ2mTbVdsD9Bg==\n"+
			"-----END EC PRIVATE KEY-----"),
	)
	if !assert.NoError(t, err) {
		return
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAPool,
	})
	if !assert.NoError(t, err) {
		return
	}
	defer func() { _ = listener.Close() }()
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, _ = fmt.Fprintf(w, "ok")
		}),
		ErrorLog: stdlog.New(ioutil.Discard, "", 0),
	}
	go func() { _ = srv.Serve(listener) }()

	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(fmt.Sprintf(`
			import http from "k6/http";
			export default function() { http.get("https://%s")}
		`, listener.Addr().String())),
	}, afero.NewMemMapFs(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(lib.Options{
		Throw: null.BoolFrom(true),
		InsecureSkipTLSVerify: null.BoolFrom(true),
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
		if !assert.NoError(t, err) {
			return
		}

		runners := map[string]*Runner{"Source": r1, "Archive": r2}
		for name, r := range runners {
			t.Run(name, func(t *testing.T) {
				r.Logger, _ = logtest.NewNullLogger()
				vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
				if assert.NoError(t, err) {
					err := vu.RunOnce(context.Background())
					assert.EqualError(t, err, fmt.Sprintf("GoError: Get https://%s: remote error: tls: bad certificate", listener.Addr().String()))
				}
			})
		}
	})

	r1.SetOptions(lib.Options{
		TLSAuth: []*lib.TLSAuth{
			{
				TLSAuthFields: lib.TLSAuthFields{
					Domains: []string{"127.0.0.1"},
					Cert: "-----BEGIN CERTIFICATE-----\n" +
						"MIIBoTCCAUigAwIBAgIUd6XedDxP+rGo+kq0APqHElGZzs4wCgYIKoZIzj0EAwIw\n" +
						"EDEOMAwGA1UEAxMFTXkgQ0EwHhcNMTcwODE3MTUwNjAwWhcNMTgwODE3MTUwNjAw\n" +
						"WjARMQ8wDQYDVQQDEwZjbGllbnQwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATL\n" +
						"mi/a1RVvk05FyrYmartbo/9cW+53DrQLW1twurII2q5ZfimdMX05A32uB3Ycoy/J\n" +
						"x+w7Ifyd/YRw0zEc3NHQo38wfTAOBgNVHQ8BAf8EBAMCBaAwHQYDVR0lBBYwFAYI\n" +
						"KwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwHQYDVR0OBBYEFN2SR/TD\n" +
						"yNW5DQWxZSkoXHQWsLY+MB8GA1UdIwQYMBaAFN6hIXKUYKbKqPVfixZ7PWvqGSGM\n" +
						"MAoGCCqGSM49BAMCA0cAMEQCICtETmyOmupmg4w3tw59VYJyOBqRTxg6SK+rOQmq\n" +
						"kE1VAiAUvsflDfmWBZ8EMPu46OhX6RX6MbvJ9NNvRco2G5ek1w==\n" +
						"-----END CERTIFICATE-----",
					Key: "-----BEGIN EC PRIVATE KEY-----\n" +
						"MHcCAQEEIOrnhT05alCeQEX66HgnSHah/m5LazjJHLDawYRnhUtZoAoGCCqGSM49\n" +
						"AwEHoUQDQgAEy5ov2tUVb5NORcq2Jmq7W6P/XFvudw60C1tbcLqyCNquWX4pnTF9\n" +
						"OQN9rgd2HKMvycfsOyH8nf2EcNMxHNzR0A==\n" +
						"-----END EC PRIVATE KEY-----",
				},
			},
		},
	})

	t.Run("Authenticated", func(t *testing.T) {
		r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
		if !assert.NoError(t, err) {
			return
		}

		runners := map[string]*Runner{"Source": r1, "Archive": r2}
		for name, r := range runners {
			t.Run(name, func(t *testing.T) {
				vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
				if assert.NoError(t, err) {
					err := vu.RunOnce(context.Background())
					assert.NoError(t, err)
				}
			})
		}
	})
}
