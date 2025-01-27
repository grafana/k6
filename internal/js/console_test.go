package js

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"testing"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/metrics"
)

func TestConsoleContext(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	logger, hook := logtest.NewNullLogger()
	_ = rt.Set("console", &console{logger})

	_, err := rt.RunString(`console.log("a")`)
	require.NoError(t, err)
	entry := hook.LastEntry()
	require.NotNil(t, entry)
	assert.Equal(t, "a", entry.Message)

	_, err = rt.RunString(`console.log("b")`)
	require.NoError(t, err)
	entry = hook.LastEntry()
	require.NotNil(t, entry)
	require.Equal(t, "b", entry.Message)
}

func getSimpleRunner(tb testing.TB, filename, data string, opts ...interface{}) (*Runner, error) {
	var (
		rtOpts      = lib.RuntimeOptions{CompatibilityMode: null.NewString("base", true)}
		logger      = testutils.NewLogger(tb)
		fsResolvers = map[string]fsext.Fs{"file": fsext.NewMemMapFs(), "https": fsext.NewMemMapFs()}
	)
	for _, o := range opts {
		switch opt := o.(type) {
		case fsext.Fs:
			fsResolvers["file"] = opt
		case map[string]fsext.Fs:
			fsResolvers = opt
		case lib.RuntimeOptions:
			rtOpts = opt
		case logrus.FieldLogger:
			logger = opt
		default:
			tb.Fatalf("unknown test option %q", opt)
		}
	}
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	return New(
		&lib.TestPreInitState{
			Logger:         logger,
			RuntimeOptions: rtOpts,
			BuiltinMetrics: builtinMetrics,
			Registry:       registry,
			LookupEnv:      func(_ string) (val string, ok bool) { return "", false },
			Usage:          usage.New(),
		},
		&loader.SourceData{
			URL:  &url.URL{Path: filename, Scheme: "file"},
			Data: []byte(data),
		},
		fsResolvers,
	)
}

func getSimpleArchiveRunner(tb testing.TB, arc *lib.Archive, opts ...interface{}) (*Runner, error) {
	var (
		rtOpts      = lib.RuntimeOptions{CompatibilityMode: null.NewString("base", true)}
		logger      = testutils.NewLogger(tb)
		fsResolvers = map[string]fsext.Fs{"file": fsext.NewMemMapFs(), "https": fsext.NewMemMapFs()}
	)
	for _, o := range opts {
		switch opt := o.(type) {
		case fsext.Fs:
			fsResolvers["file"] = opt
		case map[string]fsext.Fs:
			fsResolvers = opt
		case lib.RuntimeOptions:
			rtOpts = opt
		case logrus.FieldLogger:
			logger = opt
		default:
			tb.Fatalf("unknown test option %q", opt)
		}
	}
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	return NewFromArchive(
		&lib.TestPreInitState{
			Logger:         logger,
			RuntimeOptions: rtOpts,
			BuiltinMetrics: builtinMetrics,
			Registry:       registry,
			Usage:          usage.New(),
		}, arc)
}

// TODO: remove the need for this function, see https://github.com/grafana/k6/issues/2968
//
//nolint:forbidigo
func extractLogger(vu lib.ActiveVU) *logrus.Logger {
	vuSpecific, ok := vu.(*ActiveVU)
	if !ok {
		panic("lib.ActiveVU can't be caset to *ActiveVU")
	}
	fl := vuSpecific.Console.logger
	switch e := fl.(type) {
	case *logrus.Entry:
		return e.Logger
	case *logrus.Logger:
		return e
	default:
		panic(fmt.Sprintf("unknown logrus.FieldLogger option %q", fl))
	}
}

func TestConsoleLogWithSobekNativeObject(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	obj := rt.NewObject()
	err := obj.Set("text", "nativeObject")
	require.NoError(t, err)

	logger, hook := testutils.NewLoggerWithHook(t)

	c := newConsole(logger)
	c.Log(obj)

	entry := hook.LastEntry()
	require.NotNil(t, entry, "nothing logged")
	require.JSONEq(t, `{"text":"nativeObject"}`, entry.Message)
}

