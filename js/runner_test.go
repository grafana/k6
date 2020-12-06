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
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/test/grpc_testing"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/core"
	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/modules/k6"
	k6http "github.com/loadimpact/k6/js/modules/k6/http"
	k6metrics "github.com/loadimpact/k6/js/modules/k6/metrics"
	"github.com/loadimpact/k6/js/modules/k6/ws"
	"github.com/loadimpact/k6/lib"
	_ "github.com/loadimpact/k6/lib/executor" // TODO: figure out something better
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/dummy"
)

func TestRunnerNew(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		r, err := getSimpleRunner(t, "/script.js", `
			var counter = 0;
			exports.default = function() { counter++; }
		`)
		assert.NoError(t, err)

		t.Run("NewVU", func(t *testing.T) {
			initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
			assert.NoError(t, err)
			vuc, ok := initVU.(*VU)
			assert.True(t, ok)
			assert.Equal(t, int64(0), vuc.Runtime.Get("counter").Export())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			t.Run("RunOnce", func(t *testing.T) {
				err = vu.RunOnce()
				assert.NoError(t, err)
				assert.Equal(t, int64(1), vuc.Runtime.Get("counter").Export())
			})
		})
	})

	t.Run("Invalid", func(t *testing.T) {
		_, err := getSimpleRunner(t, "/script.js", `blarg`)
		assert.EqualError(t, err, "ReferenceError: blarg is not defined at file:///script.js:1:1(0)")
	})
}

func TestRunnerGetDefaultGroup(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `exports.default = function() {};`)
	if assert.NoError(t, err) {
		assert.NotNil(t, r1.GetDefaultGroup())
	}

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	if assert.NoError(t, err) {
		assert.NotNil(t, r2.GetDefaultGroup())
	}
}

func TestRunnerOptions(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `exports.default = function() {};`)
	if !assert.NoError(t, err) {
		return
	}

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
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
		"var options = null;",
		"var options = undefined;",
		"var options = {};",
		"var options = {teardownTimeout: '1s'};",
	}

	for i, variant := range optionVariants {
		variant := variant
		t.Run(fmt.Sprintf("Variant#%d", i), func(t *testing.T) {
			t.Parallel()
			data := variant + `
					exports.default = function() {
						if (!options) {
							throw new Error("Expected options to be defined!");
						}
						if (options.teardownTimeout != __ENV.expectedTeardownTimeout) {
							throw new Error("expected teardownTimeout to be " + __ENV.expectedTeardownTimeout + " but it was " + options.teardownTimeout);
						}
					};`
			r, err := getSimpleRunner(t, "/script.js", data,
				lib.RuntimeOptions{Env: map[string]string{"expectedTeardownTimeout": "4s"}})
			require.NoError(t, err)

			newOptions := lib.Options{TeardownTimeout: types.NullDurationFrom(4 * time.Second)}
			r.SetOptions(newOptions)
			require.Equal(t, newOptions, r.GetOptions())

			samples := make(chan stats.SampleContainer, 100)
			initVU, err := r.NewVU(1, samples)
			if assert.NoError(t, err) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
				err := vu.RunOnce()
				assert.NoError(t, err)
			}
		})
	}
}

