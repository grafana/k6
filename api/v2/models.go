package v2

import (
	"gopkg.in/guregu/null.v3"
)

type Status struct {
	Running null.Bool `json:"running"`
	Tainted null.Bool `json:"tainted"`
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
