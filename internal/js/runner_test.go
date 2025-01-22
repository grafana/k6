package js

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"go/build"
	"io"
	"io/fs"
	stdlog "log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/internal/execution"
	"go.k6.io/k6/internal/execution/local"
	k6metrics "go.k6.io/k6/internal/js/modules/k6/metrics"
	"go.k6.io/k6/internal/js/modules/k6/ws"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/internal/lib/testutils/httpmultibin/grpc_testing"
	"go.k6.io/k6/internal/lib/testutils/mockoutput"
	"go.k6.io/k6/js/modules/k6"
	k6http "go.k6.io/k6/js/modules/k6/http"
	"go.k6.io/k6/lib"
	_ "go.k6.io/k6/lib/executor" // TODO: figure out something better
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

func TestRunnerNew(t *testing.T) {
	t.Parallel()
	t.Run("Valid", func(t *testing.T) {
		t.Parallel()
		r, err := getSimpleRunner(t, "/script.js", `
			exports.counter = 0;
			exports.default = function() { exports.counter++; }
		`)
		require.NoError(t, err)

		t.Run("NewVU", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
			require.NoError(t, err)
			vuc, ok := initVU.(*VU)
			require.True(t, ok)
			assert.Equal(t, int64(0), vuc.getExported("counter").Export())

			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			t.Run("RunOnce", func(t *testing.T) {
				err = vu.RunOnce()
				require.NoError(t, err)
				assert.Equal(t, int64(1), vuc.getExported("counter").Export())
			})
		})
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		_, err := getSimpleRunner(t, "/script.js", `blarg`)
		assert.EqualError(t, err, "ReferenceError: blarg is not defined\n\tat file:///script.js:1:28(1)\n")
	})
}

func TestRunnerOptions(t *testing.T) {
	t.Parallel()
	r1, err := getSimpleRunner(t, "/script.js", `exports.default = function() {};`)
	require.NoError(t, err)

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		name, r := name, r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, r.Bundle.Options, r.GetOptions())
			assert.Equal(t, null.NewBool(false, false), r.Bundle.Options.Paused)
			require.NoError(t, r.SetOptions(lib.Options{Paused: null.BoolFrom(true)}))
			assert.Equal(t, r.Bundle.Options, r.GetOptions())
			assert.Equal(t, null.NewBool(true, true), r.Bundle.Options.Paused)
			require.NoError(t, r.SetOptions(lib.Options{Paused: null.BoolFrom(false)}))
			assert.Equal(t, r.Bundle.Options, r.GetOptions())
			assert.Equal(t, null.NewBool(false, true), r.Bundle.Options.Paused)
		})
	}
}

func TestRunnerRPSLimit(t *testing.T) {
	t.Parallel()

	var nilLimiter *rate.Limiter

	variants := []struct {
		name    string
		options lib.Options
		limiter *rate.Limiter
	}{
		{
			name:    "RPS not defined",
			options: lib.Options{},
			limiter: nilLimiter,
		},
		{
			name:    "RPS set to non-zero int",
			options: lib.Options{RPS: null.IntFrom(9)},
			limiter: rate.NewLimiter(rate.Limit(9), 1),
		},
		{
			name:    "RPS set to zero",
			options: lib.Options{RPS: null.IntFrom(0)},
			limiter: nilLimiter,
		},
		{
			name:    "RPS set to below zero value",
			options: lib.Options{RPS: null.IntFrom(-1)},
			limiter: nilLimiter,
		},
	}

	for _, variant := range variants {
		variant := variant

		t.Run(variant.name, func(t *testing.T) {
			t.Parallel()

			r, err := getSimpleRunner(t, "/script.js", `exports.default = function() {};`)
			require.NoError(t, err)
			err = r.SetOptions(variant.options)
			require.NoError(t, err)
			assert.Equal(t, variant.limiter, r.RPSLimit)
		})
	}
}

func TestOptionsSettingToScript(t *testing.T) {
	t.Parallel()

	optionVariants := []string{
		"export var options = {};",
		"export var options = {teardownTimeout: '1s'};",
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
			r, err := getSimpleRunner(t, "/script.js", data,
				lib.RuntimeOptions{Env: map[string]string{"expectedTeardownTimeout": "4s"}})
			require.NoError(t, err)

			newOptions := lib.Options{
				TeardownTimeout: types.NullDurationFrom(4 * time.Second),
			}
			require.NoError(t, r.SetOptions(newOptions))
			require.Equal(t, newOptions, r.GetOptions())

			samples := make(chan metrics.SampleContainer, 100)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, samples)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())
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

	expScriptOptions := lib.Options{
		SetupTimeout: types.NullDurationFrom(1 * time.Second),
	}
	r1, err := getSimpleRunner(t, "/script.js", data,
		lib.RuntimeOptions{Env: map[string]string{"expectedSetupTimeout": "1s"}})
	require.NoError(t, err)
	require.Equal(t, expScriptOptions, r1.GetOptions())

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive(),
		lib.RuntimeOptions{Env: map[string]string{"expectedSetupTimeout": "3s"}},
	)
	require.NoError(t, err)
	require.Equal(t, expScriptOptions, r2.GetOptions())
	r2.Bundle.Options.SetupTimeout = types.NullDurationFrom(3 * time.Second)

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			samples := make(chan metrics.SampleContainer, 100)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, samples)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())
		})
	}
}

func TestMetricName(t *testing.T) {
	t.Parallel()

	script := `
		var Counter = require("k6/metrics").Counter;

		var myCounter = new Counter("not ok name @");

		exports.default = function(data) {
			myCounter.add(1);
		}
	`

	_, err := getSimpleRunner(t, "/script.js", script)
	require.Error(t, err)
}

