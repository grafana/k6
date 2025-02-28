package secretsource

import (
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib/fsext"
)

// Constructor returns an instance of an output extension module.
type Constructor func(Params) (Source, error)

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