func TestConsoleLogObjectsWithGoTypes(t *testing.T) {
	t.Parallel()

	type value struct {
		Text string
	}

	tests := []struct {
		name string
		in   interface{}
		exp  string
	}{
		{
			name: "StructLiteral",
			in: value{
				Text: "test1",
			},
			exp: `{"text":"test1"}`,
		},
		{
			name: "StructPointer",
			in: &value{
				Text: "test2",
			},
			exp: `{"text":"test2"}`,
		},
		{
			name: "Map",
			in: map[string]interface{}{
				"text": "test3",
			},
			exp: `{"text":"test3"}`,
		},
	}

	expFields := logrus.Fields{"source": "console"}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := sobek.New()
			rt.SetFieldNameMapper(common.FieldNameMapper{})
			obj := rt.ToValue(tt.in)

			logger, hook := testutils.NewLoggerWithHook(t)

			c := newConsole(logger)
			c.Log(obj)

			entry := hook.LastEntry()
			require.NotNil(t, entry, "nothing logged")
			assert.JSONEq(t, tt.exp, entry.Message)
			assert.Equal(t, expFields, entry.Data)
		})
	}
}

func TestConsoleLog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in       string
		expected string
	}{
		{``, ``},
		{`""`, ``},
		{`undefined`, `undefined`},
		{`null`, `null`},

		{in: `"string"`, expected: "string"},
		{in: `"string","a","b"`, expected: "string a b"},
		{in: `"string",1,2`, expected: "string 1 2"},

		{in: `["bar", 1, 2]`, expected: `["bar",1,2]`},
		{in: `"bar", ["bar", 0x01, 2], 1, 2`, expected: `bar ["bar",1,2] 1 2`},

		{in: `{}`, expected: "{}"},
		{in: `{foo:"bar"}`, expected: `{"foo":"bar"}`},
		{in: `["test1", 2]`, expected: `["test1",2]`},

		// TODO: the ideal output for a circular object should be like `{a: [Circular]}`
		{in: `function() {var a = {foo: {}}; a.foo = a; return a}()`, expected: "[object Object]"},
	}

	for i, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			t.Parallel()

			r, err := getSimpleRunner(t, "/script.js", fmt.Sprintf(
				`exports.default = function() { console.log(%s); }`, tt.in))
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			samples := make(chan metrics.SampleContainer, 100)
			initVU, err := r.newVU(ctx, 1, 1, samples)
			require.NoError(t, err)

			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})

			logger := extractLogger(vu)

			logger.Out = io.Discard
			logger.Level = logrus.DebugLevel
			hook := logtest.NewLocal(logger)

			err = vu.RunOnce()
			require.NoError(t, err)

			entry := hook.LastEntry()

			require.NotNil(t, entry, "nothing logged")
			assert.Equal(t, tt.expected, entry.Message)
			assert.Equal(t, logrus.Fields{"source": "console"}, entry.Data)
		})
	}
}

func TestConsoleLevels(t *testing.T) {
	t.Parallel()
	levels := map[string]logrus.Level{
		"log":   logrus.InfoLevel,
		"debug": logrus.DebugLevel,
		"info":  logrus.InfoLevel,
		"warn":  logrus.WarnLevel,
		"error": logrus.ErrorLevel,
	}
	argsets := []struct {
		in  string
		exp string
	}{
		{in: `"string"`, exp: "string"},
		{in: `{}`, exp: "{}"},
		{in: `{foo:"bar"}`, exp: `{"foo":"bar"}`},
	}
	for name, level := range levels {
		name, level := name, level
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, tt := range argsets {
				args, result := tt.in, tt.exp
				t.Run(args, func(t *testing.T) {
					t.Parallel()
					r, err := getSimpleRunner(t, "/script.js", fmt.Sprintf(
						`exports.default = function() { console.%s(%s); }`,
						name, args,
					))
					require.NoError(t, err)

					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					samples := make(chan metrics.SampleContainer, 100)
					initVU, err := r.newVU(ctx, 1, 1, samples)
					require.NoError(t, err)

					vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})

					logger := extractLogger(vu)

					logger.Out = io.Discard
					logger.Level = logrus.DebugLevel
					hook := logtest.NewLocal(logger)

					err = vu.RunOnce()
					require.NoError(t, err)

					entry := hook.LastEntry()
					require.NotNil(t, entry, "nothing logged")

					assert.Equal(t, level, entry.Level)
					assert.Equal(t, result, entry.Message)
					assert.Equal(t, logrus.Fields{"source": "console"}, entry.Data)
				})
			}
		})
	}
}

