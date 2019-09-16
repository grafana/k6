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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"go/build"
	"io/ioutil"
	stdlog "log"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/loadimpact/k6/core"
	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/modules/k6"
	k6http "github.com/loadimpact/k6/js/modules/k6/http"
	k6metrics "github.com/loadimpact/k6/js/modules/k6/metrics"
	"github.com/loadimpact/k6/js/modules/k6/ws"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
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
		r, err := getSimpleRunner("/script.js", `
			let counter = 0;
			export default function() { counter++; }
		`)
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
		_, err := getSimpleRunner("/script.js", `blarg`)
		assert.EqualError(t, err, "ReferenceError: blarg is not defined at file:///script.js:1:1(0)")
	})
}

func TestRunnerGetDefaultGroup(t *testing.T) {
	r1, err := getSimpleRunner("/script.js", `export default function() {};`)
	if assert.NoError(t, err) {
		assert.NotNil(t, r1.GetDefaultGroup())
	}

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if assert.NoError(t, err) {
		assert.NotNil(t, r2.GetDefaultGroup())
	}
}

func TestRunnerOptions(t *testing.T) {
	r1, err := getSimpleRunner("/script.js", `export default function() {};`)
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
			data := variant + `
					export default function() {
						if (!options) {
							throw new Error("Expected options to be defined!");
						}
						if (options.teardownTimeout != __ENV.expectedTeardownTimeout) {
							throw new Error("expected teardownTimeout to be " + __ENV.expectedTeardownTimeout + " but it was " + options.teardownTimeout);
						}
					};`
			r, err := getSimpleRunnerWithOptions("/script.js", data,
				lib.RuntimeOptions{Env: map[string]string{"expectedTeardownTimeout": "4s"}})
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
	data := `
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
			};`

	expScriptOptions := lib.Options{SetupTimeout: types.NullDurationFrom(1 * time.Second)}
	r1, err := getSimpleRunnerWithOptions("/script.js", data,
		lib.RuntimeOptions{Env: map[string]string{"expectedSetupTimeout": "1s"}})
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

func TestMetricName(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	script := tb.Replacer.Replace(`
		import { Counter } from "k6/metrics";

		let myCounter = new Counter("not ok name @");

		export default function(data) {
			myCounter.add(1);
		}
	`)

	_, err := getSimpleRunner("/script.js", script)
	require.Error(t, err)
}

func TestSetupDataIsolation(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	script := tb.Replacer.Replace(`
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
	`)

	runner, err := getSimpleRunner("/script.js", script)
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

func testSetupDataHelper(t *testing.T, data string) {
	t.Helper()
	expScriptOptions := lib.Options{
		SetupTimeout:    types.NullDurationFrom(1 * time.Second),
		TeardownTimeout: types.NullDurationFrom(1 * time.Second),
	}
	r1, err := getSimpleRunner("/script.js", data) // TODO fix this
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
	testSetupDataHelper(t, `
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
	};`)
}

func TestSetupDataNoSetup(t *testing.T) {
	testSetupDataHelper(t, `
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
	};`)
}

func TestConsoleInInitContext(t *testing.T) {
	r1, err := getSimpleRunner("/script.js", `
			console.log("1");
			export default function(data) {
			};
		`)
	require.NoError(t, err)

	testdata := map[string]*Runner{"Source": r1}
	for name, r := range testdata {
		r := r
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

func TestSetupDataNoReturn(t *testing.T) {
	testSetupDataHelper(t, `
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
	};`)
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
			mod := mod
			t.Run(mod, func(t *testing.T) {
				t.Run("Source", func(t *testing.T) {
					_, err := getSimpleRunner("/script.js", fmt.Sprintf(`import "%s"; export default function() {}`, mod))
					assert.NoError(t, err)
				})
			})
		}
	})

	t.Run("Files", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		require.NoError(t, fs.MkdirAll("/path/to", 0755))
		require.NoError(t, afero.WriteFile(fs, "/path/to/lib.js", []byte(`export default "hi!";`), 0644))

		testdata := map[string]struct{ filename, path string }{
			"Absolute":       {"/path/script.js", "/path/to/lib.js"},
			"Relative":       {"/path/script.js", "./to/lib.js"},
			"Adjacent":       {"/path/to/script.js", "./lib.js"},
			"STDIN-Absolute": {"-", "/path/to/lib.js"},
			"STDIN-Relative": {"-", "./path/to/lib.js"},
		}
		for name, data := range testdata {
			name, data := name, data
			t.Run(name, func(t *testing.T) {
				r1, err := getSimpleRunnerWithFileFs(data.filename, fmt.Sprintf(`
					import hi from "%s";
					export default function() {
						if (hi != "hi!") { throw new Error("incorrect value"); }
					}`, data.path), fs)
				require.NoError(t, err)

				r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
				require.NoError(t, err)

				testdata := map[string]*Runner{"Source": r1, "Archive": r2}
				for name, r := range testdata {
					r := r
					t.Run(name, func(t *testing.T) {
						vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
						require.NoError(t, err)
						err = vu.RunOnce(context.Background())
						require.NoError(t, err)
					})
				}
			})
		}
	})
}

