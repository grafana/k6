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
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/lib/fsext"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
)

const isWindows = runtime.GOOS == "windows"

func getSimpleBundle(tb testing.TB, filename, data string, opts ...interface{}) (*Bundle, error) {
	var (
		fs     = afero.NewMemMapFs()
		rtOpts = lib.RuntimeOptions{}
	)
	for _, o := range opts {
		switch opt := o.(type) {
		case afero.Fs:
			fs = opt
		case lib.RuntimeOptions:
			rtOpts = opt
		}
	}
	return NewBundle(
		testutils.NewLogger(tb),
		&loader.SourceData{
			URL:  &url.URL{Path: filename, Scheme: "file"},
			Data: []byte(data),
		},
		map[string]afero.Fs{"file": fs, "https": afero.NewMemMapFs()},
		rtOpts,
	)
}

func TestNewBundle(t *testing.T) {
	t.Run("Blank", func(t *testing.T) {
		_, err := getSimpleBundle(t, "/script.js", "")
		assert.EqualError(t, err, "no exported functions in script")
	})
	t.Run("Invalid", func(t *testing.T) {
		_, err := getSimpleBundle(t, "/script.js", "\x00")
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "SyntaxError: file:///script.js: Unexpected character '\x00' (1:0)\n> 1 | \x00\n")
	})
	t.Run("Error", func(t *testing.T) {
		_, err := getSimpleBundle(t, "/script.js", `throw new Error("aaaa");`)
		assert.EqualError(t, err, "Error: aaaa at file:///script.js:1:7(3)")
	})
	t.Run("InvalidExports", func(t *testing.T) {
		_, err := getSimpleBundle(t, "/script.js", `exports = null`)
		assert.EqualError(t, err, "exports must be an object")
	})
	t.Run("DefaultUndefined", func(t *testing.T) {
		_, err := getSimpleBundle(t, "/script.js", `export default undefined;`)
		assert.EqualError(t, err, "no exported functions in script")
	})
	t.Run("DefaultNull", func(t *testing.T) {
		_, err := getSimpleBundle(t, "/script.js", `export default null;`)
		assert.EqualError(t, err, "no exported functions in script")
	})
	t.Run("DefaultWrongType", func(t *testing.T) {
		_, err := getSimpleBundle(t, "/script.js", `export default 12345;`)
		assert.EqualError(t, err, "no exported functions in script")
	})
	t.Run("Minimal", func(t *testing.T) {
		_, err := getSimpleBundle(t, "/script.js", `export default function() {};`)
		assert.NoError(t, err)
	})
	t.Run("stdin", func(t *testing.T) {
		b, err := getSimpleBundle(t, "-", `export default function() {};`)
		if assert.NoError(t, err) {
			assert.Equal(t, "file://-", b.Filename.String())
			assert.Equal(t, "file:///", b.BaseInitContext.pwd.String())
		}
	})
	t.Run("CompatibilityMode", func(t *testing.T) {
		t.Run("Extended/ok/CoreJS", func(t *testing.T) {
			rtOpts := lib.RuntimeOptions{
				CompatibilityMode: null.StringFrom(lib.CompatibilityModeExtended.String()),
			}
			_, err := getSimpleBundle(t, "/script.js",
				`module.exports.default = function() {}; new Promise(function(resolve, reject){});`, rtOpts)

			assert.NoError(t, err)
		})
		t.Run("Base/ok/Minimal", func(t *testing.T) {
			rtOpts := lib.RuntimeOptions{
				CompatibilityMode: null.StringFrom(lib.CompatibilityModeBase.String()),
			}
			_, err := getSimpleBundle(t, "/script.js",
				`module.exports.default = function() {};`, rtOpts)
			assert.NoError(t, err)
		})
		t.Run("Base/err", func(t *testing.T) {
			testCases := []struct {
				name       string
				compatMode string
				code       string
				expErr     string
			}{
				{
					"InvalidCompat", "es1", `export default function() {};`,
					`invalid compatibility mode "es1". Use: "extended", "base"`,
				},
				// ES2015 modules are not supported
				{
					"Modules", "base", `export default function() {};`,
					"file:///script.js: Line 1:1 Unexpected reserved word",
				},
				// Arrow functions are not supported
				{
					"ArrowFuncs", "base",
					`module.exports.default = function() {}; () => {};`,
					"file:///script.js: Line 1:42 Unexpected token ) (and 1 more errors)",
				},
				// some ES2015 objects polyfilled by core.js are not supported
				{
					"CoreJS", "base",
					`module.exports.default = function() {}; new Promise(function(resolve, reject){});`,
					"ReferenceError: Promise is not defined at file:///script.js:1:45(5)",
				},
			}

			for _, tc := range testCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					rtOpts := lib.RuntimeOptions{CompatibilityMode: null.StringFrom(tc.compatMode)}
					_, err := getSimpleBundle(t, "/script.js", tc.code, rtOpts)
					assert.EqualError(t, err, tc.expErr)
				})
			}
		})
	})
	t.Run("Options", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			_, err := getSimpleBundle(t, "/script.js", `
				export let options = {};
				export default function() {};
			`)
			assert.NoError(t, err)
		})
		t.Run("Invalid", func(t *testing.T) {
			invalidOptions := map[string]struct {
				Expr, Error string
			}{
				"Array":    {`[]`, "json: cannot unmarshal array into Go value of type lib.Options"},
				"Function": {`function(){}`, "json: unsupported type: func(goja.FunctionCall) goja.Value"},
			}
			for name, data := range invalidOptions {
				t.Run(name, func(t *testing.T) {
					_, err := getSimpleBundle(t, "/script.js", fmt.Sprintf(`
						export let options = %s;
						export default function() {};
					`, data.Expr))
					assert.EqualError(t, err, data.Error)
				})
			}
		})

		t.Run("Paused", func(t *testing.T) {
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					paused: true,
				};
				export default function() {};
			`)
			if assert.NoError(t, err) {
				assert.Equal(t, null.BoolFrom(true), b.Options.Paused)
			}
		})
		t.Run("VUs", func(t *testing.T) {
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					vus: 100,
				};
				export default function() {};
			`)
			if assert.NoError(t, err) {
				assert.Equal(t, null.IntFrom(100), b.Options.VUs)
			}
		})
		t.Run("Duration", func(t *testing.T) {
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					duration: "10s",
				};
				export default function() {};
			`)
			if assert.NoError(t, err) {
				assert.Equal(t, types.NullDurationFrom(10*time.Second), b.Options.Duration)
			}
		})
		t.Run("Iterations", func(t *testing.T) {
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					iterations: 100,
				};
				export default function() {};
			`)
			if assert.NoError(t, err) {
				assert.Equal(t, null.IntFrom(100), b.Options.Iterations)
			}
		})
		t.Run("Stages", func(t *testing.T) {
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					stages: [],
				};
				export default function() {};
			`)
			if assert.NoError(t, err) {
				assert.Len(t, b.Options.Stages, 0)
			}

			t.Run("Empty", func(t *testing.T) {
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						stages: [
							{},
						],
					};
					export default function() {};
				`)
				if assert.NoError(t, err) {
					if assert.Len(t, b.Options.Stages, 1) {
						assert.Equal(t, lib.Stage{}, b.Options.Stages[0])
					}
				}
			})
			t.Run("Target", func(t *testing.T) {
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						stages: [
							{target: 10},
						],
					};
					export default function() {};
				`)
				if assert.NoError(t, err) {
					if assert.Len(t, b.Options.Stages, 1) {
						assert.Equal(t, lib.Stage{Target: null.IntFrom(10)}, b.Options.Stages[0])
					}
				}
			})
			t.Run("Duration", func(t *testing.T) {
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						stages: [
							{duration: "10s"},
						],
					};
					export default function() {};
				`)
				if assert.NoError(t, err) {
					if assert.Len(t, b.Options.Stages, 1) {
						assert.Equal(t, lib.Stage{Duration: types.NullDurationFrom(10 * time.Second)}, b.Options.Stages[0])
					}
				}
			})
			t.Run("DurationAndTarget", func(t *testing.T) {
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						stages: [
							{duration: "10s", target: 10},
						],
					};
					export default function() {};
				`)
				if assert.NoError(t, err) {
					if assert.Len(t, b.Options.Stages, 1) {
						assert.Equal(t, lib.Stage{Duration: types.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)}, b.Options.Stages[0])
					}
				}
			})
			t.Run("RampUpAndPlateau", func(t *testing.T) {
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						stages: [
							{duration: "10s", target: 10},
							{duration: "5s"},
						],
					};
					export default function() {};
				`)
				if assert.NoError(t, err) {
					if assert.Len(t, b.Options.Stages, 2) {
						assert.Equal(t, lib.Stage{Duration: types.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)}, b.Options.Stages[0])
						assert.Equal(t, lib.Stage{Duration: types.NullDurationFrom(5 * time.Second)}, b.Options.Stages[1])
					}
				}
			})
		})
		t.Run("MaxRedirects", func(t *testing.T) {
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					maxRedirects: 10,
				};
				export default function() {};
			`)
			if assert.NoError(t, err) {
				assert.Equal(t, null.IntFrom(10), b.Options.MaxRedirects)
			}
		})
		t.Run("InsecureSkipTLSVerify", func(t *testing.T) {
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					insecureSkipTLSVerify: true,
				};
				export default function() {};
			`)
			if assert.NoError(t, err) {
				assert.Equal(t, null.BoolFrom(true), b.Options.InsecureSkipTLSVerify)
			}
		})
		t.Run("TLSCipherSuites", func(t *testing.T) {
			for suiteName, suiteID := range lib.SupportedTLSCipherSuites {
				t.Run(suiteName, func(t *testing.T) {
					script := `
					export let options = {
						tlsCipherSuites: ["%s"]
					};
					export default function() {};
					`
					script = fmt.Sprintf(script, suiteName)

					b, err := getSimpleBundle(t, "/script.js", script)
					if assert.NoError(t, err) {
						if assert.Len(t, *b.Options.TLSCipherSuites, 1) {
							assert.Equal(t, (*b.Options.TLSCipherSuites)[0], suiteID)
						}
					}
				})
			}
		})
		t.Run("TLSVersion", func(t *testing.T) {
			t.Run("Object", func(t *testing.T) {
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						tlsVersion: {
							min: "ssl3.0",
							max: "tls1.2"
						}
					};
					export default function() {};
				`)
				if assert.NoError(t, err) {
					assert.Equal(t, b.Options.TLSVersion.Min, lib.TLSVersion(tls.VersionSSL30))
					assert.Equal(t, b.Options.TLSVersion.Max, lib.TLSVersion(tls.VersionTLS12))
				}
			})
			t.Run("String", func(t *testing.T) {
				b, err := getSimpleBundle(t, "/script.js", `
					export let options = {
						tlsVersion: "ssl3.0"
					};
					export default function() {};
				`)
				if assert.NoError(t, err) {
					assert.Equal(t, b.Options.TLSVersion.Min, lib.TLSVersion(tls.VersionSSL30))
					assert.Equal(t, b.Options.TLSVersion.Max, lib.TLSVersion(tls.VersionSSL30))
				}
			})
		})
		t.Run("Thresholds", func(t *testing.T) {
			b, err := getSimpleBundle(t, "/script.js", `
				export let options = {
					thresholds: {
						http_req_duration: ["avg<100"],
					},
				};
				export default function() {};
			`)
			if assert.NoError(t, err) {
				if assert.Len(t, b.Options.Thresholds["http_req_duration"].Thresholds, 1) {
					assert.Equal(t, "avg<100", b.Options.Thresholds["http_req_duration"].Thresholds[0].Source)
				}
			}
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
		assert.Equal(t, lib.Options{VUs: null.IntFrom(12345)}, b.Options)
		bi, err := b.Instantiate(logger, 0)
		require.NoError(t, err)
		val, err := bi.exports[consts.DefaultFn](goja.Undefined())
		require.NoError(t, err)
		assert.Equal(t, "hi!", val.Export())
	}

	checkArchive := func(t *testing.T, arc *lib.Archive, rtOpts lib.RuntimeOptions, expError string) {
		b, err := NewBundleFromArchive(logger, arc, rtOpts)
		if expError != "" {
			require.Error(t, err)
			assert.Contains(t, err.Error(), expError)
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
		checkArchive(t, arc, baseCompatModeRtOpts, "Unexpected reserved word")
	})

	t.Run("es6_script_explicit", func(t *testing.T) {
		t.Parallel()
		arc, err := getArchive(t, es6Code, extCompatModeRtOpts)
		require.NoError(t, err)
		require.Equal(t, lib.CompatibilityModeExtended.String(), arc.CompatibilityMode)

		checkArchive(t, arc, lib.RuntimeOptions{}, "")
		checkArchive(t, arc, extCompatModeRtOpts, "")
		checkArchive(t, arc, baseCompatModeRtOpts, "Unexpected reserved word")
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

	t.Run("es6_archive_with_wrong_compat_mode", func(t *testing.T) {
		t.Parallel()
		arc, err := getArchive(t, es6Code, baseCompatModeRtOpts)
		require.Error(t, err)
		require.Nil(t, arc)
	})

	t.Run("messed_up_archive", func(t *testing.T) {
		t.Parallel()
		arc, err := getArchive(t, es6Code, extCompatModeRtOpts)
		require.NoError(t, err)
		arc.CompatibilityMode = "blah"                                           // intentionally break the archive
		checkArchive(t, arc, lib.RuntimeOptions{}, "invalid compatibility mode") // fails when it uses the archive one
		checkArchive(t, arc, extCompatModeRtOpts, "")                            // works when I force the compat mode
		checkArchive(t, arc, baseCompatModeRtOpts, "Unexpected reserved word")   // failes because of ES6
	})

	t.Run("script_options_dont_overwrite_metadata", func(t *testing.T) {
		t.Parallel()
		code := `export let options = { vus: 12345 }; export default function() { return options.vus; };`
		arc := &lib.Archive{
			Type:        "js",
			FilenameURL: &url.URL{Scheme: "file", Path: "/script"},
			K6Version:   consts.Version,
			Data:        []byte(code),
			Options:     lib.Options{VUs: null.IntFrom(999)},
			PwdURL:      &url.URL{Scheme: "file", Path: "/"},
			Filesystems: nil,
		}
		b, err := NewBundleFromArchive(logger, arc, lib.RuntimeOptions{})
		require.NoError(t, err)
		bi, err := b.Instantiate(logger, 0)
		require.NoError(t, err)
		val, err := bi.exports[consts.DefaultFn](goja.Undefined())
		require.NoError(t, err)
		assert.Equal(t, int64(999), val.Export())
	})
}

func TestOpen(t *testing.T) {
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
	fss := map[string]func() (afero.Fs, string, func()){
		"MemMapFS": func() (afero.Fs, string, func()) {
			fs := afero.NewMemMapFs()
			require.NoError(t, fs.MkdirAll("/path/to", 0o755))
			require.NoError(t, afero.WriteFile(fs, "/path/to/file.txt", []byte(`hi`), 0o644))
			return fs, "", func() {}
		},
		"OsFS": func() (afero.Fs, string, func()) {
			prefix, err := ioutil.TempDir("", "k6_open_test")
			require.NoError(t, err)
			fs := afero.NewOsFs()
			filePath := filepath.Join(prefix, "/path/to/file.txt")
			require.NoError(t, fs.MkdirAll(filepath.Join(prefix, "/path/to"), 0o755))
			require.NoError(t, afero.WriteFile(fs, filePath, []byte(`hi`), 0o644))
			if isWindows {
				fs = fsext.NewTrimFilePathSeparatorFs(fs)
			}
			return fs, prefix, func() { require.NoError(t, os.RemoveAll(prefix)) }
		},
	}

	logger := testutils.NewLogger(t)

	for name, fsInit := range fss {
		fs, prefix, cleanUp := fsInit()
		defer cleanUp()
		fs = afero.NewReadOnlyFs(fs)
		t.Run(name, func(t *testing.T) {
			for _, tCase := range testCases {
				tCase := tCase

				testFunc := func(t *testing.T) {
					openPath := tCase.openPath
					// if fullpath prepend prefix
					if openPath != "" && (openPath[0] == '/' || openPath[0] == '\\') {
						openPath = filepath.Join(prefix, openPath)
					}
					if isWindows {
						openPath = strings.Replace(openPath, `\`, `\\`, -1)
					}
					pwd := tCase.pwd
					if pwd == "" {
						pwd = "/path/to/"
					}
					data := `
						export let file = open("` + openPath + `");
						export default function() { return file };`

					sourceBundle, err := getSimpleBundle(t, filepath.ToSlash(filepath.Join(prefix, pwd, "script.js")), data, fs)
					if tCase.isError {
						assert.Error(t, err)
						return
					}
					require.NoError(t, err)

					arcBundle, err := NewBundleFromArchive(logger, sourceBundle.makeArchive(), lib.RuntimeOptions{})

					require.NoError(t, err)

					for source, b := range map[string]*Bundle{"source": sourceBundle, "archive": arcBundle} {
						b := b
						t.Run(source, func(t *testing.T) {
							bi, err := b.Instantiate(logger, 0)
							require.NoError(t, err)
							v, err := bi.exports[consts.DefaultFn](goja.Undefined())
							require.NoError(t, err)
							assert.Equal(t, "hi", v.Export())
						})
					}
				}

				t.Run(tCase.name, testFunc)
				if isWindows {
					// windowsify the testcase
					tCase.openPath = strings.Replace(tCase.openPath, `/`, `\`, -1)
					tCase.pwd = strings.Replace(tCase.pwd, `/`, `\`, -1)
					t.Run(tCase.name+" with windows slash", testFunc)
				}
			}
		})
	}
}

func TestBundleInstantiate(t *testing.T) {
	b, err := getSimpleBundle(t, "/script.js", `
		export let options = {
			vus: 5,
			teardownTimeout: '1s',
		};
		let val = true;
		export default function() { return val; }
	`)
	if !assert.NoError(t, err) {
		return
	}
	logger := testutils.NewLogger(t)

	bi, err := b.Instantiate(logger, 0)
	if !assert.NoError(t, err) {
		return
	}

	t.Run("Run", func(t *testing.T) {
		v, err := bi.exports[consts.DefaultFn](goja.Undefined())
		if assert.NoError(t, err) {
			assert.Equal(t, true, v.Export())
		}
	})

	t.Run("SetAndRun", func(t *testing.T) {
		bi.Runtime.Set("val", false)
		v, err := bi.exports[consts.DefaultFn](goja.Undefined())
		if assert.NoError(t, err) {
			assert.Equal(t, false, v.Export())
		}
	})

	t.Run("Options", func(t *testing.T) {
		// Ensure `options` properties are correctly marshalled
		jsOptions := bi.Runtime.Get("options").ToObject(bi.Runtime)
		vus := jsOptions.Get("vus").Export()
		assert.Equal(t, int64(5), vus)
		tdt := jsOptions.Get("teardownTimeout").Export()
		assert.Equal(t, "1s", tdt)

		// Ensure options propagate correctly from outside to the script
		optOrig := b.Options.VUs
		b.Options.VUs = null.IntFrom(10)
		bi2, err := b.Instantiate(logger, 0)
		assert.NoError(t, err)
		jsOptions = bi2.Runtime.Get("options").ToObject(bi2.Runtime)
		vus = jsOptions.Get("vus").Export()
		assert.Equal(t, int64(10), vus)
		b.Options.VUs = optOrig
	})
}

func TestBundleEnv(t *testing.T) {
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
	if !assert.NoError(t, err) {
		return
	}

	logger := testutils.NewLogger(t)
	b2, err := NewBundleFromArchive(logger, b1.makeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	bundles := map[string]*Bundle{"Source": b1, "Archive": b2}
	for name, b := range bundles {
		b := b
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "1", b.Env["TEST_A"])
			assert.Equal(t, "", b.Env["TEST_B"])

			bi, err := b.Instantiate(logger, 0)
			if assert.NoError(t, err) {
				_, err := bi.exports[consts.DefaultFn](goja.Undefined())
				assert.NoError(t, err)
			}
		})
	}
}

func TestBundleNotSharable(t *testing.T) {
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
	if !assert.NoError(t, err) {
		return
	}
	logger := testutils.NewLogger(t)

	b2, err := NewBundleFromArchive(logger, b1.makeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	bundles := map[string]*Bundle{"Source": b1, "Archive": b2}
	vus, iters := 10, 1000
	for name, b := range bundles {
		b := b
		t.Run(name, func(t *testing.T) {
			for i := 0; i < vus; i++ {
				bi, err := b.Instantiate(logger, int64(i))
				require.NoError(t, err)
				for j := 0; j < iters; j++ {
					bi.Runtime.Set("__ITER", j)
					_, err := bi.exports[consts.DefaultFn](goja.Undefined())
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestBundleMakeArchive(t *testing.T) {
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
			fs := afero.NewMemMapFs()
			_ = fs.MkdirAll("/path/to", 0o755)
			_ = afero.WriteFile(fs, "/path/to/file.txt", []byte(`hi`), 0o644)
			_ = afero.WriteFile(fs, "/path/to/exclaim.js", []byte(tc.exclaim), 0o644)

			rtOpts := lib.RuntimeOptions{CompatibilityMode: null.StringFrom(tc.cm.String())}
			b, err := getSimpleBundle(t, "/path/to/script.js", tc.script, fs, rtOpts)
			assert.NoError(t, err)

			arc := b.makeArchive()

			assert.Equal(t, "js", arc.Type)
			assert.Equal(t, lib.Options{VUs: null.IntFrom(12345)}, arc.Options)
			assert.Equal(t, "file:///path/to/script.js", arc.FilenameURL.String())
			assert.Equal(t, tc.script, string(arc.Data))
			assert.Equal(t, "file:///path/to/", arc.PwdURL.String())

			exclaimData, err := afero.ReadFile(arc.Filesystems["file"], "/path/to/exclaim.js")
			assert.NoError(t, err)
			assert.Equal(t, tc.exclaim, string(exclaimData))

			fileData, err := afero.ReadFile(arc.Filesystems["file"], "/path/to/file.txt")
			assert.NoError(t, err)
			assert.Equal(t, `hi`, string(fileData))
			assert.Equal(t, consts.Version, arc.K6Version)
			assert.Equal(t, tc.cm.String(), arc.CompatibilityMode)
		})
	}
}