func TestFileConsole(t *testing.T) {
	t.Parallel()
	var (
		levels = map[string]logrus.Level{
			"log":   logrus.InfoLevel,
			"debug": logrus.DebugLevel,
			"info":  logrus.InfoLevel,
			"warn":  logrus.WarnLevel,
			"error": logrus.ErrorLevel,
		}
		argsets = map[string]struct {
			Message string
			Data    logrus.Fields
		}{
			`"string"`:         {Message: "string", Data: logrus.Fields{}},
			`"string","a","b"`: {Message: "string a b", Data: logrus.Fields{}},
			`"string",1,2`:     {Message: "string 1 2", Data: logrus.Fields{}},
			`{}`:               {Message: "{}", Data: logrus.Fields{}},
		}
		preExisting = map[string]bool{
			"log exists":        false,
			"log doesn't exist": true,
		}
		preExistingText = "Prexisting file\n"
	)
	for name, level := range levels {
		name, level := name, level
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for args, result := range argsets {
				args, result := args, result
				t.Run(args, func(t *testing.T) {
					t.Parallel()
					// whether the file is existed before logging
					for msg, deleteFile := range preExisting {
						msg, deleteFile := msg, deleteFile
						t.Run(msg, func(t *testing.T) {
							t.Parallel()
							f, err := os.CreateTemp("", "") //nolint:forbidigo // fix with https://github.com/grafana/k6/issues/2565
							require.NoError(t, err)
							logFilename := f.Name()
							defer os.Remove(logFilename) //nolint:errcheck,forbidigo // fix with https://github.com/grafana/k6/issues/2565
							// close it as we will want to reopen it and maybe remove it
							if deleteFile {
								require.NoError(t, f.Close())
								require.NoError(t, os.Remove(logFilename)) //nolint:forbidigo // fix with https://github.com/grafana/k6/issues/2565
							} else {
								// TODO: handle case where the string was no written in full ?
								_, err = f.WriteString(preExistingText)
								assert.NoError(t, f.Close())
								require.NoError(t, err)
							}
							r, err := getSimpleRunner(t, "/script",
								fmt.Sprintf(
									`exports.default = function() { console.%s(%s); }`,
									name, args,
								))
							require.NoError(t, err)

							err = r.SetOptions(lib.Options{
								ConsoleOutput: null.StringFrom(logFilename),
							})
							require.NoError(t, err)

							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							samples := make(chan metrics.SampleContainer, 100)
							initVU, err := r.newVU(ctx, 1, 1, samples)
							require.NoError(t, err)

							vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
							logger := extractLogger(vu)

							logger.Level = logrus.DebugLevel
							hook := logtest.NewLocal(logger)

							err = vu.RunOnce()
							require.NoError(t, err)

							// Test if the file was created.
							_, err = os.Stat(logFilename) //nolint:forbidigo // fix with https://github.com/grafana/k6/issues/2565
							require.NoError(t, err)

							entry := hook.LastEntry()
							require.NotNil(t, entry, "nothing logged")
							assert.Equal(t, level, entry.Level)
							assert.Equal(t, result.Message, entry.Message)

							data := result.Data
							if data == nil {
								data = make(logrus.Fields)
							}
							require.Equal(t, data, entry.Data)

							// Test if what we logged to the hook is the same as what we logged
							// to the file.
							entryStr, err := entry.String()
							require.NoError(t, err)

							f, err = os.Open(logFilename) //nolint:forbidigo,gosec // fix with https://github.com/grafana/k6/issues/2565
							require.NoError(t, err)

							fileContent, err := io.ReadAll(f)
							require.NoError(t, err)

							expectedStr := entryStr
							if !deleteFile {
								expectedStr = preExistingText + expectedStr
							}
							require.Equal(t, expectedStr, string(fileContent))
						})
					}
				})
			}
		})
	}
}
