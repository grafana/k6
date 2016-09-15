package lib

import (
	"gopkg.in/guregu/null.v3"
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
	VUs      int           `json:"vus"`
	VUsMax   int           `json:"vus-max"`
	Duration time.Duration `json:"duration"`

	Ext map[string]interface{} `json:"ext"`
}

func (o Options) GetName() string {
	return "options"
}

func (o Options) GetID() string {
	return "default"
}
