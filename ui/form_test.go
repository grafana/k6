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

package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForm(t *testing.T) {
	t.Run("Blank", func(t *testing.T) {
		data, err := Form{}.Run(strings.NewReader(""), bytes.NewBuffer(nil))
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{}, data)
	})
	t.Run("Banner", func(t *testing.T) {
		out := bytes.NewBuffer(nil)
		data, err := Form{Banner: "Hi!"}.Run(strings.NewReader(""), out)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{}, data)
		assert.Equal(t, "Hi!\n\n", out.String())
	})
	t.Run("Field", func(t *testing.T) {
		f := Form{
			Fields: []Field{
				StringField{Key: "key", Label: "label"},
			},
		}
		in := "Value\n"
		out := bytes.NewBuffer(nil)
		data, err := f.Run(strings.NewReader(in), out)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"key": "Value"}, data)
		assert.Equal(t, "  label: ", out.String())
	})
	t.Run("Fields", func(t *testing.T) {
		f := Form{
			Fields: []Field{
				StringField{Key: "a", Label: "label a"},
				StringField{Key: "b", Label: "label b"},
			},
		}
		in := "1\n2\n"
		out := bytes.NewBuffer(nil)
		data, err := f.Run(strings.NewReader(in), out)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"a": "1", "b": "2"}, data)
		assert.Equal(t, "  label a:   label b: ", out.String())
	})
	t.Run("Defaults", func(t *testing.T) {
		f := Form{
			Fields: []Field{
				StringField{Key: "a", Label: "label a", Default: "default a"},
				StringField{Key: "b", Label: "label b", Default: "default b"},
			},
		}
		in := "\n2\n"
		out := bytes.NewBuffer(nil)
		data, err := f.Run(strings.NewReader(in), out)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"a": "default a", "b": "2"}, data)
		assert.Equal(t, "  label a [default a]:   label b [default b]: ", out.String())
	})
	t.Run("Errors", func(t *testing.T) {
		f := Form{
			Fields: []Field{
				StringField{Key: "key", Label: "label", Min: 6, Max: 10},
			},
		}
		in := "short\ntoo damn long\nperfect\n"
		out := bytes.NewBuffer(nil)
		data, err := f.Run(strings.NewReader(in), out)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"key": "perfect"}, data)
		assert.Equal(t, "  label: - invalid input, min length is 6\n  label: - invalid input, max length is 10\n  label: ", out.String())
	})
}
