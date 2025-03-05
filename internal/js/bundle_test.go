package js

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

const isWindows = runtime.GOOS == "windows"

func getTestPreInitState(tb testing.TB, logger logrus.FieldLogger, rtOpts *lib.RuntimeOptions) *lib.TestPreInitState {
	if logger == nil {
		logger = testutils.NewLogger(tb)
	}
	if rtOpts == nil {
		rtOpts = &lib.RuntimeOptions{}
	}
	reg := metrics.NewRegistry()
	return &lib.TestPreInitState{
		Logger:         logger,
		RuntimeOptions: *rtOpts,
		Registry:       reg,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(reg),
		Usage:          usage.New(),
	}
}

func getSimpleBundle(tb testing.TB, filename, data string, opts ...interface{}) (*Bundle, error) {
	fs := fsext.NewMemMapFs()
	var rtOpts *lib.RuntimeOptions
	var logger logrus.FieldLogger
	for _, o := range opts {
		switch opt := o.(type) {
		case fsext.Fs:
			fs = opt
		case lib.RuntimeOptions:
			rtOpts = &opt
		case logrus.FieldLogger:
			logger = opt
		default:
			tb.Fatalf("unknown test option %q", opt)
		}
	}

	return NewBundle(
		getTestPreInitState(tb, logger, rtOpts),
		&loader.SourceData{
			URL:  &url.URL{Path: filename, Scheme: "file"},
			Data: []byte(data),
		},
		map[string]fsext.Fs{"file": fs, "https": fsext.NewMemMapFs()},
	)
}

func getSimpleBundleStdin(tb testing.TB, pwd *url.URL, data string, opts ...interface{}) (*Bundle, error) {
	fs := fsext.NewMemMapFs()
	var rtOpts *lib.RuntimeOptions
	var logger logrus.FieldLogger
	for _, o := range opts {
		switch opt := o.(type) {
		case fsext.Fs:
			fs = opt
		case lib.RuntimeOptions:
			rtOpts = &opt
		case logrus.FieldLogger:
			logger = opt
		default:
			tb.Fatalf("unknown test option %q", opt)
		}
	}

	return NewBundle(
		getTestPreInitState(tb, logger, rtOpts),
		&loader.SourceData{
			URL:  &url.URL{Path: "/-", Scheme: "file"},
			Data: []byte(data),
			PWD:  pwd,
		},
		map[string]fsext.Fs{"file": fs, "https": fsext.NewMemMapFs()},
	)
}

