package lib

type Status struct {
	ID string `jsonapi:"primary,status"`

	Running     bool  `jsonapi:"attr,running"`
	ActiveVUs   int64 `jsonapi:"attr,active-vus"`
	InactiveVUs int64 `jsonapi:"attr,inactive-vus"`
}

type Info struct {
	ID      string `jsonapi:"primary,info"`
	Version string `jsonapi:"attr,version"`
}
