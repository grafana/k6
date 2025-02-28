package secretsource

// Source is the interface a secret source needs to implement
type Source interface {
	// A name to be used when k6 has multiple sources
	Name() string
	// Human readable description to be printed on the cli
	Description() string
	Get(key string) (value string, err error)
}
