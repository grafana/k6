package lib

import (
	"gopkg.in/guregu/null.v3"
	"sync"
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
	Parent *Group
	Name   string
	Tests  map[string]*Test

	TestMutex sync.Mutex
}

type Test struct {
	Group *Group
	Name  string

	Passes int64
	Fails  int64
}