func TestOptionsPropagationToScript(t *testing.T) {
	t.Parallel()
	data := `
			var options = { setupTimeout: "1s", myOption: "test" };
			exports.options = options;
			exports.default = function() {
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
	r1, err := getSimpleRunner(t, "/script.js", data,
		lib.RuntimeOptions{Env: map[string]string{"expectedSetupTimeout": "1s"}})
	require.NoError(t, err)
	require.Equal(t, expScriptOptions, r1.GetOptions())

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{Env: map[string]string{"expectedSetupTimeout": "3s"}})

	require.NoError(t, err)
	require.Equal(t, expScriptOptions, r2.GetOptions())

	newOptions := lib.Options{SetupTimeout: types.NullDurationFrom(3 * time.Second)}
	require.NoError(t, r2.SetOptions(newOptions))
	require.Equal(t, newOptions, r2.GetOptions())

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		r := r
		t.Run(name, func(t *testing.T) {
			samples := make(chan stats.SampleContainer, 100)

			initVU, err := r.NewVU(1, samples)
			if assert.NoError(t, err) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
				err := vu.RunOnce()
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricName(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	script := tb.Replacer.Replace(`
		var Counter = require("k6/metrics").Counter;

		var myCounter = new Counter("not ok name @");

		exports.default = function(data) {
			myCounter.add(1);
		}
	`)

	_, err := getSimpleRunner(t, "/script.js", script)
	require.Error(t, err)
}

func TestSetupDataIsolation(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	script := tb.Replacer.Replace(`
		var Counter = require("k6/metrics").Counter;

		exports.options = {
			scenarios: {
				shared_iters: {
					executor: "shared-iterations",
					vus: 5,
					iterations: 500,
				},
			},
			teardownTimeout: "5s",
			setupTimeout: "5s",
		};
		var myCounter = new Counter("mycounter");

		exports.setup = function() {
			return { v: 0 };
		}

		exports.default = function(data) {
			if (data.v !== __ITER) {
				throw new Error("default: wrong data for iter " + __ITER + ": " + JSON.stringify(data));
			}
			data.v += 1;
			myCounter.add(1);
		}

		exports.teardown = function(data) {
			if (data.v !== 0) {
				throw new Error("teardown: wrong data: " + data.v);
			}
			myCounter.add(1);
		}
	`)

	runner, err := getSimpleRunner(t, "/script.js", script)
	require.NoError(t, err)

	options := runner.GetOptions()
	require.Empty(t, options.Validate())

	execScheduler, err := local.NewExecutionScheduler(runner, testutils.NewLogger(t))
	require.NoError(t, err)
	engine, err := core.NewEngine(execScheduler, options, testutils.NewLogger(t))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	run, wait, err := engine.Init(ctx, ctx)
	require.NoError(t, err)

	collector := &dummy.Collector{}
	engine.Collectors = []lib.Collector{collector}
	require.Empty(t, runner.defaultGroup.Groups)

	errC := make(chan error)
	go func() { errC <- run() }()

	select {
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("Test timed out")
	case err := <-errC:
		cancel()
		require.NoError(t, err)
		wait()
		require.False(t, engine.IsTainted())
	}
	require.Contains(t, runner.defaultGroup.Groups, "setup")
	require.Contains(t, runner.defaultGroup.Groups, "teardown")
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
	r1, err := getSimpleRunner(t, "/script.js", data) // TODO fix this
	require.NoError(t, err)
	require.Equal(t, expScriptOptions, r1.GetOptions())

	testdata := map[string]*Runner{"Source": r1}
	for name, r := range testdata {
		r := r
		t.Run(name, func(t *testing.T) {
			samples := make(chan stats.SampleContainer, 100)

			if !assert.NoError(t, r.Setup(context.Background(), samples)) {
				return
			}
			initVU, err := r.NewVU(1, samples)
			if assert.NoError(t, err) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
				err := vu.RunOnce()
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetupDataReturnValue(t *testing.T) {
	testSetupDataHelper(t, `
	exports.options = { setupTimeout: "1s", teardownTimeout: "1s" };
	exports.setup = function() {
		return 42;
	}
	exports.default = function(data) {
		if (data != 42) {
			throw new Error("default: wrong data: " + JSON.stringify(data))
		}
	};

	exports.teardown = function(data) {
		if (data != 42) {
			throw new Error("teardown: wrong data: " + JSON.stringify(data))
		}
	};`)
}

func TestSetupDataNoSetup(t *testing.T) {
	testSetupDataHelper(t, `
	exports.options = { setupTimeout: "1s", teardownTimeout: "1s" };
	exports.default = function(data) {
		if (data !== undefined) {
			throw new Error("default: wrong data: " + JSON.stringify(data))
		}
	};

	exports.teardown = function(data) {
		if (data !== undefined) {
			console.log(data);
			throw new Error("teardown: wrong data: " + JSON.stringify(data))
		}
	};`)
}

func TestConsoleInInitContext(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
			console.log("1");
			exports.default = function(data) {
			};
		`)
	require.NoError(t, err)

	testdata := map[string]*Runner{"Source": r1}
	for name, r := range testdata {
		r := r
		t.Run(name, func(t *testing.T) {
			samples := make(chan stats.SampleContainer, 100)
			initVU, err := r.NewVU(1, samples)
			if assert.NoError(t, err) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
				err := vu.RunOnce()
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetupDataNoReturn(t *testing.T) {
	testSetupDataHelper(t, `
	exports.options = { setupTimeout: "1s", teardownTimeout: "1s" };
	exports.setup = function() { }
	exports.default = function(data) {
		if (data !== undefined) {
			throw new Error("default: wrong data: " + JSON.stringify(data))
		}
	};

	exports.teardown = function(data) {
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
		rtOpts := lib.RuntimeOptions{CompatibilityMode: null.StringFrom("extended")}
		for _, mod := range modules {
			mod := mod
			t.Run(mod, func(t *testing.T) {
				t.Run("Source", func(t *testing.T) {
					_, err := getSimpleRunner(t, "/script.js", fmt.Sprintf(`import "%s"; exports.default = function() {}`, mod), rtOpts)
					assert.NoError(t, err)
				})
			})
		}
	})

	t.Run("Files", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		require.NoError(t, fs.MkdirAll("/path/to", 0o755))
		require.NoError(t, afero.WriteFile(fs, "/path/to/lib.js", []byte(`exports.default = "hi!";`), 0o644))

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
				r1, err := getSimpleRunner(t, data.filename, fmt.Sprintf(`
					var hi = require("%s").default;
					exports.default = function() {
						if (hi != "hi!") { throw new Error("incorrect value"); }
					}`, data.path), fs)
				require.NoError(t, err)

				r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
				require.NoError(t, err)

				testdata := map[string]*Runner{"Source": r1, "Archive": r2}
				for name, r := range testdata {
					r := r
					t.Run(name, func(t *testing.T) {
						initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
						require.NoError(t, err)
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()
						vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
						err = vu.RunOnce()
						require.NoError(t, err)
					})
				}
			})
		}
	})
}

func TestVURunContext(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
		exports.options = { vus: 10 };
		exports.default = function() { fn(); }
		`)
	require.NoError(t, err)
	r1.SetOptions(r1.GetOptions().Apply(lib.Options{Throw: null.BoolFrom(true)}))

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		r := r
		t.Run(name, func(t *testing.T) {
			vu, err := r.newVU(1, make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}

			fnCalled := false
			vu.Runtime.Set("fn", func() {
				fnCalled = true

				assert.Equal(t, vu.Runtime, common.GetRuntime(*vu.Context), "incorrect runtime in context")
				assert.Nil(t, common.GetInitEnv(*vu.Context)) // shouldn't get this in the vu context

				state := lib.GetState(*vu.Context)
				if assert.NotNil(t, state) {
					assert.Equal(t, null.IntFrom(10), state.Options.VUs)
					assert.Equal(t, null.BoolFrom(true), state.Options.Throw)
					assert.NotNil(t, state.Logger)
					assert.Equal(t, r.GetDefaultGroup(), state.Group)
					assert.Equal(t, vu.Transport, state.Transport)
				}
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			activeVU := vu.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = activeVU.RunOnce()
			assert.NoError(t, err)
			assert.True(t, fnCalled, "fn() not called")
		})
	}
}

func TestVURunInterrupt(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
		exports.default = function() { while(true) {} }
		`)
	require.NoError(t, err)
	require.NoError(t, r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)}))

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	require.NoError(t, err)
	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		name, r := name, r
		t.Run(name, func(t *testing.T) {
			samples := make(chan stats.SampleContainer, 100)
			defer close(samples)
			go func() {
				for range samples {
				}
			}()

			vu, err := r.newVU(1, samples)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			activeVU := vu.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = activeVU.RunOnce()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "context canceled")
		})
	}
}

func TestVURunInterruptDoesntPanic(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
		exports.default = function() { while(true) {} }
		`)
	require.NoError(t, err)
	require.NoError(t, r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)}))

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	require.NoError(t, err)
	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		name, r := name, r
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			samples := make(chan stats.SampleContainer, 100)
			go func() {
				for range samples {
				}
			}()
			var wg sync.WaitGroup

			initVU, err := r.newVU(1, samples)
			require.NoError(t, err)
			for i := 0; i < 1000; i++ {
				wg.Add(1)
				newCtx, newCancel := context.WithCancel(ctx)
				vu := initVU.Activate(&lib.VUActivationParams{
					RunContext:         newCtx,
					DeactivateCallback: func(_ lib.InitializedVU) { wg.Done() },
				})
				ch := make(chan struct{})
				go func() {
					close(ch)
					vuErr := vu.RunOnce()
					assert.Error(t, vuErr)
					assert.Contains(t, vuErr.Error(), "context canceled")
				}()
				<-ch
				time.Sleep(time.Millisecond * 1) // NOTE: increase this in case of problems ;)
				newCancel()
				wg.Wait()
			}
		})
	}
}