func TestDataIsolation(t *testing.T) {
	t.Parallel()

	script := `
		var exec = require("k6/execution");
		var Counter = require("k6/metrics").Counter;
		var sleep = require('k6').sleep;

		exports.options = {
			scenarios: {
				sc1: {
					executor: "shared-iterations",
					vus: 2,
					iterations: 100,
					maxDuration: "9s",
					gracefulStop: 0,
					exec: "sc1",
				},
				sc2: {
					executor: "per-vu-iterations",
					vus: 1,
					iterations: 1,
					startTime: "11s",
					exec: "sc2",
				},
			},
			teardownTimeout: "5s",
			setupTimeout: "5s",
		};
		var myCounter = new Counter("mycounter");

		exports.setup = function() {
			return { v: 0 };
		}

		exports.sc1 = function(data) {
			if (data.v !== __ITER) {
				throw new Error("sc1: wrong data for iter " + __ITER + ": " + JSON.stringify(data));
			}
			if (__ITER != 0 && data.v != exec.vu.tags.myiter) {
				throw new Error("sc1: wrong vu tags for iter " + __ITER + ": " + JSON.stringify(exec.vu.tags));
			}
			data.v += 1;
			exec.vu.tags.myiter = data.v;
			myCounter.add(1);
			sleep(0.01);
		}

		exports.sc2 = function(data) {
			if (data.v === 0) {
				throw new Error("sc2: wrong data, expected VU to have modified setup data locally: " + data.v);
			}

			if (typeof exec.vu.tags.myiter !== "undefined") {
				throw new Error(
					"sc2: wrong tags, expected VU to have new tags in new scenario: " +
					JSON.stringify(exec.vu.tags),
				);
			}

			myCounter.add(1);
		}

		exports.teardown = function(data) {
			if (data.v !== 0) {
				throw new Error("teardown: wrong data: " + data.v);
			}
			myCounter.add(1);
		}
	`

	runner, err := getSimpleRunner(t, "/script.js", script)
	require.NoError(t, err)

	options := runner.GetOptions()
	require.Empty(t, options.Validate())

	testRunState := &lib.TestRunState{
		TestPreInitState: runner.preInitState,
		Options:          options,
		Runner:           runner,
		RunTags:          runner.preInitState.Registry.RootTagSet().WithTagsFromMap(options.RunTags),
	}

	execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
	require.NoError(t, err)

	globalCtx, globalCancel := context.WithCancel(context.Background())
	defer globalCancel()
	runCtx, runAbort := execution.NewTestRunContext(globalCtx, testRunState.Logger)

	mockOutput := mockoutput.New()
	outputManager := output.NewManager([]output.Output{mockOutput}, testRunState.Logger, runAbort)
	samples := make(chan metrics.SampleContainer, 1000)
	waitForMetricsFlushed, stopOutputs, err := outputManager.Start(samples)
	require.NoError(t, err)
	defer stopOutputs(nil)

	stopEmission, err := execScheduler.Init(runCtx, samples)
	require.NoError(t, err)

	errC := make(chan error)
	go func() { errC <- execScheduler.Run(globalCtx, runCtx, samples) }()

	select {
	case <-time.After(20 * time.Second):
		runAbort(fmt.Errorf("unexpected abort"))
		t.Fatal("Test timed out")
	case err := <-errC:
		stopEmission()
		close(samples)
		require.NoError(t, err)
		waitForMetricsFlushed()
	}
	var count int
	for _, s := range mockOutput.Samples {
		if s.Metric.Name == "mycounter" {
			count += int(s.Value)
		}
	}
	require.Equal(t, 102, count, "mycounter should be the number of iterations + 1 for the teardown")
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
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			samples := make(chan metrics.SampleContainer, 100)

			require.NoError(t, r.Setup(ctx, samples))
			initVU, err := r.NewVU(ctx, 1, 1, samples)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())
		})
	}
}

func TestSetupDataReturnValue(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
			samples := make(chan metrics.SampleContainer, 100)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, samples)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			require.NoError(t, vu.RunOnce())
		})
	}
}

func TestSetupDataNoReturn(t *testing.T) {
	t.Parallel()
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

func TestSetupDataPromise(t *testing.T) {
	t.Parallel()
	testSetupDataHelper(t, `
	exports.options = { setupTimeout: "1s", teardownTimeout: "1s" };
	exports.setup = async function() {
        return await Promise.resolve({"data": "correct"})
    }
	exports.default = function(data) {
		if (data.data !== "correct") {
			throw new Error("default: wrong data: " + JSON.stringify(data))
		}
	};

	exports.teardown = function(data) {
		if (data.data !== "correct") {
			throw new Error("teardown: wrong data: " + JSON.stringify(data))
		}
	};`)
}

func TestRunnerIntegrationImports(t *testing.T) {
	t.Parallel()
	t.Run("Modules", func(t *testing.T) {
		t.Parallel()
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
					_, err := getSimpleRunner(t, "/script.js",
						fmt.Sprintf(`import "%s"; export default function() {}`, mod), rtOpts)
					require.NoError(t, err)
				})
			})
		}
	})

	t.Run("Files", func(t *testing.T) {
		t.Parallel()

		testdata := map[string]struct{ filename, path string }{
			"Absolute":       {"/path/script.js", "/path/to/lib.js"},
			"Relative":       {"/path/script.js", "./to/lib.js"},
			"Adjacent":       {"/path/to/script.js", "./lib.js"},
			"STDIN-Absolute": {"/-", "/path/to/lib.js"},
			"STDIN-Relative": {"/-", "./path/to/lib.js"},
		}
		for name, data := range testdata {
			name, data := name, data
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				fs := fsext.NewMemMapFs()
				require.NoError(t, fs.MkdirAll("/path/to", 0o755))
				require.NoError(t, fsext.WriteFile(fs, "/path/to/lib.js", []byte(`exports.default = "hi!";`), 0o644))
				r1, err := getSimpleRunner(t, data.filename, fmt.Sprintf(`
					var hi = require("%s").default;
					exports.default = function() {
						if (hi != "hi!") { throw new Error("incorrect value"); }
					}`, data.path), fs)
				require.NoError(t, err)

				r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
				require.NoError(t, err)

				testdata := map[string]*Runner{"Source": r1, "Archive": r2}
				for name, r := range testdata {
					r := r
					t.Run(name, func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()
						initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
						require.NoError(t, err)
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
	t.Parallel()
	r1, err := getSimpleRunner(t, "/script.js", `
		exports.options = { vus: 10 };
		exports.default = function() { fn(); }
	`)
	require.NoError(t, err)
	require.NoError(t, r1.SetOptions(r1.GetOptions().Apply(lib.Options{Throw: null.BoolFrom(true)})))

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			vu, err := r.newVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
			require.NoError(t, err)

			fnCalled := false
			require.NoError(t, vu.Runtime.Set("fn", func() {
				fnCalled = true

				require.NotNil(t, vu.moduleVUImpl.Runtime())
				require.Nil(t, vu.moduleVUImpl.InitEnv())

				state := vu.moduleVUImpl.State()
				require.NotNil(t, state)
				assert.Equal(t, null.IntFrom(10), state.Options.VUs)
				assert.Equal(t, null.BoolFrom(true), state.Options.Throw)
				assert.NotNil(t, state.Logger)
				assert.Equal(t, vu.Transport, state.Transport)
			}))

			activeVU := vu.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = activeVU.RunOnce()
			require.NoError(t, err)
			assert.True(t, fnCalled, "fn() not called")
		})
	}
}

func TestVURunInterrupt(t *testing.T) {
	t.Parallel()
	r1, err := getSimpleRunner(t, "/script.js", `
		exports.default = function() { while(true) {} }
		`)
	require.NoError(t, err)
	require.NoError(t, r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)}))

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)
	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		name, r := name, r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			samples := newDevNullSampleChannel()
			defer close(samples)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()

			vu, err := r.newVU(ctx, 1, 1, samples)
			require.NoError(t, err)
			activeVU := vu.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = activeVU.RunOnce()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "context canceled")
		})
	}
}

