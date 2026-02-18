package v1

type setUpJSONAPI struct {
	Data setUpData `json:"data"`
}

type setUpData struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Attributes any    `json:"attributes"`
}

func newSetUpJSONAPI(setup any) setUpJSONAPI {
	return setUpJSONAPI{
		Data: setUpData{
			Type:       "setupData",
			ID:         "default",
			Attributes: setup,
		},
	}
}
