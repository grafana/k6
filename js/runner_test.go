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
	"context"
	"crypto/tls"
	"fmt"
	"testing"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

func TestRunnerNew(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		r, err := New(&lib.SourceData{
			Filename: "/script.js",
			Data: []byte(`
			let counter = 0;
			export default function() { counter++; }
		`),
		}, afero.NewMemMapFs())
		assert.NoError(t, err)

		t.Run("NewVU", func(t *testing.T) {
			vu_, err := r.NewVU()
			assert.NoError(t, err)
			vu := vu_.(*VU)
			assert.Equal(t, int64(0), vu.Runtime.Get("counter").Export())

			t.Run("RunOnce", func(t *testing.T) {
				_, err = vu.RunOnce(context.Background())
				assert.NoError(t, err)
				assert.Equal(t, int64(1), vu.Runtime.Get("counter").Export())
			})
		})
	})

	t.Run("Invalid", func(t *testing.T) {
		_, err := New(&lib.SourceData{
			Filename: "/script.js",
			Data:     []byte(`blarg`),
		}, afero.NewMemMapFs())
		assert.EqualError(t, err, "ReferenceError: blarg is not defined at /script.js:1:14(0)")
	})
}

func TestRunnerGetDefaultGroup(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data:     []byte(`export default function() {};`),
	}, afero.NewMemMapFs())
	if assert.NoError(t, err) {
		assert.NotNil(t, r1.GetDefaultGroup())
	}

	r2, err := NewFromArchive(r1.MakeArchive())
	if assert.NoError(t, err) {
		assert.NotNil(t, r2.GetDefaultGroup())
	}
}

func TestRunnerOptions(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data:     []byte(`export default function() {};`),
	}, afero.NewMemMapFs())
	if !assert.NoError(t, err) {
		return
	}

	r2, err := NewFromArchive(r1.MakeArchive())
	if !assert.NoError(t, err) {
		return
	}

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, r.Bundle.Options, r.GetOptions())
			assert.Equal(t, null.NewBool(false, false), r.Bundle.Options.Paused)
			r.ApplyOptions(lib.Options{Paused: null.BoolFrom(true)})
			assert.Equal(t, r.Bundle.Options, r.GetOptions())
			assert.Equal(t, null.NewBool(true, true), r.Bundle.Options.Paused)
			r.ApplyOptions(lib.Options{Paused: null.BoolFrom(false)})
			assert.Equal(t, r.Bundle.Options, r.GetOptions())
			assert.Equal(t, null.NewBool(false, true), r.Bundle.Options.Paused)
		})
	}
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
			t.Run(mod, func(t *testing.T) {
				t.Run("Source", func(t *testing.T) {
					_, err := New(&lib.SourceData{
						Filename: "/script.js",
						Data:     []byte(fmt.Sprintf(`import "%s"; export default function() {}`, mod)),
					}, afero.NewMemMapFs())
					assert.NoError(t, err)
				})
			})
		}
	})

	t.Run("Files", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		assert.NoError(t, fs.MkdirAll("/path/to", 0755))
		assert.NoError(t, afero.WriteFile(fs, "/path/to/lib.js", []byte(`export default "hi!";`), 0644))

		testdata := map[string]struct{ filename, path string }{
			"Absolute":       {"/path/script.js", "/path/to/lib.js"},
			"Relative":       {"/path/script.js", "./to/lib.js"},
			"Adjacent":       {"/path/to/script.js", "./lib.js"},
			"STDIN-Absolute": {"-", "/path/to/lib.js"},
			"STDIN-Relative": {"-", "./path/to/lib.js"},
		}
		for name, data := range testdata {
			t.Run(name, func(t *testing.T) {
				r1, err := New(&lib.SourceData{
					Filename: data.filename,
					Data: []byte(fmt.Sprintf(`
					import hi from "%s";
					export default function() {
						if (hi != "hi!") { throw new Error("incorrect value"); }
					}`, data.path)),
				}, fs)
				if !assert.NoError(t, err) {
					return
				}

				r2, err := NewFromArchive(r1.MakeArchive())
				if !assert.NoError(t, err) {
					return
				}

				testdata := map[string]*Runner{"Source": r1, "Archive": r2}
				for name, r := range testdata {
					t.Run(name, func(t *testing.T) {
						vu, err := r.NewVU()
						if !assert.NoError(t, err) {
							return
						}
						_, err = vu.RunOnce(context.Background())
						assert.NoError(t, err)
					})
				}
			})
		}
	})
}

