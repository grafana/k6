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

package k6

import (
	"context"
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js2/common"
	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
)

func TestGroup(t *testing.T) {
	rt := goja.New()

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)
	state := &common.State{Group: root}

	mod := Module
	mod.Context = common.WithState(context.Background(), state)
	rt.Set("mod", mod.Export(rt))

	t.Run("Valid", func(t *testing.T) {
		assert.Equal(t, state.Group, root)
		rt.Set("fn", func() {
			assert.Equal(t, state.Group.Name, "my group")
			assert.Equal(t, state.Group.Parent, root)
		})
		_, err = common.RunString(rt, `mod.group("my group", fn)`)
		assert.NoError(t, err)
		assert.Equal(t, state.Group, root)
	})

	t.Run("Invalid", func(t *testing.T) {
		_, err := common.RunString(rt, `mod.group("::", function() { throw new Error("nooo") })`)
		assert.EqualError(t, err, "GoError: group and check names may not contain '::'")
	})
}

func TestCheck(t *testing.T) {
	rt := goja.New()

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)
	state := &common.State{Group: root}

	mod := Module
	mod.Context = context.Background()
	mod.Context = common.WithState(mod.Context, state)
	mod.Context = common.WithRuntime(mod.Context, rt)
	rt.Set("mod", mod.Export(rt))

	t.Run("Object", func(t *testing.T) {
		_, err := common.RunString(rt, `mod.check(null, { "check": true })`)
		assert.NoError(t, err)
	})
	t.Run("Array", func(t *testing.T) {
		_, err := common.RunString(rt, `mod.check(null, [ true ])`)
		assert.NoError(t, err)
	})
	t.Run("Literal", func(t *testing.T) {
		_, err := common.RunString(rt, `mod.check(null, null)`)
		assert.EqualError(t, err, "TypeError: Cannot convert undefined or null to object")
	})

	t.Run("Throws", func(t *testing.T) {
		_, err := common.RunString(rt, `
		mod.check(null, {
			"a": function() { throw new Error("error A") },
			"b": function() { throw new Error("error B") },
		})
		`)
		assert.EqualError(t, err, "GoError: Error: error A")
	})

	t.Run("Types", func(t *testing.T) {
		templates := map[string]string{
			"Literal":      `mod.check(null,{"check": %s})`,
			"Callable":     `mod.check(null,{"check": ()=>%s})`,
			"Callable/Arg": `mod.check(%s,{"check":(v)=>v})`,
		}
		testdata := map[string]bool{
			`0`:         false,
			`1`:         true,
			`-1`:        true,
			`""`:        false,
			`"true"`:    true,
			`"false"`:   true,
			`true`:      true,
			`false`:     false,
			`null`:      false,
			`undefined`: false,
		}
		for name, tpl := range templates {
			t.Run(name, func(t *testing.T) {
				for value, succ := range testdata {
					t.Run(value, func(t *testing.T) {
						v, err := common.RunString(rt, fmt.Sprintf(tpl, value))
						if assert.NoError(t, err) {
							assert.Equal(t, succ, v.Export())
						}
					})
				}
			})
		}
	})
}
