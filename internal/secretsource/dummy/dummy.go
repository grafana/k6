// Package dummy implements a secret source that returns random values for any
// secret that is requested.
//
// The sole purpose of this package is to allow subcommands like inspect and
// archive to operate on scripts that request secrets at the top level context,
// for example:
//
//	import secrets from 'k6/secrets';
//
//	const secretValue = await secrets.get('secret_name');
//
//	export default function() { }
//
// Since inspect and archive need the actual value of the top-level options
// constant, it needs to partially evaluate the input script. Since there's no
// way to decide whether a particular portion of the program will run when
// trying to evaluate the options constant (halting problem), we have to assume
// they will be evaluated. In the name of UX, other secret sources will return
// an error when a non-existent secret is requested, meaning you cannot use
// something like `mock` to supply the need for a secret source.
//
// Considering all of that, this secret source returns a random value for any
// and all secrets requested. Why random data? Because of the way JavaScript
// works, it's possible to have functions execute during initialization. Since
// those functions can have arbitrary logic in them (like validating that the
// string is not empty despite having received no error), we have to return
// *something*. Since this is a secret, the user should have no way of
// validating the value, so returning a random string should cover the majority
// of normal use cases. This will still fail for example in case the secrets
// are used to build the options value, which is completely possible. We will
// treat that as a known limitation of inspect and archive. If you are doing
// that, please don't.
package dummy

import (
	"crypto/rand"
	"sync"

	"go.k6.io/k6/v2/secretsource"
)

// Name is the external name of the secrets source, so that it can be
// referenced by other parts of the code.
const Name = "dummy"

// Registration must happen exactly once. We want to avoid the init() route
// because we don't want to make this secret source generally available.
//
// Declare a global variable that makes sure the registration happens at most
// one time, at export the Register() function that allows the rest of the code
// to trigger it.
var register sync.Once //nolint:gochecknoglobals // See above.

// Register adds "dummy" to the list of known secret sources on demand.
func Register() {
	register.Do(func() {
		secretsource.RegisterExtension(Name, newSecretSourceFromParams)
	})
}

func newSecretSourceFromParams(_ secretsource.Params) (secretsource.Source, error) {
	return &secretSource{}, nil
}

type secretSource struct{}

// secretSource implements the secretsource.Source interface.
var _ secretsource.Source = &secretSource{}

func (secretSource) Description() string {
	return "this is a dummy secret source"
}

func (secretSource) Get(_ string) (string, error) {
	return generateRandomString(16)
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateRandomString(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	for i := range buf {
		buf[i] = charset[int(buf[i])%len(charset)]
	}

	return string(buf), nil
}