func TestVUIntegrationGroups(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
		var group = require("k6").group;
		exports.default = function() {
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

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	require.NoError(t, err)

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		r := r
		t.Run(name, func(t *testing.T) {
			vu, err := r.newVU(1, make(chan stats.SampleContainer, 100))
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			activeVU := vu.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = activeVU.RunOnce()
			assert.NoError(t, err)
			assert.True(t, fnOuterCalled, "fnOuter() not called")
			assert.True(t, fnInnerCalled, "fnInner() not called")
			assert.True(t, fnNestedCalled, "fnNested() not called")
		})
	}
}

func TestVUIntegrationMetrics(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
		var group = require("k6").group;
		var Trend = require("k6/metrics").Trend;
		var myMetric = new Trend("my_metric");
		exports.default = function() { myMetric.add(5); }
		`)
	require.NoError(t, err)

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	require.NoError(t, err)

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		r := r
		t.Run(name, func(t *testing.T) {
			samples := make(chan stats.SampleContainer, 100)
			vu, err := r.newVU(1, samples)
			if !assert.NoError(t, err) {
				return
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			activeVU := vu.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = activeVU.RunOnce()
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
					case 4:
						assert.Equal(t, metrics.Iterations, s.Metric, "`iterations` sample is after `iteration_duration`")
						assert.Equal(t, float64(1), s.Value)
					}
				}
			}
			assert.Equal(t, sampleCount, 5)
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
			"x509: certificate has expired or is not yet valid",
		},
		"False": {
			lib.Options{InsecureSkipTLSVerify: null.BoolFrom(false)},
			"x509: certificate has expired or is not yet valid",
		},
		"True": {
			lib.Options{InsecureSkipTLSVerify: null.BoolFrom(true)},
			"",
		},
	}
	for name, data := range testdata {
		data := data
		t.Run(name, func(t *testing.T) {
			r1, err := getSimpleRunner(t, "/script.js", `
					var http = require("k6/http");;
					exports.default = function() { http.get("https://expired.badssl.com/"); }
				`)
			require.NoError(t, err)
			require.NoError(t, r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)}.Apply(data.opts)))

			r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
			require.NoError(t, err)
			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for name, r := range runners {
				r := r
				t.Run(name, func(t *testing.T) {
					r.Logger, _ = logtest.NewNullLogger()

					initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
					if !assert.NoError(t, err) {
						return
					}

					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
					err = vu.RunOnce()
					if data.errMsg != "" {
						require.Error(t, err)
						assert.Contains(t, err.Error(), data.errMsg)
					} else {
						assert.NoError(t, err)
					}
				})
			}
		})
	}
}

func TestVUIntegrationBlacklistOption(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
					var http = require("k6/http");;
					exports.default = function() { http.get("http://10.1.2.3/"); }
				`)
	require.NoError(t, err)

	cidr, err := lib.ParseCIDR("10.0.0.0/8")

	if !assert.NoError(t, err) {
		return
	}
	require.NoError(t, r1.SetOptions(lib.Options{
		Throw:        null.BoolFrom(true),
		BlacklistIPs: []*lib.IPNet{cidr},
	}))

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "IP (10.1.2.3) is in a blacklisted range (10.0.0.0/8)")
		})
	}
}