func TestNewBundle(t *testing.T) {
	t.Parallel()
	t.Run("Blank", func(t *testing.T) {
		t.Parallel()
		_, err := getSimpleBundle(t, "/script.js", "")
		require.EqualError(t, err, "no exported functions in script")
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		_, err := getSimpleBundle(t, "/script.js", "\x00")
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "file:///script.js: Line 1:1 Unexpected token ILLEGAL (and 1 more errors)")
	})
	t.Run("Error", func(t *testing.T) {
		t.Parallel()
		_, err := getSimpleBundle(t, "/script.js", `throw new Error("aaaa");`)
		exception := new(scriptExceptionError)
		require.ErrorAs(t, err, &exception)
		require.EqualError(t, err, "Error: aaaa\n\tat file:///script.js:1:34(3)\n")
	})
	t.Run("InvalidExports", func(t *testing.T) {
		t.Parallel()
		_, err := getSimpleBundle(t, "/script.js", `module.exports = null`)
		require.EqualError(t, err, "CommonJS's exports must not be null")
	})
	t.Run("DefaultUndefined", func(t *testing.T) {
		t.Parallel()
		_, err := getSimpleBundle(t, "/script.js", `export default undefined;`)
		require.EqualError(t, err, "no exported functions in script")
	})
	t.Run("DefaultNull", func(t *testing.T) {
		t.Parallel()
		_, err := getSimpleBundle(t, "/script.js", `export default null;`)
		require.EqualError(t, err, "no exported functions in script")
	})
	t.Run("DefaultWrongType", func(t *testing.T) {
		t.Parallel()
		_, err := getSimpleBundle(t, "/script.js", `export default 12345;`)
		require.EqualError(t, err, "no exported functions in script")
	})
	t.Run("Minimal", func(t *testing.T) {
		t.Parallel()
		_, err := getSimpleBundle(t, "/script.js", `export default function() {};`)
		require.NoError(t, err)
	})
	t.Run("stdin", func(t *testing.T) {
		t.Parallel()
		b, err := getSimpleBundle(t, "-", `export default function() {};`)
		require.NoError(t, err)
		assert.Equal(t, "file://-", b.sourceData.URL.String())
		assert.Equal(t, "file:///", b.pwd.String())
	})
	t.Run("CompatibilityMode", func(t *testing.T) {
		t.Parallel()
		t.Run("Extended/ok/global", func(t *testing.T) {
			t.Parallel()
			rtOpts := lib.RuntimeOptions{
				CompatibilityMode: null.StringFrom(lib.CompatibilityModeExtended.String()),
			}
			_, err := getSimpleBundle(t, "/script.js",
				`module.exports.default = function() {}
				if (global.Math != Math) {
					throw new Error("global is not defined");
				}`, rtOpts)

			require.NoError(t, err)
		})
		t.Run("Base/ok/Minimal", func(t *testing.T) {
			t.Parallel()
			rtOpts := lib.RuntimeOptions{
				CompatibilityMode: null.StringFrom(lib.CompatibilityModeBase.String()),
			}
			_, err := getSimpleBundle(t, "/script.js",
				`module.exports.default = function() {};`, rtOpts)
			require.NoError(t, err)
		})
		t.Run("Base/err", func(t *testing.T) {
			t.Parallel()
			testCases := []struct {
				name       string
				compatMode string
				code       string
				expErr     string
			}{
				{
					"InvalidCompat", "es1", `export default function() {};`,
					`invalid compatibility mode "es1". Use: "extended", "base", "experimental_enhanced"`,
				},
			}

			for _, tc := range testCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					rtOpts := lib.RuntimeOptions{CompatibilityMode: null.StringFrom(tc.compatMode)}
					_, err := getSimpleBundle(t, "/script.js", tc.code, rtOpts)
					require.EqualError(t, err, tc.expErr)
				})
			}
		})
	})
	t.Run("Options", func(t *testing.T) {
		t.Parallel()
		t.Run("Empty", func(t *testing.T) {
			t.Parallel()
			_, err := getSimpleBundle(t, "/script.js", `
				export let options = {};
				export default function() {};
			`)
			require.NoError(t, err)
		})
		t.Run("Null", func(t *testing.T) {
			t.Parallel()
			fs := fsext.NewMemMapFs()
			require.NoError(t, fsext.WriteFile(fs, "/options.js", []byte("module.exports={}"), 0o644))
			_, err := getSimpleBundle(t, "/script.js", `
				export {options} from "./options.js";
				export default function() {};
			`, fs)
			require.NoError(t, err)
		})
		t.Run("Invalid", func(t *testing.T) {
			t.Parallel()
			invalidOptions := map[string]struct {
				Expr, Error string
			}{
				"Array":    {`[]`, "json: cannot unmarshal array into Go value of type lib.Options"},
				"Function": {`function(){}`, "error parsing script options: json: unsupported type: func(sobek.FunctionCall) sobek.Value"},
			}
			for name, data := range invalidOptions {
				t.Run(name, func(t *testing.T) {
					_, err := getSimpleBundle(t, "/script.js", fmt.Sprintf(`
						export let options = %s;
						export default function() {};
					`, data.Expr))
					require.EqualError(t, err, data.Error)
				})
			}
		})

		t.Run("Paused", func(t *testing.T) {
			t.Parallel()
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					paused: true,
				};
				export default function() {};
			`)
			require.NoError(t, err)
			require.Equal(t, null.BoolFrom(true), b.Options.Paused)
		})
		t.Run("VUs", func(t *testing.T) {
			t.Parallel()
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					vus: 100,
				};
				export default function() {};
			`)
			require.NoError(t, err)
			require.Equal(t, null.IntFrom(100), b.Options.VUs)
		})
		t.Run("Duration", func(t *testing.T) {
			t.Parallel()
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					duration: "10s",
				};
				export default function() {};
			`)
			require.NoError(t, err)
			require.Equal(t, types.NullDurationFrom(10*time.Second), b.Options.Duration)
		})
		t.Run("Iterations", func(t *testing.T) {
			t.Parallel()
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					iterations: 100,
				};
				export default function() {};
			`)
			require.NoError(t, err)
			require.Equal(t, null.IntFrom(100), b.Options.Iterations)
		})
		t.Run("Stages", func(t *testing.T) {
			t.Parallel()
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					stages: [],
				};
				export default function() {};
			`)
			require.NoError(t, err)
			require.Len(t, b.Options.Stages, 0)

			t.Run("Empty", func(t *testing.T) {
				t.Parallel()
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						stages: [
							{},
						],
					};
					export default function() {};
				`)
				require.NoError(t, err)
				require.Len(t, b.Options.Stages, 1)
				require.Equal(t, lib.Stage{}, b.Options.Stages[0])
			})
			t.Run("Target", func(t *testing.T) {
				t.Parallel()
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						stages: [
							{target: 10},
						],
					};
					export default function() {};
				`)
				require.NoError(t, err)
				require.Len(t, b.Options.Stages, 1)
				require.Equal(t, lib.Stage{Target: null.IntFrom(10)}, b.Options.Stages[0])
			})
			t.Run("Duration", func(t *testing.T) {
				t.Parallel()
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						stages: [
							{duration: "10s"},
						],
					};
					export default function() {};
				`)
				require.NoError(t, err)
				require.Len(t, b.Options.Stages, 1)
				require.Equal(t, lib.Stage{Duration: types.NullDurationFrom(10 * time.Second)}, b.Options.Stages[0])
			})
			t.Run("DurationAndTarget", func(t *testing.T) {
				t.Parallel()
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						stages: [
							{duration: "10s", target: 10},
						],
					};
					export default function() {};
				`)
				require.NoError(t, err)
				require.Len(t, b.Options.Stages, 1)
				require.Equal(t, lib.Stage{Duration: types.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)}, b.Options.Stages[0])
			})
			t.Run("RampUpAndPlateau", func(t *testing.T) {
				t.Parallel()
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						stages: [
							{duration: "10s", target: 10},
							{duration: "5s"},
						],
					};
					export default function() {};
				`)
				require.NoError(t, err)
				require.Len(t, b.Options.Stages, 2)
				assert.Equal(t, lib.Stage{Duration: types.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)}, b.Options.Stages[0])
				assert.Equal(t, lib.Stage{Duration: types.NullDurationFrom(5 * time.Second)}, b.Options.Stages[1])
			})
		})
		t.Run("MaxRedirects", func(t *testing.T) {
			t.Parallel()
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					maxRedirects: 10,
				};
				export default function() {};
			`)
			require.NoError(t, err)
			require.Equal(t, null.IntFrom(10), b.Options.MaxRedirects)
		})
		t.Run("InsecureSkipTLSVerify", func(t *testing.T) {
			t.Parallel()
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					insecureSkipTLSVerify: true,
				};
				export default function() {};
			`)
			require.NoError(t, err)
			require.Equal(t, null.BoolFrom(true), b.Options.InsecureSkipTLSVerify)
		})
		t.Run("TLSCipherSuites", func(t *testing.T) {
			t.Parallel()
			for suiteName, suiteID := range lib.SupportedTLSCipherSuites {
				suiteName, suiteID := suiteName, suiteID
				t.Run(suiteName, func(t *testing.T) {
					t.Parallel()
					script := `
					export let options = {
						tlsCipherSuites: ["%s"]
					};
					export default function() {};
					`
					script = fmt.Sprintf(script, suiteName)

					b, err := getSimpleBundle(t, "/script.js", script)
					require.NoError(t, err)
					require.Len(t, *b.Options.TLSCipherSuites, 1)
					require.Equal(t, (*b.Options.TLSCipherSuites)[0], suiteID)
				})
			}
		})
		t.Run("TLSVersion", func(t *testing.T) {
			t.Parallel()
			t.Run("Object", func(t *testing.T) {
				t.Parallel()
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						tlsVersion: {
							min: "tls1.0",
							max: "tls1.2"
						}
					};
					export default function() {};
				`)
				require.NoError(t, err)
				assert.Equal(t, b.Options.TLSVersion.Min, lib.TLSVersion(tls.VersionTLS10))
				assert.Equal(t, b.Options.TLSVersion.Max, lib.TLSVersion(tls.VersionTLS12))
			})
			t.Run("String", func(t *testing.T) {
				t.Parallel()
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						tlsVersion: "tls1.0"
					};
					export default function() {};
				`)
				require.NoError(t, err)
				assert.Equal(t, b.Options.TLSVersion.Min, lib.TLSVersion(tls.VersionTLS10))
				assert.Equal(t, b.Options.TLSVersion.Max, lib.TLSVersion(tls.VersionTLS10))
			})
		})
		t.Run("Thresholds", func(t *testing.T) {
			t.Parallel()
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					thresholds: {
						http_req_duration: ["avg<100"],
					},
				};
				export default function() {};
			`)
			require.NoError(t, err)
			require.Len(t, b.Options.Thresholds["http_req_duration"].Thresholds, 1)
			require.Equal(t, "avg<100", b.Options.Thresholds["http_req_duration"].Thresholds[0].Source)
		})

		t.Run("Unknown field", func(t *testing.T) {
			t.Parallel()
			logger := logrus.New()
			logger.SetLevel(logrus.InfoLevel)
			logger.Out = io.Discard
			hook := testutils.NewLogHook(
				logrus.WarnLevel, logrus.InfoLevel, logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel,
			)
			logger.AddHook(hook)

			_, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					something: {
						http_req_duration: ["avg<100"],
					},
				};
				export default function() {};
			`, logger)
			require.NoError(t, err)
			entries := hook.Drain()
			require.Len(t, entries, 1)
			assert.Equal(t, logrus.WarnLevel, entries[0].Level)
			assert.Contains(t, entries[0].Message, "There were unknown fields")
			assert.Contains(t, entries[0].Data["error"].(error).Error(), "unknown field \"something\"")
		})
	})
}

