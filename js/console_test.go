package js

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"testing"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/metrics"
)

func TestConsoleContext(t *testing.T) {
	t.Parallel()
	rt := goja.New()
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
		fsResolvers = map[string]afero.Fs{"file": afero.NewMemMapFs(), "https": afero.NewMemMapFs()}
	)
	for _, o := range opts {
		switch opt := o.(type) {
		case afero.Fs:
			fsResolvers["file"] = opt
		case map[string]afero.Fs:
			fsResolvers = opt
		case lib.RuntimeOptions:
			rtOpts = opt
		case *logrus.Logger:
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
			LookupEnv:      func(key string) (val string, ok bool) { return "", false },
		},
		&loader.SourceData{
			URL:  &url.URL{Path: filename, Scheme: "file"},
			Data: []byte(data),
		},
		fsResolvers,
	)
}

func extractLogger(fl logrus.FieldLogger) *logrus.Logger {
	switch e := fl.(type) {
	case *logrus.Entry:
		return e.Logger
	case *logrus.Logger:
		return e
	default:
		panic(fmt.Sprintf("unknown logrus.FieldLogger option %q", fl))
	}
}

func TestConsoleLogWithGojaNativeObject(t *testing.T) {
	t.Parallel()

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	obj := rt.NewObject()
	err := obj.Set("text", "nativeObject")
	require.NoError(t, err)

	logger := testutils.NewLogger(t)
	hook := logtest.NewLocal(logger)

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

			rt := goja.New()
			rt.SetFieldNameMapper(common.FieldNameMapper{})
			obj := rt.ToValue(tt.in)

			logger := testutils.NewLogger(t)
			hook := logtest.NewLocal(logger)

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

			logger := extractLogger(vu.(*ActiveVU).Console.logger)

			logger.Out = ioutil.Discard
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

					logger := extractLogger(vu.(*ActiveVU).Console.logger)

					logger.Out = ioutil.Discard
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
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for args, result := range argsets {
				t.Run(args, func(t *testing.T) {
					t.Parallel()
					// whether the file is existed before logging
					for msg, deleteFile := range preExisting {
						t.Run(msg, func(t *testing.T) {
							t.Parallel()
							f, err := ioutil.TempFile("", "")
							if err != nil {
								t.Fatalf("Couldn't create temporary file for testing: %s", err)
							}
							logFilename := f.Name()
							defer os.Remove(logFilename)
							// close it as we will want to reopen it and maybe remove it
							if deleteFile {
								f.Close()
								if err := os.Remove(logFilename); err != nil {
									t.Fatalf("Couldn't remove tempfile: %s", err)
								}
							} else {
								// TODO: handle case where the string was no written in full ?
								_, err = f.WriteString(preExistingText)
								_ = f.Close()
								if err != nil {
									t.Fatalf("Error while writing text to preexisting logfile: %s", err)
								}

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
							logger := extractLogger(vu.(*ActiveVU).Console.logger)

							logger.Level = logrus.DebugLevel
							hook := logtest.NewLocal(logger)

							err = vu.RunOnce()
							require.NoError(t, err)

							// Test if the file was created.
							_, err = os.Stat(logFilename)
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

							f, err = os.Open(logFilename) //nolint:gosec
							require.NoError(t, err)

							fileContent, err := ioutil.ReadAll(f)
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
