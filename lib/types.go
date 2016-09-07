package lib

type Status struct {
	Running     bool  `json:"running" yaml:"Running"`
	ActiveVUs   int64 `json:"active-vus" yaml:"ActiveVUs"`
	InactiveVUs int64 `json:"inactive-vus" yaml:"InactiveVUs"`
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
	ID      string `json:"-"`
	Version string `json:"version"`
}

func (i Info) GetName() string {
	return "info"
}

func (i Info) GetID() string {
	return "default"
}