func TestVUIntegrationBlacklistScript(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
					var http = require("k6/http");;

					exports.options = {
						throw: true,
						blacklistIPs: ["10.0.0.0/8"],
					};

					exports.default = function() { http.get("http://10.1.2.3/"); }
				`)
	if !assert.NoError(t, err) {
		return
	}

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}

	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "IP (10.1.2.3) is in a blacklisted range (10.0.0.0/8)")
		})
	}
}

func TestVUIntegrationBlockHostnamesOption(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
					var http = require("k6/http");
					exports.default = function() { http.get("https://k6.io/"); }
				`)
	require.NoError(t, err)

	hostnames, err := types.NewNullHostnameTrie([]string{"*.io"})
	require.NoError(t, err)
	require.NoError(t, r1.SetOptions(lib.Options{
		Throw:            null.BoolFrom(true),
		BlockedHostnames: hostnames,
	}))

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}

	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			initVu, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
			require.NoError(t, err)
			vu := initVu.Activate(&lib.VUActivationParams{RunContext: context.Background()})
			err = vu.RunOnce()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "hostname (k6.io) is in a blocked pattern (*.io)")
		})
	}
}

func TestVUIntegrationBlockHostnamesScript(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
					var http = require("k6/http");

					exports.options = {
						throw: true,
						blockHostnames: ["*.io"],
					};

					exports.default = function() { http.get("https://k6.io/"); }
				`)
	if !assert.NoError(t, err) {
		return
	}

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}

	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			initVu, err := r.NewVU(0, make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}
			vu := initVu.Activate(&lib.VUActivationParams{RunContext: context.Background()})
			err = vu.RunOnce()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "hostname (k6.io) is in a blocked pattern (*.io)")
		})
	}
}

func TestVUIntegrationHosts(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r1, err := getSimpleRunner(t, "/script.js",
		tb.Replacer.Replace(`
					var k6 = require("k6");
					var check = k6.check;
					var fail = k6.fail;
					var http = require("k6/http");;
					exports.default = function() {
						var res = http.get("http://test.loadimpact.com:HTTPBIN_PORT/");
						check(res, {
							"is correct IP": function(r) { return r.remote_ip === "127.0.0.1" }
						}) || fail("failed to override dns");
					}
				`))
	if !assert.NoError(t, err) {
		return
	}

	r1.SetOptions(lib.Options{
		Throw: null.BoolFrom(true),
		Hosts: map[string]*lib.HostAddress{
			"test.loadimpact.com": {IP: net.ParseIP("127.0.0.1")},
		},
	})

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			if !assert.NoError(t, err) {
				return
			}
		})
	}
}

func TestVUIntegrationTLSConfig(t *testing.T) {
	unsupportedVersionErrorMsg := "remote error: tls: handshake failure"
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
			"remote error: tls: handshake failure",
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
			unsupportedVersionErrorMsg,
		},
	}
	for name, data := range testdata {
		data := data
		t.Run(name, func(t *testing.T) {
			r1, err := getSimpleRunner(t, "/script.js", `
					var http = require("k6/http");;
					exports.default = function() { http.get("https://sha256.badssl.com/"); }
				`)
			if !assert.NoError(t, err) {
				return
			}
			require.NoError(t, r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)}.Apply(data.opts)))

			r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
			if !assert.NoError(t, err) {
				return
			}

			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for name, r := range runners {
				r := r
				t.Run(name, func(t *testing.T) {
					r.Logger, _ = logtest.NewNullLogger()

					initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
					if !assert.NoError(t, err) {
						return
					}
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
					err = vu.RunOnce()
					if data.errMsg != "" {
						require.Error(t, err)
						assert.Contains(t, err.Error(), data.errMsg)
					} else {
						assert.NoError(t, err)
					}
				})
			}
		})
	}
}

func TestVUIntegrationOpenFunctionError(t *testing.T) {
	r, err := getSimpleRunner(t, "/script.js", `
			exports.default = function() { open("/tmp/foo") }
		`)
	assert.NoError(t, err)

	initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
	assert.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
	err = vu.RunOnce()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only available in the init stage")
}

func TestVUIntegrationOpenFunctionErrorWhenSneaky(t *testing.T) {
	r, err := getSimpleRunner(t, "/script.js", `
			var sneaky = open;
			exports.default = function() { sneaky("/tmp/foo") }
		`)
	assert.NoError(t, err)

	initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
	assert.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
	err = vu.RunOnce()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only available in the init stage")
}

func TestVUIntegrationCookiesReset(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r1, err := getSimpleRunner(t, "/script.js", tb.Replacer.Replace(`
			var http = require("k6/http");;
			exports.default = function() {
				var url = "HTTPBIN_URL";
				var preRes = http.get(url + "/cookies");
				if (preRes.status != 200) { throw new Error("wrong status (pre): " + preRes.status); }
				if (preRes.json().k1 || preRes.json().k2) {
					throw new Error("cookies persisted: " + preRes.body);
				}

				var res = http.get(url + "/cookies/set?k2=v2&k1=v1");
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

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			for i := 0; i < 2; i++ {
				err = vu.RunOnce()
				assert.NoError(t, err)
			}
		})
	}
}

