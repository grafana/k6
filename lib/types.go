package lib

import (
	"github.com/manyminds/api2go/jsonapi"
	"gopkg.in/guregu/null.v3"
	"strconv"
	"sync"
	"sync/atomic"
)

type Status struct {
	Running null.Bool `json:"running"`
	Tainted null.Bool `json:"tainted"`
	VUs     null.Int  `json:"vus"`
	VUsMax  null.Int  `json:"vus-max"`
	AtTime  null.Int  `json:"at-time"`
}

func (s Status) GetName() string {
	return "status"
}

func (s Status) GetID() string {
	return "default"
}

func (s Status) SetID(id string) error {
	return nil
}

type Stage struct {
	ID int64 `json:"-"`

	Order    null.Int `json:"order"`
	Duration null.Int `json:"duration"`
	VUTarget null.Int `json:"vu-target"`
}

func (s Stage) GetName() string {
	return "stage"
}

func (s Stage) GetID() string {
	return strconv.FormatInt(s.ID, 10)
}

func (s *Stage) SetID(v string) error {
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return err
	}
	s.ID = id
	return nil
}

type Info struct {
	Version string `json:"version"`
}

func (i Info) GetName() string {
	return "info"
}

func (i Info) GetID() string {
	return "default"
}

type Options struct {
	VUs      null.Int    `json:"vus"`
	VUsMax   null.Int    `json:"vus-max"`
	Duration null.String `json:"duration"`
}

func (o Options) GetName() string {
	return "options"
}

func (o Options) GetID() string {
	return "default"
}

type Group struct {
	ID int64 `json:"-"`

	Name   string            `json:"name"`
	Parent *Group            `json:"-"`
	Groups map[string]*Group `json:"-"`
	Checks map[string]*Check `json:"-"`

	groupMutex sync.Mutex `json:"-"`
	checkMutex sync.Mutex `json:"-"`
}

func NewGroup(name string, parent *Group, idCounter *int64) *Group {
	var id int64
	if idCounter != nil {
		id = atomic.AddInt64(idCounter, 1)
	}

	return &Group{
		ID:     id,
		Name:   name,
		Parent: parent,
		Groups: make(map[string]*Group),
		Checks: make(map[string]*Check),
	}
}

func (g *Group) Group(name string, idCounter *int64) (*Group, bool) {
	snapshot := g.Groups
	group, ok := snapshot[name]
	if !ok {
		g.groupMutex.Lock()
		group, ok = g.Groups[name]
		if !ok {
			group = NewGroup(name, g, idCounter)
			g.Groups[name] = group
		}
		g.groupMutex.Unlock()
	}
	return group, ok
}

func (g *Group) Check(name string, idCounter *int64) (*Check, bool) {
	snapshot := g.Checks
	check, ok := snapshot[name]
	if !ok {
		g.checkMutex.Lock()
		check, ok = g.Checks[name]
		if !ok {
			check = NewCheck(name, g, idCounter)
			g.Checks[name] = check
		}
		g.checkMutex.Unlock()
	}
	return check, ok
}

func (g Group) GetID() string {
	return strconv.FormatInt(g.ID, 10)
}

func (g Group) GetReferences() []jsonapi.Reference {
	return []jsonapi.Reference{
		jsonapi.Reference{
			Name:         "parent",
			Type:         "groups",
			Relationship: jsonapi.ToOneRelationship,
		},
		jsonapi.Reference{
			Name:         "checks",
			Type:         "checks",
			Relationship: jsonapi.ToManyRelationship,
		},
	}
}

func (g Group) GetReferencedIDs() []jsonapi.ReferenceID {
	ids := make([]jsonapi.ReferenceID, 0, len(g.Checks)+len(g.Groups))
	for _, check := range g.Checks {
		ids = append(ids, jsonapi.ReferenceID{
			ID:           check.GetID(),
			Type:         "checks",
			Name:         "checks",
			Relationship: jsonapi.ToManyRelationship,
		})
	}
	for _, group := range g.Groups {
		ids = append(ids, jsonapi.ReferenceID{
			ID:           group.GetID(),
			Type:         "groups",
			Name:         "groups",
			Relationship: jsonapi.ToManyRelationship,
		})
	}
	if g.Parent != nil {
		ids = append(ids, jsonapi.ReferenceID{
			ID:           g.Parent.GetID(),
			Type:         "groups",
			Name:         "parent",
			Relationship: jsonapi.ToOneRelationship,
		})
	}
	return ids
}

func (g Group) GetReferencedStructs() []jsonapi.MarshalIdentifier {
	// Note: we're not sideloading the parent, that snowballs into making requests for a single
	// group return *every single known group* thanks to the common root group.
	refs := make([]jsonapi.MarshalIdentifier, 0, len(g.Checks)+len(g.Groups))
	for _, check := range g.Checks {
		refs = append(refs, check)
	}
	for _, group := range g.Groups {
		refs = append(refs, group)
	}
	return refs
}

type Check struct {
	ID int64 `json:"-"`

	Group *Group `json:"-"`
	Name  string `json:"name"`

	Passes int64 `json:"passes"`
	Fails  int64 `json:"fails"`
}

func NewCheck(name string, group *Group, idCounter *int64) *Check {
	var id int64
	if idCounter != nil {
		id = atomic.AddInt64(idCounter, 1)
	}
	return &Check{ID: id, Name: name, Group: group}
}

func (c Check) GetID() string {
	return strconv.FormatInt(c.ID, 10)
}

func (c Check) GetReferences() []jsonapi.Reference {
	return []jsonapi.Reference{
		jsonapi.Reference{
			Name:         "group",
			Type:         "groups",
			Relationship: jsonapi.ToOneRelationship,
		},
	}
}

func (c Check) GetReferencedIDs() []jsonapi.ReferenceID {
	return []jsonapi.ReferenceID{
		jsonapi.ReferenceID{
			ID:   c.Group.GetID(),
			Type: "groups",
			Name: "group",
		},
	}
}

func (c Check) GetReferencedStructs() []jsonapi.MarshalIdentifier {
	return []jsonapi.MarshalIdentifier{c.Group}
}
