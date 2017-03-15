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

package js2

import (
	"testing"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

func TestNewBundle(t *testing.T) {
	t.Run("Blank", func(t *testing.T) {
		_, err := NewBundle(&lib.SourceData{
			Filename: "/script.js",
			Data:     []byte(``),
		}, afero.NewMemMapFs())
		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("DefaultUndefined", func(t *testing.T) {
		_, err := NewBundle(&lib.SourceData{
			Filename: "/script.js",
			Data: []byte(`
				export default undefined;
			`),
		}, afero.NewMemMapFs())
		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("DefaultNull", func(t *testing.T) {
		_, err := NewBundle(&lib.SourceData{
			Filename: "/script.js",
			Data: []byte(`
				export default null;
			`),
		}, afero.NewMemMapFs())
		assert.EqualError(t, err, "script must export a default function")
	})
	t.Run("DefaultWrongType", func(t *testing.T) {
		_, err := NewBundle(&lib.SourceData{
			Filename: "/script.js",
			Data: []byte(`
				export default 12345;
			`),
		}, afero.NewMemMapFs())
		assert.EqualError(t, err, "default export must be a function")
	})
	t.Run("Minimal", func(t *testing.T) {
		_, err := NewBundle(&lib.SourceData{
			Filename: "/script.js",
			Data: []byte(`
				export default function() {};
			`),
		}, afero.NewMemMapFs())
		assert.NoError(t, err)
	})
	t.Run("Options", func(t *testing.T) {
		_, err := NewBundle(&lib.SourceData{
			Filename: "/script.js",
			Data: []byte(`
					export let options = {};
					export default function() {};
				`),
		}, afero.NewMemMapFs())
		assert.NoError(t, err)

		t.Run("Paused", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					export let options = {
						paused: true,
					};
					export default function() {};
				`),
			}, afero.NewMemMapFs())
			if assert.NoError(t, err) {
				assert.Equal(t, null.BoolFrom(true), b.Options.Paused)
			}
		})
		t.Run("VUs", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					export let options = {
						vus: 100,
					};
					export default function() {};
				`),
			}, afero.NewMemMapFs())
			if assert.NoError(t, err) {
				assert.Equal(t, null.IntFrom(100), b.Options.VUs)
			}
		})
		t.Run("VUsMax", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					export let options = {
						vusMax: 100,
					};
					export default function() {};
				`),
			}, afero.NewMemMapFs())
			if assert.NoError(t, err) {
				assert.Equal(t, null.IntFrom(100), b.Options.VUsMax)
			}
		})
		t.Run("Duration", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					export let options = {
						duration: "10s",
					};
					export default function() {};
				`),
			}, afero.NewMemMapFs())
			if assert.NoError(t, err) {
				assert.Equal(t, null.StringFrom("10s"), b.Options.Duration)
			}
		})
		t.Run("Iterations", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					export let options = {
						iterations: 100,
					};
					export default function() {};
				`),
			}, afero.NewMemMapFs())
			if assert.NoError(t, err) {
				assert.Equal(t, null.IntFrom(100), b.Options.Iterations)
			}
		})
		t.Run("Stages", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					export let options = {
						stages: [],
					};
					export default function() {};
				`),
			}, afero.NewMemMapFs())
			if assert.NoError(t, err) {
				assert.Len(t, b.Options.Stages, 0)
			}

			t.Run("Empty", func(t *testing.T) {
				b, err := NewBundle(&lib.SourceData{
					Filename: "/script.js",
					Data: []byte(`
						export let options = {
							stages: [
								{},
							],
						};
						export default function() {};
					`),
				}, afero.NewMemMapFs())
				if assert.NoError(t, err) {
					if assert.Len(t, b.Options.Stages, 1) {
						assert.Equal(t, lib.Stage{}, b.Options.Stages[0])
					}
				}
			})
			t.Run("Target", func(t *testing.T) {
				b, err := NewBundle(&lib.SourceData{
					Filename: "/script.js",
					Data: []byte(`
						export let options = {
							stages: [
								{target: 10},
							],
						};
						export default function() {};
					`),
				}, afero.NewMemMapFs())
				if assert.NoError(t, err) {
					if assert.Len(t, b.Options.Stages, 1) {
						assert.Equal(t, lib.Stage{Target: null.IntFrom(10)}, b.Options.Stages[0])
					}
				}
			})
			t.Run("Duration", func(t *testing.T) {
				b, err := NewBundle(&lib.SourceData{
					Filename: "/script.js",
					Data: []byte(`
						export let options = {
							stages: [
								{duration: "10s"},
							],
						};
						export default function() {};
					`),
				}, afero.NewMemMapFs())
				if assert.NoError(t, err) {
					if assert.Len(t, b.Options.Stages, 1) {
						assert.Equal(t, lib.Stage{Duration: 10 * time.Second}, b.Options.Stages[0])
					}
				}
			})
			t.Run("DurationAndTarget", func(t *testing.T) {
				b, err := NewBundle(&lib.SourceData{
					Filename: "/script.js",
					Data: []byte(`
						export let options = {
							stages: [
								{duration: "10s", target: 10},
							],
						};
						export default function() {};
					`),
				}, afero.NewMemMapFs())
				if assert.NoError(t, err) {
					if assert.Len(t, b.Options.Stages, 1) {
						assert.Equal(t, lib.Stage{Duration: 10 * time.Second, Target: null.IntFrom(10)}, b.Options.Stages[0])
					}
				}
			})
			t.Run("RampUpAndPlateau", func(t *testing.T) {
				b, err := NewBundle(&lib.SourceData{
					Filename: "/script.js",
					Data: []byte(`
						export let options = {
							stages: [
								{duration: "10s", target: 10},
								{duration: "5s"},
							],
						};
						export default function() {};
					`),
				}, afero.NewMemMapFs())
				if assert.NoError(t, err) {
					if assert.Len(t, b.Options.Stages, 2) {
						assert.Equal(t, lib.Stage{Duration: 10 * time.Second, Target: null.IntFrom(10)}, b.Options.Stages[0])
						assert.Equal(t, lib.Stage{Duration: 5 * time.Second}, b.Options.Stages[1])
					}
				}
			})
		})
		t.Run("Linger", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					export let options = {
						linger: true,
					};
					export default function() {};
				`),
			}, afero.NewMemMapFs())
			if assert.NoError(t, err) {
				assert.Equal(t, null.BoolFrom(true), b.Options.Linger)
			}
		})
		t.Run("NoUsageReport", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					export let options = {
						noUsageReport: true,
					};
					export default function() {};
				`),
			}, afero.NewMemMapFs())
			if assert.NoError(t, err) {
				assert.Equal(t, null.BoolFrom(true), b.Options.NoUsageReport)
			}
		})
		t.Run("MaxRedirects", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					export let options = {
						maxRedirects: 10,
					};
					export default function() {};
				`),
			}, afero.NewMemMapFs())
			if assert.NoError(t, err) {
				assert.Equal(t, null.IntFrom(10), b.Options.MaxRedirects)
			}
		})
		t.Run("InsecureSkipTLSVerify", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					export let options = {
						insecureSkipTLSVerify: true,
					};
					export default function() {};
				`),
			}, afero.NewMemMapFs())
			if assert.NoError(t, err) {
				assert.Equal(t, null.BoolFrom(true), b.Options.InsecureSkipTLSVerify)
			}
		})
		t.Run("Thresholds", func(t *testing.T) {
			b, err := NewBundle(&lib.SourceData{
				Filename: "/script.js",
				Data: []byte(`
					export let options = {
						thresholds: {
							http_req_duration: ["avg<100"],
						},
					};
					export default function() {};
				`),
			}, afero.NewMemMapFs())
			if assert.NoError(t, err) {
				if assert.Len(t, b.Options.Thresholds["http_req_duration"].Thresholds, 1) {
					assert.Equal(t, "avg<100", b.Options.Thresholds["http_req_duration"].Thresholds[0].Source)
				}
			}
		})
	})
}
