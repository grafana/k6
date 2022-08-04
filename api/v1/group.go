package v1

import (
	"fmt"

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