func TestVURunInterruptDoesntPanic(t *testing.T) {
	t.Parallel()
	r1, err := getSimpleRunner(t, "/script.js", `
		exports.default = function() { while(true) {} }
		`)
	require.NoError(t, err)
	require.NoError(t, r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)}))

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)
	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			samples := newDevNullSampleChannel()
			defer close(samples)
			var wg sync.WaitGroup

			initVU, err := r.newVU(ctx, 1, 1, samples)
			require.NoError(t, err)
			for i := 0; i < 100; i++ {
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
					require.Error(t, vuErr)
					assert.Contains(t, vuErr.Error(), "context canceled")
				}()
				<-ch
				time.Sleep(time.Microsecond * 1) // NOTE: increase this in case of problems ;)
				newCancel()
				wg.Wait()
			}
		})
	}
}

func TestVUIntegrationMetrics(t *testing.T) {
	t.Parallel()
	testdata := make(map[string]*Runner, 2)
	{
		r1, err := getSimpleRunner(t, "/script.js", `
		var group = require("k6").group;
		var Trend = require("k6/metrics").Trend;
		var myMetric = new Trend("my_metric");
		exports.default = function() { myMetric.add(5); }
		`)
		require.NoError(t, err)
		testdata["Source"] = r1

		r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
		require.NoError(t, err)
		testdata["Archive"] = r2
	}

	for name, r := range testdata {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			samples := make(chan metrics.SampleContainer, 100)
			defer close(samples)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			vu, err := r.newVU(ctx, 1, 1, samples)
			require.NoError(t, err)
			activeVU := vu.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = activeVU.RunOnce()
			require.NoError(t, err)
			sampleCount := 0
			builtinMetrics := r.preInitState.BuiltinMetrics
			for _, sampleC := range metrics.GetBufferedSamples(samples) {
				for _, s := range sampleC.GetSamples() {
					switch sampleCount {
					case 0:
						assert.Equal(t, 5.0, s.Value)
						assert.Equal(t, "my_metric", s.Metric.Name)
						assert.Equal(t, metrics.Trend, s.Metric.Type)
					case 1:
						assert.Equal(t, 0.0, s.Value)
						assert.Same(t, builtinMetrics.DataSent, s.Metric, "`data_sent` sample is before `data_received` and `iteration_duration`")
					case 2:
						assert.Equal(t, 0.0, s.Value)
						assert.Same(t, builtinMetrics.DataReceived, s.Metric, "`data_received` sample is after `data_received`")
					case 3:
						assert.Same(t, builtinMetrics.IterationDuration, s.Metric, "`iteration-duration` sample is after `data_received`")
					case 4:
						assert.Same(t, builtinMetrics.Iterations, s.Metric, "`iterations` sample is after `iteration_duration`")
						assert.Equal(t, float64(1), s.Value)
					}
					sampleCount++
				}
			}
			assert.Equal(t, sampleCount, 5)
		})
	}
}

func generateTLSCertificate(t *testing.T, host string, notBefore time.Time, validFor time.Duration) ([]byte, []byte) {
	return generateTLSCertificateWithCA(t, host, notBefore, validFor, nil, nil)
}

func generateTLSCertificateWithCA(t *testing.T, host string, notBefore time.Time, validFor time.Duration, parent *x509.Certificate, ppriv *rsa.PrivateKey) ([]byte, []byte) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// ECDSA, ED25519 and RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	keyUsage := x509.KeyUsageDigitalSignature
	// Only RSA subject keys should have the KeyEncipherment KeyUsage bits set. In
	// the context of TLS this KeyUsage is particular to RSA key exchange and
	// authentication.
	keyUsage |= x509.KeyUsageKeyEncipherment

	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.SHA256WithRSA,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if parent == nil {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
		parent = &template
		ppriv = priv
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, parent, &priv.PublicKey, ppriv)
	require.NoError(t, err)

	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	require.NoError(t, err)

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	keyPem := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	require.NoError(t, err)
	return certPem, keyPem
}

func getTestServerWithCertificate(t *testing.T, certPem, key []byte) *httptest.Server {
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       time.Second,
	}
	s := &httptest.Server{}
	s.Config = server

	s.TLS = new(tls.Config)
	if s.TLS.NextProtos == nil {
		nextProtos := []string{"http/1.1"}
		if s.EnableHTTP2 {
			nextProtos = []string{"h2"}
		}
		s.TLS.NextProtos = nextProtos
	}
	cert, err := tls.X509KeyPair(certPem, key)
	require.NoError(t, err)
	s.TLS.Certificates = append(s.TLS.Certificates, cert)
	for _, suite := range tls.CipherSuites() {
		if !strings.Contains(suite.Name, "256") {
			continue
		}
		s.TLS.CipherSuites = append(s.TLS.CipherSuites, suite.ID)
	}
	certpool := x509.NewCertPool()
	certificate, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	certpool.AddCert(certificate)
	client := &http.Client{Transport: &http.Transport{}}
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{ //nolint:gosec
			RootCAs: certpool,
		},
		ForceAttemptHTTP2: s.EnableHTTP2,
	}
	s.Listener, err = net.Listen("tcp", "")
	require.NoError(t, err)
	s.Listener = tls.NewListener(s.Listener, s.TLS)
	s.URL = "https://" + s.Listener.Addr().String()
	return s
}

func TestVUIntegrationInsecureRequests(t *testing.T) {
	t.Parallel()
	certPem, keyPem := generateTLSCertificate(t, "mybadssl.localhost", time.Now(), 0)
	s := getTestServerWithCertificate(t, certPem, keyPem)
	go func() {
		_ = s.Config.Serve(s.Listener)
	}()
	t.Cleanup(func() {
		require.NoError(t, s.Config.Close())
	})
	host, port, err := net.SplitHostPort(s.Listener.Addr().String())
	require.NoError(t, err)
	ip := net.ParseIP(host)
	mybadsslHostname, err := types.NewHost(ip, port)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(s.TLS.Certificates[0].Certificate[0])
	require.NoError(t, err)

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
			t.Parallel()
			r1, err := getSimpleRunner(t, "/script.js", `
			  var http = require("k6/http");;
        exports.default = function() { http.get("https://mybadssl.localhost/"); }
				`)
			require.NoError(t, err)
			require.NoError(t, r1.SetOptions(lib.Options{Throw: null.BoolFrom(true)}.Apply(data.opts)))

			r1.Bundle.Options.Hosts, err = types.NewNullHosts(map[string]types.Host{
				"mybadssl.localhost": *mybadsslHostname,
			})
			require.NoError(t, err)
			r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
			require.NoError(t, err)
			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for name, r := range runners {
				r := r
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					r.preInitState.Logger, _ = logtest.NewNullLogger()

					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
					require.NoError(t, err)
					initVU.(*VU).TLSConfig.RootCAs = x509.NewCertPool()
					initVU.(*VU).TLSConfig.RootCAs.AddCert(cert)

					vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
					err = vu.RunOnce()
					if data.errMsg != "" {
						require.Error(t, err)
						assert.Contains(t, err.Error(), data.errMsg)
					} else {
						require.NoError(t, err)
					}
				})
			}
		})
	}
}

