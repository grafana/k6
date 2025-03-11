package secretsource

// Source is the interface a secret source needs to implement
type Source interface {
	// Human readable description to be printed on the cli
	Description() string
	// Get retrives the value for a given key and returns it.
	// Logging the value before it is returned is going to lead to it being leaked.
	// The error might lead to an exception visible to users.
	Get(key string) (value string, err error)
}
