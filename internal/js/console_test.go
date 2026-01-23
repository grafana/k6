package js

import (
	"context"
	"errors"
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
	filenameURL := &url.URL{Path: filename, Scheme: "file"}

	preInitState := &lib.TestPreInitState{
		Logger:         logger,
		RuntimeOptions: rtOpts,
		BuiltinMetrics: builtinMetrics,
		Registry:       registry,
		LookupEnv:      func(_ string) (val string, ok bool) { return "", false },
		Usage:          usage.New(),
	}
	moduleResolver := NewModuleResolver(loader.Dir(filenameURL), preInitState, fsResolvers)
	return New(
		preInitState,
		&loader.SourceData{
			URL:  filenameURL,
			Data: []byte(data),
		},
		fsResolvers,
		moduleResolver,
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
	preInitState := &lib.TestPreInitState{
		Logger:         logger,
		RuntimeOptions: rtOpts,
		BuiltinMetrics: builtinMetrics,
		Registry:       registry,
		Usage:          usage.New(),
	}
	moduleResolver := NewModuleResolver(arc.PwdURL, preInitState, arc.Filesystems)
	return NewFromArchive(preInitState, arc, moduleResolver)
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
	require.Equal(t, `{ text: "nativeObject" }`, entry.Message)
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
			exp: `{ text: "test1" }`,
		},
		{
			name: "StructPointer",
			in: &value{
				Text: "test2",
			},
			exp: `{ text: "test2" }`,
		},
		{
			name: "Map",
			in: map[string]interface{}{
				"text": "test3",
			},
			exp: `{ text: "test3" }`,
		},
	}

	expFields := logrus.Fields{"source": "console"}
	for _, tt := range tests {
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
			assert.Equal(t, tt.exp, entry.Message)
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

		{in: `true`, expected: "true"},

		{in: `Infinity`, expected: "Infinity"},
		{in: `1e5`, expected: "100000"},
		{in: `1.23e-4`, expected: "0.000123"},

		{in: `function() {}`, expected: "[object Function]"},
		{in: `() => {}`, expected: "[object Function]"},

		{in: `new Date(0)`, expected: `"1970-01-01T00:00:00.000Z"`},
		{in: `new Error("test error")`, expected: "Error: test error"},

		{in: `["bar", 1, 2]`, expected: `[ "bar", 1, 2 ]`},
		{in: `"bar", ["bar", 0x01, 2], 1, 2`, expected: `bar [ "bar", 1, 2 ] 1 2`},

		{in: `{}`, expected: "{}"},
		{in: `{foo:"bar"}`, expected: `{ foo: "bar" }`},
		{in: `["test1", 2]`, expected: `[ "test1", 2 ]`},

		{in: `{fn: function(){}}`, expected: `{ fn: [object Function] }`},
		{in: `{fn: function(){}, dt: new Date(0)}`, expected: `{ fn: [object Function], dt: "1970-01-01T00:00:00.000Z" }`},
		{in: `{fn: () => {}}`, expected: `{ fn: [object Function] }`},
		{in: `{a: 1, fn: function(){}, b: "two"}`, expected: `{ a: 1, fn: [object Function], b: "two" }`},
		{in: `{nested: {fn: function(){}}}`, expected: `{ nested: { fn: [object Function] } }`},
		{in: `[function(){}, 1, "two"]`, expected: `[ [object Function], 1, "two" ]`},
		{
			in: `{
				arr: [1, 2],
				obj: {
					'a': 'foo', 'b': {
						'c': { 'd': 123 }
					}
				},
				str: 'hi'
			}`,
			expected: `{ arr: [ 1, 2 ], obj: { a: "foo", b: { c: { d: 123 } } }, str: "hi" }`,
		},

		{in: `[null, undefined, 1]`, expected: `[ null, null, 1 ]`},
		{in: `[1, , 3]`, expected: `[ 1, null, 3 ]`}, // sparse arrays (holes in arrays)
		{in: `[[function(){}], [1, 2]]`, expected: `[ [ [object Function] ], [ 1, 2 ] ]`},
		{in: `new RegExp("test")`, expected: `{}`}, // JSON.stringify of RegExp is {}
		{in: `{[Symbol("test")]: "value", a: 1}`, expected: `{ a: 1 }`},
		{in: `Object.defineProperty({}, 'x', {get: function() { throw new Error(); }})`, expected: `{}`},
		{in: `Object.create({inherited: 1}, {own: {value: 2, enumerable: true}})`, expected: `{ own: 2 }`},

		// circular reference AND function (both code paths)
		{
			in:       `function() {var a = {fn: function(){}, foo: {}}; a.foo = a; return a}()`,
			expected: `{ fn: [object Function], foo: [Circular] }`,
		},

		// TypedArray and ArrayBuffer formatting
		{in: `new Int8Array([1, -2, 127, -128])`, expected: `Int8Array(4) [ 1, -2, 127, -128 ]`},
		{in: `new Uint8Array([0, 128, 255])`, expected: `Uint8Array(3) [ 0, 128, 255 ]`},
		{in: `new Uint8ClampedArray([0, 128, 255])`, expected: `Uint8Array(3) [ 0, 128, 255 ]`}, // shows as Uint8Array
		{in: `new Int16Array([100, -100, 32767])`, expected: `Int16Array(3) [ 100, -100, 32767 ]`},
		{in: `new Uint16Array([0, 32768, 65535])`, expected: `Uint16Array(3) [ 0, 32768, 65535 ]`},
		{in: `new Int32Array([4, 2, -2147483648])`, expected: `Int32Array(3) [ 4, 2, -2147483648 ]`},
		{in: `new Uint32Array([0, 2147483648, 4294967295])`, expected: `Uint32Array(3) [ 0, 2147483648, 4294967295 ]`},
		{in: `new Float32Array([1.5, -2.5])`, expected: `Float32Array(2) [ 1.5, -2.5 ]`},
		{in: `new Float64Array([3.141592653589793, 2.718281828459045])`, expected: `Float64Array(2) [ 3.141592653589793, 2.718281828459045 ]`},
		{in: `new BigInt64Array([BigInt(1), BigInt(-1)])`, expected: `BigInt64Array(2) [ 1, -1 ]`},
		{in: `new BigUint64Array([BigInt(0), BigInt(1)])`, expected: `BigUint64Array(2) [ 0, 1 ]`},

		{in: `new Int8Array(0)`, expected: `Int8Array(0) []`},
		{in: `new Uint8Array(0)`, expected: `Uint8Array(0) []`},
		{in: `new Float64Array([])`, expected: `Float64Array(0) []`},
		{in: `new Int32Array([0])`, expected: `Int32Array(1) [ 0 ]`},

		{in: `new ArrayBuffer(0)`, expected: `ArrayBuffer { [Uint8Contents]: <>, byteLength: 0 }`},
		{in: `new ArrayBuffer(4)`, expected: `ArrayBuffer { [Uint8Contents]: <00 00 00 00>, byteLength: 4 }`},
		{in: `new ArrayBuffer(8)`, expected: `ArrayBuffer { [Uint8Contents]: <00 00 00 00 00 00 00 00>, byteLength: 8 }`},
		{
			in: `function() {
				var buf = new ArrayBuffer(8);
				var view = new Int32Array(buf);
				view[0] = 4;
				view[1] = 2;
				return buf;
			}()`,
			expected: `ArrayBuffer { [Uint8Contents]: <04 00 00 00 02 00 00 00>, byteLength: 8 }`,
		},

		{in: `{v: new Int32Array([4, 2])}`, expected: `{ v: Int32Array(2) [ 4, 2 ] }`},
		{in: `{b: new ArrayBuffer(4)}`, expected: `{ b: ArrayBuffer { [Uint8Contents]: <00 00 00 00>, byteLength: 4 } }`},
		{in: `{arr: new Int8Array([1, 2]), buf: new ArrayBuffer(2)}`, expected: `{ arr: Int8Array(2) [ 1, 2 ], buf: ArrayBuffer { [Uint8Contents]: <00 00>, byteLength: 2 } }`},

		{in: `[new Int8Array([1, 2])]`, expected: `[ Int8Array(2) [ 1, 2 ] ]`},
		{in: `[new ArrayBuffer(2), new ArrayBuffer(4)]`, expected: `[ ArrayBuffer { [Uint8Contents]: <00 00>, byteLength: 2 }, ArrayBuffer { [Uint8Contents]: <00 00 00 00>, byteLength: 4 } ]`},

		{in: `{count: 2, name: "test", data: new Int8Array([1, 2])}`, expected: `{ count: 2, name: "test", data: Int8Array(2) [ 1, 2 ] }`},
		{in: `{fn: function(){}, arr: new Uint8Array([255])}`, expected: `{ fn: [object Function], arr: Uint8Array(1) [ 255 ] }`},
		{in: `{buf: new ArrayBuffer(4), items: [1, 2, 3]}`, expected: `{ buf: ArrayBuffer { [Uint8Contents]: <00 00 00 00>, byteLength: 4 }, items: [ 1, 2, 3 ] }`},
		{in: `{err: new Error("test"), data: new Int8Array([1])}`, expected: `{ err: Error: test, data: Int8Array(1) [ 1 ] }`},

		{in: `{outer: {inner: new Int32Array([100])}}`, expected: `{ outer: { inner: Int32Array(1) [ 100 ] } }`},
		{in: `{a: {b: {c: new ArrayBuffer(2)}}}`, expected: `{ a: { b: { c: ArrayBuffer { [Uint8Contents]: <00 00>, byteLength: 2 } } } }`},
		{in: `{level1: {level2: {level3: {data: new Float32Array([1.5])}}}}`, expected: `{ level1: { level2: { level3: { data: Float32Array(1) [ 1.5 ] } } } }`},

		{in: `[1, new Int8Array([2]), "three"]`, expected: `[ 1, Int8Array(1) [ 2 ], "three" ]`},
		{in: `[new ArrayBuffer(2), null, new Int16Array([100])]`, expected: `[ ArrayBuffer { [Uint8Contents]: <00 00>, byteLength: 2 }, null, Int16Array(1) [ 100 ] ]`},

		{in: `{users: [{name: "a", scores: new Int32Array([10, 20])}]}`, expected: `{ users: [ { name: "a", scores: Int32Array(2) [ 10, 20 ] } ] }`},
		{in: `[[new Int8Array([1])], [new Int8Array([2])]]`, expected: `[ [ Int8Array(1) [ 1 ] ], [ Int8Array(1) [ 2 ] ] ]`},
		{in: `{matrix: [[new Int8Array([1, 2])], [new Int8Array([3, 4])]]}`, expected: `{ matrix: [ [ Int8Array(2) [ 1, 2 ] ], [ Int8Array(2) [ 3, 4 ] ] ] }`},
		{in: `{buffers: {a: new ArrayBuffer(2), b: new ArrayBuffer(4)}}`, expected: `{ buffers: { a: ArrayBuffer { [Uint8Contents]: <00 00>, byteLength: 2 }, b: ArrayBuffer { [Uint8Contents]: <00 00 00 00>, byteLength: 4 } } }`},
		{in: `[{arr: new Int8Array([1])}, {arr: new Int8Array([2])}]`, expected: `[ { arr: Int8Array(1) [ 1 ] }, { arr: Int8Array(1) [ 2 ] } ]`},
		{in: `{data: {nums: [1, 2], typed: new Float64Array([3.14])}}`, expected: `{ data: { nums: [ 1, 2 ], typed: Float64Array(1) [ 3.14 ] } }`},
		{in: `{config: {enabled: true, buffer: new ArrayBuffer(4), name: "test"}}`, expected: `{ config: { enabled: true, buffer: ArrayBuffer { [Uint8Contents]: <00 00 00 00>, byteLength: 4 }, name: "test" } }`},
		{in: `{items: [{id: 1, data: new Uint8Array([255])}, {id: 2, data: new Uint8Array([128])}]}`, expected: `{ items: [ { id: 1, data: Uint8Array(1) [ 255 ] }, { id: 2, data: Uint8Array(1) [ 128 ] } ] }`},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
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

func TestConsoleLogWithGoValues(t *testing.T) { //nolint:tparallel // actually faster with parallel and also we need the rt to create some of the testdata
	t.Parallel()

	rt := sobek.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	tests := []struct {
		in       any
		expected string
	}{
		{in: "string", expected: "string"},

		{in: []any{}, expected: `[]`},
		{in: []string{"a", "b", "c"}, expected: `[ "a", "b", "c" ]`},
		{in: []int{1, 2, 3}, expected: `[ 1, 2, 3 ]`},
		{in: []any{"hello", 42, true, nil}, expected: `[ "hello", 42, true, null ]`},
		{in: []any{[]int{1, 2}, []string{"a", "b"}}, expected: `[ [ 1, 2 ], [ "a", "b" ] ]`},

		{in: map[string]any{}, expected: "{}"},
		{in: map[string]any{"outer": map[string]any{"inner": "value"}}, expected: `{ outer: { inner: "value" } }`},

		{in: struct {
			Name string
			Age  int
		}{"John", 30}, expected: `{ name: "John", age: 30 }`},

		{in: errors.New("this is an error"), expected: `this is an error`},
		{in: fmt.Errorf("this is a wrap of: %w", errors.New("error")), expected: `this is a wrap of: error`},

		{in: rt.NewGoError(errors.New("this is a go error")), expected: `GoError: this is a go error`},
		{in: rt.NewTypeError("type error"), expected: `TypeError: type error`},
	}

	for _, tt := range tests { //nolint:paralleltest
		t.Run(fmt.Sprintf("%v", tt.in), func(t *testing.T) {
			logger, hook := testutils.NewLoggerWithHook(t)
			c := newConsole(logger)

			// Convert Go in to JavaScript in
			jsValue := rt.ToValue(tt.in)

			// Call console.log with the converted in
			c.Log(jsValue)

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
		{in: `{foo:"bar"}`, exp: `{ foo: "bar" }`},
	}
	for name, level := range levels {
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
			`"string"`:           {Message: "string", Data: logrus.Fields{}},
			`"string", "a", "b"`: {Message: "string a b", Data: logrus.Fields{}},
			`"string", 1, 2`:     {Message: "string 1 2", Data: logrus.Fields{}},
			`{}`:                 {Message: "{}", Data: logrus.Fields{}},
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
							f, err := os.CreateTemp(t.TempDir(), "") //nolint:forbidigo // fix with https://github.com/grafana/k6/issues/2565
							require.NoError(t, err)
							logFilename := f.Name()
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
							t.Cleanup(func() {
								if loggerOut, canBeClosed := logger.Out.(io.Closer); canBeClosed {
									require.NoError(t, loggerOut.Close())
								}
							})

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
							require.NoError(t, f.Close())

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
