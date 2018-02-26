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

package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDuration(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "1m15s", Duration(75*time.Second).String())
	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("Unmarshal", func(t *testing.T) {
			t.Run("Number", func(t *testing.T) {
				var d Duration
				assert.NoError(t, json.Unmarshal([]byte(`75000000000`), &d))
				assert.Equal(t, Duration(75*time.Second), d)
			})
			t.Run("Seconds", func(t *testing.T) {
				var d Duration
				assert.NoError(t, json.Unmarshal([]byte(`"75s"`), &d))
				assert.Equal(t, Duration(75*time.Second), d)
			})
			t.Run("String", func(t *testing.T) {
				var d Duration
				assert.NoError(t, json.Unmarshal([]byte(`"1m15s"`), &d))
				assert.Equal(t, Duration(75*time.Second), d)
			})
		})
		t.Run("Marshal", func(t *testing.T) {
			d := Duration(75 * time.Second)
			data, err := json.Marshal(d)
			assert.NoError(t, err)
			assert.Equal(t, `"1m15s"`, string(data))
		})
	})
	t.Run("Text", func(t *testing.T) {
		var d Duration
		assert.NoError(t, d.UnmarshalText([]byte(`10s`)))
		assert.Equal(t, Duration(10*time.Second), d)
	})
}

func TestNullDuration(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "1m15s", Duration(75*time.Second).String())
	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("Unmarshal", func(t *testing.T) {
			t.Run("Number", func(t *testing.T) {
				var d NullDuration
				assert.NoError(t, json.Unmarshal([]byte(`75000000000`), &d))
				assert.Equal(t, NullDuration{Duration(75 * time.Second), true}, d)
			})
			t.Run("Seconds", func(t *testing.T) {
				var d NullDuration
				assert.NoError(t, json.Unmarshal([]byte(`"75s"`), &d))
				assert.Equal(t, NullDuration{Duration(75 * time.Second), true}, d)
			})
			t.Run("String", func(t *testing.T) {
				var d NullDuration
				assert.NoError(t, json.Unmarshal([]byte(`"1m15s"`), &d))
				assert.Equal(t, NullDuration{Duration(75 * time.Second), true}, d)
			})
			t.Run("Null", func(t *testing.T) {
				var d NullDuration
				assert.NoError(t, json.Unmarshal([]byte(`null`), &d))
				assert.Equal(t, NullDuration{Duration(0), false}, d)
			})
		})
		t.Run("Marshal", func(t *testing.T) {
			t.Run("Valid", func(t *testing.T) {
				d := NullDuration{Duration(75 * time.Second), true}
				data, err := json.Marshal(d)
				assert.NoError(t, err)
				assert.Equal(t, `"1m15s"`, string(data))
			})
			t.Run("null", func(t *testing.T) {
				var d NullDuration
				data, err := json.Marshal(d)
				assert.NoError(t, err)
				assert.Equal(t, `null`, string(data))
			})
		})
	})
	t.Run("Text", func(t *testing.T) {
		var d NullDuration
		assert.NoError(t, d.UnmarshalText([]byte(`10s`)))
		assert.Equal(t, NullDurationFrom(10*time.Second), d)

		t.Run("Empty", func(t *testing.T) {
			var d NullDuration
			assert.NoError(t, d.UnmarshalText([]byte(``)))
			assert.Equal(t, NullDuration{}, d)
		})
	})
}

func TestNullDurationFrom(t *testing.T) {
	assert.Equal(t, NullDuration{Duration(10 * time.Second), true}, NullDurationFrom(10*time.Second))
}
