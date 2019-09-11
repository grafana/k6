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
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/lib/fsext"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

const isWindows = runtime.GOOS == "windows"

func getSimpleBundle(filename, data string) (*Bundle, error) {
	return getSimpleBundleWithFs(filename, data, afero.NewMemMapFs())
}

func getSimpleBundleWithOptions(filename, data string, options lib.RuntimeOptions) (*Bundle, error) {
	return NewBundle(
		&loader.SourceData{
			URL:  &url.URL{Path: filename, Scheme: "file"},
			Data: []byte(data),
		},
		map[string]afero.Fs{"file": afero.NewMemMapFs(), "https": afero.NewMemMapFs()},
		options,
	)
}

func getSimpleBundleWithFs(filename, data string, fs afero.Fs) (*Bundle, error) {
	return NewBundle(
		&loader.SourceData{
			URL:  &url.URL{Path: filename, Scheme: "file"},
			Data: []byte(data),
		},
		map[string]afero.Fs{"file": fs, "https": afero.NewMemMapFs()},
		lib.RuntimeOptions{},
	)
}

func TestNewBundle(t *testing.T) {
	t.Run("Blank", func(t *testing.T) {
		_, err := getSimpleBundle("/script.js", "")
		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("Invalid", func(t *testing.T) {
		_, err := getSimpleBundle("/script.js", "\x00")
		assert.Contains(t, err.Error(), "SyntaxError: file:///script.js: Unexpected character '\x00' (1:0)\n> 1 | \x00\n")
	})
	t.Run("Error", func(t *testing.T) {
		_, err := getSimpleBundle("/script.js", `throw new Error("aaaa");`)
		assert.EqualError(t, err, "Error: aaaa at file:///script.js:1:7(3)")
	})
	t.Run("InvalidExports", func(t *testing.T) {
		_, err := getSimpleBundle("/script.js", `exports = null`)
		assert.EqualError(t, err, "exports must be an object")
	})
	t.Run("DefaultUndefined", func(t *testing.T) {
		_, err := getSimpleBundle("/script.js", `export default undefined;`)
		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("DefaultNull", func(t *testing.T) {
		_, err := getSimpleBundle("/script.js", `export default null;`)
		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("DefaultWrongType", func(t *testing.T) {
		_, err := getSimpleBundle("/script.js", `export default 12345;`)
		assert.EqualError(t, err, "default export must be a function")
	})
	t.Run("Minimal", func(t *testing.T) {
		_, err := getSimpleBundle("/script.js", `export default function() {};`)
		assert.NoError(t, err)
	})
	t.Run("stdin", func(t *testing.T) {
		b, err := getSimpleBundle("-", `export default function() {};`)
		if assert.NoError(t, err) {
			assert.Equal(t, "file://-", b.Filename.String())
			assert.Equal(t, "file:///", b.BaseInitContext.pwd.String())
		}
	})
	t.Run("Options", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			_, err := getSimpleBundle("/script.js", `
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
					_, err := getSimpleBundle("/script.js", fmt.Sprintf(`
						export let options = %s;
						export default function() {};
					`, data.Expr))
					assert.EqualError(t, err, data.Error)
				})
			}
		})

		t.Run("Paused", func(t *testing.T) {
			b, err := getSimpleBundle("/script.js", `
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
			b, err := getSimpleBundle("/script.js", `
				export let options = {
					vus: 100,
				};
				export default function() {};
			`)
			if assert.NoError(t, err) {
				assert.Equal(t, null.IntFrom(100), b.Options.VUs)
			}
		})
		t.Run("VUsMax", func(t *testing.T) {
			b, err := getSimpleBundle("/script.js", `
				export let options = {
					vusMax: 100,
				};
				export default function() {};
			`)
			if assert.NoError(t, err) {
				assert.Equal(t, null.IntFrom(100), b.Options.VUsMax)
			}
		})
		t.Run("Duration", func(t *testing.T) {
			b, err := getSimpleBundle("/script.js", `
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
			b, err := getSimpleBundle("/script.js", `
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
			b, err := getSimpleBundle("/script.js", `
				export let options = {
					stages: [],
				};
				export default function() {};
			`)
			if assert.NoError(t, err) {
				assert.Len(t, b.Options.Stages, 0)
			}

			t.Run("Empty", func(t *testing.T) {
				b, err := getSimpleBundle("/script.js", `
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
				b, err := getSimpleBundle("/script.js", `
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
				b, err := getSimpleBundle("/script.js", `
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
				b, err := getSimpleBundle("/script.js", `
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
				b, err := getSimpleBundle("/script.js", `
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
			b, err := getSimpleBundle("/script.js", `
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
			b, err := getSimpleBundle("/script.js", `
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

					b, err := getSimpleBundle("/script.js", script)
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
				b, err := getSimpleBundle("/script.js", `
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
				b, err := getSimpleBundle("/script.js", `
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
			b, err := getSimpleBundle("/script.js", `
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

func TestNewBundleFromArchive(t *testing.T) {
	fs := afero.NewMemMapFs()
	assert.NoError(t, fs.MkdirAll("/path/to", 0755))
	assert.NoError(t, afero.WriteFile(fs, "/path/to/file.txt", []byte(`hi`), 0644))
	assert.NoError(t, afero.WriteFile(fs, "/path/to/exclaim.js", []byte(`export default function(s) { return s + "!" };`), 0644))

	data := `
			import exclaim from "./exclaim.js";
			export let options = { vus: 12345 };
			export let file = open("./file.txt");
			export default function() { return exclaim(file); };
		`
	b, err := getSimpleBundleWithFs("/path/to/script.js", data, fs)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, lib.Options{VUs: null.IntFrom(12345)}, b.Options)

	bi, err := b.Instantiate()
	if !assert.NoError(t, err) {
		return
	}
	v, err := bi.Default(goja.Undefined())
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "hi!", v.Export())

	arc := b.makeArchive()
	assert.Equal(t, "js", arc.Type)
	assert.Equal(t, lib.Options{VUs: null.IntFrom(12345)}, arc.Options)
	assert.Equal(t, "file:///path/to/script.js", arc.FilenameURL.String())
	assert.Equal(t, data, string(arc.Data))
	assert.Equal(t, "file:///path/to/", arc.PwdURL.String())

	exclaimData, err := afero.ReadFile(arc.Filesystems["file"], "/path/to/exclaim.js")
	assert.NoError(t, err)
	assert.Equal(t, `export default function(s) { return s + "!" };`, string(exclaimData))

	fileData, err := afero.ReadFile(arc.Filesystems["file"], "/path/to/file.txt")
	assert.NoError(t, err)
	assert.Equal(t, `hi`, string(fileData))
	assert.Equal(t, consts.Version, arc.K6Version)

	b2, err := NewBundleFromArchive(arc, lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, lib.Options{VUs: null.IntFrom(12345)}, b2.Options)

	bi2, err := b.Instantiate()
	if !assert.NoError(t, err) {
		return
	}
	v2, err := bi2.Default(goja.Undefined())
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "hi!", v2.Export())
}

func TestOpen(t *testing.T) {
	var testCases = [...]struct {
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
			require.NoError(t, fs.MkdirAll("/path/to", 0755))
			require.NoError(t, afero.WriteFile(fs, "/path/to/file.txt", []byte(`hi`), 0644))
			return fs, "", func() {}
		},
		"OsFS": func() (afero.Fs, string, func()) {
			prefix, err := ioutil.TempDir("", "k6_open_test")
			require.NoError(t, err)
			fs := afero.NewOsFs()
			filePath := filepath.Join(prefix, "/path/to/file.txt")
			require.NoError(t, fs.MkdirAll(filepath.Join(prefix, "/path/to"), 0755))
			require.NoError(t, afero.WriteFile(fs, filePath, []byte(`hi`), 0644))
			if isWindows {
				fs = fsext.NewTrimFilePathSeparatorFs(fs)
			}
			return fs, prefix, func() { require.NoError(t, os.RemoveAll(prefix)) }
		},
	}

	for name, fsInit := range fss {
		fs, prefix, cleanUp := fsInit()
		defer cleanUp()
		fs = afero.NewReadOnlyFs(fs)
		t.Run(name, func(t *testing.T) {
			for _, tCase := range testCases {
				tCase := tCase

				var testFunc = func(t *testing.T) {
					var openPath = tCase.openPath
					// if fullpath prepend prefix
					if openPath != "" && (openPath[0] == '/' || openPath[0] == '\\') {
						openPath = filepath.Join(prefix, openPath)
					}
					if isWindows {
						openPath = strings.Replace(openPath, `\`, `\\`, -1)
					}
					var pwd = tCase.pwd
					if pwd == "" {
						pwd = "/path/to/"
					}
					data := `
						export let file = open("` + openPath + `");
						export default function() { return file };`

					sourceBundle, err := getSimpleBundleWithFs(filepath.ToSlash(filepath.Join(prefix, pwd, "script.js")), data, fs)
					if tCase.isError {
						assert.Error(t, err)
						return
					}
					require.NoError(t, err)

					arcBundle, err := NewBundleFromArchive(sourceBundle.makeArchive(), lib.RuntimeOptions{})

					require.NoError(t, err)

					for source, b := range map[string]*Bundle{"source": sourceBundle, "archive": arcBundle} {
						b := b
						t.Run(source, func(t *testing.T) {
							bi, err := b.Instantiate()
							require.NoError(t, err)
							v, err := bi.Default(goja.Undefined())
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
	b, err := getSimpleBundle("/script.js", `
		export let options = {
			vus: 5,
		};
		let val = true;
		export default function() { return val; }
	`)
	if !assert.NoError(t, err) {
		return
	}

	bi, err := b.Instantiate()
	if !assert.NoError(t, err) {
		return
	}

	t.Run("Run", func(t *testing.T) {
		v, err := bi.Default(goja.Undefined())
		if assert.NoError(t, err) {
			assert.Equal(t, true, v.Export())
		}
	})

	t.Run("SetAndRun", func(t *testing.T) {
		bi.Runtime.Set("val", false)
		v, err := bi.Default(goja.Undefined())
		if assert.NoError(t, err) {
			assert.Equal(t, false, v.Export())
		}
	})

	t.Run("Options", func(t *testing.T) {
		// Ensure `options` properties are correctly marshalled
		jsOptions := bi.Runtime.Get("options").ToObject(bi.Runtime)
		val := jsOptions.Get("vus").Export()
		assert.Equal(t, int64(5), val)
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
	b1, err := getSimpleBundleWithOptions("/script.js", data, rtOpts)
	if !assert.NoError(t, err) {
		return
	}

	b2, err := NewBundleFromArchive(b1.makeArchive(), lib.RuntimeOptions{})
	if !assert.NoError(t, err) {
		return
	}

	bundles := map[string]*Bundle{"Source": b1, "Archive": b2}
	for name, b := range bundles {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "1", b.Env["TEST_A"])
			assert.Equal(t, "", b.Env["TEST_B"])

			bi, err := b.Instantiate()
			if assert.NoError(t, err) {
				_, err := bi.Default(goja.Undefined())
				assert.NoError(t, err)
			}
		})
	}
}