func TestVUIntegrationCookiesNoReset(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r1, err := getSimpleRunner(t, "/script.js", tb.Replacer.Replace(`
			var http = require("k6/http");;
			exports.default = function() {
				var url = "HTTPBIN_URL";
				if (__ITER == 0) {
					var res = http.get(url + "/cookies/set?k2=v2&k1=v1");
					if (res.status != 200) { throw new Error("wrong status: " + res.status) }
					if (res.json().k1 != "v1" || res.json().k2 != "v2") {
						throw new Error("wrong cookies: " + res.body);
					}
				}

				if (__ITER == 1) {
					var res = http.get(url + "/cookies");
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

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			assert.NoError(t, err)

			err = vu.RunOnce()
			assert.NoError(t, err)
		})
	}
}

func TestVUIntegrationVUID(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
			exports.default = function() {
				if (__VU != 1234) { throw new Error("wrong __VU: " + __VU); }
			}`,
	)
	if !assert.NoError(t, err) {
		return
	}
	r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)})

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			initVU, err := r.NewVU(1234, make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
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

	r1, err := getSimpleRunner(t, "/script.js", fmt.Sprintf(`
			var http = require("k6/http");;
			exports.default = function() { http.get("https://%s")}
		`, listener.Addr().String()))
	if !assert.NoError(t, err) {
		return
	}
	require.NoError(t, r1.SetOptions(lib.Options{
		Throw:                 null.BoolFrom(true),
		InsecureSkipTLSVerify: null.BoolFrom(true),
	}))

	t.Run("Unauthenticated", func(t *testing.T) {
		r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
		if !assert.NoError(t, err) {
			return
		}

		runners := map[string]*Runner{"Source": r1, "Archive": r2}
		for name, r := range runners {
			r := r
			t.Run(name, func(t *testing.T) {
				r.Logger, _ = logtest.NewNullLogger()
				initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
				if assert.NoError(t, err) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
					err := vu.RunOnce()
					require.Error(t, err)
					assert.Contains(t, err.Error(), "remote error: tls: bad certificate")
				}
			})
		}
	})

	require.NoError(t, r1.SetOptions(lib.Options{
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
	}))

	t.Run("Authenticated", func(t *testing.T) {
		r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
		if !assert.NoError(t, err) {
			return
		}

		runners := map[string]*Runner{"Source": r1, "Archive": r2}
		for name, r := range runners {
			r := r
			t.Run(name, func(t *testing.T) {
				initVU, err := r.NewVU(1, make(chan stats.SampleContainer, 100))
				if assert.NoError(t, err) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
					err := vu.RunOnce()
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestHTTPRequestInInitContext(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	_, err := getSimpleRunner(t, "/script.js", tb.Replacer.Replace(`
					var k6 = require("k6");
					var check = k6.check;
					var fail = k6.fail;
					var http = require("k6/http");;
					var res = http.get("HTTPBIN_URL/");
					exports.default = function() {
						console.log(test);
					}
				`))
	if assert.Error(t, err) {
		assert.Contains(
			t,
			err.Error(),
			k6http.ErrHTTPForbiddenInInitContext.Error())
	}
}

func TestInitContextForbidden(t *testing.T) {
	table := [...][3]string{
		{
			"http.request",
			`var http = require("k6/http");;
			 var res = http.get("HTTPBIN_URL");
			 exports.default = function() { console.log("p"); }`,
			k6http.ErrHTTPForbiddenInInitContext.Error(),
		},
		{
			"http.batch",
			`var http = require("k6/http");;
			 var res = http.batch("HTTPBIN_URL/something", "HTTPBIN_URL/else");
			 exports.default = function() { console.log("p"); }`,
			k6http.ErrBatchForbiddenInInitContext.Error(),
		},
		{
			"http.cookieJar",
			`var http = require("k6/http");;
			 var jar = http.cookieJar();
			 exports.default = function() { console.log("p"); }`,
			k6http.ErrJarForbiddenInInitContext.Error(),
		},
		{
			"check",
			`var check = require("k6").check;
			 check("test", {'is test': function(test) { return test == "test"}})
			 exports.default = function() { console.log("p"); }`,
			k6.ErrCheckInInitContext.Error(),
		},
		{
			"group",
			`var group = require("k6").group;
			 group("group1", function () { console.log("group1");})
			 exports.default = function() { console.log("p"); }`,
			k6.ErrGroupInInitContext.Error(),
		},
		{
			"ws",
			`var ws = require("k6/ws");
			 var url = "ws://echo.websocket.org";
			 var params = { "tags": { "my_tag": "hello" } };
			 var response = ws.connect(url, params, function (socket) {
			   socket.on('open', function open() {
					console.log('connected');
			   })
		   });

			 exports.default = function() { console.log("p"); }`,
			ws.ErrWSInInitContext.Error(),
		},
		{
			"metric",
			`var Counter = require("k6/metrics").Counter;
			 var counter = Counter("myCounter");
			 counter.add(1);
			 exports.default = function() { console.log("p"); }`,
			k6metrics.ErrMetricsAddInInitContext.Error(),
		},
	}
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	for _, test := range table {
		test := test
		t.Run(test[0], func(t *testing.T) {
			_, err := getSimpleRunner(t, "/script.js", tb.Replacer.Replace(test[1]))
			if assert.Error(t, err) {
				assert.Contains(
					t,
					err.Error(),
					test[2])
			}
		})
	}
}

func TestArchiveRunningIntegrity(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	fs := afero.NewMemMapFs()
	data := tb.Replacer.Replace(`
			var fput = open("/home/somebody/test.json");
			exports.options = { setupTimeout: "10s", teardownTimeout: "10s" };
			exports.setup = function () {
				return JSON.parse(fput);
			}
			exports.default = function(data) {
				if (data != 42) {
					throw new Error("incorrect answer " + data);
				}
			}
		`)
	require.NoError(t, afero.WriteFile(fs, "/home/somebody/test.json", []byte(`42`), os.ModePerm))
	require.NoError(t, afero.WriteFile(fs, "/script.js", []byte(data), os.ModePerm))
	r1, err := getSimpleRunner(t, "/script.js", data, fs)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	require.NoError(t, r1.MakeArchive().Write(buf))

	arc, err := lib.ReadArchive(buf)
	require.NoError(t, err)
	r2, err := NewFromArchive(testutils.NewLogger(t), arc, lib.RuntimeOptions{})
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			ch := make(chan stats.SampleContainer, 100)
			err = r.Setup(context.Background(), ch)
			require.NoError(t, err)
			initVU, err := r.NewVU(1, ch)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.NoError(t, err)
		})
	}
}

func TestArchiveNotPanicking(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/non/existent", []byte(`42`), os.ModePerm))
	r1, err := getSimpleRunner(t, "/script.js", tb.Replacer.Replace(`
			var fput = open("/non/existent");
			exports.default = function(data) {}
		`), fs)
	require.NoError(t, err)

	arc := r1.MakeArchive()
	arc.Filesystems = map[string]afero.Fs{"file": afero.NewMemMapFs()}
	r2, err := NewFromArchive(testutils.NewLogger(t), arc, lib.RuntimeOptions{})
	// we do want this to error here as this is where we find out that a given file is not in the
	// archive
	require.Error(t, err)
	require.Nil(t, r2)
}

func TestStuffNotPanicking(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r, err := getSimpleRunner(t, "/script.js", tb.Replacer.Replace(`
			var http = require("k6/http");
			var ws = require("k6/ws");
			var group = require("k6").group;
			var parseHTML = require("k6/html").parseHTML;

			exports.options = { iterations: 1, vus: 1, vusMax: 1 };

			exports.default = function() {
				var doc = parseHTML(http.get("HTTPBIN_URL/html").body);

				var testCases = [
					function() { return group()},
					function() { return group("test")},
					function() { return group("test", "wat")},
					function() { return doc.find('p').each()},
					function() { return doc.find('p').each("wat")},
					function() { return doc.find('p').map()},
					function() { return doc.find('p').map("wat")},
					function() { return ws.connect("WSBIN_URL/ws-echo")},
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
	initVU, err := r.NewVU(1, ch)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
	errC := make(chan error)
	go func() { errC <- vu.RunOnce() }()

	select {
	case <-time.After(15 * time.Second):
		cancel()
		t.Fatal("Test timed out")
	case err := <-errC:
		cancel()
		require.NoError(t, err)
	}
}

func TestPanicOnSimpleHTML(t *testing.T) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	r, err := getSimpleRunner(t, "/script.js", tb.Replacer.Replace(`
			var parseHTML = require("k6/html").parseHTML;

			exports.options = { iterations: 1, vus: 1, vusMax: 1 };

			exports.default = function() {
				var doc = parseHTML("<html>");
				var o = doc.find(".something").slice(0, 4).toArray()
			};
		`))
	require.NoError(t, err)

	ch := make(chan stats.SampleContainer, 1000)
	initVU, err := r.NewVU(1, ch)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
	errC := make(chan error)
	go func() { errC <- vu.RunOnce() }()

	select {
	case <-time.After(15 * time.Second):
		cancel()
		t.Fatal("Test timed out")
	case err := <-errC:
		cancel()
		require.NoError(t, err)
	}
}

func TestSystemTags(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	// Handle paths with custom logic
	tb.Mux.HandleFunc("/wrong-redirect", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Location", "%")
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	r, err := getSimpleRunner(t, "/script.js", tb.Replacer.Replace(`
		var http = require("k6/http");

		exports.http_get = function() {
			http.get("HTTPBIN_IP_URL");
		};
		exports.https_get = function() {
			http.get("HTTPSBIN_IP_URL");
		};
		exports.bad_url_get = function() {
			http.get("http://127.0.0.1:1");
		};
		exports.noop = function() {};
	`), lib.RuntimeOptions{CompatibilityMode: null.StringFrom("base")})
	require.NoError(t, err)

	httpURL, err := url.Parse(tb.ServerHTTP.URL)
	require.NoError(t, err)

	testedSystemTags := []struct{ tag, exec, expVal string }{
		{"proto", "http_get", "HTTP/1.1"},
		{"status", "http_get", "200"},
		{"method", "http_get", "GET"},
		{"url", "http_get", tb.ServerHTTP.URL},
		{"url", "https_get", tb.ServerHTTPS.URL},
		{"ip", "http_get", httpURL.Hostname()},
		{"name", "http_get", tb.ServerHTTP.URL},
		{"group", "http_get", ""},
		{"vu", "http_get", "8"},
		{"vu", "noop", "9"},
		{"iter", "http_get", "0"},
		{"iter", "noop", "0"},
		{"tls_version", "https_get", "tls1.3"},
		{"ocsp_status", "https_get", "unknown"},
		{"error", "bad_url_get", `dial: connection refused`},
		{"error_code", "bad_url_get", "1212"},
		{"scenario", "http_get", "default"},
		// TODO: add more tests
	}

	samples := make(chan stats.SampleContainer, 100)
	for num, tc := range testedSystemTags {
		num, tc := num, tc
		t.Run(fmt.Sprintf("TC %d with only %s", num, tc.tag), func(t *testing.T) {
			require.NoError(t, r.SetOptions(r.GetOptions().Apply(lib.Options{
				Throw:                 null.BoolFrom(false),
				TLSVersion:            &lib.TLSVersions{Max: lib.TLSVersion13},
				SystemTags:            stats.ToSystemTagSet([]string{tc.tag}),
				InsecureSkipTLSVerify: null.BoolFrom(true),
			})))

			vu, err := r.NewVU(int64(num), samples)
			require.NoError(t, err)
			activeVU := vu.Activate(&lib.VUActivationParams{
				RunContext: context.Background(),
				Exec:       tc.exec,
				Scenario:   "default",
			})
			require.NoError(t, activeVU.RunOnce())

			bufSamples := stats.GetBufferedSamples(samples)
			assert.NotEmpty(t, bufSamples)
			for _, sample := range bufSamples[0].GetSamples() {
				assert.NotEmpty(t, sample.Tags)
				for emittedTag, emittedVal := range sample.Tags.CloneTags() {
					assert.Equal(t, tc.tag, emittedTag)
					assert.Equal(t, tc.expVal, emittedVal)
				}
			}
		})
	}
}

func TestVUPanic(t *testing.T) {
	r1, err := getSimpleRunner(t, "/script.js", `
			var group = require("k6").group;
			exports.default = function() {
				group("panic here", function() {
					if (__ITER == 0) {
						panic("here we panic");
					}
					console.log("here we don't");
				})
			}`,
	)
	require.NoError(t, err)

	r2, err := NewFromArchive(testutils.NewLogger(t), r1.MakeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			initVU, err := r.NewVU(1234, make(chan stats.SampleContainer, 100))
			if !assert.NoError(t, err) {
				return
			}

			logger := logrus.New()
			logger.SetLevel(logrus.InfoLevel)
			logger.Out = ioutil.Discard
			hook := testutils.SimpleLogrusHook{
				HookedLevels: []logrus.Level{logrus.InfoLevel, logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel},
			}
			logger.AddHook(&hook)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			vu.(*ActiveVU).Runtime.Set("panic", func(str string) { panic(str) })
			vu.(*ActiveVU).state.Logger = logger

			vu.(*ActiveVU).Console.logger = logger.WithField("source", "console")
			err = vu.RunOnce()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "a panic occurred in VU code but was caught: here we panic")
			entries := hook.Drain()
			require.Len(t, entries, 1)
			assert.Equal(t, logrus.ErrorLevel, entries[0].Level)
			require.True(t, strings.HasPrefix(entries[0].Message, "panic: here we panic"))
			require.True(t, strings.HasSuffix(entries[0].Message, "Goja stack:\nfile:///script.js:3:4(12)"))

			err = vu.RunOnce()
			assert.NoError(t, err)

			entries = hook.Drain()
			require.Len(t, entries, 1)
			assert.Equal(t, logrus.InfoLevel, entries[0].Level)
			require.Contains(t, entries[0].Message, "here we don't")
		})
	}
}

type multiFileTestCase struct {
	fses       map[string]afero.Fs
	rtOpts     lib.RuntimeOptions
	cwd        string
	script     string
	expInitErr bool
	expVUErr   bool
	samples    chan stats.SampleContainer
}

func runMultiFileTestCase(t *testing.T, tc multiFileTestCase, tb *httpmultibin.HTTPMultiBin) {
	logger := testutils.NewLogger(t)
	runner, err := New(
		logger,
		&loader.SourceData{
			URL:  &url.URL{Path: tc.cwd + "/script.js", Scheme: "file"},
			Data: []byte(tc.script),
		},
		tc.fses,
		tc.rtOpts,
	)
	if tc.expInitErr {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	options := runner.GetOptions()
	require.Empty(t, options.Validate())

	vu, err := runner.NewVU(1, tc.samples)
	require.NoError(t, err)

	jsVU, ok := vu.(*VU)
	require.True(t, ok)
	jsVU.state.Dialer = tb.Dialer
	jsVU.state.TLSConfig = tb.TLSClientConfig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	activeVU := vu.Activate(&lib.VUActivationParams{RunContext: ctx})

	err = activeVU.RunOnce()
	if tc.expVUErr {
		require.Error(t, err)
	} else {
		require.NoError(t, err)
	}

	arc := runner.MakeArchive()
	runnerFromArc, err := NewFromArchive(logger, arc, tc.rtOpts)
	require.NoError(t, err)
	vuFromArc, err := runnerFromArc.NewVU(2, tc.samples)
	require.NoError(t, err)
	jsVUFromArc, ok := vuFromArc.(*VU)
	require.True(t, ok)
	jsVUFromArc.state.Dialer = tb.Dialer
	jsVUFromArc.state.TLSConfig = tb.TLSClientConfig
	activeVUFromArc := jsVUFromArc.Activate(&lib.VUActivationParams{RunContext: ctx})
	err = activeVUFromArc.RunOnce()
	if tc.expVUErr {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)
}

func TestComplicatedFileImportsForGRPC(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()

	tb.GRPCStub.UnaryCallFunc = func(ctx context.Context, sreq *grpc_testing.SimpleRequest) (
		*grpc_testing.SimpleResponse, error,
	) {
		return &grpc_testing.SimpleResponse{
			Username: "foo",
		}, nil
	}

	fs := afero.NewMemMapFs()
	protoFile, err := ioutil.ReadFile("../vendor/google.golang.org/grpc/test/grpc_testing/test.proto")
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, "/path/to/service.proto", protoFile, 0644))
	require.NoError(t, afero.WriteFile(fs, "/path/to/same-dir.proto", []byte(
		`syntax = "proto3";package whatever;import "service.proto";`,
	), 0644))
	require.NoError(t, afero.WriteFile(fs, "/path/subdir.proto", []byte(
		`syntax = "proto3";package whatever;import "to/service.proto";`,
	), 0644))
	require.NoError(t, afero.WriteFile(fs, "/path/to/abs.proto", []byte(
		`syntax = "proto3";package whatever;import "/path/to/service.proto";`,
	), 0644))

	grpcTestCase := func(expInitErr, expVUErr bool, cwd, loadCode string) multiFileTestCase {
		script := tb.Replacer.Replace(fmt.Sprintf(`
			var grpc = require('k6/net/grpc');
			var client = new grpc.Client();

			%s // load statements

			exports.default = function() {
				client.connect('GRPCBIN_ADDR', {timeout: '3s'});
				var resp = client.invoke('grpc.testing.TestService/UnaryCall', {})
				if (!resp.message || resp.error || resp.message.username !== 'foo') {
					throw new Error('unexpected response message: ' + JSON.stringify(resp.message))
				}
			}
		`, loadCode))

		return multiFileTestCase{
			fses:    map[string]afero.Fs{"file": fs, "https": afero.NewMemMapFs()},
			rtOpts:  lib.RuntimeOptions{CompatibilityMode: null.NewString("base", true)},
			samples: make(chan stats.SampleContainer, 100),
			cwd:     cwd, expInitErr: expInitErr, expVUErr: expVUErr, script: script,
		}
	}

	testCases := []multiFileTestCase{
		grpcTestCase(false, true, "/", `/* no grpc loads */`), // exp VU error with no proto files loaded

		// Init errors when the protobuf file can't be loaded
		grpcTestCase(true, false, "/", `client.load(null, 'service.proto');`),
		grpcTestCase(true, false, "/", `client.load(null, '/wrong/path/to/service.proto');`),
		grpcTestCase(true, false, "/", `client.load(['/', '/path/'], 'service.proto');`),

		// Direct imports of service.proto
		grpcTestCase(false, false, "/", `client.load(null, '/path/to/service.proto');`), // full path should be fine
		grpcTestCase(false, false, "/path/to/", `client.load([], 'service.proto');`),    // file name from same folder
		grpcTestCase(false, false, "/", `client.load(['./path//to/'], 'service.proto');`),
		grpcTestCase(false, false, "/path/", `client.load(['./to/'], 'service.proto');`),

		grpcTestCase(false, false, "/whatever", `client.load(['/path/to/'], 'service.proto');`),  // with import paths
		grpcTestCase(false, false, "/path", `client.load(['/', '/path/to/'], 'service.proto');`), // with import paths
		grpcTestCase(false, false, "/whatever", `client.load(['../path/to/'], 'service.proto');`),

		// Import another file that imports "service.proto" directly
		grpcTestCase(true, false, "/", `client.load([], '/path/to/same-dir.proto');`),
		grpcTestCase(true, false, "/path/", `client.load([], 'to/same-dir.proto');`),
		grpcTestCase(true, false, "/", `client.load(['/path/'], 'to/same-dir.proto');`),
		grpcTestCase(false, false, "/path/to/", `client.load([], 'same-dir.proto');`),
		grpcTestCase(false, false, "/", `client.load(['/path/to/'], 'same-dir.proto');`),
		grpcTestCase(false, false, "/whatever", `client.load(['/other', '/path/to/'], 'same-dir.proto');`),
		grpcTestCase(false, false, "/", `client.load(['./path//to/'], 'same-dir.proto');`),
		grpcTestCase(false, false, "/path/", `client.load(['./to/'], 'same-dir.proto');`),
		grpcTestCase(false, false, "/whatever", `client.load(['../path/to/'], 'same-dir.proto');`),

		// Import another file that imports "to/service.proto" directly
		grpcTestCase(true, false, "/", `client.load([], '/path/to/subdir.proto');`),
		grpcTestCase(false, false, "/path/", `client.load([], 'subdir.proto');`),
		grpcTestCase(false, false, "/", `client.load(['/path/'], 'subdir.proto');`),
		grpcTestCase(false, false, "/", `client.load(['./path/'], 'subdir.proto');`),
		grpcTestCase(false, false, "/whatever", `client.load(['/other', '/path/'], 'subdir.proto');`),
		grpcTestCase(false, false, "/whatever", `client.load(['../other', '../path/'], 'subdir.proto');`),

		// Import another file that imports "/path/to/service.proto" directly
		grpcTestCase(true, false, "/", `client.load(['/path'], '/path/to/abs.proto');`),
		grpcTestCase(false, false, "/", `client.load([], '/path/to/abs.proto');`),
		grpcTestCase(false, false, "/whatever", `client.load(['/'], '/path/to/abs.proto');`),
	}

	for i, tc := range testCases {
		i, tc := i, tc
		t.Run(fmt.Sprintf("TestCase_%d", i), func(t *testing.T) {
			t.Logf(
				"CWD: %s, expInitErr: %t, expVUErr: %t, script injected with: `%s`",
				tc.cwd, tc.expInitErr, tc.expVUErr, tc.script,
			)
			runMultiFileTestCase(t, tc, tb)
		})
	}
}
