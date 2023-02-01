package v1

import (
	"encoding/json"
	"net/http"
)

func handleGetGroups(cs *ControlSurface, rw http.ResponseWriter, _ *http.Request) {
	root := NewGroup(cs.RunState.Runner.GetDefaultGroup(), nil)
	groups := FlattenGroup(root)

	data, err := json.Marshal(newGroupsJSONAPI(groups))
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}

func handleGetGroup(cs *ControlSurface, rw http.ResponseWriter, _ *http.Request, id string) {
	root := NewGroup(cs.RunState.Runner.GetDefaultGroup(), nil)
	groups := FlattenGroup(root)

	var group *Group
	for _, g := range groups {
		if g.ID == id {
			group = g
			break
		}
	}
	if group == nil {
		apiError(rw, "Not Found", "No group with that ID was found", http.StatusNotFound)
		return
	}

	data, err := json.Marshal(newGroupJSONAPI(group))
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}
