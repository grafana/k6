package statsd

// ExtraConfig contains extra statsd config
type ExtraConfig struct {
	Namespace    string
	TagWhitelist string
}

// Config represents the statsd config with possible extra fields
type Config interface {
	Address() string
	Port() string
	BufferSize() int
	Extra() *ExtraConfig
}
