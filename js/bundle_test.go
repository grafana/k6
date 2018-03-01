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
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

func getSimpleBundle(filename, data string) (*Bundle, error) {
	return NewBundle(
		&lib.SourceData{
			Filename: filename,
			Data:     []byte(data),
		},
		afero.NewMemMapFs(),
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
		assert.EqualError(t, err, "SyntaxError: /script.js: Unexpected character '\x00' (1:0)\n> 1 | \x00\n    | ^ at <eval>:2:26853(114)")
	})
	t.Run("Error", func(t *testing.T) {
		_, err := getSimpleBundle("/script.js", `throw new Error("aaaa");`)
		assert.EqualError(t, err, "Error: aaaa at /script.js:1:7(3)")
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
			assert.Equal(t, "-", b.Filename)
			assert.Equal(t, "/", b.BaseInitContext.pwd)
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

	src := &lib.SourceData{
		Filename: "/path/to/script.js",
		Data: []byte(`
			import exclaim from "./exclaim.js";
			export let options = { vus: 12345 };
			export let file = open("./file.txt");
			export default function() { return exclaim(file); };
		`),
	}
	b, err := NewBundle(src, fs, lib.RuntimeOptions{})
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

	arc := b.MakeArchive()
	assert.Equal(t, "js", arc.Type)
	assert.Equal(t, lib.Options{VUs: null.IntFrom(12345)}, arc.Options)
	assert.Equal(t, "/path/to/script.js", arc.Filename)
	assert.Equal(t, string(src.Data), string(arc.Data))
	assert.Equal(t, "/path/to", arc.Pwd)
	assert.Len(t, arc.Scripts, 1)
	assert.Equal(t, `export default function(s) { return s + "!" };`, string(arc.Scripts["/path/to/exclaim.js"]))
	assert.Len(t, arc.Files, 1)
	assert.Equal(t, `hi`, string(arc.Files["/path/to/file.txt"]))

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

func TestBundleInstantiate(t *testing.T) {
	b, err := getSimpleBundle("/script.js", `
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
}

func TestBundleEnv(t *testing.T) {
	rtOpts := lib.RuntimeOptions{Env: map[string]string{
		"TEST_A": "1",
		"TEST_B": "",
	}}

	b1, err := NewBundle(
		&lib.SourceData{
			Filename: "/script.js",
			Data: []byte(`
				export default function() {
					if (__ENV.TEST_A !== "1") { throw new Error("Invalid TEST_A: " + __ENV.TEST_A); }
					if (__ENV.TEST_B !== "") { throw new Error("Invalid TEST_B: " + __ENV.TEST_B); }
				}
			`),
		},
		afero.NewMemMapFs(), rtOpts,
	)
	if !assert.NoError(t, err) {
		return
	}

	b2, err := NewBundleFromArchive(b1.MakeArchive(), lib.RuntimeOptions{})
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
