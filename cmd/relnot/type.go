package main

//go:generate enumer -type=PullType -text -transform=kebab -output type_gen.go

// PullType defines the available type of pull requests.
type PullType int

// The available pull requests types.
const (
	Undefined PullType = iota
	EpicFeature
	Feature
	EpicBreaking
	Breaking
	UX
	Internal
	Bug
)
