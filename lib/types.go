package lib

import (
	"github.com/manyminds/api2go/jsonapi"
	"gopkg.in/guregu/null.v3"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Status struct {
	Running null.Bool `json:"running"`
	VUs     null.Int  `json:"vus"`
	VUsMax  null.Int  `json:"vus-max"`
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
	VUs      int64         `json:"vus"`
	VUsMax   int64         `json:"vus-max"`
	Duration time.Duration `json:"duration"`
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
	Tests  map[string]*Test  `json:"-"`

	groupMutex sync.Mutex `json:"-"`
	testMutex  sync.Mutex `json:"-"`
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
		Tests:  make(map[string]*Test),
	}
}

func (g *Group) Group(name string, idCounter *int64) (*Group, bool) {
	group, ok := g.Groups[name]
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

func (g *Group) Test(name string, idCounter *int64) (*Test, bool) {
	test, ok := g.Tests[name]
	if !ok {
		g.testMutex.Lock()
		test, ok = g.Tests[name]
		if !ok {
			test = NewTest(name, g, idCounter)
			g.Tests[name] = test
		}
		g.testMutex.Unlock()
	}
	return test, ok
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
			Name:         "tests",
			Type:         "tests",
			Relationship: jsonapi.ToManyRelationship,
		},
	}
}

func (g Group) GetReferencedIDs() []jsonapi.ReferenceID {
	ids := make([]jsonapi.ReferenceID, 0, len(g.Tests)+len(g.Groups))
	for _, test := range g.Tests {
		ids = append(ids, jsonapi.ReferenceID{
			ID:           test.GetID(),
			Type:         "tests",
			Name:         "tests",
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
	refs := make([]jsonapi.MarshalIdentifier, 0, len(g.Tests)+len(g.Groups))
	for _, test := range g.Tests {
		refs = append(refs, test)
	}
	for _, group := range g.Groups {
		refs = append(refs, group)
	}
	return refs
}

type Test struct {
	ID int64 `json:"-"`

	Group *Group `json:"-"`
	Name  string `json:"name"`

	Passes int64 `json:"passes"`
	Fails  int64 `json:"fails"`
}

func NewTest(name string, group *Group, idCounter *int64) *Test {
	var id int64
	if idCounter != nil {
		id = atomic.AddInt64(idCounter, 1)
	}
	return &Test{ID: id, Name: name, Group: group}
}

func (t Test) GetID() string {
	return strconv.FormatInt(t.ID, 10)
}

func (t Test) GetReferences() []jsonapi.Reference {
	return []jsonapi.Reference{
		jsonapi.Reference{
			Name:         "group",
			Type:         "groups",
			Relationship: jsonapi.ToOneRelationship,
		},
	}
}

func (t Test) GetReferencedIDs() []jsonapi.ReferenceID {
	return []jsonapi.ReferenceID{
		jsonapi.ReferenceID{
			ID:   t.Group.GetID(),
			Type: "groups",
			Name: "group",
		},
	}
}

func (t Test) GetReferencedStructs() []jsonapi.MarshalIdentifier {
	return []jsonapi.MarshalIdentifier{t.Group}
}