func TestVURunContext(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
		export let options = { vus: 10 };
		export default function() { fn(); }
		`),
	}, afero.NewMemMapFs())
	if !assert.NoError(t, err) {
		return
	}
	r1.ApplyOptions(lib.Options{Throw: null.BoolFrom(true)})

	r2, err := NewFromArchive(r1.MakeArchive())
	if !assert.NoError(t, err) {
		return
	}

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		t.Run(name, func(t *testing.T) {
			vu, err := r.newVU()
			if !assert.NoError(t, err) {
				return
			}

			fnCalled := false
			vu.Runtime.Set("fn", func() {
				fnCalled = true

				assert.Equal(t, vu.Runtime, common.GetRuntime(*vu.Context), "incorrect runtime in context")

				state := common.GetState(*vu.Context)
				if assert.NotNil(t, state) {
					assert.Equal(t, null.IntFrom(10), state.Options.VUs)
					assert.Equal(t, null.BoolFrom(true), state.Options.Throw)
					assert.NotNil(t, state.Logger)
					assert.Equal(t, r.GetDefaultGroup(), state.Group)
					assert.Equal(t, vu.HTTPTransport, state.HTTPTransport)
				}
			})
			_, err = vu.RunOnce(context.Background())
			assert.NoError(t, err)
			assert.True(t, fnCalled, "fn() not called")
		})
	}
}

func TestVUIntegrationGroups(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
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
		`),
	}, afero.NewMemMapFs())
	if !assert.NoError(t, err) {
		return
	}

	r2, err := NewFromArchive(r1.MakeArchive())
	if !assert.NoError(t, err) {
		return
	}

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		t.Run(name, func(t *testing.T) {
			vu, err := r.newVU()
			if !assert.NoError(t, err) {
				return
			}

			fnOuterCalled := false
			fnInnerCalled := false
			fnNestedCalled := false
			vu.Runtime.Set("fnOuter", func() {
				fnOuterCalled = true
				assert.Equal(t, r.GetDefaultGroup(), common.GetState(*vu.Context).Group)
			})
			vu.Runtime.Set("fnInner", func() {
				fnInnerCalled = true
				g := common.GetState(*vu.Context).Group
				assert.Equal(t, "my group", g.Name)
				assert.Equal(t, r.GetDefaultGroup(), g.Parent)
			})
			vu.Runtime.Set("fnNested", func() {
				fnNestedCalled = true
				g := common.GetState(*vu.Context).Group
				assert.Equal(t, "nested group", g.Name)
				assert.Equal(t, "my group", g.Parent.Name)
				assert.Equal(t, r.GetDefaultGroup(), g.Parent.Parent)
			})
			_, err = vu.RunOnce(context.Background())
			assert.NoError(t, err)
			assert.True(t, fnOuterCalled, "fnOuter() not called")
			assert.True(t, fnInnerCalled, "fnInner() not called")
			assert.True(t, fnNestedCalled, "fnNested() not called")
		})
	}
}

