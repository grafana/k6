package v1

import (
	"encoding/json"
	"net/http"

	"go.k6.io/k6/api/common"
)

func handleGetGroups(rw http.ResponseWriter, r *http.Request) {
	engine := common.GetEngine(r.Context())

	root := NewGroup(engine.ExecutionScheduler.GetRunner().GetDefaultGroup(), nil)
	groups := FlattenGroup(root)

	data, err := json.Marshal(newGroupsJSONAPI(groups))
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}

func handleGetGroup(rw http.ResponseWriter, r *http.Request, id string) {
	engine := common.GetEngine(r.Context())

	root := NewGroup(engine.ExecutionScheduler.GetRunner().GetDefaultGroup(), nil)
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
