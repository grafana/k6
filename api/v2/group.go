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

package v2

import (
	"github.com/loadimpact/k6/lib"
	"github.com/manyminds/api2go/jsonapi"
	"github.com/pkg/errors"
	"strconv"
)

type Check struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Passes int64  `json:"passes"`
	Fails  int64  `json:"fails"`
}

func NewCheck(c *lib.Check) Check {
	return Check{
		ID:     c.ID,
		Name:   c.Name,
		Passes: c.Passes,
		Fails:  c.Fails,
	}
}

type Group struct {
	ID     int64   `json:"-"`
	Name   string  `json:"name"`
	Checks []Check `json:"checks"`

	Parent   *Group   `json:"-"`
	ParentID int64    `json:"-"`
	Groups   []*Group `json:"-"`
	GroupIDs []int64  `json:"-"`
}

func NewGroup(g *lib.Group, parent *Group) *Group {
	group := &Group{
		ID:   g.ID,
		Name: g.Name,
	}

	if parent != nil {
		group.Parent = parent
		group.ParentID = parent.ID
	} else if g.Parent != nil {
		group.Parent = NewGroup(g.Parent, nil)
		group.ParentID = g.Parent.ID
	}

	for _, gp := range g.Groups {
		group.Groups = append(group.Groups, NewGroup(gp, group))
		group.GroupIDs = append(group.GroupIDs, gp.ID)
	}
	for _, c := range g.Checks {
		group.Checks = append(group.Checks, NewCheck(c))
	}

	return group
}

func (g Group) GetID() string {
	return strconv.FormatInt(g.ID, 10)
}

func (g *Group) SetID(v string) error {
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return err
	}
	g.ID = id
	return nil
}

func (g Group) GetReferences() []jsonapi.Reference {
	return []jsonapi.Reference{
		jsonapi.Reference{
			Type:         "groups",
			Name:         "parent",
			Relationship: jsonapi.ToOneRelationship,
		},
		jsonapi.Reference{
			Type:         "groups",
			Name:         "groups",
			Relationship: jsonapi.ToManyRelationship,
		},
	}
}

func (g Group) GetReferencedIDs() []jsonapi.ReferenceID {
	refs := []jsonapi.ReferenceID{}
	if g.Parent != nil {
		refs = append(refs, jsonapi.ReferenceID{
			ID:           g.Parent.GetID(),
			Type:         "groups",
			Name:         "parent",
			Relationship: jsonapi.ToOneRelationship,
		})
	}
	for _, gp := range g.Groups {
		refs = append(refs, jsonapi.ReferenceID{
			ID:           gp.GetID(),
			Type:         "groups",
			Name:         "groups",
			Relationship: jsonapi.ToManyRelationship,
		})
	}
	return refs
}

func (g *Group) SetToManyReferenceIDs(name string, IDs []string) error {
	switch name {
	case "groups":
		ids := make([]int64, len(IDs))
		for i, ID := range IDs {
			id, err := strconv.ParseInt(ID, 10, 64)
			if err != nil {
				return err
			}
			ids[i] = id
		}
		g.Groups = nil
		g.GroupIDs = ids
		return nil
	default:
		return errors.New("Unknown to many relation: " + name)
	}
}

func (g *Group) SetToOneReferenceID(name, ID string) error {
	switch name {
	case "parent":
		id, err := strconv.ParseInt(ID, 10, 64)
		if err != nil {
			return err
		}
		g.Parent = nil
		g.ParentID = id
		return nil
	default:
		return errors.New("Unknown to one relation: " + name)
	}
}

func FlattenGroup(g *Group) []*Group {
	groups := []*Group{g}
	for _, gp := range g.Groups {
		groups = append(groups, FlattenGroup(gp)...)
	}
	return groups
}