func TestVUIntegrationMetrics(t *testing.T) {
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
		import { group } from "k6";
		import { Trend } from "k6/metrics";
		let myMetric = new Trend("my_metric");
		export default function() { myMetric.add(5); }
		`),
	}, afero.NewMemMapFs())
	if !assert.NoError(t, err) {
		return
	}

	r2, err := NewFromArchive(r1.MakeArchive())
	if !assert.NoError(t, err) {
		return
	}

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		t.Run(name, func(t *testing.T) {
			vu, err := r.newVU()
			if !assert.NoError(t, err) {
				return
			}

			samples, err := vu.RunOnce(context.Background())
			assert.NoError(t, err)
			assert.Len(t, samples, 3)
			for i, s := range samples {
				switch i {
				case 0:
					assert.Equal(t, 5.0, s.Value)
					assert.Equal(t, "my_metric", s.Metric.Name)
					assert.Equal(t, stats.Trend, s.Metric.Type)
				case 1:
					assert.Equal(t, 0.0, s.Value)
					assert.Equal(t, metrics.DataSent, s.Metric)
				case 2:
					assert.Equal(t, 0.0, s.Value)
					assert.Equal(t, metrics.DataReceived, s.Metric)
				}
			}
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
			r1, err := New(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					import http from "k6/http";
					export default function() { http.get("https://expired.badssl.com/"); }
				`),
			}, afero.NewMemMapFs())
			if !assert.NoError(t, err) {
				return
			}
			r1.ApplyOptions(lib.Options{Throw: null.BoolFrom(true)})
			r1.ApplyOptions(data.opts)

			r2, err := NewFromArchive(r1.MakeArchive())
			if !assert.NoError(t, err) {
				return
			}

			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for name, r := range runners {
				t.Run(name, func(t *testing.T) {
					r.Logger, _ = logtest.NewNullLogger()

					vu, err := r.NewVU()
					if !assert.NoError(t, err) {
						return
					}
					_, err = vu.RunOnce(context.Background())
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
func TestVUIntegrationTLSConfig(t *testing.T) {
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
			lib.Options{TLSVersion: &lib.TLSVersion{Min: tls.VersionTLS12, Max: tls.VersionTLS12}},
			"",
		},
		"UnsupportedVersion": {
			lib.Options{TLSVersion: &lib.TLSVersion{Min: tls.VersionSSL30, Max: tls.VersionSSL30}},
			"GoError: Get https://sha256.badssl.com/: remote error: tls: handshake failure",
		},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			r1, err := New(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					import http from "k6/http";
					export default function() { http.get("https://sha256.badssl.com/"); }
				`),
			}, afero.NewMemMapFs())
			if !assert.NoError(t, err) {
				return
			}
			r1.ApplyOptions(lib.Options{Throw: null.BoolFrom(true)})
			r1.ApplyOptions(data.opts)

			r2, err := NewFromArchive(r1.MakeArchive())
			if !assert.NoError(t, err) {
				return
			}

			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for name, r := range runners {
				t.Run(name, func(t *testing.T) {
					r.Logger, _ = logtest.NewNullLogger()

					vu, err := r.NewVU()
					if !assert.NoError(t, err) {
						return
					}
					_, err = vu.RunOnce(context.Background())
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
	r1, err := New(&lib.SourceData{
		Filename: "/script.js",
		Data: []byte(`
			import http from "k6/http";
			export default function() {
				let res = http.request("GET", "https://http2.akamai.com/demo");
				if (res.status != 200) { throw new Error("wrong status: " + res.status) }
				if (res.proto != "HTTP/2.0") { throw new Error("wrong proto: " + res.proto) }
			}
		`),
	}, afero.NewMemMapFs())
	if !assert.NoError(t, err) {
		return
	}
	r1.ApplyOptions(lib.Options{Throw: null.BoolFrom(true)})

	r2, err := NewFromArchive(r1.MakeArchive())
	if !assert.NoError(t, err) {
		return
	}

	runners := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range runners {
		t.Run(name, func(t *testing.T) {
			vu, err := r.NewVU()
			if !assert.NoError(t, err) {
				return
			}
			samples, err := vu.RunOnce(context.Background())
			assert.NoError(t, err)

			protoFound := false
			for _, sample := range samples {
				if proto := sample.Tags["proto"]; proto != "" {
					protoFound = true
					assert.Equal(t, "HTTP/2.0", proto)
				}
			}
			assert.True(t, protoFound)
		})
	}
}
