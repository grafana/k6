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

package v1

import (
	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewCheck(t *testing.T) {
	c := NewCheck(&lib.Check{ID: 1, Name: "my check", Passes: 1234, Fails: 5678})
	assert.Equal(t, int64(1), c.ID)
	assert.Equal(t, "my check", c.Name)
	assert.Equal(t, int64(1234), c.Passes)
	assert.Equal(t, int64(5678), c.Fails)
}

func TestNewGroup(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		g := NewGroup(&lib.Group{ID: 1, Name: "My Group"}, nil)
		assert.Equal(t, int64(1), g.ID)
		assert.Equal(t, "My Group", g.Name)
		assert.Nil(t, g.Parent)
		assert.Empty(t, g.Groups)
	})
	t.Run("groups", func(t *testing.T) {
		og := &lib.Group{ID: 1, Name: "My Group"}
		og.Groups = map[string]*lib.Group{
			"Child": {ID: 2, Name: "Child", Parent: og},
		}
		og.Groups["Child"].Groups = map[string]*lib.Group{
			"Inner": {ID: 3, Name: "Inner", Parent: og.Groups["Child"]},
		}

		g := NewGroup(og, nil)
		assert.Equal(t, int64(1), g.ID)
		assert.Equal(t, "My Group", g.Name)
		assert.Nil(t, g.Parent)
		assert.Len(t, g.Groups, 1)
		assert.Len(t, g.Checks, 0)

		assert.Equal(t, "Child", g.Groups[0].Name)
		assert.Equal(t, int64(2), g.Groups[0].ID)
		assert.Equal(t, "My Group", g.Groups[0].Parent.Name)
		assert.Equal(t, int64(1), g.Groups[0].Parent.ID)

		assert.Equal(t, "Inner", g.Groups[0].Groups[0].Name)
		assert.Equal(t, int64(3), g.Groups[0].Groups[0].ID)
		assert.Equal(t, "Child", g.Groups[0].Groups[0].Parent.Name)
		assert.Equal(t, int64(2), g.Groups[0].Groups[0].Parent.ID)
		assert.Equal(t, "My Group", g.Groups[0].Groups[0].Parent.Parent.Name)
		assert.Equal(t, int64(1), g.Groups[0].Groups[0].Parent.Parent.ID)
	})
	t.Run("checks", func(t *testing.T) {
		og := &lib.Group{ID: 1, Name: "My Group"}
		og.Checks = map[string]*lib.Check{
			"my check": {ID: 1, Name: "my check", Group: og},
		}

		g := NewGroup(og, nil)
		assert.Equal(t, int64(1), g.ID)
		assert.Equal(t, "My Group", g.Name)
		assert.Nil(t, g.Parent)
		assert.Len(t, g.Groups, 0)
		assert.Len(t, g.Checks, 1)

		assert.Equal(t, int64(1), g.Checks[0].ID)
		assert.Equal(t, "my check", g.Checks[0].Name)
	})
}

func TestFlattenGroup(t *testing.T) {
	t.Run("blank", func(t *testing.T) {
		g := &Group{}
		assert.EqualValues(t, []*Group{g}, FlattenGroup(g))
	})
	t.Run("one level", func(t *testing.T) {
		g := &Group{}
		g1 := &Group{Parent: g}
		g2 := &Group{Parent: g}
		g.Groups = []*Group{g1, g2}
		assert.EqualValues(t, []*Group{g, g1, g2}, FlattenGroup(g))
	})
	t.Run("two levels", func(t *testing.T) {
		g := &Group{}
		g1 := &Group{Parent: g}
		g1a := &Group{Parent: g1}
		g1b := &Group{Parent: g1}
		g1.Groups = []*Group{g1a, g1b}
		g2 := &Group{Parent: g}
		g.Groups = []*Group{g1, g2}
		assert.EqualValues(t, []*Group{g, g1, g1a, g1b, g2}, FlattenGroup(g))
	})
}