func TestVURunContext(t *testing.T) {
	r1, err := getSimpleRunner("/script.js", `
		export let options = { vus: 10 };
		export default function() { fn(); }
		`)
	require.NoError(t, err)
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

				state := lib.GetState(*vu.Context)
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
	//TODO: figure out why interrupt sometimes fails... data race in goja?
	if isWindows {
		t.Skip()
	}

	r1, err := getSimpleRunner("/script.js", `
		export default function() { while(true) {} }
		`)
	require.NoError(t, err)
	require.NoError(t, r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)}))

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	require.NoError(t, err)
	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		name, r := name, r
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			samples := make(chan stats.SampleContainer, 100)
			defer close(samples)
			go func() {
				for range samples {
				}
			}()

			vu, err := r.newVU(samples)
			require.NoError(t, err)

			err = vu.RunOnce(ctx)
			assert.Error(t, err)
			assert.True(t, strings.HasPrefix(err.Error(), "context cancelled at "))
		})
	}
}

func TestVUIntegrationGroups(t *testing.T) {
	r1, err := getSimpleRunner("/script.js", `
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
		`)
	require.NoError(t, err)

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	require.NoError(t, err)

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		r := r
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
				assert.Equal(t, r.GetDefaultGroup(), lib.GetState(*vu.Context).Group)
			})
			vu.Runtime.Set("fnInner", func() {
				fnInnerCalled = true
				g := lib.GetState(*vu.Context).Group
				assert.Equal(t, "my group", g.Name)
				assert.Equal(t, r.GetDefaultGroup(), g.Parent)
			})
			vu.Runtime.Set("fnNested", func() {
				fnNestedCalled = true
				g := lib.GetState(*vu.Context).Group
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
	r1, err := getSimpleRunner("/script.js", `
		import { group } from "k6";
		import { Trend } from "k6/metrics";
		let myMetric = new Trend("my_metric");
		export default function() { myMetric.add(5); }
		`)
	require.NoError(t, err)

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	require.NoError(t, err)

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
			r1, err := getSimpleRunner("/script.js", `
					import http from "k6/http";
					export default function() { http.get("https://expired.badssl.com/"); }
				`)
			require.NoError(t, err)
			r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)}.Apply(data.opts))

			r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
			require.NoError(t, err)
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

