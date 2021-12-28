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
	"fmt"

	"github.com/manyminds/api2go/jsonapi"

	"go.k6.io/k6/lib"
)

type Check struct {
	ID     string `json:"id" yaml:"id"`
	Path   string `json:"path" yaml:"path"`
	Name   string `json:"name" yaml:"name"`
	Passes int64  `json:"passes" yaml:"passes"`
	Fails  int64  `json:"fails" yaml:"fails"`
}

func NewCheck(c *lib.Check) Check {
	return Check{
		ID:     c.ID,
		Path:   c.Path,
		Name:   c.Name,
		Passes: c.Passes,
		Fails:  c.Fails,
	}
}

type Group struct {
	ID     string  `json:"-" yaml:"id"`
	Path   string  `json:"path" yaml:"path"`
	Name   string  `json:"name" yaml:"name"`
	Checks []Check `json:"checks" yaml:"checks"`

	Parent   *Group   `json:"-" yaml:"-"`
	ParentID string   `json:"-" yaml:"parent-id"`
	Groups   []*Group `json:"-" yaml:"-"`
	GroupIDs []string `json:"-" yaml:"group-ids"`
}

func NewGroup(g *lib.Group, parent *Group) *Group {
	group := &Group{
		ID:   g.ID,
		Path: g.Path,
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

// GetID gets a group ID
// Deprecated: use instead g.ID directly
// This method will be removed with the one of the PRs of (https://github.com/grafana/k6/issues/911)
func (g Group) GetID() string {
	return g.ID
}

// SetID sets a group ID
// Deprecated: use instead g.ID directly
// This method will be removed with the one of the PRs of (https://github.com/grafana/k6/issues/911)
func (g *Group) SetID(v string) error {
	g.ID = v
	return nil
}

// GetReferences returns the slice of jsonapi.References
// Deprecated: use instead g.Groups properties
// This method will be removed with the one of the PRs of (https://github.com/grafana/k6/issues/911)
func (g Group) GetReferences() []jsonapi.Reference {
	return []jsonapi.Reference{
		{
			Type:         "groups",
			Name:         "parent",
			Relationship: jsonapi.ToOneRelationship,
		},
		{
			Type:         "groups",
			Name:         "groups",
			Relationship: jsonapi.ToManyRelationship,
		},
	}
}

// GetReferencedIDs returns the slice of jsonapi.ReferenceID
// Deprecated: use instead g.GroupIDs properties
// This method will be removed with the one of the PRs of (https://github.com/grafana/k6/issues/911)
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

func (g *Group) SetToManyReferenceIDs(name string, ids []string) error {
	switch name {
	case "groups":
		g.Groups = nil
		g.GroupIDs = ids
		return nil
	default:
		return fmt.Errorf("unknown to many relation: %s", name)
	}
}

func (g *Group) SetToOneReferenceID(name, id string) error {
	switch name {
	case "parent":
		g.Parent = nil
		g.ParentID = id
		return nil
	default:
		return fmt.Errorf("unknown to one relation: %s", name)
	}
}

func FlattenGroup(g *Group) []*Group {
	groups := []*Group{g}
	for _, gp := range g.Groups {
		groups = append(groups, FlattenGroup(gp)...)
	}
	return groups
}
