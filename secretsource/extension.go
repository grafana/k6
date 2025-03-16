package secretsource

import (
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib/fsext"
)

// Constructor returns an instance of a secret source extension module.
// This should return an instance of [Source] given the parameters.
// The Secret Source should not log its secrets and any returned secret will be cached and redacted
// by the [Manager]. No additional work needs to be done by the Secret source apart from retrieving
// the secret.
type Constructor func(Params) (Source, error)

// Source is the interface a secret source needs to implement
type Source interface {
	// Human readable description to be printed on the cli
	Description() string
	// Get retrives the value for a given key and returns it.
	// Logging the value before it is returned is going to lead to it being leaked.
	// The error might lead to an exception visible to users.
	Get(key string) (value string, err error)
}

// Params contains all possible constructor parameters an output may need.
type Params struct {
	ConfigArgument string // the string on the cli

	Logger      logrus.FieldLogger
	Environment map[string]string
	FS          fsext.Fs
	Usage       *usage.Usage
}

// RegisterExtension registers the given secret source extension constructor.
// This function panics if a module with the same name is already registered.
func RegisterExtension(name string, c Constructor) {
	ext.Register(name, ext.SecretSourceExtension, c)
}