func TestVUIntegrationBlacklistOption(t *testing.T) {
	t.Parallel()
	r1, err := getSimpleRunner(t, "/script.js", `
					var http = require("k6/http");;
					exports.default = function() { http.get("http://10.1.2.3/"); }
				`)
	require.NoError(t, err)

	cidr, err := lib.ParseCIDR("10.0.0.0/8")
	require.NoError(t, err)
	r1.Bundle.Options.Throw = null.BoolFrom(true)
	r1.Bundle.Options.BlacklistIPs = []*lib.IPNet{cidr}

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "IP (10.1.2.3) is in a blacklisted range (10.0.0.0/8)")
		})
	}
}

func TestVUIntegrationBlacklistScript(t *testing.T) {
	t.Parallel()
	r1, err := getSimpleRunner(t, "/script.js", `
					var http = require("k6/http");;

					exports.options = {
						throw: true,
						blacklistIPs: ["10.0.0.0/8"],
					};

					exports.default = function() { http.get("http://10.1.2.3/"); }
				`)
	require.NoError(t, err)

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}

	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "IP (10.1.2.3) is in a blacklisted range (10.0.0.0/8)")
		})
	}
}

func TestVUIntegrationBlockHostnamesOption(t *testing.T) {
	t.Parallel()
	r1, err := getSimpleRunner(t, "/script.js", `
					var http = require("k6/http");
					exports.default = function() { http.get("https://k6.io/"); }
				`)
	require.NoError(t, err)

	hostnames, err := types.NewNullHostnameTrie([]string{"*.io"})
	require.NoError(t, err)

	r1.Bundle.Options.Throw = null.BoolFrom(true)
	r1.Bundle.Options.BlockedHostnames = hostnames

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}

	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			initVu, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
			require.NoError(t, err)

			vu := initVu.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "hostname (k6.io) is in a blocked pattern (*.io)")
		})
	}
}

func TestVUIntegrationBlockHostnamesScript(t *testing.T) {
	t.Parallel()
	r1, err := getSimpleRunner(t, "/script.js", `
					var http = require("k6/http");

					exports.options = {
						throw: true,
						blockHostnames: ["*.io"],
					};

					exports.default = function() { http.get("https://k6.io/"); }
				`)
	require.NoError(t, err)

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}

	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVu, err := r.NewVU(ctx, 0, 0, make(chan metrics.SampleContainer, 100))
			require.NoError(t, err)
			vu := initVu.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "hostname (k6.io) is in a blocked pattern (*.io)")
		})
	}
}

func TestVUIntegrationHosts(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

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
	require.NoError(t, err)

	require.NoError(t, r1.SetOptions(lib.Options{
		Throw: null.BoolFrom(true),
		Hosts: func() types.NullHosts {
			hosts, er := types.NewNullHosts(map[string]types.Host{
				"test.loadimpact.com": {IP: net.ParseIP("127.0.0.1")},
			})
			require.NoError(t, er)

			return hosts
		}(),
	}))

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
			require.NoError(t, err)

			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.NoError(t, err)
		})
	}
}

func TestVUIntegrationTLSConfig(t *testing.T) {
	t.Parallel()
	certPem, keyPem := generateTLSCertificate(t, "sha256-badssl.localhost", time.Now(), time.Hour)
	s := getTestServerWithCertificate(t, certPem, keyPem)
	go func() {
		_ = s.Config.Serve(s.Listener)
	}()
	t.Cleanup(func() {
		require.NoError(t, s.Config.Close())
	})
	host, port, err := net.SplitHostPort(s.Listener.Addr().String())
	require.NoError(t, err)
	ip := net.ParseIP(host)
	mybadsslHostname, err := types.NewHost(ip, port)
	require.NoError(t, err)
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
			lib.Options{
				TLSCipherSuites: &lib.TLSCipherSuites{tls.TLS_RSA_WITH_RC4_128_SHA},
				TLSVersion:      &lib.TLSVersions{Max: tls.VersionTLS12},
			},
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
			lib.Options{TLSVersion: &lib.TLSVersions{Min: tls.VersionSSL30, Max: tls.VersionSSL30}}, //nolint:staticcheck
			unsupportedVersionErrorMsg,
		},
	}
	cert, err := x509.ParseCertificate(s.TLS.Certificates[0].Certificate[0])
	require.NoError(t, err)
	for name, data := range testdata {
		data := data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r1, err := getSimpleRunner(t, "/script.js", `
					var http = require("k6/http");;
					exports.default = function() { http.get("https://sha256-badssl.localhost/"); }
				`)
			require.NoError(t, err)

			opts := lib.Options{Throw: null.BoolFrom(true)}
			require.NoError(t, r1.SetOptions(opts.Apply(data.opts)))

			r1.Bundle.Options.Hosts, err = types.NewNullHosts(map[string]types.Host{
				"sha256-badssl.localhost": *mybadsslHostname,
			})
			require.NoError(t, err)
			r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
			require.NoError(t, err)

			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for name, r := range runners {
				r := r
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					r.preInitState.Logger, _ = logtest.NewNullLogger()

					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
					require.NoError(t, err)
					initVU.(*VU).TLSConfig.RootCAs = x509.NewCertPool()
					initVU.(*VU).TLSConfig.RootCAs.AddCert(cert)
					vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
					err = vu.RunOnce()
					if data.errMsg != "" {
						require.Error(t, err, "for message %q", data.errMsg)
						assert.Contains(t, err.Error(), data.errMsg)
					} else {
						require.NoError(t, err)
					}
				})
			}
		})
	}
}

func TestVUIntegrationRequireFunctionError(t *testing.T) {
	t.Parallel()
	r, err := getSimpleRunner(t, "/script.js", `
			exports.default = function() { require("k6/http") }
		`)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
	require.NoError(t, err)
	vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
	err = vu.RunOnce()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only available in the init stage")
}

func TestVUIntegrationOpenFunctionError(t *testing.T) {
	t.Parallel()
	r, err := getSimpleRunner(t, "/script.js", `
			exports.default = function() { open("/tmp/foo") }
		`)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
	require.NoError(t, err)
	vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
	err = vu.RunOnce()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only available in the init stage")
}

func TestVUIntegrationOpenFunctionErrorWhenSneaky(t *testing.T) {
	t.Parallel()
	r, err := getSimpleRunner(t, "/script.js", `
			var sneaky = open;
			exports.default = function() { sneaky("/tmp/foo") }
		`)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
	require.NoError(t, err)
	vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
	err = vu.RunOnce()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only available in the init stage")
}

func TestVUDoesOpenUnderV0Condition(t *testing.T) {
	t.Parallel()

	baseFS := fsext.NewMemMapFs()
	data := `
			if (__VU == 0) {
				let data = open("/home/somebody/test.json");
			}
			exports.default = function() {
				console.log("hey")
			}
		`
	require.NoError(t, fsext.WriteFile(baseFS, "/home/somebody/test.json", []byte(`42`), fs.ModePerm))
	require.NoError(t, fsext.WriteFile(baseFS, "/script.js", []byte(data), fs.ModePerm))

	fs := fsext.NewCacheOnReadFs(baseFS, fsext.NewMemMapFs(), 0)

	r, err := getSimpleRunner(t, "/script.js", data, fs)
	require.NoError(t, err)

	_, err = r.NewVU(context.Background(), 1, 1, make(chan metrics.SampleContainer, 100))
	require.NoError(t, err)
}