func getArchive(tb testing.TB, data string, rtOpts lib.RuntimeOptions) (*lib.Archive, error) {
	b, err := getSimpleBundle(tb, "script.js", data, rtOpts)
	if err != nil {
		return nil, err
	}
	return b.makeArchive(), nil
}

func TestNewBundleFromArchive(t *testing.T) {
	t.Parallel()

	es5Code := `module.exports.options = { vus: 12345 }; module.exports.default = function() { return "hi!" };`
	es6Code := `export let options = { vus: 12345 }; export default function() { return "hi!"; };`
	baseCompatModeRtOpts := lib.RuntimeOptions{CompatibilityMode: null.StringFrom(lib.CompatibilityModeBase.String())}
	extCompatModeRtOpts := lib.RuntimeOptions{CompatibilityMode: null.StringFrom(lib.CompatibilityModeExtended.String())}

	logger := testutils.NewLogger(t)
	checkBundle := func(t *testing.T, b *Bundle) {
		require.Equal(t, lib.Options{VUs: null.IntFrom(12345)}, b.Options)
		bi, err := b.Instantiate(context.Background(), 0)
		require.NoError(t, err)
		val, err := bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
		require.NoError(t, err)
		require.Equal(t, "hi!", val.Export())
	}

	checkArchive := func(t *testing.T, arc *lib.Archive, rtOpts lib.RuntimeOptions, expError string) {
		b, err := NewBundleFromArchive(getTestPreInitState(t, logger, &rtOpts), arc)
		if expError != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), expError)
		} else {
			require.NoError(t, err)
			checkBundle(t, b)
		}
	}

	t.Run("es6_script_default", func(t *testing.T) {
		t.Parallel()
		arc, err := getArchive(t, es6Code, lib.RuntimeOptions{}) // default options
		require.NoError(t, err)
		require.Equal(t, lib.CompatibilityModeExtended.String(), arc.CompatibilityMode)

		checkArchive(t, arc, lib.RuntimeOptions{}, "") // default options
		checkArchive(t, arc, extCompatModeRtOpts, "")
		checkArchive(t, arc, baseCompatModeRtOpts, "")
	})

	t.Run("es6_script_explicit", func(t *testing.T) {
		t.Parallel()
		arc, err := getArchive(t, es6Code, extCompatModeRtOpts)
		require.NoError(t, err)
		require.Equal(t, lib.CompatibilityModeExtended.String(), arc.CompatibilityMode)

		checkArchive(t, arc, lib.RuntimeOptions{}, "")
		checkArchive(t, arc, extCompatModeRtOpts, "")
		checkArchive(t, arc, baseCompatModeRtOpts, "")
	})

	t.Run("es5_script_with_extended", func(t *testing.T) {
		t.Parallel()
		arc, err := getArchive(t, es5Code, lib.RuntimeOptions{})
		require.NoError(t, err)
		require.Equal(t, lib.CompatibilityModeExtended.String(), arc.CompatibilityMode)

		checkArchive(t, arc, lib.RuntimeOptions{}, "")
		checkArchive(t, arc, extCompatModeRtOpts, "")
		checkArchive(t, arc, baseCompatModeRtOpts, "")
	})

	t.Run("es5_script", func(t *testing.T) {
		t.Parallel()
		arc, err := getArchive(t, es5Code, baseCompatModeRtOpts)
		require.NoError(t, err)
		require.Equal(t, lib.CompatibilityModeBase.String(), arc.CompatibilityMode)

		checkArchive(t, arc, lib.RuntimeOptions{}, "")
		checkArchive(t, arc, extCompatModeRtOpts, "")
		checkArchive(t, arc, baseCompatModeRtOpts, "")
	})

	t.Run("messed_up_archive", func(t *testing.T) {
		t.Parallel()
		arc, err := getArchive(t, es6Code, extCompatModeRtOpts)
		require.NoError(t, err)
		arc.CompatibilityMode = "blah"                                           // intentionally break the archive
		checkArchive(t, arc, lib.RuntimeOptions{}, "invalid compatibility mode") // fails when it uses the archive one
		checkArchive(t, arc, extCompatModeRtOpts, "")                            // works when I force the compat mode
		checkArchive(t, arc, baseCompatModeRtOpts, "")                           // still works as even base compatibility supports ESM
	})

	t.Run("script_options_dont_overwrite_metadata", func(t *testing.T) {
		t.Parallel()
		code := `export let options = { vus: 12345 }; export default function() { return options.vus; };`
		arc := &lib.Archive{
			Type:        "js",
			FilenameURL: &url.URL{Scheme: "file", Path: "/script"},
			K6Version:   build.Version,
			Data:        []byte(code),
			Options:     lib.Options{VUs: null.IntFrom(999)},
			PwdURL:      &url.URL{Scheme: "file", Path: "/"},
			Filesystems: nil,
		}
		b, err := NewBundleFromArchive(getTestPreInitState(t, logger, nil), arc)
		require.NoError(t, err)
		bi, err := b.Instantiate(context.Background(), 0)
		require.NoError(t, err)
		val, err := bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
		require.NoError(t, err)
		require.Equal(t, int64(999), val.Export())
	})
}

