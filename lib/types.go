package lib

type Status struct {
	Running     bool  `json:"running"`
	ActiveVUs   int64 `json:"active-vus"`
	InactiveVUs int64 `json:"inactive-vus"`
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