func TestVUDoesNotOpenUnderConditions(t *testing.T) {
	t.Parallel()

	baseFS := fsext.NewMemMapFs()
	data := `
			if (__VU > 0) {
				let data = open("/home/somebody/test.json");
			}
			exports.default = function() {
				console.log("hey")
			}
		`
	require.NoError(t, fsext.WriteFile(baseFS, "/home/somebody/test.json", []byte(`42`), fs.ModePerm))
	require.NoError(t, fsext.WriteFile(baseFS, "/script.js", []byte(data), fs.ModePerm))

	fs := fsext.NewCacheOnReadFs(baseFS, fsext.NewMemMapFs(), 0)

	r, err := getSimpleRunner(t, "/script.js", data, fs)
	require.NoError(t, err)

	_, err = r.NewVU(context.Background(), 1, 1, make(chan metrics.SampleContainer, 100))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open() can't be used with files that weren't previously opened during initialization (__VU==0)")
}

func TestVUDoesNonExistingPathnUnderConditions(t *testing.T) {
	t.Parallel()

	baseFS := fsext.NewMemMapFs()
	data := `
			if (__VU == 1) {
				let data = open("/home/nobody");
			}
			exports.default = function() {
				console.log("hey")
			}
		`
	require.NoError(t, fsext.WriteFile(baseFS, "/script.js", []byte(data), fs.ModePerm))

	fs := fsext.NewCacheOnReadFs(baseFS, fsext.NewMemMapFs(), 0)

	r, err := getSimpleRunner(t, "/script.js", data, fs)
	require.NoError(t, err)

	_, err = r.NewVU(context.Background(), 1, 1, make(chan metrics.SampleContainer, 100))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open() can't be used with files that weren't previously opened during initialization (__VU==0)")
}

func TestVUDoesRequireUnderV0Condition(t *testing.T) {
	t.Parallel()

	baseFS := fsext.NewMemMapFs()
	data := `
			if (__VU == 0) {
				let data = require("/home/somebody/test.js");
			}
			exports.default = function() {
				console.log("hey")
			}
		`
	require.NoError(t, fsext.WriteFile(baseFS, "/home/somebody/test.js", []byte(`exports=42`), fs.ModePerm))
	require.NoError(t, fsext.WriteFile(baseFS, "/script.js", []byte(data), fs.ModePerm))

	fs := fsext.NewCacheOnReadFs(baseFS, fsext.NewMemMapFs(), 0)

	r, err := getSimpleRunner(t, "/script.js", data, fs)
	require.NoError(t, err)

	_, err = r.NewVU(context.Background(), 1, 1, make(chan metrics.SampleContainer, 100))
	require.NoError(t, err)
}

func TestVUDoesNotRequireUnderConditions(t *testing.T) {
	t.Parallel()

	baseFS := fsext.NewMemMapFs()
	data := `
			if (__VU > 0) {
				let data = require("/home/somebody/test.js");
			}
			exports.default = function() {
				console.log("hey")
			}
		`
	require.NoError(t, fsext.WriteFile(baseFS, "/home/somebody/test.js", []byte(`exports=42`), fs.ModePerm))
	require.NoError(t, fsext.WriteFile(baseFS, "/script.js", []byte(data), fs.ModePerm))

	fs := fsext.NewCacheOnReadFs(baseFS, fsext.NewMemMapFs(), 0)

	r, err := getSimpleRunner(t, "/script.js", data, fs)
	require.NoError(t, err)

	_, err = r.NewVU(context.Background(), 1, 1, make(chan metrics.SampleContainer, 100))
	require.Error(t, err)
	assert.Contains(t, err.Error(), " was not previously resolved during initialization (__VU==0)")
}

func TestVUDoesRequireUnderConditions(t *testing.T) {
	t.Parallel()

	baseFS := fsext.NewMemMapFs()
	data := `
			if (__VU == 0) {
				require("/home/somebody/test.js");
				require("/home/somebody/test2.js");
			}

			if (__VU % 2 == 1) {
				require("/home/somebody/test.js");
			}

			if (__VU % 2 == 0) {
				require("/home/somebody/test2.js");
			}

			exports.default = function() {
				console.log("hey")
			}
		`
	require.NoError(t, fsext.WriteFile(baseFS, "/home/somebody/test.js", []byte(`console.log("test.js", __VU)`), fs.ModePerm))
	require.NoError(t, fsext.WriteFile(baseFS, "/home/somebody/test2.js", []byte(`console.log("test2.js", __VU)`), fs.ModePerm))
	require.NoError(t, fsext.WriteFile(baseFS, "/script.js", []byte(data), fs.ModePerm))

	fs := fsext.NewCacheOnReadFs(baseFS, fsext.NewMemMapFs(), 0)

	logger, hook := testutils.NewLoggerWithHook(t, logrus.InfoLevel)
	r, err := getSimpleRunner(t, "/script.js", data, fs, logger)
	require.NoError(t, err)
	logs := hook.Drain()
	require.Len(t, logs, 2)

	_, err = r.NewVU(context.Background(), 1, 1, make(chan metrics.SampleContainer, 100))
	require.NoError(t, err)
	logs = hook.Drain()
	require.Len(t, logs, 1)
	require.Contains(t, logs[0].Message, "test.js 1")
	_, err = r.NewVU(context.Background(), 2, 2, make(chan metrics.SampleContainer, 100))
	require.NoError(t, err)
	logs = hook.Drain()
	require.Len(t, logs, 1)
	require.Contains(t, logs[0].Message, "test2.js 2")
}

func TestVUIntegrationCookiesReset(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

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
	require.NoError(t, err)
	r1.Bundle.Options.Throw = null.BoolFrom(true)
	r1.Bundle.Options.MaxRedirects = null.IntFrom(10)
	r1.Bundle.Options.Hosts = types.NullHosts{Trie: tb.Dialer.Hosts}

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			for i := 0; i < 2; i++ {
				require.NoError(t, vu.RunOnce())
			}
		})
	}
}

func TestVUIntegrationCookiesNoReset(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

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
	require.NoError(t, err)
	require.NoError(t, r1.SetOptions(lib.Options{
		Throw:          null.BoolFrom(true),
		MaxRedirects:   null.IntFrom(10),
		Hosts:          types.NullHosts{Trie: tb.Dialer.Hosts},
		NoCookiesReset: null.BoolFrom(true),
	}))

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
			require.NoError(t, err)

			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.NoError(t, err)

			err = vu.RunOnce()
			require.NoError(t, err)
		})
	}
}

func TestVUIntegrationVUID(t *testing.T) {
	t.Parallel()
	r1, err := getSimpleRunner(t, "/script.js", `
			exports.default = function() {
				if (__VU != 1234) { throw new Error("wrong __VU: " + __VU); }
			}`,
	)
	require.NoError(t, err)
	r1.Bundle.Options.Throw = null.BoolFrom(true)

	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			initVU, err := r.NewVU(ctx, 1234, 1234, make(chan metrics.SampleContainer, 100))
			require.NoError(t, err)

			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.NoError(t, err)
		})
	}
}

