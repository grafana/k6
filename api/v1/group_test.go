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
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/lib"
)

func TestNewCheck(t *testing.T) {
	og, err := lib.NewGroup("", nil)
	assert.NoError(t, err)
	oc, err := lib.NewCheck("my check", og)
	assert.NoError(t, err)
	oc.Passes = 1234
	oc.Fails = 5678

	c := NewCheck(oc)
	assert.Equal(t, oc.ID, c.ID)
	assert.Equal(t, "my check", c.Name)
	assert.Equal(t, int64(1234), c.Passes)
	assert.Equal(t, int64(5678), c.Fails)
}

func TestNewGroup(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		og, err := lib.NewGroup("My Group", nil)
		assert.NoError(t, err)

		g := NewGroup(og, nil)
		assert.Equal(t, og.ID, g.ID)
		assert.Equal(t, og.Name, g.Name)
		assert.Nil(t, g.Parent)
		assert.Empty(t, g.Groups)
	})
	t.Run("groups", func(t *testing.T) {
		root, _ := lib.NewGroup("My Group", nil)
		child, _ := root.Group("Child")
		inner, _ := child.Group("Inner")

		g := NewGroup(root, nil)
		assert.Equal(t, root.ID, g.ID)
		assert.Equal(t, "My Group", g.Name)
		assert.Nil(t, g.Parent)
		assert.Len(t, g.Groups, 1)
		assert.Len(t, g.Checks, 0)

		assert.Equal(t, "Child", g.Groups[0].Name)
		assert.Equal(t, child.ID, g.Groups[0].ID)
		assert.Equal(t, "My Group", g.Groups[0].Parent.Name)
		assert.Equal(t, root.ID, g.Groups[0].Parent.ID)

		assert.Equal(t, "Inner", g.Groups[0].Groups[0].Name)
		assert.Equal(t, inner.ID, g.Groups[0].Groups[0].ID)
		assert.Equal(t, "Child", g.Groups[0].Groups[0].Parent.Name)
		assert.Equal(t, child.ID, g.Groups[0].Groups[0].Parent.ID)
		assert.Equal(t, "My Group", g.Groups[0].Groups[0].Parent.Parent.Name)
		assert.Equal(t, root.ID, g.Groups[0].Groups[0].Parent.Parent.ID)
	})
	t.Run("checks", func(t *testing.T) {
		og, _ := lib.NewGroup("My Group", nil)
		check, _ := og.Check("my check")

		g := NewGroup(og, nil)
		assert.Equal(t, og.ID, g.ID)
		assert.Equal(t, "My Group", g.Name)
		assert.Nil(t, g.Parent)
		assert.Len(t, g.Groups, 0)
		assert.Len(t, g.Checks, 1)

		assert.Equal(t, check.ID, g.Checks[0].ID)
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