func TestOpen(t *testing.T) {
	t.Parallel()
	testCases := [...]struct {
		name           string
		openPath       string
		pwd            string
		isError        bool
		isArchiveError bool
	}{
		{
			name:     "notOpeningUrls",
			openPath: "github.com",
			isError:  true,
			pwd:      "/path/to",
		},
		{
			name:     "simple",
			openPath: "file.txt",
			pwd:      "/path/to",
		},
		{
			name:     "simple with dot",
			openPath: "./file.txt",
			pwd:      "/path/to",
		},
		{
			name:     "simple with two dots",
			openPath: "../to/file.txt",
			pwd:      "/path/not",
		},
		{
			name:     "fullpath",
			openPath: "/path/to/file.txt",
			pwd:      "/path/to",
		},
		{
			name:     "fullpath2",
			openPath: "/path/to/file.txt",
			pwd:      "/path",
		},
		{
			name:     "file scheme",
			openPath: "file:///path/to/file.txt",
			pwd:      "/path",
		},
		{
			name:     "file is dir",
			openPath: "/path/to/",
			pwd:      "/path/to",
			isError:  true,
		},
		{
			name:     "file is missing",
			openPath: "/path/to/missing.txt",
			isError:  true,
		},
		{
			name:     "relative1",
			openPath: "to/file.txt",
			pwd:      "/path",
		},
		{
			name:     "relative2",
			openPath: "./path/to/file.txt",
			pwd:      "/",
		},
		{
			name:     "relative wonky",
			openPath: "../path/to/file.txt",
			pwd:      "/path",
		},
		{
			name:     "empty open doesn't panic",
			openPath: "",
			pwd:      "/path",
			isError:  true,
		},
	}
	fss := map[string]func() (fsext.Fs, string, func()){
		"MemMapFS": func() (fsext.Fs, string, func()) {
			fs := fsext.NewMemMapFs()
			require.NoError(t, fs.MkdirAll("/path/to", 0o755))
			require.NoError(t, fsext.WriteFile(fs, "/path/to/file.txt", []byte(`hi`), 0o644))
			return fs, "", func() {}
		},
		"OsFS": func() (fsext.Fs, string, func()) {
			prefix, err := os.MkdirTemp("", "k6_open_test") //nolint:forbidigo
			require.NoError(t, err)
			fs := fsext.NewOsFs()
			filePath := filepath.Join(prefix, "/path/to/file.txt")
			require.NoError(t, fs.MkdirAll(filepath.Join(prefix, "/path/to"), 0o755))
			require.NoError(t, fsext.WriteFile(fs, filePath, []byte(`hi`), 0o644))
			fs = fsext.NewChangePathFs(fs, func(name string) (string, error) {
				// Drop the prefix effectively building something like https://pkg.go.dev/os#DirFS
				return filepath.Join(prefix, name), nil
			})
			if isWindows {
				fs = fsext.NewTrimFilePathSeparatorFs(fs)
			}
			return fs, prefix, func() { require.NoError(t, os.RemoveAll(prefix)) } //nolint:forbidigo
		},
	}

	logger := testutils.NewLogger(t)

	for name, fsInit := range fss {
		name, fsInit := name, fsInit
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, tCase := range testCases {
				tCase := tCase

				testFunc := func(t *testing.T) {
					t.Parallel()
					fs, _, cleanUp := fsInit()
					defer cleanUp()
					fs = fsext.NewReadOnlyFs(fs)
					openPath := tCase.openPath
					// if fullpath prepend prefix
					if isWindows {
						openPath = strings.ReplaceAll(openPath, `\`, `\\`)
					}
					pwd := tCase.pwd
					if pwd == "" {
						pwd = "/path/to/"
					}
					data := `
						export let file = open("` + openPath + `");
						export default function() { return file };`

					sourceBundle, err := getSimpleBundle(t, filepath.ToSlash(filepath.Join(pwd, "script.js")), data, fs)
					if tCase.isError {
						require.Error(t, err)
						return
					}
					require.NoError(t, err)

					arcBundle, err := NewBundleFromArchive(getTestPreInitState(t, logger, nil), sourceBundle.makeArchive())

					require.NoError(t, err)

					for source, b := range map[string]*Bundle{"source": sourceBundle, "archive": arcBundle} {
						b := b
						t.Run(source, func(t *testing.T) {
							bi, err := b.Instantiate(context.Background(), 0)
							require.NoError(t, err)
							v, err := bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
							require.NoError(t, err)
							require.Equal(t, "hi", v.Export())
						})
					}
				}

				t.Run(tCase.name, testFunc)
				if isWindows {
					tCase := tCase // copy test case before making modifications
					// windowsify the testcase
					tCase.openPath = strings.ReplaceAll(tCase.openPath, `/`, `\`)
					tCase.pwd = strings.ReplaceAll(tCase.pwd, `/`, `\`)
					t.Run(tCase.name+" with windows slash", testFunc)
				}
			}
		})
	}
}

func TestBundleInstantiate(t *testing.T) {
	t.Parallel()
	t.Run("Run", func(t *testing.T) {
		t.Parallel()
		b, err := getSimpleBundle(t, "/script.js", `
		export let options = {
			vus: 5,
			teardownTimeout: '1s',
		};
		let val = true;
		export default function() { return val; }
	`)
		require.NoError(t, err)

		bi, err := b.Instantiate(context.Background(), 0)
		require.NoError(t, err)
		v, err := bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
		require.NoError(t, err)
		require.Equal(t, true, v.Export())
	})

	t.Run("Options", func(t *testing.T) {
		t.Parallel()
		b, err := getSimpleBundle(t, "/script.js", `
			export let options = {
				vus: 5,
				teardownTimeout: '1s',
			};
			let val = true;
			export default function() { return val; }
		`)
		require.NoError(t, err)

		bi, err := b.Instantiate(context.Background(), 0)
		require.NoError(t, err)
		// Ensure `options` properties are correctly marshalled
		jsOptions := bi.getExported("options").ToObject(bi.Runtime)
		vus := jsOptions.Get("vus").Export()
		require.Equal(t, int64(5), vus)
		tdt := jsOptions.Get("teardownTimeout").Export()
		require.Equal(t, "1s", tdt)

		// Ensure options propagate correctly from outside to the script
		optOrig := b.Options.VUs
		b.Options.VUs = null.IntFrom(10)
		bi2, err := b.Instantiate(context.Background(), 0)
		require.NoError(t, err)
		jsOptions = bi2.getExported("options").ToObject(bi2.Runtime)
		vus = jsOptions.Get("vus").Export()
		require.Equal(t, int64(10), vus)
		b.Options.VUs = optOrig
	})
}

func TestBundleEnv(t *testing.T) {
	t.Parallel()
	rtOpts := lib.RuntimeOptions{Env: map[string]string{
		"TEST_A": "1",
		"TEST_B": "",
	}}
	data := `
		export default function() {
			if (__ENV.TEST_A !== "1") { throw new Error("Invalid TEST_A: " + __ENV.TEST_A); }
			if (__ENV.TEST_B !== "") { throw new Error("Invalid TEST_B: " + __ENV.TEST_B); }
		}
	`
	b1, err := getSimpleBundle(t, "/script.js", data, rtOpts)
	require.NoError(t, err)

	logger := testutils.NewLogger(t)
	b2, err := NewBundleFromArchive(getTestPreInitState(t, logger, nil), b1.makeArchive())
	require.NoError(t, err)

	bundles := map[string]*Bundle{"Source": b1, "Archive": b2}
	for name, b := range bundles {
		b := b
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, "1", b.preInitState.RuntimeOptions.Env["TEST_A"])
			require.Equal(t, "", b.preInitState.RuntimeOptions.Env["TEST_B"])

			bi, err := b.Instantiate(context.Background(), 0)
			require.NoError(t, err)
			_, err = bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
			require.NoError(t, err)
		})
	}
}

func TestBundleNotSharable(t *testing.T) {
	t.Parallel()
	data := `
		export default function() {
			if (__ITER == 0) {
				if (typeof __ENV.something !== "undefined") {
					throw new Error("invalid something: " + __ENV.something + " should be undefined");
				}
				__ENV.something = __VU;
			} else if (__ENV.something != __VU) {
				throw new Error("invalid something: " + __ENV.something+ " should be "+ __VU);
			}
		}
	`
	b1, err := getSimpleBundle(t, "/script.js", data)
	require.NoError(t, err)
	logger := testutils.NewLogger(t)

	b2, err := NewBundleFromArchive(getTestPreInitState(t, logger, nil), b1.makeArchive())
	require.NoError(t, err)

	bundles := map[string]*Bundle{"Source": b1, "Archive": b2}
	var vus, iters uint64 = 10, 1000
	for name, b := range bundles {
		b := b
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for i := uint64(0); i < vus; i++ {
				bi, err := b.Instantiate(context.Background(), i)
				require.NoError(t, err)
				for j := uint64(0); j < iters; j++ {
					require.NoError(t, bi.Runtime.Set("__ITER", j))
					_, err := bi.getCallableExport(consts.DefaultFn)(sobek.Undefined())
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestBundleMakeArchive(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		cm      lib.CompatibilityMode
		script  string
		exclaim string
	}{
		{
			lib.CompatibilityModeExtended, `
				import exclaim from "./exclaim.js";
				export let options = { vus: 12345 };
				export let file = open("./file.txt");
				export default function() { return exclaim(file); };`,
			`export default function(s) { return s + "!" };`,
		},
		{
			lib.CompatibilityModeBase, `
				var exclaim = require("./exclaim.js");
				module.exports.options = { vus: 12345 };
				module.exports.file = open("./file.txt");
				module.exports.default = function() { return exclaim(module.exports.file); };`,
			`module.exports.default = function(s) { return s + "!" };`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.cm.String(), func(t *testing.T) {
			t.Parallel()
			fs := fsext.NewMemMapFs()
			_ = fs.MkdirAll("/path/to", 0o755)
			_ = fsext.WriteFile(fs, "/path/to/file.txt", []byte(`hi`), 0o644)
			_ = fsext.WriteFile(fs, "/path/to/exclaim.js", []byte(tc.exclaim), 0o644)

			rtOpts := lib.RuntimeOptions{CompatibilityMode: null.StringFrom(tc.cm.String())}
			b, err := getSimpleBundle(t, "/path/to/script.js", tc.script, fs, rtOpts)
			require.NoError(t, err)

			arc := b.makeArchive()

			assert.Equal(t, "js", arc.Type)
			assert.Equal(t, lib.Options{VUs: null.IntFrom(12345)}, arc.Options)
			assert.Equal(t, "file:///path/to/script.js", arc.FilenameURL.String())
			assert.Equal(t, tc.script, string(arc.Data))
			assert.Equal(t, "file:///path/to/", arc.PwdURL.String())

			exclaimData, err := fsext.ReadFile(arc.Filesystems["file"], "/path/to/exclaim.js")
			require.NoError(t, err)
			assert.Equal(t, tc.exclaim, string(exclaimData))

			fileData, err := fsext.ReadFile(arc.Filesystems["file"], "/path/to/file.txt")
			require.NoError(t, err)
			assert.Equal(t, `hi`, string(fileData))
			assert.Equal(t, build.Version, arc.K6Version)
			assert.Equal(t, tc.cm.String(), arc.CompatibilityMode)
		})
	}
}

func TestGlobalTimers(t *testing.T) {
	t.Parallel()
	data := `
			import timers from "k6/timers";
			if (setTimeout != timers.setTimeout) {
				throw "setTimeout doesn't match";
			}
			if (clearTimeout != timers.clearTimeout) {
				throw "clearTimeout doesn't match";
			}
			if (setInterval != timers.setInterval) {
				throw "setInterval doesn't match";
			}
			if (clearInterval != timers.clearInterval) {
				throw "clearInterval doesn't match";
			}
			export default function() {}
	`

	b1, err := getSimpleBundle(t, "/script.js", data)
	require.NoError(t, err)
	logger := testutils.NewLogger(t)

	b2, err := NewBundleFromArchive(getTestPreInitState(t, logger, nil), b1.makeArchive())
	require.NoError(t, err)

	bundles := map[string]*Bundle{"Source": b1, "Archive": b2}
	for name, b := range bundles {
		b := b
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := b.Instantiate(context.Background(), 1)
			require.NoError(t, err)
		})
	}
}

func TestTopLevelAwaitErrors(t *testing.T) {
	t.Parallel()
	data := `
		const delay = (delayInms) => {
			return new Promise(resolve => setTimeout(resolve, delayInms));
		}

		await delay(10).then(() => {something});
		export default () => {}
	`

	_, err := getSimpleBundle(t, "/script.js", data)
	require.ErrorContains(t, err, "ReferenceError: something is not defined")
}