func TestVUIntegrationClientCerts(t *testing.T) {
	t.Parallel()

	// Generate CA key and certificate
	caCertPem, caKeyPem := generateTLSCertificate(t, "127.0.0.1", time.Now(), time.Hour)

	caCertBlock, _ := pem.Decode(caCertPem)
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	require.NoError(t, err)

	caKeyBlock, _ := pem.Decode(caKeyPem)
	caKeyAny, err := x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
	require.NoError(t, err)
	caKey, ok := caKeyAny.(*rsa.PrivateKey)
	require.True(t, ok)

	// Generate server key and certificate
	srvCertPem, srvKeyPem := generateTLSCertificateWithCA(t, "127.0.0.1", time.Now(), time.Hour, caCert, caKey)

	// Generate client Key and Certificate
	clCertPem, clKeyPem := generateTLSCertificateWithCA(t, "127.0.0.1", time.Now(), time.Hour, caCert, caKey)

	clientCAPool := x509.NewCertPool()
	assert.True(t, clientCAPool.AppendCertsFromPEM(caCertPem))

	serverCert, err := tls.X509KeyPair(append(srvCertPem, caCertPem...), srvKeyPem)
	require.NoError(t, err)

	testdata := map[string]struct {
		withClientCert     bool
		withDomains        bool
		insecureSkipVerify bool
		errMsg             string
	}{
		"WithoutCert":      {false, false, true, "remote error: tls:"},
		"WithCert":         {true, true, true, ""},
		"VerifyServerCert": {true, false, false, "certificate signed by unknown authority"},
		"WithoutDomains":   {true, false, true, ""},
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{ //nolint:gosec
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAPool,
	})
	require.NoError(t, err)
	srv := &http.Server{ //nolint:gosec
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, "ok")
		}),
		ErrorLog: stdlog.New(io.Discard, "", 0),
	}
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() { _ = listener.Close() })
	for name, data := range testdata {
		data := data

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r1, err := getSimpleRunner(t, "/script.js", fmt.Sprintf(`
			var http = require("k6/http");
			var k6 = require("k6");
			var check = k6.check;
			exports.default = function() {
				const res = http.get("https://%s")
				check(res, {
					'is status 200': (r) => r.status === 200,
					'verify resp': (r) => r.body.includes('ok'),
				})
			}`, listener.Addr().String()))
			require.NoError(t, err)

			opt := lib.Options{Throw: null.BoolFrom(true)}
			if data.insecureSkipVerify {
				opt.InsecureSkipTLSVerify = null.BoolFrom(true)
			}
			if data.withClientCert {
				opt.TLSAuth = []*lib.TLSAuth{
					{
						TLSAuthFields: lib.TLSAuthFields{
							Cert: string(clCertPem),
							Key:  string(clKeyPem),
						},
					},
				}
				if data.withDomains {
					opt.TLSAuth[0].TLSAuthFields.Domains = []string{"127.0.0.1"}
				}
				_, _ = opt.TLSAuth[0].Certificate()
			}
			require.NoError(t, r1.SetOptions(opt))
			r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
			require.NoError(t, err)

			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for name, r := range runners {
				r := r
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					r.preInitState.Logger, _ = logtest.NewNullLogger()
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
					require.NoError(t, err)
					vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
					err = vu.RunOnce()
					if len(data.errMsg) > 0 {
						require.Error(t, err)
						assert.ErrorContains(t, err, data.errMsg)
					} else {
						require.NoError(t, err)
					}
				})
			}
		})
	}
}

func TestHTTPRequestInInitContext(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

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
	require.Error(t, err)
	assert.Contains(
		t,
		err.Error(),
		k6http.ErrHTTPForbiddenInInitContext.Error())
}

func TestInitContextForbidden(t *testing.T) {
	t.Parallel()
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
			"abortTest",
			`var test = require("k6/execution").test;
			 test.abort();
			 exports.default = function() { console.log("p"); }`,
			errext.AbortTest,
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

	for _, test := range table {
		test := test
		t.Run(test[0], func(t *testing.T) {
			t.Parallel()
			_, err := getSimpleRunner(t, "/script.js", tb.Replacer.Replace(test[1]))
			require.Error(t, err)
			assert.Contains(
				t,
				err.Error(),
				test[2])
		})
	}
}

func TestArchiveRunningIntegrity(t *testing.T) {
	t.Parallel()

	fileSystem := fsext.NewMemMapFs()
	data := `
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
		`
	require.NoError(t, fsext.WriteFile(fileSystem, "/home/somebody/test.json", []byte(`42`), fs.ModePerm))
	require.NoError(t, fsext.WriteFile(fileSystem, "/script.js", []byte(data), fs.ModePerm))
	r1, err := getSimpleRunner(t, "/script.js", data, fileSystem)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	require.NoError(t, r1.MakeArchive().Write(buf))

	arc, err := lib.ReadArchive(buf)
	require.NoError(t, err)
	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var err error
			ch := make(chan metrics.SampleContainer, 100)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err = r.Setup(ctx, ch)
			cancel()
			require.NoError(t, err)
			ctx, cancel = context.WithCancel(context.Background())
			defer cancel()
			initVU, err := r.NewVU(ctx, 1, 1, ch)
			require.NoError(t, err)
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.NoError(t, err)
		})
	}
}

func TestArchiveNotPanicking(t *testing.T) {
	t.Parallel()
	fileSystem := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fileSystem, "/non/existent", []byte(`42`), fs.ModePerm))
	r1, err := getSimpleRunner(t, "/script.js", `
			var fput = open("/non/existent");
			exports.default = function(data) {}
		`, fileSystem)
	require.NoError(t, err)

	arc := r1.MakeArchive()
	arc.Filesystems = map[string]fsext.Fs{"file": fsext.NewMemMapFs()}
	r2, err := getSimpleArchiveRunner(t, arc)
	// we do want this to error here as this is where we find out that a given file is not in the
	// archive
	require.Error(t, err)
	require.Nil(t, r2)
}

