package lib

import (
	"gopkg.in/guregu/null.v3"
)

type Status struct {
	Running     null.Bool `json:"running"`
	ActiveVUs   null.Int  `json:"active-vus"`
	InactiveVUs null.Int  `json:"inactive-vus"`
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
