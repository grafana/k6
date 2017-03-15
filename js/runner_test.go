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
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
)

func TestNewRunner(t *testing.T) {
	if testing.Short() {
		return
	}

	rt, err := New()
	assert.NoError(t, err)
	srcdata := &lib.SourceData{
		Filename: "test.js",
		Data:     []byte("export default function() {}"),
	}
	exp, err := rt.load(srcdata.Filename, srcdata.Data)
	assert.NoError(t, err)
	r, err := NewRunner(rt, exp)
	assert.NoError(t, err)
	if !assert.NotNil(t, r) {
		return
	}

	t.Run("GetDefaultGroup", func(t *testing.T) {
		assert.Equal(t, r.DefaultGroup, r.GetDefaultGroup())
	})

	t.Run("VU", func(t *testing.T) {
		vu_, err := r.NewVU()
		assert.NoError(t, err)
		vu := vu_.(*VU)

		t.Run("Reconfigure", func(t *testing.T) {
			assert.NoError(t, vu.Reconfigure(12345))
			assert.Equal(t, int64(12345), vu.ID)
		})

		t.Run("RunOnce", func(t *testing.T) {
			_, err := vu.RunOnce(context.Background())
			assert.NoError(t, err)
		})
	})
}

func TestVUSelfIdentity(t *testing.T) {
	r, err := newSnippetRunner(`
	export default function() {}
	`)
	assert.NoError(t, err)

	vu_, err := r.NewVU()
	assert.NoError(t, err)
	vu := vu_.(*VU)

	assert.NoError(t, vu.Reconfigure(1234))
	_, err = vu.vm.Eval(`if(__VU != 1234) { throw new Error(__VU); }`)
	assert.NoError(t, err)
	_, err = vu.vm.Eval(`if(__ITER != 0) { throw new Error(__ITER); }`)
	assert.NoError(t, err)

	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)
	_, err = vu.vm.Eval(`if(__VU != 1234) { throw new Error(__VU); }`)
	assert.NoError(t, err)
	_, err = vu.vm.Eval(`if(__ITER != 0) { throw new Error(__ITER); }`)
	assert.NoError(t, err)

	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)
	_, err = vu.vm.Eval(`if(__VU != 1234) { throw new Error(__VU); }`)
	assert.NoError(t, err)
	_, err = vu.vm.Eval(`if(__ITER != 1) { throw new Error(__ITER); }`)
	assert.NoError(t, err)

	assert.NoError(t, vu.Reconfigure(1234))
	_, err = vu.vm.Eval(`if(__VU != 1234) { throw new Error(__VU); }`)
	assert.NoError(t, err)
	_, err = vu.vm.Eval(`if(__ITER != 0) { throw new Error(__ITER); }`)
	assert.NoError(t, err)
}
