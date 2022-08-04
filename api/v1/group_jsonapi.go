package v1

import "encoding/json"

type groupJSONAPI struct {
	Data groupData `json:"data"`
}

type groupsJSONAPI struct {
	Data []groupData `json:"data"`
}

type groupData struct {
	Type          string         `json:"type"`
	ID            string         `json:"id"`
	Attributes    Group          `json:"attributes"`
	Relationships groupRelations `json:"relationships"`
}

type groupRelations struct {
	Groups struct {
		Data []groupRelation `json:"data"`
	} `json:"groups"`
	Parent struct {
		Data *groupRelation `json:"data"`
	} `json:"parent"`
}

type groupRelation struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// UnmarshalJSON unmarshal group data properly (extract the ID)
func (g *groupData) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type          string         `json:"type"`
		ID            string         `json:"id"`
		Attributes    Group          `json:"attributes"`
		Relationships groupRelations `json:"relationships"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	g.ID = raw.ID
	g.Type = raw.Type
	g.Relationships = raw.Relationships
	g.Attributes = raw.Attributes

	if g.Attributes.ID == "" {
		g.Attributes.ID = raw.ID
	}

	if g.Relationships.Parent.Data != nil {
		g.Attributes.ParentID = g.Relationships.Parent.Data.ID
	}

	g.Attributes.GroupIDs = make([]string, 0, len(g.Relationships.Groups.Data))
	for _, rel := range g.Relationships.Groups.Data {
		g.Attributes.GroupIDs = append(g.Attributes.GroupIDs, rel.ID)
	}

	return nil
}

func newGroupJSONAPI(g *Group) groupJSONAPI {
	return groupJSONAPI{
		Data: newGroupData(g),
	}
}

func newGroupsJSONAPI(groups []*Group) groupsJSONAPI {
	envelop := groupsJSONAPI{
		Data: make([]groupData, 0, len(groups)),
	}

	for _, g := range groups {
		envelop.Data = append(envelop.Data, newGroupData(g))
	}

	return envelop
}

func newGroupData(group *Group) groupData {
	data := groupData{
		Type:       "groups",
		ID:         group.ID,
		Attributes: *group,
		Relationships: groupRelations{
			Groups: struct {
				Data []groupRelation `json:"data"`
			}{
				Data: make([]groupRelation, 0, len(group.Groups)),
			},
			Parent: struct {
				Data *groupRelation `json:"data"`
			}{},
		},
	}

	if group.Parent != nil {
		data.Relationships.Parent.Data = &groupRelation{
			Type: "groups",
			ID:   group.Parent.ID,
		}
	}

	for _, gp := range group.Groups {
		data.Relationships.Groups.Data = append(data.Relationships.Groups.Data, groupRelation{
			ID:   gp.ID,
			Type: "groups",
		})
	}

	return data
}