func TestVUIntegrationBlacklistOption(t *testing.T) {
	r1, err := getSimpleRunner("/script.js", `
					import http from "k6/http";
					export default function() { http.get("http://10.1.2.3/"); }
				`)
	require.NoError(t, err)

	cidr, err := lib.ParseCIDR("10.0.0.0/8")

	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(lib.Options{
		Throw:        null.BoolFrom(true),
		BlacklistIPs: []*lib.IPNet{cidr},
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

func TestVUIntegrationBlacklistScript(t *testing.T) {
	r1, err := getSimpleRunner("/script.js", `
					import http from "k6/http";

					export let options = {
						throw: true,
						blacklistIPs: ["10.0.0.0/8"],
					};

					export default function() { http.get("http://10.1.2.3/"); }
				`)
	if !assert.NoError(t, err) {
		return
	}

	r2, err := NewFromArchive(r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}

	for name, r := range runners {
		r := r
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
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r1, err := getSimpleRunner("/script.js",
		tb.Replacer.Replace(`
					import { check, fail } from "k6";
					import http from "k6/http";
					export default function() {
						let res = http.get("http://test.loadimpact.com:HTTPBIN_PORT/");
						check(res, {
							"is correct IP": (r) => r.remote_ip === "127.0.0.1"
						}) || fail("failed to override dns");
					}
				`))
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
	var unsupportedVersionErrorMsg = "remote error: tls: handshake failure"
	for _, tag := range build.Default.ReleaseTags {
		if tag == "go1.12" {
			unsupportedVersionErrorMsg = "tls: no supported versions satisfy MinVersion and MaxVersion"
			break
		}
	}
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
			"GoError: Get https://sha256.badssl.com/: " + unsupportedVersionErrorMsg,
		},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			r1, err := getSimpleRunner("/script.js", `
					import http from "k6/http";
					export default function() { http.get("https://sha256.badssl.com/"); }
				`)
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
	r1, err := getSimpleRunner("/script.js", `
			import http from "k6/http";
			export default function() {
				let res = http.request("GET", "https://http2.akamai.com/demo");
				if (res.status != 200) { throw new Error("wrong status: " + res.status) }
				if (res.proto != "HTTP/2.0") { throw new Error("wrong proto: " + res.proto) }
			}
		`)
	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(lib.Options{
		Throw:      null.BoolFrom(true),
		SystemTags: stats.ToSystemTagSet([]string{stats.TagProto.String()}),
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
	r, err := getSimpleRunner("/script.js", `
			export default function() { open("/tmp/foo") }
		`)
	assert.NoError(t, err)

	vu, err := r.NewVU(make(chan stats.SampleContainer, 100))
	assert.NoError(t, err)
	err = vu.RunOnce(context.Background())
	assert.EqualError(t, err, "GoError: \"open\" function is only available to the init code (aka global scope), see https://docs.k6.io/docs/test-life-cycle for more information")
}

func TestVUIntegrationCookiesReset(t *testing.T) {

	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r1, err := getSimpleRunner("/script.js", tb.Replacer.Replace(`
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
		`))
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
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r1, err := getSimpleRunner("/script.js", tb.Replacer.Replace(`
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
		`))
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
	r1, err := getSimpleRunner("/script.js", `
			export default function() {
				if (__VU != 1234) { throw new Error("wrong __VU: " + __VU); }
			}`,
	)
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

	r1, err := getSimpleRunner("/script.js", fmt.Sprintf(`
			import http from "k6/http";
			export default function() { http.get("https://%s")}
		`, listener.Addr().String()))
	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(lib.Options{
		Throw:                 null.BoolFrom(true),
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
					require.NotNil(t, err)
					assert.Contains(t, err.Error(), "remote error: tls: bad certificate")
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

func TestHTTPRequestInInitContext(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	_, err := getSimpleRunner("/script.js", tb.Replacer.Replace(`
					import { check, fail } from "k6";
					import http from "k6/http";
					let res = http.get("HTTPBIN_URL/");
					export default function() {
						console.log(test);
					}
				`))
	if assert.Error(t, err) {
		assert.Equal(
			t,
			"GoError: "+k6http.ErrHTTPForbiddenInInitContext.Error(),
			err.Error())
	}
}

func TestInitContextForbidden(t *testing.T) {
	var table = [...][3]string{
		{
			"http.request",
			`import http from "k6/http";
			 let res = http.get("HTTPBIN_URL");
			 export default function() { console.log("p"); }`,
			k6http.ErrHTTPForbiddenInInitContext.Error(),
		},
		{
			"http.batch",
			`import http from "k6/http";
			 let res = http.batch("HTTPBIN_URL/something", "HTTPBIN_URL/else");
			 export default function() { console.log("p"); }`,
			k6http.ErrBatchForbiddenInInitContext.Error(),
		},
		{
			"http.cookieJar",
			`import http from "k6/http";
			 let jar = http.cookieJar();
			 export default function() { console.log("p"); }`,
			k6http.ErrJarForbiddenInInitContext.Error(),
		},
		{
			"check",
			`import { check } from "k6";
			 check("test", {'is test': (test) => test == "test"})
			 export default function() { console.log("p"); }`,
			k6.ErrCheckInInitContext.Error(),
		},
		{
			"group",
			`import { group } from "k6";
			 group("group1", function () { console.log("group1");})
			 export default function() { console.log("p"); }`,
			k6.ErrGroupInInitContext.Error(),
		},
		{
			"ws",
			`import ws from "k6/ws";
			 var url = "ws://echo.websocket.org";
			 var params = { "tags": { "my_tag": "hello" } };
			 var response = ws.connect(url, params, function (socket) {
			   socket.on('open', function open() {
					console.log('connected');
			   })
		   });

			 export default function() { console.log("p"); }`,
			ws.ErrWSInInitContext.Error(),
		},
		{
			"metric",
			`import { Counter } from "k6/metrics";
			 let counter = Counter("myCounter");
			 counter.add(1);
			 export default function() { console.log("p"); }`,
			k6metrics.ErrMetricsAddInInitContext.Error(),
		},
	}
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	for _, test := range table {
		test := test
		t.Run(test[0], func(t *testing.T) {
			_, err := getSimpleRunner("/script.js", tb.Replacer.Replace(test[1]))
			if assert.Error(t, err) {
				assert.Equal(
					t,
					"GoError: "+test[2],
					err.Error())
			}
		})
	}
}

func TestArchiveRunningIntegraty(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	fs := afero.NewMemMapFs()
	data := tb.Replacer.Replace(`
			let fput = open("/home/somebody/test.json");
			export let options = { setupTimeout: "10s", teardownTimeout: "10s" };
			export function setup() {
				return JSON.parse(fput);
			}
			export default function(data) {
				if (data != 42) {
					throw new Error("incorrect answer " + data);
				}
			}
		`)
	require.NoError(t, afero.WriteFile(fs, "/home/somebody/test.json", []byte(`42`), os.ModePerm))
	require.NoError(t, afero.WriteFile(fs, "/script.js", []byte(data), os.ModePerm))
	r1, err := getSimpleRunnerWithFileFs("/script.js", data, fs)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	require.NoError(t, r1.MakeArchive().Write(buf))

	arc, err := lib.ReadArchive(buf)
	require.NoError(t, err)
	r2, err := NewFromArchive(arc, lib.RuntimeOptions{})
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		t.Run(name, func(t *testing.T) {
			ch := make(chan stats.SampleContainer, 100)
			err = r.Setup(context.Background(), ch)
			require.NoError(t, err)
			vu, err := r.NewVU(ch)
			require.NoError(t, err)
			err = vu.RunOnce(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestArchiveNotPanicking(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/non/existent", []byte(`42`), os.ModePerm))
	r1, err := getSimpleRunnerWithFileFs("/script.js", tb.Replacer.Replace(`
			let fput = open("/non/existent");
			export default function(data) {
			}
		`), fs)
	require.NoError(t, err)

	arc := r1.MakeArchive()
	arc.Filesystems = map[string]afero.Fs{"file": afero.NewMemMapFs()}
	r2, err := NewFromArchive(arc, lib.RuntimeOptions{})
	// we do want this to error here as this is where we find out that a given file is not in the
	// archive
	require.Error(t, err)
	require.Nil(t, r2)
}

func TestStuffNotPanicking(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r, err := getSimpleRunner("/script.js", tb.Replacer.Replace(`
			import http from "k6/http";
			import ws from "k6/ws";
			import { group } from "k6";
			import { parseHTML } from "k6/html";

			export let options = { iterations: 1, vus: 1, vusMax: 1 };

			export default function() {
				const doc = parseHTML(http.get("HTTPBIN_URL/html").body);

				let testCases = [
					() => group(),
					() => group("test"),
					() => group("test", "wat"),
					() => doc.find('p').each(),
					() => doc.find('p').each("wat"),
					() => doc.find('p').map(),
					() => doc.find('p').map("wat"),
					() => ws.connect("WSBIN_URL/ws-echo"),
				];

				testCases.forEach(function(fn, idx) {
					var hasException;
					try {
						fn();
						hasException = false;
					} catch (e) {
						hasException = true;
					}

					if (hasException === false) {
						throw new Error("Expected test case #" + idx + " to return an error");
					} else if (hasException === undefined) {
						throw new Error("Something strange happened with test case #" + idx);
					}
				});
			}
		`))
	require.NoError(t, err)

	ch := make(chan stats.SampleContainer, 1000)
	vu, err := r.NewVU(ch)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errC := make(chan error)
	go func() { errC <- vu.RunOnce(ctx) }()

	select {
	case <-time.After(15 * time.Second):
		cancel()
		t.Fatal("Test timed out")
	case err := <-errC:
		cancel()
		require.NoError(t, err)
	}
}