func TestStuffNotPanicking(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	r, err := getSimpleRunner(t, "/script.js", tb.Replacer.Replace(`
			var http = require("k6/http");
			var ws = require("k6/ws");
			var group = require("k6").group;
			var parseHTML = require("k6/html").parseHTML;

			exports.options = { iterations: 1, vus: 1 };

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

	ctx, cancel := context.WithCancel(context.Background())

	ch := make(chan metrics.SampleContainer, 1000)
	initVU, err := r.NewVU(ctx, 1, 1, ch)
	require.NoError(t, err)

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
	t.Parallel()

	r, err := getSimpleRunner(t, "/script.js", `
			var parseHTML = require("k6/html").parseHTML;

			exports.options = { iterations: 1, vus: 1 };

			exports.default = function() {
				var doc = parseHTML("<html>");
				var o = doc.find(".something").slice(0, 4).toArray()
			};
		`)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	ch := make(chan metrics.SampleContainer, 1000)
	initVU, err := r.NewVU(ctx, 1, 1, ch)
	require.NoError(t, err)

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

	// Handle paths with custom logic
	tb.Mux.HandleFunc("/wrong-redirect", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Location", "%")
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

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

	for num, tc := range testedSystemTags {
		num, tc := num, tc
		t.Run(fmt.Sprintf("TC %d with only %s", num, tc.tag), func(t *testing.T) {
			t.Parallel()
			samples := make(chan metrics.SampleContainer, 100)
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
			require.NoError(t, r.SetOptions(r.GetOptions().Apply(lib.Options{
				Throw:                 null.BoolFrom(false),
				TLSVersion:            &lib.TLSVersions{Max: tls.VersionTLS13},
				SystemTags:            metrics.ToSystemTagSet([]string{tc.tag}),
				InsecureSkipTLSVerify: null.BoolFrom(true),
			})))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			vu, err := r.NewVU(ctx, uint64(num), 0, samples) //nolint:gosec
			require.NoError(t, err)
			activeVU := vu.Activate(&lib.VUActivationParams{
				RunContext: ctx,
				Exec:       tc.exec,
				Scenario:   "default",
			})
			require.NoError(t, activeVU.RunOnce())

			bufSamples := metrics.GetBufferedSamples(samples)
			require.NotEmpty(t, bufSamples)
			for _, sample := range bufSamples[0].GetSamples() {
				assert.NotEmpty(t, sample.Tags)
				for emittedTag, emittedVal := range sample.Tags.Map() {
					assert.Equal(t, tc.tag, emittedTag)
					assert.Equal(t, tc.expVal, emittedVal)
				}
			}
		})
	}
}

type multiFileTestCase struct {
	fses       map[string]fsext.Fs
	rtOpts     lib.RuntimeOptions
	cwd        string
	script     string
	expInitErr bool
	expVUErr   bool
	samples    chan metrics.SampleContainer
}

func runMultiFileTestCase(t *testing.T, tc multiFileTestCase, tb *httpmultibin.HTTPMultiBin) {
	t.Helper()
	runner, err := getSimpleRunner(t, strings.TrimRight(tc.cwd, "/")+"/script.js", tc.script, tc.rtOpts, tc.fses)
	if tc.expInitErr {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	options := runner.GetOptions()
	require.Empty(t, options.Validate())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	vu, err := runner.NewVU(ctx, 1, 1, tc.samples)
	require.NoError(t, err)

	jsVU, ok := vu.(*VU)
	require.True(t, ok)
	jsVU.state.Dialer = tb.Dialer
	jsVU.state.TLSConfig = tb.TLSClientConfig

	activeVU := vu.Activate(&lib.VUActivationParams{RunContext: ctx})

	err = activeVU.RunOnce()
	if tc.expVUErr {
		require.Error(t, err)
	} else {
		require.NoError(t, err)
	}

	arc := runner.MakeArchive()
	runnerFromArc, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)
	vuFromArc, err := runnerFromArc.NewVU(ctx, 2, 2, tc.samples)
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

	tb.GRPCStub.UnaryCallFunc = func(_ context.Context, _ *grpc_testing.SimpleRequest) (
		*grpc_testing.SimpleResponse, error,
	) {
		return &grpc_testing.SimpleResponse{
			Username: "foo",
		}, nil
	}

	fs := fsext.NewMemMapFs()
	protoFile, err := os.ReadFile("../lib/testutils/httpmultibin/grpc_testing/test.proto") //nolint:forbidigo
	require.NoError(t, err)
	require.NoError(t, fsext.WriteFile(fs, "/path/to/service.proto", protoFile, 0o644))
	require.NoError(t, fsext.WriteFile(fs, "/path/to/same-dir.proto", []byte(
		`syntax = "proto3";package whatever;import "service.proto";`,
	), 0o644))
	require.NoError(t, fsext.WriteFile(fs, "/path/subdir.proto", []byte(
		`syntax = "proto3";package whatever;import "to/service.proto";`,
	), 0o644))
	require.NoError(t, fsext.WriteFile(fs, "/path/to/abs.proto", []byte(
		`syntax = "proto3";package whatever;import "/path/to/service.proto";`,
	), 0o644))

	grpcTestCase := func(expInitErr, expVUErr bool, cwd, loadCode string) multiFileTestCase {
		script := tb.Replacer.Replace(fmt.Sprintf(`
			var grpc = require('k6/net/grpc');
			var client = new grpc.Client();

			%s // load statements

			exports.default = function() {
				client.connect('GRPCBIN_ADDR', {timeout: '3s'});
				try {
					var resp = client.invoke('grpc.testing.TestService/UnaryCall', {})
					if (!resp.message || resp.error || resp.message.username !== 'foo') {
						throw new Error('unexpected response message: ' + JSON.stringify(resp.message))
					}
				} finally {
					client.close();
				}
			}
		`, loadCode))

		return multiFileTestCase{
			fses:    map[string]fsext.Fs{"file": fs, "https": fsext.NewMemMapFs()},
			rtOpts:  lib.RuntimeOptions{CompatibilityMode: null.NewString("base", true)},
			samples: make(chan metrics.SampleContainer, 100),
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
			t.Parallel()
			t.Logf(
				"CWD: %s, expInitErr: %t, expVUErr: %t, script injected with: `%s`",
				tc.cwd, tc.expInitErr, tc.expVUErr, tc.script,
			)
			runMultiFileTestCase(t, tc, tb)
		})
	}
}

func TestMinIterationDurationIsCancellable(t *testing.T) {
	t.Parallel()

	r, err := getSimpleRunner(t, "/script.js", `
			exports.options = { iterations: 1, vus: 1, minIterationDuration: '1m' };

			exports.default = function() { /* do nothing */ };
		`)
	require.NoError(t, err)

	ch := make(chan metrics.SampleContainer, 1000)
	ctx, cancel := context.WithCancel(context.Background())
	initVU, err := r.NewVU(ctx, 1, 1, ch)
	require.NoError(t, err)

	vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
	errC := make(chan error)
	go func() { errC <- vu.RunOnce() }()

	time.Sleep(200 * time.Millisecond) // give it some time to actually start

	cancel() // simulate the end of gracefulStop or a Ctrl+C event

	select {
	case <-time.After(3 * time.Second):
		t.Fatal("Test timed out or minIterationDuration prevailed")
	case err := <-errC:
		require.NoError(t, err)
	}
}

func TestForceHTTP1Feature(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		godebug               string
		expectedForceH1Result bool
		protocol              string
	}{
		"Force H1 Enabled. Checking for H1": {
			godebug:               "http2client=0,gctrace=1",
			expectedForceH1Result: true,
			protocol:              "HTTP/1.1",
		},
		"Force H1 Disabled. Checking for H2": {
			godebug:               "test=0",
			expectedForceH1Result: false,
			protocol:              "HTTP/2.0",
		},
	}

	for name, tc := range cases {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			lookupEnv := func(key string) (string, bool) {
				if key == "GODEBUG" {
					return tc.godebug, true
				}
				return "", false
			}
			tb := httpmultibin.NewHTTPMultiBin(t)

			data := fmt.Sprintf(`var k6 = require("k6");
			var check = k6.check;
			var fail = k6.fail;
			var http = require("k6/http");;
			exports.default = function() {
				var res = http.get("HTTP2BIN_URL");
				if (
					!check(res, {
					'checking to see if status was 200': (res) => res.status === 200,
					'checking to see protocol': (res) => res.proto === '%s'
					})
				) {
					fail('test failed')
				}
			}`, tc.protocol)

			r1, err := getSimpleRunner(t, "/script.js", tb.Replacer.Replace(data))
			require.NoError(t, err)
			r1.preInitState.LookupEnv = lookupEnv

			assert.Equal(t, tc.expectedForceH1Result, r1.forceHTTP1())

			err = r1.SetOptions(lib.Options{
				Hosts: types.NullHosts{Trie: tb.Dialer.Hosts},
				// We disable TLS verify so that we don't get a TLS handshake error since
				// the certificates on the endpoint are not certified by a certificate authority
				InsecureSkipTLSVerify: null.BoolFrom(true),
			})

			require.NoError(t, err)

			r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
			require.NoError(t, err)
			r2.preInitState.LookupEnv = lookupEnv
			assert.Equal(t, tc.expectedForceH1Result, r2.forceHTTP1())

			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for name, r := range runners {
				r := r
				t.Run(name, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
					require.NoError(t, err)

					vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
					err = vu.RunOnce()
					require.NoError(t, err)
				})
			}
		})
	}
}

func TestExecutionInfo(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name, script, expErr string
	}{
		{name: "vu_ok", script: `
		var exec = require('k6/execution');

		exports.default = function() {
			if (exec.vu.idInInstance !== 1) throw new Error('unexpected VU ID: '+exec.vu.idInInstance);
			if (exec.vu.idInTest !== 10) throw new Error('unexpected global VU ID: '+exec.vu.idInTest);
			if (exec.vu.iterationInInstance !== 0) throw new Error('unexpected VU iteration: '+exec.vu.iterationInInstance);
			if (exec.vu.iterationInScenario !== 0) throw new Error('unexpected scenario iteration: '+exec.vu.iterationInScenario);
		}`},
		{name: "vu_err", script: `
		var exec = require('k6/execution');
		exec.vu;
		`, expErr: "getting VU information in the init context is not supported"},
		{name: "scenario_ok", script: `
		var exec = require('k6/execution');
		var sleep = require('k6').sleep;

		exports.default = function() {
			var si = exec.scenario;
			sleep(0.1);
			if (si.name !== 'default') throw new Error('unexpected scenario name: '+si.name);
			if (si.executor !== 'test-exec') throw new Error('unexpected executor: '+si.executor);
			if (si.startTime > new Date().getTime()) throw new Error('unexpected startTime: '+si.startTime);
			if (si.progress !== 0.1) throw new Error('unexpected progress: '+si.progress);
			if (si.iterationInInstance !== 3) throw new Error('unexpected scenario local iteration: '+si.iterationInInstance);
			if (si.iterationInTest !== 4) throw new Error('unexpected scenario local iteration: '+si.iterationInTest);
		}`},
		{name: "scenario_err", script: `
		var exec = require('k6/execution');
		exec.scenario;
		`, expErr: "getting scenario information outside of the VU context is not supported"},
		{name: "test_ok", script: `
		var exec = require('k6/execution');

		exports.default = function() {
			var ti = exec.instance;
			if (ti.currentTestRunDuration !== 0) throw new Error('unexpected test duration: '+ti.currentTestRunDuration);
			if (ti.vusActive !== 1) throw new Error('unexpected vusActive: '+ti.vusActive);
			if (ti.vusInitialized !== 0) throw new Error('unexpected vusInitialized: '+ti.vusInitialized);
			if (ti.iterationsCompleted !== 0) throw new Error('unexpected iterationsCompleted: '+ti.iterationsCompleted);
			if (ti.iterationsInterrupted !== 0) throw new Error('unexpected iterationsInterrupted: '+ti.iterationsInterrupted);
		}`},
		{name: "test_err", script: `
		var exec = require('k6/execution');
		exec.instance;
		`, expErr: "getting instance information in the init context is not supported"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := getSimpleRunner(t, "/script.js", tc.script)
			if tc.expErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expErr)
				return
			}
			require.NoError(t, err)

			r.Bundle.Options.SystemTags = &metrics.DefaultSystemTagSet
			samples := make(chan metrics.SampleContainer, 100)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			initVU, err := r.NewVU(ctx, 1, 10, samples)
			require.NoError(t, err)

			testRunState := &lib.TestRunState{
				TestPreInitState: r.preInitState,
				Options:          r.GetOptions(),
				Runner:           r,
			}

			execScheduler, err := execution.NewScheduler(testRunState, local.NewController())
			require.NoError(t, err)

			ctx = lib.WithExecutionState(ctx, execScheduler.GetState())
			ctx = lib.WithScenarioState(ctx, &lib.ScenarioState{
				Name:      "default",
				Executor:  "test-exec",
				StartTime: time.Now(),
				ProgressFn: func() (float64, []string) {
					return 0.1, nil
				},
			})
			vu := initVU.Activate(&lib.VUActivationParams{
				RunContext:               ctx,
				Exec:                     "default",
				GetNextIterationCounters: func() (uint64, uint64) { return 3, 4 },
			})

			execState := execScheduler.GetState()
			execState.ModCurrentlyActiveVUsCount(+1)
			err = vu.RunOnce()
			require.NoError(t, err)
		})
	}
}

func TestPromiseRejectionIsCleared(t *testing.T) {
	t.Parallel()

	r1, err := getSimpleRunner(t, "/script.js", `
exports.default = () => {
    let p = new Promise((res) => {
        if (__ITER == 1) {
            throw "oops"
        }
        res("yes");
    })
    p.then((r) => {
        console.log(r);
    })
}`)
	require.NoError(t, err)
	r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
	require.NoError(t, err)

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
			require.NoError(t, err)

			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.NoError(t, err)

			err = vu.RunOnce()
			require.ErrorContains(t, err, "Uncaught (in promise) oops")

			err = vu.RunOnce()
			require.NoError(t, err)
		})
	}
}

func TestArchivingAnArchiveWorks(t *testing.T) {
	t.Parallel()
	r1, err := getSimpleRunner(t, "/script.js", `
			exports.default = function() {}
		`)
	require.NoError(t, err)

	arc := r1.MakeArchive()
	r2, err := getSimpleArchiveRunner(t, arc)
	require.NoError(t, err)
	require.NotNil(t, r2)

	arc2 := r2.MakeArchive()
	r3, err := getSimpleArchiveRunner(t, arc2)
	require.NoError(t, err)
	require.NotNil(t, r3)
}
