package secretsource

// SecretSource is the interface a secret source needs to implement
type SecretSource interface {
	Name() string
	Description() string
	Get(key string) (value string, err error)
}
